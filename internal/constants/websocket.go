// Package constants defines global constants used throughout runvoy.
package constants

// ConnectionTTLHours is the time-to-live for connection records in the database (24 hours)
const ConnectionTTLHours = 24

// FunctionalityLogStreaming identifies connections used for streaming execution logs
const FunctionalityLogStreaming = "log_streaming"

// MaxConcurrentSends is the maximum number of concurrent sends to WebSocket connections
const MaxConcurrentSends = 10
