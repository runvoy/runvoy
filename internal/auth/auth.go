// Package auth provides authentication utilities for runvoy.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"runvoy/internal/constants"
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
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}
