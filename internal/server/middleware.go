package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/auth"
	"runvoy/internal/auth/authorization"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"
	loggerPkg "runvoy/internal/logger"
)

const (
	loggerContextKey      contextKey = "logger"
	lastUsedUpdateTimeout            = 5 * time.Second
)

// requestIDMiddleware extracts the request ID from the context (if present) or generates a random one.
// Priority: 1) Existing request ID in context, 2) Provider-specific request ID (via registered
// extractors), 3) Generated random ID.
func (r *Router) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestID := loggerPkg.ExtractRequestIDFromContext(req.Context())

		if requestID == "" {
			requestID = auth.GenerateUUID()
		}

		ctx := loggerPkg.WithRequestID(req.Context(), requestID)
		log := r.svc.Logger.With(constants.RequestIDLogField, requestID)
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
				logger.Warn("request timeout exceeded", "method", req.Method, "path", req.URL.Path, "timeout", timeout)

				// Note: Response may have already been written by handler
				// The context cancellation will have already propagated to
				// any operations (like DynamoDB calls) that were using the context
			}
		})
	}
}

// normalizeOrigin removes trailing slashes from an origin URL for comparison
func normalizeOrigin(origin string) string {
	return strings.TrimSuffix(origin, "/")
}

// corsMiddleware handles CORS headers for cross-origin requests
// Normalizes allowed origins once at middleware creation time to avoid repeated string operations.
// Supports "*" as a wildcard to allow all origins.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	normalizedAllowedOrigins := make([]string, len(allowedOrigins))
	allowAllOrigins := false
	for i, origin := range allowedOrigins {
		normalized := normalizeOrigin(origin)
		if normalized == "*" {
			allowAllOrigins = true
		}
		normalizedAllowedOrigins[i] = normalized
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			if origin != "" {
				normalizedOrigin := normalizeOrigin(origin)
				allowed := allowAllOrigins || slices.Contains(normalizedAllowedOrigins, normalizedOrigin)
				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
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

// authorizeRequest checks if a user can perform an action on a resource.
// Returns true if allowed, false if denied.
// If user is not found in context, returns false (not authorized).
func (r *Router) authorizeRequest(req *http.Request, action authorization.Action) bool {
	ctx := req.Context()
	logger := r.GetLoggerFromContext(ctx)

	user, ok := r.getUserFromContext(req)
	if !ok {
		logger.Warn("authorization denied: user not found in context", "resource", req.URL.Path, "action", action)
		return false
	}

	enforcer := r.svc.GetEnforcer()
	resourceObject := req.URL.Path
	userEmail := user.Email

	allowed, err := enforcer.Enforce(userEmail, resourceObject, action)
	if err != nil {
		logger.Error("authorization check error",
			"error", err,
			"user", userEmail,
			"resource", resourceObject,
			"action", action)
		return false
	}

	if !allowed {
		logger.Warn("authorization denied", "user", userEmail, "resource", resourceObject, "action", action)
		return false
	}

	return true
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

		if user.LastUsed != nil {
			logger.Debug("updating user's last_used timestamp (async)",
				"email", email,
				"previous_last_used", user.LastUsed.Format(time.RFC3339))
		} else {
			logger.Debug("updating user's last_used timestamp (async)", "email", email)
		}

		newLastUsed, err := r.svc.UpdateUserLastUsed(ctx, email)
		if err != nil {
			logger.Error("failed to update user's last_used timestamp", "error", err, "email", email)
		} else {
			if user.LastUsed != nil {
				logger.Debug("user's last_used timestamp updated successfully",
					"email", email,
					"last_used", newLastUsed.Format(time.RFC3339),
					"previous_last_used", user.LastUsed.Format(time.RFC3339))
			} else {
				logger.Debug("user's last_used timestamp updated successfully",
					"email", email,
					"last_used", newLastUsed.Format(time.RFC3339))
			}
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

		logger.Info("processing incoming client request",
			"method", req.Method,
			"path", req.URL.Path,
			"remoteAddr", req.RemoteAddr,
			"deadline", deadlineString)

		next.ServeHTTP(wrapped, req)
		duration := time.Since(start)

		logger.Info("response sent to client", "status", wrapped.statusCode, "duration", duration.String())
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

// getActionFromRequest maps HTTP method and path to an authorization action.
// This is only called for authenticated routes, so no need to check for public routes.
func (r *Router) getActionFromRequest(method string) authorization.Action {
	switch method {
	case http.MethodGet:
		return authorization.ActionRead
	case http.MethodPost:
		return authorization.ActionCreate
	case http.MethodPut:
		return authorization.ActionUpdate
	case http.MethodDelete:
		return authorization.ActionDelete
	default:
		// Fallback for unexpected methods
		return authorization.ActionRead
	}
}

// authorizeRequestMiddleware checks authorization for authenticated routes.
// It should be applied after authenticateRequestMiddleware.
// The /run endpoint gets general create permission here; resource-level checks
// (resolved images/secrets) happen in the handler layer via ResolveImage and ValidateExecutionResourceAccess.
func (r *Router) authorizeRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		action := r.getActionFromRequest(req.Method)

		if !r.authorizeRequest(req, action) {
			// Generate a generic denial message based on action
			denialMsg := fmt.Sprintf("you do not have permission to %s this resource", action)
			writeErrorResponse(w, http.StatusForbidden, "Forbidden", denialMsg)
			return
		}

		next.ServeHTTP(w, req)
	})
}
