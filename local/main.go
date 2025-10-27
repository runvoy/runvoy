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

	"runvoy/internal/handlers"
	"runvoy/internal/services"
	"runvoy/local/mocks"
)

var (
	port = flag.Int("port", 8080, "Port to run the local server on")
)

func main() {
	flag.Parse()

	// Initialize mock services for local development
	storage := mocks.NewMockStorage()
	ecs := mocks.NewMockECSService()
	lock := mocks.NewMockLockService(storage)
	logService := mocks.NewMockLogService()
	auth := mocks.NewMockAuthService(storage)

	// Initialize execution service
	execution := services.NewExecutionService(storage, ecs, lock, logService)

	// Initialize handlers
	h := handlers.NewHandlers(auth, execution)

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/executions", h.ExecutionHandler)
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/status/", h.StatusHandler)

	// Create server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting local server on port %d", *port)
		log.Printf("Health check: http://localhost:%d/health", *port)
		log.Printf("API endpoint: http://localhost:%d/executions", *port)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}