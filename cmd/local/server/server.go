// Package server provides the async event processor HTTP server setup.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"runvoy/internal/app/events"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates a chi router for the async event processor.
func NewRouter(processor events.Processor, log *slog.Logger) *chi.Mux {
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
	r.Post("/process", func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			_ = req.Body.Close()
		}()

		body, readErr := io.ReadAll(req.Body)
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

		result, procErr := processor.Handle(req.Context(), &rawEvent)
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
