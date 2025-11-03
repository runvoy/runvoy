package client

import (
	"testing"

	"runvoy/internal/config"
	"runvoy/internal/testutil"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		APIEndpoint: "https://example.com",
		APIKey:      "test-key",
	}
	logger := testutil.SilentLogger()

	client := New(cfg, logger)

	if client.config != cfg {
		t.Errorf("Expected config to be %v, got %v", cfg, client.config)
	}

	if client.logger != logger {
		t.Errorf("Expected logger to be %v, got %v", logger, client.logger)
	}
}
