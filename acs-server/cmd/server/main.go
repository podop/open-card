package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pressly/goose/v3"
	acshttp "github.com/your-org/acs-server/internal/adapter/http"
	"github.com/your-org/acs-server/internal/adapter/http/handler"
	"github.com/your-org/acs-server/internal/adapter/repository/postgres"
	"github.com/your-org/acs-server/internal/domain/entity"
	"github.com/your-org/acs-server/internal/domain/service"
	"github.com/your-org/acs-server/internal/infrastructure/config"
	"github.com/your-org/acs-server/internal/infrastructure/crypto"
	"github.com/your-org/acs-server/internal/infrastructure/database"
	"github.com/your-org/acs-server/internal/infrastructure/dsclient"
	"github.com/your-org/acs-server/internal/infrastructure/logger"
	"github.com/your-org/acs-server/internal/usecase"
	"go.uber.org/zap"
)

func main() {
	cfgFile := flag.String("config", "", "path to config file (default: ./config.yaml)")
	flag.Parse()

	// ── Config ────────────────────────────────────────────────────
	cfg, err := config.Load(*cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// ── Logger ────────────────────────────────────────────────────
	log, err := logger.New(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync() //nolint:errcheck

	// ── Database ──────────────────────────────────────────────────
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := goose.SetDialect("pgx"); err != nil {
		log.Fatal("goose dialect error", zap.Error(err))
	}
	if err := goose.Up(db.DB, "migrations"); err != nil {
		log.Fatal("migration failed", zap.Error(err))
	}
	log.Info("migrations applied")

	// ── Repositories (infrastructure adapters) ────────────────────
	txRepo        := postgres.NewTransactionRepo(db)
	rangeRepo     := postgres.NewCardRangeRepo(db)
	sessionRepo   := postgres.NewSessionRepo(db)
	auditRepo     := postgres.NewAuditRepo(db)
	msgRepo       := postgres.NewMessageRepo(db)
	auditLogRepo  := postgres.NewAuditLogRepo(db)

	// ── Crypto services ───────────────────────────────────────────
	networkKeyHexes := make(map[entity.Network]string, len(cfg.Networks))
	for name, nc := range cfg.Networks {
		if nc.MasterKeyHex != "" {
			networkKeyHexes[entity.Network(name)] = nc.MasterKeyHex
		}
	}
	authValSvc, err := crypto.NewHMACAuthValueService(cfg.Crypto.MasterKeyHex, networkKeyHexes)
	if err != nil {
		log.Fatal("auth value service init failed", zap.Error(err))
	}

	// ── Domain services ───────────────────────────────────────────
	riskEngine := service.NewDefaultRiskEngine(log)
	otpSvc     := service.NewOTPServiceWithConfig(cfg.OTP.Source, cfg.OTP.SecretsManagerURL, cfg.OTP.HsmEndpointId)
	pinSvc     := service.NewPINService()

	// ── Network policies (shared mutable store) ─────────────────
	initialPolicies := make(map[entity.Network]usecase.NetworkPolicy, len(cfg.Networks))
	for name, nc := range cfg.Networks {
		initialPolicies[entity.Network(name)] = usecase.NetworkPolicy{
			AllowFrictionless: nc.AllowFrictionless,
		}
	}
	policyStore := usecase.NewInMemoryPolicyStore(initialPolicies)

	// ── Network repos ──────────────────────────────────────────
	networkRuleRepo := postgres.NewNetworkRuleRepo(db)
	networkRepo     := postgres.NewNetworkRepo(db)

	// ── Channel resolvers (simulated) ───────────────────────────
	otpResolver := service.NewSimulatedOTPResolver(true)
	oobResolver := service.NewSimulatedOOBResolver(true)

	// ── MFA provider repo ────────────────────────────────────────
	mfaProviderRepo := postgres.NewMFAProviderRepo(db)

	// ── Use cases ─────────────────────────────────────────────────
	authUC := usecase.NewAuthenticateUseCase(
		txRepo, rangeRepo, sessionRepo,
		riskEngine, otpSvc, authValSvc,
		auditRepo,
		cfg.ACS.BaseURL, policyStore, networkRuleRepo, networkRepo,
		otpResolver, oobResolver, mfaProviderRepo, log,
	)
	methodUC    := usecase.NewThreeDSMethodUseCase(txRepo, sessionRepo, log)
	challengeUC := usecase.NewChallengeUseCase(txRepo, sessionRepo, otpSvc, pinSvc, authValSvc, auditRepo, mfaProviderRepo, log)


	// ── SDK encryption (app channel) ──────────────────────────────
	var sdkDecryptor *crypto.SDKEncDataDecryptor
	if cfg.ACS.SDKPrivateKeyFile != "" {
		pemBytes, readErr := os.ReadFile(cfg.ACS.SDKPrivateKeyFile)
		if readErr != nil {
			log.Fatal("read SDK private key file", zap.String("path", cfg.ACS.SDKPrivateKeyFile), zap.Error(readErr))
		}
		sdkDecryptor, err = crypto.NewSDKEncDataDecryptorFromPEM(pemBytes)
		if err != nil {
			log.Fatal("init SDK enc data decryptor", zap.Error(err))
		}
		log.Info("SDK enc data decryptor initialised", zap.String("key_file", cfg.ACS.SDKPrivateKeyFile))
	} else {
		log.Warn("ACS sdk_private_key_file not set — app-channel sdkEncData decryption disabled")
	}

	// ── Scenario repo ────────────────────────────────────────────
	scenarioRepo := postgres.NewScenarioRepo(db)

	// ── DS client (sends RReq to Directory Server) ──────────────
	dsC := dsclient.New(cfg.ACS.DSURL, msgRepo, log)
	if cfg.ACS.DSURL != "" {
		log.Info("DS client configured", zap.String("ds_url", cfg.ACS.DSURL))
	} else {
		log.Warn("DS URL not configured — RReq/RRes will be saved locally only")
	}

	// ── HTTP handlers ─────────────────────────────────────────────
	authHandler      := handler.NewAuthHandler(authUC, cfg.ACS.ReferenceNumber, sdkDecryptor, msgRepo, dsC, log)
	methodHandler    := handler.NewMethodHandler(methodUC, log)
	challengeHandler := handler.NewChallengeHandler(challengeUC, msgRepo, dsC, log)
	apiHandler       := handler.NewApiHandler(txRepo, auditLogRepo, rangeRepo, msgRepo, networkRuleRepo, networkRepo, mfaProviderRepo, cfg, policyStore, otpResolver, oobResolver, log)
	oreqHandler      := handler.NewOReqHandler(cfg, msgRepo, log)

	// Wire up per-network client certificate provider for DS mutual TLS.
	dsC.SetClientCertProvider(func(network string) (*tls.Certificate, error) {
		ci := apiHandler.GetNetworkCert(network, "client")
		if ci == nil || ci.PEMData == "" || ci.KeyPEM == "" {
			return nil, nil
		}
		pair, err := tls.X509KeyPair([]byte(ci.PEMData), []byte(ci.KeyPEM))
		if err != nil {
			log.Warn("failed to load client cert pair", zap.String("network", network), zap.Error(err))
			return nil, nil
		}
		return &pair, nil
	})
	// Internal loopback URL for scenario runner to call ACS endpoints
	acsLoopback := fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
	scenarioHandler := handler.NewScenarioHandler(scenarioRepo, acsLoopback, otpResolver, oobResolver, log)

	// ── Router ────────────────────────────────────────────────────
	if cfg.Identity.URL != "" {
		log.Info("identity server configured", zap.String("url", cfg.Identity.URL))
	} else {
		log.Warn("identity URL not configured — API auth middleware disabled (dev only)")
	}
	router := acshttp.NewRouter(authHandler, methodHandler, challengeHandler, apiHandler, scenarioHandler, oreqHandler, cfg.Identity.URL, log)

	// ── HTTP server ───────────────────────────────────────────────
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Start server in background
	go func() {
		log.Info("ACS server starting", zap.String("addr", addr))
		var serveErr error
		if cfg.Server.TLSCertFile != "" && cfg.Server.TLSKeyFile != "" {
			serveErr = srv.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile)
		} else {
			log.Warn("TLS not configured — running in plaintext mode (development only)")
			serveErr = srv.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(serveErr))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("server stopped")
}
