package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	idp "github.com/your-org/identity-server/internal/identity"
	"go.uber.org/zap"
)

func main() {
	// Logger
	log, _ := zap.NewDevelopment()
	defer log.Sync() //nolint:errcheck

	dsn := envOrDefault("IDP_DATABASE_DSN", "postgres://acs_user:secret@localhost:5432/acs_db?sslmode=disable")
	port := envOrDefault("IDP_PORT", "8444")

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
	store := idp.NewStore(db)
	ctx := context.Background()
	if err := idp.Seed(ctx, store, log); err != nil {
		log.Fatal("seed failed", zap.Error(err))
	}

	// Load RSA keys and create JWT service
	settings, err := store.GetAllSettings(ctx)
	if err != nil {
		log.Fatal("load settings failed", zap.Error(err))
	}
	jwtSvc, err := idp.NewJWTService(settings["rsa_private_key"], settings["rsa_public_key"])
	if err != nil {
		log.Fatal("JWT service init failed", zap.Error(err))
	}

	// Handler + router
	handler := idp.NewHandler(store, jwtSvc, log)
	router := idp.NewRouter(handler, log)

	// HTTP server
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info("Identity Server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("Identity Server stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
