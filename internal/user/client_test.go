package user

import (
	"log/slog"
	"testing"

	"runvoy/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		APIEndpoint: "https://example.com",
		APIKey:      "test-key",
	}
	logger := slog.Default()

	userClient := New(cfg, logger)

	if userClient.client == nil {
		t.Error("Expected client to be initialized")
	}
}
