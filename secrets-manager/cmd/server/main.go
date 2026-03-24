package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/your-org/secrets-manager/internal/hsm"
	"github.com/your-org/secrets-manager/internal/secrets"
	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewDevelopment()
	defer log.Sync() //nolint:errcheck

	dsn := envOrDefault("SM_DATABASE_DSN", "postgres://acs_user:secret@localhost:5432/acs_db?sslmode=disable")
	port := envOrDefault("SM_PORT", "8445")
	masterKeyHex := envOrDefault("SM_MASTER_KEY_HEX", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	identityURL := envOrDefault("SM_IDENTITY_URL", "")

	// Database
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		log.Fatal("database connection failed", zap.Error(err))
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatal("database ping failed", zap.Error(err))
	}
	log.Info("connected to database")

	// Migrations
	if err := goose.SetDialect("pgx"); err != nil {
		log.Fatal("goose dialect error", zap.Error(err))
	}
	if err := goose.Up(db.DB, "migrations"); err != nil {
		log.Fatal("migration failed", zap.Error(err))
	}
	log.Info("migrations applied")

	// Store + seed
	store := secrets.NewStore(db)
	ctx := context.Background()
	if err := secrets.Seed(ctx, store, log); err != nil {
		log.Fatal("seed failed", zap.Error(err))
	}

	// Handler + router
	handler := secrets.NewHandler(store, log, masterKeyHex)
	router := secrets.NewRouter(handler, identityURL, log)

	// HSM Gateway
	hsmStore := hsm.NewStore(db)
	gateway := hsm.NewGatewayManager(hsmStore, log)
	gateway.Start()
	hsmHandler := hsm.NewHandler(hsmStore, gateway, log)
	hsm.RegisterRoutes(router, hsmHandler, identityURL, log)

	// Wire cross-store linkers for Provider ↔ Endpoint coupling
	handler.SetHsmLinker(&hsmEndpointLinker{hsmStore: hsmStore, gateway: gateway})
	hsmHandler.SetSecretsLinker(&secretsProviderLinker{store: store})

	// Rotation engine
	engine := secrets.NewRotationEngine(store, handler, log)
	engine.Start()

	// HTTP server
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info("Secrets Manager starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	gateway.Stop()
	engine.Stop()
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("Secrets Manager stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── Cross-store linkers ─────────────────────────────────────────

// hsmEndpointLinker implements secrets.HsmEndpointLinker
type hsmEndpointLinker struct {
	hsmStore *hsm.Store
	gateway  *hsm.GatewayManager
}

func (l *hsmEndpointLinker) CreateLinkedEndpoint(ctx context.Context, name string, config json.RawMessage, createdBy int) (int, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal(config, &cfg); err != nil {
		cfg = map[string]interface{}{}
	}

	ep := &hsm.HSMEndpoint{
		Name:               name,
		Model:              strVal(cfg, "model"),
		Address:            strVal(cfg, "address"),
		Port:               intVal(cfg, "port", 1500),
		Protocol:           strVal(cfg, "protocol"),
		HeaderLength:       intVal(cfg, "headerLength", 4),
		FirmwareVersion:    strVal(cfg, "firmwareVersion"),
		HostCommandTimeout: intVal(cfg, "hostCommandTimeout", 15),
		ConnectionTimeout:  intVal(cfg, "connectionTimeout", 10),
		MaxConnections:     intVal(cfg, "maxConnections", 8),
		LMKIdentifier:      intVal(cfg, "lmkIdentifier", 0),
		Config:             json.RawMessage(`{}`),
		Weight:             100,
		Capabilities:       hsm.StringArray(hsm.GetDefaultCapabilities(strVal(cfg, "model"))),
		Enabled:            true,
		CreatedBy:          createdBy,
	}
	if ep.Protocol == "" {
		ep.Protocol = "TCP"
	}

	id, err := l.hsmStore.CreateEndpoint(ctx, ep)
	if err != nil {
		return 0, err
	}
	created, _ := l.hsmStore.GetEndpoint(ctx, id)
	if created != nil && created.Enabled {
		l.gateway.AddEndpoint(*created)
	}
	return id, nil
}

func (l *hsmEndpointLinker) UpdateLinkedEndpoint(ctx context.Context, endpointID int, name string, config json.RawMessage) error {
	existing, err := l.hsmStore.GetEndpoint(ctx, endpointID)
	if err != nil {
		return err
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(config, &cfg); err == nil {
		if v := strVal(cfg, "address"); v != "" {
			existing.Address = v
		}
		if v := intVal(cfg, "port", 0); v > 0 {
			existing.Port = v
		}
		if v := strVal(cfg, "model"); v != "" {
			existing.Model = v
		}
	}
	existing.Name = name
	return l.hsmStore.UpdateEndpoint(ctx, endpointID, existing)
}

func (l *hsmEndpointLinker) DeleteLinkedEndpoint(ctx context.Context, endpointID int) error {
	l.gateway.RemoveEndpoint(endpointID)
	return l.hsmStore.DeleteEndpoint(ctx, endpointID)
}

// secretsProviderLinker implements hsm.SecretsProviderLinker
type secretsProviderLinker struct {
	store *secrets.Store
}

func (l *secretsProviderLinker) CreateLinkedProvider(ctx context.Context, name string, config json.RawMessage, createdBy int) (int, error) {
	provider := &secrets.SecurityProvider{
		Name:         name,
		ProviderType: "hsm",
		Config:       config,
		Active:       true,
		CreatedBy:    createdBy,
	}
	return l.store.CreateProvider(ctx, provider)
}

func (l *secretsProviderLinker) UpdateLinkedProvider(ctx context.Context, providerID int, name string, config json.RawMessage) error {
	return l.store.UpdateProvider(ctx, providerID, name, config, true)
}

func (l *secretsProviderLinker) DeleteLinkedProvider(ctx context.Context, providerID int) error {
	count, _ := l.store.CountKeysByProvider(ctx, providerID)
	if count > 0 {
		return fmt.Errorf("provider has associated keys")
	}
	return l.store.DeleteProvider(ctx, providerID)
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intVal(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return def
}
