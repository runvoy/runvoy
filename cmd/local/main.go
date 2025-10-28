package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/server"
)

func main() {
	cfg := config.MustLoadEnv()
	svc, err := app.Initialize(context.Background(), constants.AWS, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
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
		log.Printf("→ Starting local server on :%s (Ctrl+C to stop)", cfg.Port)
		log.Printf("→ Health check: http://localhost:%s/api/v1/health", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("→ Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		os.Exit(1)
	}

	log.Println("→ Server stopped")
}
