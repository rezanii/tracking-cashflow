// Package main starts the Financial Management API.
//
//	@title						Financial Management API
//	@version					1.0
//	@description				Personal finance tracking: transactions, categories, dashboard and cash flow reports.
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Send the access token as: Bearer {token}
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gorm.io/gorm"

	_ "github.com/rezanii/tracking-cashflow/apps/backend/docs"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/router"
)

func main() {
	// The container image carries no shell or curl, so the binary probes itself.
	healthcheck := flag.Bool("healthcheck", false, "probe the local /health endpoint and exit")
	flag.Parse()

	if *healthcheck {
		if err := probeHealth(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func probeHealth() error {
	port := strings.TrimSpace(os.Getenv("APP_PORT"))
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return fmt.Errorf("health probe failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health probe returned status %d", response.StatusCode)
	}
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	setupLogger(cfg)

	db, err := repository.NewDatabase(cfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer closeDatabase(db)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.AppPort),
		Handler:           router.New(cfg, db),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute, // report exports stream a generated file
		IdleTimeout:       90 * time.Second,
	}

	// Serve in the background so the main goroutine can wait for a shutdown signal.
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", cfg.AppPort, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("listen: %w", err)
	case signalReceived := <-shutdown:
		slog.Info("shutdown requested", "signal", signalReceived.String())

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			// Close is the last resort when in-flight requests refuse to finish in time.
			_ = server.Close()
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		slog.Info("server stopped cleanly")
		return nil
	}
}

func setupLogger(cfg config.Config) {
	level := slog.LevelDebug
	if cfg.IsProduction() {
		level = slog.LevelInfo
	}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	if !cfg.IsProduction() {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

// closeDatabase releases the pool on the way out so a restart does not leave connections
// lingering on the server.
func closeDatabase(db *gorm.DB) {
	pool, err := db.DB()
	if err != nil {
		slog.Warn("could not access connection pool on shutdown", "error", err)
		return
	}
	if err := pool.Close(); err != nil {
		slog.Warn("could not close connection pool", "error", err)
	}
}
