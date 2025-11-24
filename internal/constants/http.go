package constants

import "time"

// APIKeyHeader is the HTTP header name for API key authentication
//
//nolint:gosec // G101: This is a header name constant, not a hardcoded credential
const APIKeyHeader = "X-API-Key"

// ContentTypeHeader is the HTTP Content-Type header name.
const ContentTypeHeader = "Content-Type"

// HTTPStatusBadRequest is the HTTP status code for bad requests (400)
const HTTPStatusBadRequest = 400

// HTTPStatusServerError is the HTTP status code for server errors (500)
const HTTPStatusServerError = 500

// ServerReadTimeout is the HTTP server read timeout
const ServerReadTimeout = 15 * time.Second

// ServerWriteTimeout is the HTTP server write timeout
const ServerWriteTimeout = 15 * time.Second

// ServerIdleTimeout is the HTTP server idle timeout
const ServerIdleTimeout = 60 * time.Second

// ServerShutdownTimeout is the timeout for graceful server shutdown
const ServerShutdownTimeout = 5 * time.Second
