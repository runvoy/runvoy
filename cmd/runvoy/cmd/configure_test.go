package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/config"
)

// mockOutputInterfaceWithPrompt extends mockOutputInterface with configurable Prompt behavior
type mockOutputInterfaceWithPrompt struct {
	*mockOutputInterface
	promptFunc func(prompt string) string
}

func (m *mockOutputInterfaceWithPrompt) Prompt(prompt string) string {
	m.calls = append(m.calls, call{method: "Prompt", args: []interface{}{prompt}})
	if m.promptFunc != nil {
		return m.promptFunc(prompt)
	}
	return ""
}

// mockConfigLoader is a mock for ConfigLoader interface
type mockConfigLoader struct {
	loadFunc func() (*config.Config, error)
}

func (m *mockConfigLoader) Load() (*config.Config, error) {
	if m.loadFunc != nil {
		return m.loadFunc()
	}
	return nil, fmt.Errorf("not implemented")
}

func TestConfigureService_Configure(t *testing.T) {
	tests := []struct {
		name            string
		setupPrompt     func(*mockOutputInterfaceWithPrompt)
		setupLoader     func(*mockConfigLoader)
		setupSaver      func(*mockConfigSaver)
		setupPathGetter func() (string, error)
		wantErr         bool
		verifyOutput    func(*testing.T, *mockOutputInterfaceWithPrompt)
	}{
		{
			name: "successfully creates new configuration",
			setupPrompt: func(m *mockOutputInterfaceWithPrompt) {
				m.promptFunc = func(prompt string) string {
					if prompt == "Enter API endpoint URL" {
						return "https://api.example.com"
					}
					if prompt == "Enter API key" {
						return "sk_live_abc123"
					}
					return ""
				}
			},
			setupLoader: func(m *mockConfigLoader) {
				m.loadFunc = func() (*config.Config, error) {
					return nil, fmt.Errorf("config not found")
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(cfg *config.Config) error {
					assert.Equal(t, "https://api.example.com", cfg.APIEndpoint)
					assert.Equal(t, "sk_live_abc123", cfg.APIKey)
					return nil
				}
			},
			setupPathGetter: func() (string, error) {
				return "/home/user/.runvoy/config.yaml", nil
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterfaceWithPrompt) {
				hasInfof := false
				hasSuccess := false
				hasKeyValue := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Configuration path" {
							hasKeyValue = true
						}
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue call")
			},
		},
		{
			name: "updates existing configuration with new values",
			setupPrompt: func(m *mockOutputInterfaceWithPrompt) {
				m.promptFunc = func(prompt string) string {
					if prompt == "Enter API endpoint URL" {
						return "https://api.new.com"
					}
					if prompt == "Enter API key" {
						return "sk_live_newkey"
					}
					return ""
				}
			},
			setupLoader: func(m *mockConfigLoader) {
				m.loadFunc = func() (*config.Config, error) {
					return &config.Config{
						APIEndpoint: "https://api.old.com",
						APIKey:      "sk_live_oldkey",
					}, nil
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(cfg *config.Config) error {
					assert.Equal(t, "https://api.new.com", cfg.APIEndpoint)
					assert.Equal(t, "sk_live_newkey", cfg.APIKey)
					return nil
				}
			},
			setupPathGetter: func() (string, error) {
				return "/home/user/.runvoy/config.yaml", nil
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterfaceWithPrompt) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name: "uses existing values when prompt returns empty",
			setupPrompt: func(m *mockOutputInterfaceWithPrompt) {
				m.promptFunc = func(prompt string) string {
					return "" // User presses Enter without typing
				}
			},
			setupLoader: func(m *mockConfigLoader) {
				m.loadFunc = func() (*config.Config, error) {
					return &config.Config{
						APIEndpoint: "https://api.existing.com",
						APIKey:      "sk_live_existing",
					}, nil
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(cfg *config.Config) error {
					assert.Equal(t, "https://api.existing.com", cfg.APIEndpoint)
					assert.Equal(t, "sk_live_existing", cfg.APIKey)
					return nil
				}
			},
			setupPathGetter: func() (string, error) {
				return "/home/user/.runvoy/config.yaml", nil
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterfaceWithPrompt) {
				hasInfof := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						format := call.args[0].(string)
						if format == "Using existing endpoint: %s" || format == "Using existing API key" {
							hasInfof = true
						}
					}
				}
				assert.True(t, hasInfof, "Expected Infof call for using existing values")
			},
		},
		{
			name: "returns error when endpoint is required but not provided",
			setupPrompt: func(m *mockOutputInterfaceWithPrompt) {
				m.promptFunc = func(prompt string) string {
					return "" // Empty response
				}
			},
			setupLoader: func(m *mockConfigLoader) {
				m.loadFunc = func() (*config.Config, error) {
					return nil, fmt.Errorf("config not found")
				}
			},
			setupSaver: func(m *mockConfigSaver) {},
			setupPathGetter: func() (string, error) {
				return "", nil
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterfaceWithPrompt) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.False(t, hasSuccess, "Should not have Successf on error")
			},
		},
		{
			name: "returns error when config save fails",
			setupPrompt: func(m *mockOutputInterfaceWithPrompt) {
				m.promptFunc = func(prompt string) string {
					if prompt == "Enter API endpoint URL" {
						return "https://api.example.com"
					}
					if prompt == "Enter API key" {
						return "sk_live_abc123"
					}
					return ""
				}
			},
			setupLoader: func(m *mockConfigLoader) {
				m.loadFunc = func() (*config.Config, error) {
					return nil, fmt.Errorf("config not found")
				}
			},
			setupSaver: func(m *mockConfigSaver) {
				m.saveFunc = func(cfg *config.Config) error {
					return fmt.Errorf("permission denied")
				}
			},
			setupPathGetter: func() (string, error) {
				return "", nil
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterfaceWithPrompt) {
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.False(t, hasSuccess, "Should not have Successf on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOutput := &mockOutputInterfaceWithPrompt{
				mockOutputInterface: &mockOutputInterface{},
			}
			tt.setupPrompt(mockOutput)

			mockLoader := &mockConfigLoader{}
			tt.setupLoader(mockLoader)

			mockSaver := &mockConfigSaver{}
			tt.setupSaver(mockSaver)

			service := NewConfigureService(
				mockOutput,
				mockSaver,
				mockLoader.Load,
				tt.setupPathGetter,
			)

			err := service.Configure(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, mockOutput)
			}
		})
	}
}
