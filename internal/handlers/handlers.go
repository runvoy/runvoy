package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/services"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	auth      services.AuthService
	execution services.ExecutionService
}

// NewHandlers creates a new handlers instance
func NewHandlers(auth services.AuthService, execution services.ExecutionService) *Handlers {
	return &Handlers{
		auth:      auth,
		execution: execution,
	}
}

// ExecutionHandler handles execution requests
func (h *Handlers) ExecutionHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get API key from header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		h.writeError(w, "Missing API key", "MISSING_API_KEY", http.StatusUnauthorized)
		return
	}

	// Validate API key
	user, err := h.auth.ValidateAPIKey(ctx, apiKey)
	if err != nil {
		h.writeError(w, "Invalid API key", "INVALID_API_KEY", http.StatusUnauthorized)
		return
	}

	// Parse request body
	var req api.ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", "INVALID_REQUEST", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Command == "" {
		h.writeError(w, "Command is required", "MISSING_COMMAND", http.StatusBadRequest)
		return
	}

	// Start execution
	resp, err := h.execution.StartExecution(ctx, &req, user)
	if err != nil {
		// Check if it's a lock conflict
		if err.Error() == "lock already held" {
			h.writeError(w, "Lock is already held by another execution", "LOCK_CONFLICT", http.StatusConflict)
			return
		}
		h.writeError(w, fmt.Sprintf("Failed to start execution: %v", err), "EXECUTION_FAILED", http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// StatusHandler handles status requests
func (h *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get execution ID from URL path
	executionID := r.URL.Path[len("/status/"):]
	if executionID == "" {
		h.writeError(w, "Execution ID is required", "MISSING_EXECUTION_ID", http.StatusBadRequest)
		return
	}

	// Get execution
	execution, err := h.execution.GetExecution(ctx, executionID)
	if err != nil {
		h.writeError(w, "Execution not found", "EXECUTION_NOT_FOUND", http.StatusNotFound)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(execution)
}

// HealthHandler handles health checks
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// writeError writes an error response
func (h *Handlers) writeError(w http.ResponseWriter, message, code string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(api.ErrorResponse{
		Error: message,
		Code:  code,
	})
}