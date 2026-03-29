package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-org/hsm-emulator/internal/admin"
	"github.com/your-org/hsm-emulator/internal/config"
	"github.com/your-org/hsm-emulator/internal/lmk"
	"github.com/your-org/hsm-emulator/internal/processor"
	"github.com/your-org/hsm-emulator/internal/server"
)

func main() {
	cfgFile := flag.String("config", "", "path to config JSON file")
	lmkFile := flag.String("lmk", "", "path to LMK key file (binary)")
	testLMK := flag.Bool("test-lmk", false, "load well-known test LMK set (not for production)")
	port := flag.Int("port", 0, "override host command TCP port")
	adminPort := flag.Int("admin-port", 8086, "HTTP admin API port")
	headerLen := flag.Int("header", 0, "override header length (2 or 4)")
	autoAuth := flag.Bool("auto-authorize", false, "automatically authorize HSM on startup")
	flag.Parse()

	// ── Load or create config ──────────────────────────
	var cfg *config.Config
	if *cfgFile != "" {
		var err error
		cfg, err = config.Load(*cfgFile)
		if err != nil {
			// If file doesn't exist, create with defaults
			cfg = config.DefaultConfig()
			log.Printf("[HSM] Config file not found, using defaults")
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Apply CLI overrides
	if *port > 0 {
		cfg.HostPort = *port
	}
	if *headerLen > 0 {
		cfg.HeaderLength = *headerLen
	}

	// ── Load LMK keys ──────────────────────────────────
	lmkStore := lmk.New()

	lmkLoaded := false
	if *lmkFile != "" {
		if err := lmkStore.LoadFromFile(*lmkFile); err == nil {
			cfg.SetLMKLoaded(true)
			lmkLoaded = true
			log.Println("[HSM] LMK loaded from file")
		} else {
			log.Printf("[HSM] LMK file not found (%s), will try other sources", *lmkFile)
		}
	}
	if !lmkLoaded && *testLMK {
		lmkStore.LoadTestLMK()
		cfg.SetLMKLoaded(true)
		lmkLoaded = true
		log.Println("[HSM] Test LMK set loaded (deterministic — NOT for production)")
		// Persist to file for next restart
		if *lmkFile != "" {
			if err := lmkStore.SaveToFile(*lmkFile); err == nil {
				log.Printf("[HSM] LMK saved to %s", *lmkFile)
			}
		}
	}
	if !lmkLoaded {
		log.Println("[HSM] No LMK loaded. Use console to load LMK or restart with --test-lmk / --lmk flags.")
	}

	// Print LMK check values
	if cfg.IsLMKLoaded() {
		kcvs, err := lmkStore.CheckValues()
		if err == nil {
			log.Println("[HSM] LMK Check Values:")
			for i, kcv := range kcvs {
				log.Printf("  LMK %02d-%02d: %s", i*2, i*2+1, string(kcv[:]))
			}
		}
	}

	// ── Auto-authorize ─────────────────────────────────
	if *autoAuth && cfg.IsLMKLoaded() {
		cfg.SetAuthorized(true)
		log.Println("[HSM] HSM auto-authorized")
	}

	// Persist config
	if *cfgFile != "" {
		cfg.Save(*cfgFile)
	}

	// ── Print startup banner ───────────────────────────
	printBanner(cfg, *adminPort)

	// ── Start HTTP admin API ──────────────────────────
	adminHandler := admin.New(cfg, lmkStore, *cfgFile, *lmkFile)
	adminMux := http.NewServeMux()
	adminHandler.RegisterRoutes(adminMux)
	adminSrv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", *adminPort),
		Handler: adminMux,
	}
	go func() {
		log.Printf("[HSM] Admin HTTP API listening on :%d", *adminPort)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HSM] Admin HTTP error: %v", err)
		}
	}()

	// ── Start host command server ──────────────────────
	proc := processor.New(cfg, lmkStore)
	srv := server.New(cfg, proc)

	if err := srv.Start(); err != nil {
		log.Fatalf("Start server: %v", err)
	}

	// ── Wait for shutdown signal ───────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[HSM] Received %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	adminSrv.Shutdown(ctx)
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[HSM] Shutdown error: %v", err)
	}

	conns, cmds, uptime := srv.Stats()
	log.Printf("[HSM] Shutdown complete. Processed %d commands in %v (final active connections: %d)", cmds, uptime.Round(time.Second), conns)
}

func printBanner(cfg *config.Config, adminPort int) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Thales PayShield 10K — HSM Emulator               ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Model:           %-40s ║\n", cfg.Model)
	fmt.Printf("║  Serial:          %-40s ║\n", cfg.SerialNumber)
	fmt.Printf("║  Firmware:        %-40s ║\n", cfg.FirmwareVersion)
	fmt.Printf("║  Host Port:       %-40d ║\n", cfg.HostPort)
	fmt.Printf("║  Admin Port:      %-40d ║\n", adminPort)
	fmt.Printf("║  Header Length:   %-40d ║\n", cfg.HeaderLength)
	fmt.Printf("║  Max Connections: %-40d ║\n", cfg.MaxConnections)

	lmkStatus := "NOT LOADED"
	if cfg.IsLMKLoaded() {
		lmkStatus = "LOADED"
	}
	authStatus := "NOT AUTHORIZED"
	if cfg.IsAuthorized() {
		authStatus = "AUTHORIZED"
	}
	fmt.Printf("║  LMK Status:      %-40s ║\n", lmkStatus)
	fmt.Printf("║  Auth Status:     %-40s ║\n", authStatus)
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}
