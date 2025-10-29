package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/logger"
	"runvoy/internal/server"
)

func main() {
	cfg := config.MustLoadEnv()
	log := logger.Initialize(constants.Development, cfg.LogLevel)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()
	svc, err := app.Initialize(ctx, constants.AWS, cfg, log)
	if err != nil {
		log.Error("Failed to initialize service", "error", err)
		os.Exit(1)
	}

	router := server.NewRouter(svc)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Starting local server",
			"port", cfg.Port,
			"version", *constants.GetVersion(),
			"log_level", cfg.LogLevel,
		)
		log.Info("Health check available",
			"url", fmt.Sprintf("http://localhost:%s/api/v1/health", cfg.Port),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error", "error", err)
		os.Exit(1)
	}

	log.Info("Server stopped")
}
