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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"runvoy/internal/app"
	dynamorepo "runvoy/internal/database/dynamodb"
	"runvoy/internal/server"
)

func main() {
	// Optional: Initialize DynamoDB if credentials are available
	var userRepo *dynamorepo.UserRepository
	apiKeysTableName := os.Getenv("API_KEYS_TABLE")

	if apiKeysTableName != "" {
		// Try to load AWS configuration (will fail gracefully if not configured)
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			log.Printf("WARNING: Could not load AWS config: %v", err)
			log.Println("→ Running without DynamoDB (user operations disabled)")
		} else {
			dynamoClient := dynamodb.NewFromConfig(cfg)
			userRepo = dynamorepo.NewUserRepository(dynamoClient, apiKeysTableName)
			log.Println("→ DynamoDB user repository initialized")
		}
	} else {
		log.Println("→ API_KEYS_TABLE not set, running without user operations")
	}

	// Create service and router
	svc := app.NewService(userRepo)
	router := server.NewRouter(svc)

	// Create HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("→ Starting local server on :%s", port)
		log.Printf("→ Health check: http://localhost:%s/api/v1/health", port)
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
