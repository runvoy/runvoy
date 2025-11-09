package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	loggerPkg "runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	loggerContextKey      contextKey = "logger"
	lastUsedUpdateTimeout            = 5 * time.Second
)

// generateRequestID generates a random request ID using crypto/rand
func generateRequestID() string {
	b := make([]byte, constants.RequestIDByteSize)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// requestIDMiddleware extracts the request ID from the context (if present) or generates a random one.
// Priority: 1) Existing request ID in context, 2) Lambda request ID, 3) Generated random ID.
func (r *Router) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := loggerPkg.GetRequestID(req.Context())

		if requestID == "" {
			if lc, ok := lambdacontext.FromContext(req.Context()); ok && lc.AwsRequestID != "" {
				requestID = lc.AwsRequestID
			}
		}

		if requestID == "" {
			requestID = generateRequestID()
		}

		ctx := loggerPkg.WithRequestID(req.Context(), requestID)
		log := r.svc.Logger.With("requestID", requestID)
		ctx = context.WithValue(ctx, loggerContextKey, log)

		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// requestTimeoutMiddleware creates a context with timeout for each request.
// The timeout starts when the request is received, ensuring each request has
// a fair timeout regardless of connection reuse.
func (r *Router) requestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx, cancel := context.WithTimeout(req.Context(), timeout)
			defer cancel()

			req = req.WithContext(ctx)

			next.ServeHTTP(w, req)

			if ctx.Err() == context.DeadlineExceeded {
				logger := r.GetLoggerFromContext(req.Context())
				logger.Warn("request timeout exceeded", "request", map[string]any{
					"method":  req.Method,
					"path":    req.URL.Path,
					"timeout": timeout,
				})

				// Note: Response may have already been written by handler
				// The context cancellation will have already propagated to
				// any operations (like DynamoDB calls) that were using the context
			}
		})
	}
}

// corsMiddleware handles CORS headers for cross-origin requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// If no Origin header, allow all origins (fallback)
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// setContentTypeJSONMiddleware sets Content-Type to application/json for all responses
func setContentTypeJSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set(constants.ContentTypeHeader, "application/json")
		next.ServeHTTP(w, req)
	})
}

// handleAuthError handles authentication errors and writes appropriate responses.
func handleAuthError(w http.ResponseWriter, err error) {
	statusCode := apperrors.GetStatusCode(err)
	errorCode := apperrors.GetErrorCode(err)
	errorMsg := apperrors.GetErrorMessage(err)

	if statusCode < 400 || statusCode >= 600 {
		statusCode = http.StatusUnauthorized
	}

	messagePrefix := "Unauthorized"
	if statusCode >= constants.HTTPStatusServerError {
		messagePrefix = "Server error"
	}

	writeErrorResponseWithCode(w, statusCode, errorCode, messagePrefix, errorMsg)
}

// updateLastUsedAsync updates the user's last_used timestamp asynchronously.
func (r *Router) updateLastUsedAsync(user *api.User, requestID string, logger *slog.Logger) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go func(email string, reqID string) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), lastUsedUpdateTimeout)
		defer cancel()
		ctx = loggerPkg.WithRequestID(ctx, reqID)

		userLogData := map[string]any{
			"email": email,
		}
		if user.LastUsed != nil {
			userLogData["previous_last_used"] = user.LastUsed.Format(time.RFC3339)
		}
		logger.Debug("updating user's last_used timestamp (async)", "user", userLogData)

		newLastUsed, err := r.svc.UpdateUserLastUsed(ctx, email)
		if err != nil {
			logger.Error("failed to update user's last_used timestamp", "error", map[string]any{
				"error": err,
				"user": map[string]any{
					"email": email,
				},
			})
		} else {
			successLogData := map[string]any{
				"email":     email,
				"last_used": newLastUsed.Format(time.RFC3339),
			}
			if user.LastUsed != nil {
				successLogData["previous_last_used"] = user.LastUsed.Format(time.RFC3339)
			}
			logger.Debug("user's last_used timestamp updated successfully", "user", successLogData)
		}
	}(user.Email, requestID)
	return &wg
}

// authenticateRequestMiddleware authenticates requests
// Adds authenticated user to request context
// Updates user's last_used timestamp asynchronously after successful authentication
func (r *Router) authenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := r.GetLoggerFromContext(req.Context())
		apiKey := req.Header.Get(constants.APIKeyHeader)
		logger.Debug("authenticating request")

		if apiKey == "" {
			writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "API key is required")
			return
		}

		user, err := r.svc.AuthenticateUser(req.Context(), apiKey)
		if err != nil {
			handleAuthError(w, err)
			return
		}

		logger.Info("user authenticated successfully", "email", user.Email)

		requestID := loggerPkg.GetRequestID(req.Context())
		wg := r.updateLastUsedAsync(user, requestID, logger)

		ctx := context.WithValue(req.Context(), userContextKey, user)
		next.ServeHTTP(w, req.WithContext(ctx))

		wg.Wait()
	})
}

// requestLoggingMiddleware logs incoming requests and their responses
// Uses logger from context (includes request ID if available)
func (r *Router) requestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := r.GetLoggerFromContext(req.Context())
		start := time.Now()
		deadlineString := ""
		if deadline, ok := req.Context().Deadline(); ok {
			deadlineString = deadline.Format(time.RFC3339)
		}

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default status code
		}

		logger.Info("processing incoming client request", "request", map[string]string{
			"method":     req.Method,
			"path":       req.URL.Path,
			"remoteAddr": req.RemoteAddr,
			"deadline":   deadlineString,
		})

		next.ServeHTTP(wrapped, req)
		duration := time.Since(start)

		logger.Info("response sent to client", "response", map[string]any{
			"status":   wrapped.statusCode,
			"duration": duration.String(),
		})
	})
}

// GetLoggerFromContext extracts the logger from request context
// Returns the request-scoped logger (with request ID if available) or falls back to service logger
func (r *Router) GetLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok && logger != nil {
		return logger
	}

	return r.svc.Logger
}
