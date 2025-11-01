// Package server implements the HTTP server and handlers for runvoy.
// It provides REST API endpoints for user management and command execution.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"runvoy/internal/api"
	"runvoy/internal/constants"
	apperrors "runvoy/internal/errors"

	"github.com/go-chi/chi/v5"
)

// handleCreateUser handles POST /api/v1/users to create a new user with an API key
func (r *Router) handleCreateUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var createReq api.CreateUserRequest

	if err := json.NewDecoder(req.Body).Decode(&createReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())

		return
	}

	// Extract admin user from context
	user, ok := req.Context().Value(userContextKey).(*api.User)
	if !ok || user == nil {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	// Build base URL from request
	scheme := "https"
	if req.TLS == nil {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, req.Host)

	resp, err := r.svc.CreateUser(req.Context(), createReq, baseURL, user.Email)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to create user", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to create user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleRevokeUser handles POST /api/v1/users/revoke to revoke a user's API key
func (r *Router) handleRevokeUser(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())
	var revokeReq api.RevokeUserRequest

	if err := json.NewDecoder(req.Body).Decode(&revokeReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid request body", err.Error())

		return
	}

	if err := r.svc.RevokeUser(req.Context(), revokeReq.Email); err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to revoke user", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to revoke user", errorMsg)

		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.RevokeUserResponse{
		Message: "user API key revoked successfully",
		Email:   revokeReq.Email,
	})
}

// handleRunCommand handles POST /api/v1/run to execute a command in an ephemeral container
func (r *Router) handleRunCommand(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	user, ok := req.Context().Value(userContextKey).(*api.User)
	if !ok || user == nil {
		writeErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "user not found in context")
		return
	}

	var execReq api.ExecutionRequest
	if err := json.NewDecoder(req.Body).Decode(&execReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	resp, err := r.svc.RunCommand(req.Context(), user.Email, execReq)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to run command", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to run command", errorMsg)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionLogs handles GET /api/v1/executions/{executionID}/logs to fetch logs for an execution
func (r *Router) handleGetExecutionLogs(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	resp, err := r.svc.GetLogsByExecutionID(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to get execution logs", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to get execution logs", errorMsg)

		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetExecutionStatus handles GET /api/v1/executions/{executionID}/status to fetch execution status
func (r *Router) handleGetExecutionStatus(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	resp, err := r.svc.GetExecutionStatus(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to get execution status",
			"executionID", executionID,
			"error", err,
			"statusCode", statusCode,
			"errorCode", errorCode)

		writeErrorResponseWithCode(
			w, statusCode, errorCode,
			"failed to get execution status for executionID "+executionID,
			errorMsg,
		)
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleKillExecution handles POST /api/v1/executions/{executionID}/kill to terminate a running execution
func (r *Router) handleKillExecution(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executionID := strings.TrimSpace(chi.URLParam(req, "executionID"))
	if executionID == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid execution id", "executionID is required")
		return
	}

	err := r.svc.KillExecution(req.Context(), executionID)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to kill execution",
			"executionID", executionID,
			"error", err,
			"statusCode", statusCode,
			"errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to kill execution", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.KillExecutionResponse{
		ExecutionID: executionID,
		Message:     "Execution termination initiated",
	})
}

// handleListExecutions handles GET /api/v1/executions to list all executions
func (r *Router) handleListExecutions(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	executions, err := r.svc.ListExecutions(req.Context())
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to list executions", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		writeErrorResponseWithCode(w, statusCode, errorCode, "failed to list executions", errorMsg)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(executions)
}

// handleHealth returns a simple health check response
func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status:  "ok",
		Version: *constants.GetVersion(),
	})
}

// handleClaimAPIKey handles GET /claim/{token} to claim a pending API key
func (r *Router) handleClaimAPIKey(w http.ResponseWriter, req *http.Request) {
	logger := r.GetLoggerFromContext(req.Context())

	// Extract token from URL path
	secretToken := strings.TrimSpace(chi.URLParam(req, "token"))
	if secretToken == "" {
		writeErrorResponse(w, http.StatusBadRequest, "invalid token", "token is required")
		return
	}

	// Get client IP address
	ipAddress := getClientIP(req)

	// Claim the API key
	claimResp, err := r.svc.ClaimAPIKey(req.Context(), secretToken, ipAddress)
	if err != nil {
		statusCode := apperrors.GetStatusCode(err)
		errorCode := apperrors.GetErrorCode(err)
		errorMsg := apperrors.GetErrorMessage(err)

		logger.Debug("failed to claim API key", "error", err, "statusCode", statusCode, "errorCode", errorCode)

		// Render error HTML
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(statusCode)
		_ = renderClaimError(w, errorMsg)
		return
	}

	// Render success HTML with API key
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = renderClaimSuccess(w, claimResp.APIKey, claimResp.UserEmail)
}

// getClientIP extracts the client IP address from request headers
func getClientIP(req *http.Request) string {
	// Check X-Forwarded-For header (used by proxies/load balancers)
	xff := req.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Fall back to RemoteAddr
	return req.RemoteAddr
}

// renderClaimSuccess renders the success page with the API key
func renderClaimSuccess(w http.ResponseWriter, apiKey, userEmail string) error {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Key Claimed - runvoy</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        .container {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 30px;
            background: #f9f9f9;
        }
        h1 { color: #28a745; margin-top: 0; }
        .warning {
            background: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
        }
        .api-key {
            background: #f8f9fa;
            border: 2px solid #007bff;
            border-radius: 4px;
            padding: 15px;
            font-family: 'Courier New', monospace;
            font-size: 14px;
            word-break: break-all;
            margin: 20px 0;
            position: relative;
        }
        .copy-btn {
            background: #007bff;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        .copy-btn:hover { background: #0056b3; }
        .instructions {
            background: #e7f3ff;
            border-left: 4px solid #007bff;
            padding: 15px;
            margin-top: 20px;
        }
        .error { color: #dc3545; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✓ API Key Claimed Successfully</h1>
        
        <p><strong>User Email:</strong> %s</p>
        
        <div class="warning">
            <strong>⚠️ IMPORTANT:</strong> Save this API key now. You will not be able to view it again!
        </div>
        
        <div class="api-key">
            <strong>Your API Key:</strong><br>
            <span id="apiKey">%s</span>
            <button class="copy-btn" onclick="copyToClipboard()" style="margin-top: 10px;">📋 Copy to Clipboard</button>
        </div>
        
        <div class="instructions">
            <h3>Next Steps:</h3>
            <ol>
                <li>Run: <code>runvoy configure</code></li>
                <li>Paste your API key when prompted</li>
                <li>Enter your API endpoint URL</li>
            </ol>
        </div>
    </div>
    
    <script>
        function copyToClipboard() {
            const apiKey = document.getElementById('apiKey').textContent;
            navigator.clipboard.writeText(apiKey).then(function() {
                alert('✓ API key copied to clipboard!');
            }, function() {
                alert('Failed to copy. Please manually select and copy the key.');
            });
        }
        
        window.addEventListener('beforeunload', function(e) {
            e.preventDefault();
            e.returnValue = '';
        });
    </script>
</body>
</html>`, userEmail, apiKey)

	_, err := w.Write([]byte(html))
	return err
}

// renderClaimError renders the error page
func renderClaimError(w http.ResponseWriter, errorMsg string) error {
	var title, message string

	if strings.Contains(errorMsg, "already been claimed") || strings.Contains(errorMsg, "already viewed") {
		title = "Already Claimed"
		message = "This API key has already been claimed by another user."
	} else if strings.Contains(errorMsg, "expired") {
		title = "Link Expired"
		message = "This claim link has expired. Please contact your administrator for a new link."
	} else if strings.Contains(errorMsg, "invalid") || strings.Contains(errorMsg, "not found") {
		title = "Invalid Link"
		message = "This claim link is invalid or has already been used."
	} else {
		title = "Error"
		message = errorMsg
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Claim Error - runvoy</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 600px;
            margin: 50px auto;
            padding: 20px;
            line-height: 1.6;
        }
        .container {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 30px;
            background: #f9f9f9;
        }
        h1 { color: #dc3545; margin-top: 0; }
        .error-box {
            background: #f8d7da;
            border: 1px solid #dc3545;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>✗ %s</h1>
        <div class="error-box">
            <p>%s</p>
            <p>If you believe this is an error, please contact your administrator.</p>
        </div>
    </div>
</body>
</html>`, title, message)

	_, err := w.Write([]byte(html))
	return err
}
