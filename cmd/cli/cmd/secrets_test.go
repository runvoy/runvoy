package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
)

// mockClientInterfaceForSecrets extends mockClientInterface with secrets management methods
type mockClientInterfaceForSecrets struct {
	*mockClientInterface
	createSecretFunc func(ctx context.Context, req api.CreateSecretRequest) (*api.CreateSecretResponse, error)
	getSecretFunc    func(ctx context.Context, name string) (*api.GetSecretResponse, error)
	listSecretsFunc  func(ctx context.Context) (*api.ListSecretsResponse, error)
	updateSecretFunc func(ctx context.Context, name string, req api.UpdateSecretRequest) (*api.UpdateSecretResponse, error)
	deleteSecretFunc func(ctx context.Context, name string) (*api.DeleteSecretResponse, error)
}

func (m *mockClientInterfaceForSecrets) CreateSecret(
	ctx context.Context, req api.CreateSecretRequest,
) (*api.CreateSecretResponse, error) {
	if m.createSecretFunc != nil {
		return m.createSecretFunc(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForSecrets) GetSecret(ctx context.Context, name string) (*api.GetSecretResponse, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(ctx, name)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForSecrets) ListSecrets(ctx context.Context) (*api.ListSecretsResponse, error) {
	if m.listSecretsFunc != nil {
		return m.listSecretsFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForSecrets) UpdateSecret(
	ctx context.Context, name string, req api.UpdateSecretRequest,
) (*api.UpdateSecretResponse, error) {
	if m.updateSecretFunc != nil {
		return m.updateSecretFunc(ctx, name, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForSecrets) DeleteSecret(
	ctx context.Context,
	name string,
) (*api.DeleteSecretResponse, error) {
	if m.deleteSecretFunc != nil {
		return m.deleteSecretFunc(ctx, name)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForSecrets) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	return nil, nil
}

func TestSecretsService_CreateSecret(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		keyName      string
		value        string
		description  string
		setupMock    func(*mockClientInterfaceForSecrets)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully creates secret",
			secretName:  "github-token",
			keyName:     "GITHUB_TOKEN",
			value:       "ghp_xxxxx",
			description: "GitHub API token",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.createSecretFunc = func(_ context.Context, req api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
					assert.Equal(t, "github-token", req.Name)
					assert.Equal(t, "GITHUB_TOKEN", req.KeyName)
					assert.Equal(t, "ghp_xxxxx", req.Value)
					assert.Equal(t, "GitHub API token", req.Description)
					return &api.CreateSecretResponse{
						Message: "Secret created successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
						if call.args[0] == "Name" || call.args[0] == "Key Name" || call.args[0] == "Description" {
							hasKeyValue = true
						}
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue calls")
			},
		},
		{
			name:        "successfully creates secret without description",
			secretName:  "db-password",
			keyName:     "DB_PASSWORD",
			value:       "secret123",
			description: "",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.createSecretFunc = func(_ context.Context, req api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
					assert.Equal(t, "db-password", req.Name)
					assert.Equal(t, "DB_PASSWORD", req.KeyName)
					assert.Equal(t, "secret123", req.Value)
					assert.Empty(t, req.Description)
					return &api.CreateSecretResponse{
						Message: "Secret created successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			name:        "handles client error",
			secretName:  "github-token",
			keyName:     "GITHUB_TOKEN",
			value:       "ghp_xxxxx",
			description: "",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.createSecretFunc = func(_ context.Context, _ api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
					return nil, fmt.Errorf("secret already exists")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			mockClient := &mockClientInterfaceForSecrets{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewSecretsService(mockClient, mockOutput)

			err := service.CreateSecret(context.Background(), tt.secretName, tt.keyName, tt.value, tt.description)

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

func TestSecretsService_GetSecret(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		setupMock    func(*mockClientInterfaceForSecrets)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:       "successfully gets secret",
			secretName: "github-token",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.getSecretFunc = func(_ context.Context, name string) (*api.GetSecretResponse, error) {
					assert.Equal(t, "github-token", name)
					return &api.GetSecretResponse{
						Secret: &api.Secret{
							Name:        "github-token",
							KeyName:     "GITHUB_TOKEN",
							Description: "GitHub API token",
							Value:       "ghp_xxxxx",
							CreatedBy:   "alice@example.com",
							CreatedAt:   time.Now(),
							UpdatedBy:   "alice@example.com",
							UpdatedAt:   time.Now(),
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
					if call.method == "KeyValue" {
						hasKeyValue = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue calls")
			},
		},
		{
			name:       "handles secret not found",
			secretName: "nonexistent",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.getSecretFunc = func(_ context.Context, _ string) (*api.GetSecretResponse, error) {
					return nil, fmt.Errorf("secret not found")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			mockClient := &mockClientInterfaceForSecrets{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewSecretsService(mockClient, mockOutput)

			err := service.GetSecret(context.Background(), tt.secretName)

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

func TestSecretsService_ListSecrets(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockClientInterfaceForSecrets)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name: "successfully lists secrets",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.listSecretsFunc = func(_ context.Context) (*api.ListSecretsResponse, error) {
					return &api.ListSecretsResponse{
						Secrets: []*api.Secret{
							{
								Name:        "github-token",
								KeyName:     "GITHUB_TOKEN",
								Description: "GitHub API token",
								CreatedBy:   "alice@example.com",
								CreatedAt:   time.Now(),
							},
							{
								Name:        "db-password",
								KeyName:     "DB_PASSWORD",
								Description: "",
								CreatedBy:   "bob@example.com",
								CreatedAt:   time.Now(),
							},
						},
						Total: 2,
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				hasTable := false
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Table" {
						hasTable = true
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasTable, "Expected Table call")
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name: "handles empty secret list",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.listSecretsFunc = func(_ context.Context) (*api.ListSecretsResponse, error) {
					return &api.ListSecretsResponse{
						Secrets: []*api.Secret{},
						Total:   0,
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasWarning := false
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Warningf" {
						hasWarning = true
					}
					if call.method == "Table" {
						hasTable = true
					}
				}
				assert.True(t, hasWarning, "Expected warning for empty list")
				assert.False(t, hasTable, "Should not call Table for empty list")
			},
		},
		{
			name: "handles client error",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.listSecretsFunc = func(_ context.Context) (*api.ListSecretsResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasTable := false
				for _, call := range m.calls {
					if call.method == "Table" {
						hasTable = true
					}
				}
				assert.False(t, hasTable, "Should not call Table on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForSecrets{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewSecretsService(mockClient, mockOutput)

			err := service.ListSecrets(context.Background())

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

func TestSecretsService_UpdateSecret(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		keyName      string
		value        string
		description  string
		setupMock    func(*mockClientInterfaceForSecrets)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:        "successfully updates secret with all fields",
			secretName:  "github-token",
			keyName:     "GITHUB_API_TOKEN",
			value:       "new-token",
			description: "Updated description",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.updateSecretFunc = func(
					_ context.Context,
					name string,
					req api.UpdateSecretRequest,
				) (*api.UpdateSecretResponse, error) {
					assert.Equal(t, "github-token", name)
					assert.Equal(t, "GITHUB_API_TOKEN", req.KeyName)
					assert.Equal(t, "new-token", req.Value)
					assert.Equal(t, "Updated description", req.Description)
					return &api.UpdateSecretResponse{
						Message: "Secret updated successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				hasSuccess := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
			},
		},
		{
			name:        "successfully updates secret with only value",
			secretName:  "github-token",
			keyName:     "",
			value:       "new-token",
			description: "",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.updateSecretFunc = func(
					_ context.Context,
					name string,
					req api.UpdateSecretRequest,
				) (*api.UpdateSecretResponse, error) {
					assert.Equal(t, "github-token", name)
					assert.Equal(t, "new-token", req.Value)
					assert.Empty(t, req.KeyName)
					assert.Empty(t, req.Description)
					return &api.UpdateSecretResponse{
						Message: "Secret updated successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			name:        "handles error when no fields provided",
			secretName:  "github-token",
			keyName:     "",
			value:       "",
			description: "",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.updateSecretFunc = func(
					_ context.Context,
					_ string,
					_ api.UpdateSecretRequest,
				) (*api.UpdateSecretResponse, error) {
					return nil, fmt.Errorf("should not be called")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			name:        "handles client error",
			secretName:  "github-token",
			keyName:     "",
			value:       "new-token",
			description: "",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.updateSecretFunc = func(
					_ context.Context,
					_ string,
					_ api.UpdateSecretRequest,
				) (*api.UpdateSecretResponse, error) {
					return nil, fmt.Errorf("secret not found")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			mockClient := &mockClientInterfaceForSecrets{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewSecretsService(mockClient, mockOutput)

			err := service.UpdateSecret(context.Background(), tt.secretName, tt.keyName, tt.value, tt.description)

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

func TestSecretsService_DeleteSecret(t *testing.T) {
	tests := []struct {
		name         string
		secretName   string
		setupMock    func(*mockClientInterfaceForSecrets)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:       "successfully deletes secret",
			secretName: "github-token",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.deleteSecretFunc = func(_ context.Context, name string) (*api.DeleteSecretResponse, error) {
					assert.Equal(t, "github-token", name)
					return &api.DeleteSecretResponse{
						Name:    "github-token",
						Message: "Secret deleted successfully",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
						if call.args[0] == "Name" {
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
			name:       "handles secret not found error",
			secretName: "nonexistent",
			setupMock: func(m *mockClientInterfaceForSecrets) {
				m.deleteSecretFunc = func(_ context.Context, _ string) (*api.DeleteSecretResponse, error) {
					return nil, fmt.Errorf("secret not found")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
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
			mockClient := &mockClientInterfaceForSecrets{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewSecretsService(mockClient, mockOutput)

			err := service.DeleteSecret(context.Background(), tt.secretName)

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
