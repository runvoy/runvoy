package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
	"runvoy/internal/config"
)

// mockClientInterfaceForClaim extends mockClientInterface with ClaimAPIKey
type mockClientInterfaceForClaim struct {
	*mockClientInterface
	claimAPIKeyFunc func(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error)
}

func (m *mockClientInterfaceForClaim) ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error) {
	if m.claimAPIKeyFunc != nil {
		return m.claimAPIKeyFunc(ctx, token)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForClaim) GetLogStreamURL(_ context.Context, _ string) (*api.LogStreamResponse, error) {
	return &api.LogStreamResponse{}, nil
}

// mockConfigSaver is a mock for ConfigSaver interface
type mockConfigSaver struct {
	saveFunc func(cfg *config.Config) error
}

func (m *mockConfigSaver) Save(cfg *config.Config) error {
	if m.saveFunc != nil {
		return m.saveFunc(cfg)
	}
	return nil
}

func TestClaimService_ClaimAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		token        string
		cfg          *config.Config
		setupClient  func(*mockClientInterfaceForClaim)
		setupSaver   func(*mockConfigSaver)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
		verifyConfig func(*testing.T, *config.Config)
	}{
		{
			name:  "successfully claims API key and saves to config",
			token: "valid-token-123",
			cfg:   &config.Config{APIEndpoint: "https://api.example.com"},
			setupClient: func(m *mockClientInterfaceForClaim) {
				m.claimAPIKeyFunc = func(_ context.Context, token string) (*api.ClaimAPIKeyResponse, error) {
					assert.Equal(t, "valid-token-123", token)
					return &api.ClaimAPIKeyResponse{
						APIKey:    "sk_live_abc123",
						UserEmail: "user@example.com",
					}, nil
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(cfg *config.Config) error {
					assert.Equal(t, "sk_live_abc123", cfg.APIKey)
					return nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
						break
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
			},
			verifyConfig: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "sk_live_abc123", cfg.APIKey)
			},
		},
		{
			name:  "handles invalid token",
			token: "invalid-token",
			cfg:   &config.Config{APIEndpoint: "https://api.example.com"},
			setupClient: func(m *mockClientInterfaceForClaim) {
				m.claimAPIKeyFunc = func(_ context.Context, _ string) (*api.ClaimAPIKeyResponse, error) {
					return nil, fmt.Errorf("invalid token")
				}
			},
			setupSaver: func(_ *mockConfigSaver) {},
			wantErr:    true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should not have Successf call when there's an error
				for _, call := range m.calls {
					assert.NotEqual(t, "Successf", call.method, "Should not display success on error")
				}
			},
			verifyConfig: func(t *testing.T, cfg *config.Config) {
				// APIKey should not be set on error
				assert.Empty(t, cfg.APIKey)
			},
		},
		{
			name:  "handles network error",
			token: "network-token",
			cfg:   &config.Config{APIEndpoint: "https://api.example.com"},
			setupClient: func(m *mockClientInterfaceForClaim) {
				m.claimAPIKeyFunc = func(_ context.Context, _ string) (*api.ClaimAPIKeyResponse, error) {
					return nil, fmt.Errorf("network error: connection refused")
				}
			},
			setupSaver: func(_ *mockConfigSaver) {},
			wantErr:    true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Service returns error, output.Errorf is called in runClaim handler
				// So no output calls should be made in the service itself
				assert.Equal(t, 0, len(m.calls), "Service should not call output on network error")
			},
			verifyConfig: func(t *testing.T, cfg *config.Config) {
				assert.Empty(t, cfg.APIKey)
			},
		},
		{
			name:  "handles config save failure",
			token: "valid-token",
			cfg:   &config.Config{APIEndpoint: "https://api.example.com"},
			setupClient: func(m *mockClientInterfaceForClaim) {
				m.claimAPIKeyFunc = func(_ context.Context, _ string) (*api.ClaimAPIKeyResponse, error) {
					return &api.ClaimAPIKeyResponse{
						APIKey:    "sk_live_xyz789",
						UserEmail: "user@example.com",
					}, nil
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(_ *config.Config) error {
					return fmt.Errorf("failed to write config file: permission denied")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasError := false
				hasWarning := false
				for _, call := range m.calls {
					if call.method == "Errorf" {
						hasError = true
					}
					if call.method == "Warningf" {
						hasWarning = true
					}
				}
				assert.True(t, hasError, "Expected Errorf call on save failure")
				assert.True(t, hasWarning, "Expected Warningf call with API key when save fails")
			},
			verifyConfig: func(t *testing.T, cfg *config.Config) {
				// APIKey should still be set even if save fails
				assert.Equal(t, "sk_live_xyz789", cfg.APIKey)
			},
		},
		{
			name:  "displays warning with API key when save fails",
			token: "token-456",
			cfg:   &config.Config{APIEndpoint: "https://api.example.com"},
			setupClient: func(m *mockClientInterfaceForClaim) {
				m.claimAPIKeyFunc = func(_ context.Context, _ string) (*api.ClaimAPIKeyResponse, error) {
					return &api.ClaimAPIKeyResponse{
						APIKey: "sk_live_warning123",
					}, nil
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(_ *config.Config) error {
					return fmt.Errorf("save error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasWarningWithAPIKey := false
				for _, call := range m.calls {
					if call.method == "Warningf" && len(call.args) >= 1 {
						format := call.args[0].(string)
						if fmt.Sprintf(format, call.args[1:]...) != "" {
							hasWarningWithAPIKey = true
						}
					}
				}
				assert.True(t, hasWarningWithAPIKey, "Expected warning with API key")
			},
			verifyConfig: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "sk_live_warning123", cfg.APIKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForClaim{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupClient(mockClient)

			mockSaver := &mockConfigSaver{}
			tt.setupSaver(mockSaver)

			mockOutput := &mockOutputInterface{}
			service := NewClaimService(mockClient, mockOutput, mockSaver)

			err := service.ClaimAPIKey(context.Background(), tt.token, tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}

			if tt.verifyConfig != nil {
				tt.verifyConfig(t, tt.cfg)
			}
		})
	}
}
