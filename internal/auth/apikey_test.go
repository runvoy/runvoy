package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "generates valid API key"},
		{name: "generates unique keys on multiple calls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateAPIKey()

			require.NoError(t, err, "GenerateAPIKey should not return an error")
			assert.NotEmpty(t, key, "Generated key should not be empty")

			// Verify it's base64 URL encoded
			_, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(key)
			assert.NoError(t, err, "Key should be valid base64 URL encoding")

			// Verify minimum length (24 bytes encoded should be ~32 chars)
			assert.GreaterOrEqual(t, len(key), 30, "Key should be at least 30 characters")

			// Verify it doesn't contain invalid characters
			assert.True(t, isValidBase64URL(key), "Key should only contain valid base64 URL characters")
		})
	}

	t.Run("generates unique keys", func(t *testing.T) {
		key1, err1 := GenerateAPIKey()
		key2, err2 := GenerateAPIKey()

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, key1, key2, "Two consecutive API keys should be different")
	})
}

func TestHashAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   string // Expected hash (SHA-256 of input, base64 encoded)
	}{
		{
			name:   "hashes simple key",
			apiKey: "test-key-123",
			want:   computeExpectedHash("test-key-123"),
		},
		{
			name:   "hashes empty string",
			apiKey: "",
			want:   computeExpectedHash(""),
		},
		{
			name:   "hashes special characters",
			apiKey: "key-with-!@#$%^&*()",
			want:   computeExpectedHash("key-with-!@#$%^&*()"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := HashAPIKey(tt.apiKey)

			assert.Equal(t, tt.want, hash, "Hash should match expected value")

			// Verify it's valid base64
			_, err := base64.StdEncoding.DecodeString(hash)
			assert.NoError(t, err, "Hash should be valid base64")

			// Verify hash length (SHA-256 produces 32 bytes, base64 encoded is 44 chars)
			assert.Equal(t, 44, len(hash), "SHA-256 hash should be 44 characters when base64 encoded")
		})
	}

	t.Run("same input produces same hash", func(t *testing.T) {
		apiKey := "consistent-key"

		hash1 := HashAPIKey(apiKey)
		hash2 := HashAPIKey(apiKey)

		assert.Equal(t, hash1, hash2, "Hashing the same key twice should produce the same hash")
	})

	t.Run("different inputs produce different hashes", func(t *testing.T) {
		hash1 := HashAPIKey("key1")
		hash2 := HashAPIKey("key2")

		assert.NotEqual(t, hash1, hash2, "Different keys should produce different hashes")
	})

	t.Run("hash is irreversible", func(t *testing.T) {
		apiKey := "secret-key-12345"
		hash := HashAPIKey(apiKey)

		// Verify the original key is not contained in the hash
		decodedHash, _ := base64.StdEncoding.DecodeString(hash)
		assert.NotContains(t, string(decodedHash), apiKey, "Hash should not contain the original key")
	})
}

// Helper function to check if a string is valid base64 URL encoding
func isValidBase64URL(s string) bool {
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for _, c := range s {
		if !strings.ContainsRune(validChars, c) {
			return false
		}
	}
	return true
}

// Helper to compute expected hash for test verification
func computeExpectedHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// Benchmark for API key generation
func BenchmarkGenerateAPIKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GenerateAPIKey()
	}
}

// Benchmark for API key hashing
func BenchmarkHashAPIKey(b *testing.B) {
	apiKey := "test-key-for-benchmarking-12345"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = HashAPIKey(apiKey)
	}
}
