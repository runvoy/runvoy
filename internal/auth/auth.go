// Package auth provides authentication utilities for runvoy.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/runvoy/runvoy/internal/constants"
)

// HashAPIKey creates a SHA-256 hash of the API key for secure storage.
// NOTICE: we never store plain API keys in the database.
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))

	return base64.StdEncoding.EncodeToString(hash[:])
}

// GenerateSecretToken creates a cryptographically secure random secret token.
// Used for claim URLs, WebSocket authentication, and other temporary access tokens.
// The token is base64-encoded and approximately 32 characters long.
func GenerateSecretToken() (string, error) {
	b := make([]byte, constants.SecretTokenByteSize)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// GenerateUUID generates a UUID-like identifier using crypto/rand.
// Returns a hex-encoded string of 32 characters (16 random bytes).
// Used for generating unique identifiers such as task definition family names.
func GenerateUUID() string {
	b := make([]byte, constants.UUIDByteSize)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based identifier if random generation fails
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

// GenerateEventID creates a deterministic unique event ID from timestamp and message.
// Uses SHA-256 hash of the concatenated timestamp and message, then returns the first 16 bytes (32 hex characters).
// Used for generating event IDs for log entries when the source doesn't provide one.
func GenerateEventID(timestamp int64, message string) string {
	var buf []byte
	buf = fmt.Appendf(buf, "%d:%s", timestamp, message)
	hash := sha256.Sum256(buf)
	return hex.EncodeToString(hash[:])[:16] // Use first 16 bytes (32 hex chars)
}
