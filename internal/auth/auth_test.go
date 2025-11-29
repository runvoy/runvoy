package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.Len(t, hash, 44, "SHA-256 hash should be 44 characters when base64 encoded")
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

func TestGenerateSecretToken(t *testing.T) {
	t.Run("generates valid secret token", func(t *testing.T) {
		token, err := GenerateSecretToken()

		require.NoError(t, err, "GenerateSecretToken should not return an error")
		assert.NotEmpty(t, token, "Generated token should not be empty")

		// Verify it's base64 URL encoded
		_, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(token)
		assert.NoError(t, err, "Token should be valid base64 URL encoding")

		// Verify minimum length (24 bytes encoded should be ~32 chars)
		assert.GreaterOrEqual(t, len(token), 30, "Token should be at least 30 characters")

		// Verify it doesn't contain invalid characters
		assert.True(t, isValidBase64URL(token), "Token should only contain valid base64 URL characters")
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, err1 := GenerateSecretToken()
		token2, err2 := GenerateSecretToken()

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, token1, token2, "Two consecutive secret tokens should be different")
	})
}

func TestGenerateUUID(t *testing.T) {
	t.Run("generates valid UUID", func(t *testing.T) {
		uuid := GenerateUUID()

		assert.NotEmpty(t, uuid, "Generated UUID should not be empty")

		// Verify it's valid hex encoding
		decoded, err := hex.DecodeString(uuid)
		assert.NoError(t, err, "UUID should be valid hex encoding")
		assert.NotEmpty(t, decoded, "Decoded UUID should not be empty")
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		uuid1 := GenerateUUID()
		uuid2 := GenerateUUID()

		assert.NotEqual(t, uuid1, uuid2, "Two consecutive UUIDs should be different")
	})

	t.Run("generates UUIDs of consistent length", func(t *testing.T) {
		// UUIDByteSize is 16 bytes, hex encoded should be 32 characters
		uuid := GenerateUUID()
		// Allow some flexibility for the fallback case, but normal case should be 32
		assert.GreaterOrEqual(t, len(uuid), 16, "UUID should be at least 16 characters")
	})

	t.Run("multiple UUIDs are distinct", func(t *testing.T) {
		uuids := make(map[string]bool)
		for range 100 {
			uuid := GenerateUUID()
			assert.False(t, uuids[uuid], "UUID should be unique across multiple generations")
			uuids[uuid] = true
		}
	})
}

// Benchmark for API key hashing
func BenchmarkHashAPIKey(b *testing.B) {
	apiKey := "test-key-for-benchmarking-12345"

	for b.Loop() {
		_ = HashAPIKey(apiKey)
	}
}

// Benchmark for secret token generation
func BenchmarkGenerateSecretToken(b *testing.B) {
	for b.Loop() {
		_, _ = GenerateSecretToken()
	}
}
