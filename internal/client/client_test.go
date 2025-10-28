package client

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

	client := New(cfg, logger)

	if client.config != cfg {
		t.Errorf("Expected config to be %v, got %v", cfg, client.config)
	}

	if client.logger != logger {
		t.Errorf("Expected logger to be %v, got %v", logger, client.logger)
	}
}
