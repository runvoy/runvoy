package user

import (
	"log/slog"
	"testing"

	"runvoy/internal/config"
)

func TestNewService(t *testing.T) {
	cfg := &config.Config{
		APIEndpoint: "https://example.com",
		APIKey:      "test-key",
	}
	logger := slog.Default()

	service := NewService(cfg, logger)

	if service.config != cfg {
		t.Errorf("Expected config to be %v, got %v", cfg, service.config)
	}

	if service.logger != logger {
		t.Errorf("Expected logger to be %v, got %v", logger, service.logger)
	}
}

func TestNewClient(t *testing.T) {
	cfg := &config.Config{
		APIEndpoint: "https://example.com",
		APIKey:      "test-key",
	}
	logger := slog.Default()

	client := NewClient(cfg, logger)

	if client.service == nil {
		t.Error("Expected service to be initialized")
	}

	if client.service.config != cfg {
		t.Errorf("Expected service config to be %v, got %v", cfg, client.service.config)
	}
}