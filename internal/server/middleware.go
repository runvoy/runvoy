package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	loggerPkg "runvoy/internal/logger"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

const (
	loggerContextKey      contextKey = "logger"
	lastUsedUpdateTimeout            = 5 * time.Second
)

// generateRequestID generates a random UUID v4-like request ID
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	
	return hex.EncodeToString(b[0:4]) + "-" +
		hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" +
		hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}

// requestIDMiddleware extracts the request ID from the context (if present) or generates a random one.
// Priority: 1) Existing request ID in context, 2) Lambda request ID, 3) Generated random UUID.
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

		ctx := context.WithValue(req.Context(), loggerPkg.RequestIDContextKey(), requestID)
		log := r.svc.Logger.With(string(loggerPkg.RequestIDContextKey()), requestID)
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
				logger.Warn("request timeout exceeded", "request", map[string]interface{}{
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

// authenticateRequestMiddleware authenticates requests
// Adds authenticated user to request context
// Updates user's last_used timestamp asynchronously after successful authentication
func (r *Router) authenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		logger := r.GetLoggerFromContext(req.Context())
		apiKey := req.Header.Get(constants.ApiKeyHeader)
		logger.Debug("authenticating request")

		if apiKey == "" {
			writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "API key is required")
			return
		}

		user, err := r.svc.AuthenticateUser(req.Context(), apiKey)
		if err != nil {
			statusCode := apperrors.GetStatusCode(err)
			errorCode := apperrors.GetErrorCode(err)
			errorMsg := apperrors.GetErrorMessage(err)

			if statusCode < 400 || statusCode >= 600 {
				statusCode = http.StatusUnauthorized
			}

			var messagePrefix string
			if statusCode >= 500 {
				messagePrefix = "Server error"
			} else {
				messagePrefix = "Unauthorized"
			}

			writeErrorResponseWithCode(w, statusCode, errorCode, messagePrefix, errorMsg)
			return
		}

		logger.Info("user authenticated successfully", "email", user.Email)

		// Update last_used timestamp asynchronously, but wait for completion
		// before returning to ensure it completes in Lambda environments.
		// Copy context values (like requestID) to a new background context since the
		// request context will be canceled when the request completes.
		requestID := loggerPkg.GetRequestID(req.Context())
		var wg sync.WaitGroup
		wg.Add(1)
		go func(email string, reqID string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), lastUsedUpdateTimeout)
			defer cancel()
			ctx = context.WithValue(ctx, loggerPkg.RequestIDContextKey(), reqID)

			logger.Debug("updating user's last_used timestamp (async)", "user", map[string]any{
				"email":              email,
				"previous_last_used": user.LastUsed.Format(time.RFC3339),
			})

			newLastUsed, err := r.svc.UpdateUserLastUsed(ctx, email)
			if err != nil {
				logger.Error("failed to update user's last_used timestamp", "error", map[string]any{
					"error": err,
					"user": map[string]any{
						"email": email,
					},
				})
			} else {
				logger.Debug("user's last_used timestamp updated successfully", "user", map[string]any{
					"email":              email,
					"last_used":          newLastUsed.Format(time.RFC3339),
					"previous_last_used": user.LastUsed.Format(time.RFC3339),
				})
			}
		}(user.Email, requestID)

		ctx := context.WithValue(req.Context(), userContextKey, user)
		next.ServeHTTP(w, req.WithContext(ctx))

		// Wait for the background update to complete before returning.
		// This ensures the update completes in Lambda before the handler returns.
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

		logger.Info("sent response to client", "response", map[string]interface{}{
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
