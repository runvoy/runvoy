// Package main implements the local async event processor server for runvoy.
// It runs the event processor service locally for testing and development.
// This allows testing of async Lambda events without deploying to AWS.
package main

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
	"runvoy/internal/logger"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func writeErrorResponse(w http.ResponseWriter, statusCode int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = fmt.Fprintf(w, `{"error":"%s","details":"%s"}`, message, details)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok","component":"async-processor"}`)
}

func handleProcessEvent(processor *events.Processor, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

func main() {
	cfg := config.MustLoadEventProcessor()
	log := logger.Initialize(constants.Development, cfg.GetLogLevel())
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)

	processor, err := events.NewProcessor(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("failed to initialize event processor", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", handleHealth)

	// Process raw Lambda event
	// Accepts a JSON payload and processes it through the event processor
	// Example: curl -X POST http://localhost:8081/process -d @event.json
	r.Post("/process", handleProcessEvent(processor, log))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  constants.ServerReadTimeout,
		WriteTimeout: constants.ServerWriteTimeout,
		IdleTimeout:  constants.ServerIdleTimeout,
	}

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
			log.Error("failed to start server", "error", serveErr)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down async processor server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), constants.ServerShutdownTimeout)

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		shutdownCancel()
		log.Error("server shutdown error", "error", shutdownErr)
		os.Exit(1)
	}
	shutdownCancel()

	log.Info("async processor server shutdown complete")
}
