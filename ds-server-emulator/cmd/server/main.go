package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ds-server-emulator/internal/handler"
	"github.com/your-org/ds-server-emulator/internal/store"
	"go.uber.org/zap"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────
	logLevel := os.Getenv("DS_LOG_LEVEL")
	var log *zap.Logger
	var err error
	if logLevel == "debug" {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync() //nolint:errcheck

	// ── Port ─────────────────────────────────────────────────────
	port := os.Getenv("DS_PORT")
	if port == "" {
		port = "9443"
	}

	// ── ACS URL (for sending OReq) ──────────────────────────────
	acsURL := os.Getenv("ACS_URL")

	// ── Store & Handler ──────────────────────────────────────────
	s := store.New()
	h := handler.New(s, acsURL, log)

	// ── Gin router ───────────────────────────────────────────────
	if os.Getenv("DS_LOG_LEVEL") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(log))

	// Protocol endpoint — receives RReq from ACS
	r.POST("/ds/v2/rreq", h.HandleRReq)

	// Health
	r.GET("/health", h.HealthCheck)

	// API for monitoring
	api := r.Group("/api")
	{
		api.GET("/records", h.ListRecords)
		api.GET("/oreq-records", h.ListOReqRecords)
		api.POST("/send-oreq", h.SendOReqToACS)
		api.GET("/stats", h.GetStats)
		api.GET("/communication-log", h.ListCommunication)
		api.GET("/status", h.GetStatus)
	}

	// ── HTTP server ──────────────────────────────────────────────
	addr := "0.0.0.0:" + port
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info("DS emulator starting", zap.String("addr", addr))
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(serveErr))
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("DS emulator stopped")
}

func requestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("client", c.ClientIP()),
		)
	}
}