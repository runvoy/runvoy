// Package server provides the async event processor HTTP server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/events"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Run starts the async event processor HTTP server and blocks until shutdown.
func Run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	initCtx, cancel := context.WithTimeout(ctx, cfg.InitTimeout)
	processor, err := events.NewProcessor(initCtx, cfg, log)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to initialize event processor: %w", err)
	}

	router := NewRouter(processor, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  constants.ServerReadTimeout,
		WriteTimeout: constants.ServerWriteTimeout,
		IdleTimeout:  constants.ServerIdleTimeout,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("starting local async processor server",
			"port", cfg.Port,
			"version", *constants.GetVersion(),
			"log_level", cfg.LogLevel,
		)
		log.Debug("event processor available",
			"url", fmt.Sprintf("http://localhost:%s/process", cfg.Port),
		)

		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("failed to start server: %w", serveErr)
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case runErr := <-serverErrors:
		return runErr
	case <-quit:
		log.Info("shutting down async processor server...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constants.ServerShutdownTimeout)
	defer shutdownCancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return fmt.Errorf("server shutdown error: %w", shutdownErr)
	}

	log.Info("async processor server shutdown complete")
	return nil
}

// NewRouter creates a chi router for the async event processor.
func NewRouter(processor *events.Processor, log *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok","component":"async-processor"}`)
	})

	// Process raw Lambda event
	// Accepts a JSON payload and processes it through the event processor
	// Example: curl -X POST http://localhost:8081/process -d @event.json
	r.Post("/process", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			_ = r.Body.Close()
		}()

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "failed to read request body", readErr.Error())
			return
		}

		var rawEvent json.RawMessage
		unmarshalErr := json.Unmarshal(body, &rawEvent)
		if unmarshalErr != nil {
			writeErrorResponse(w, http.StatusBadRequest, "invalid JSON payload", unmarshalErr.Error())
			return
		}

		result, procErr := processor.Handle(r.Context(), &rawEvent)
		if procErr != nil {
			writeErrorResponse(w, http.StatusInternalServerError, "event processing failed", procErr.Error())
			return
		}

		// Handle different response types
		w.Header().Set("Content-Type", "application/json")
		if result != nil {
			w.WriteHeader(http.StatusOK)
			encodeErr := json.NewEncoder(w).Encode(result)
			if encodeErr != nil {
				log.Error("failed to encode response", "error", encodeErr)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"processed"}`)
		}
	})

	return r
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"error":"%s","details":"%s"}`, message, details)
}
