package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"runvoy/internal/api"
)

// mockClientInterfaceForUsers extends mockClientInterface with user management methods
type mockClientInterfaceForUsers struct {
	*mockClientInterface
	createUserFunc func(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error)
	listUsersFunc  func(ctx context.Context) (*api.ListUsersResponse, error)
	revokeUserFunc func(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error)
}

func (m *mockClientInterfaceForUsers) CreateUser(
	ctx context.Context, req api.CreateUserRequest,
) (*api.CreateUserResponse, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForUsers) ListUsers(ctx context.Context) (*api.ListUsersResponse, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForUsers) RevokeUser(
	ctx context.Context, req api.RevokeUserRequest,
) (*api.RevokeUserResponse, error) {
	if m.revokeUserFunc != nil {
		return m.revokeUserFunc(ctx, req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClientInterfaceForUsers) FetchBackendLogs(_ context.Context, _ string) ([]api.LogEvent, error) {
	return nil, nil
}

func TestUsersService_CreateUser(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		setupMock    func(*mockClientInterfaceForUsers)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:  "successfully creates user",
			email: "alice@example.com",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.createUserFunc = func(_ context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
					assert.Equal(t, "alice@example.com", req.Email)
					assert.Equal(t, "viewer", req.Role)
					return &api.CreateUserResponse{
						User: &api.User{
							Email:     "alice@example.com",
							Role:      "viewer",
							CreatedAt: time.Now(),
						},
						ClaimToken: "token-123",
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				hasInfof := false
				hasSuccess := false
				hasKeyValue := false
				hasWarning := false
				hasRole := false
				for _, call := range m.calls {
					if call.method == "Infof" {
						hasInfof = true
					}
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" && len(call.args) >= 2 {
						if call.args[0] == "Email" || call.args[0] == "Claim Token" || call.args[0] == "Role" {
							hasKeyValue = true
						}
						if call.args[0] == "Role" {
							hasRole = true
						}
					}
					if call.method == "Warningf" {
						hasWarning = true
					}
				}
				assert.True(t, hasInfof, "Expected Infof call")
				assert.True(t, hasSuccess, "Expected Successf call")
				assert.True(t, hasKeyValue, "Expected KeyValue calls")
				assert.True(t, hasRole, "Expected Role KeyValue call")
				assert.True(t, hasWarning, "Expected Warningf calls for token info")
			},
		},
		{
			name:  "handles client error",
			email: "error@example.com",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.createUserFunc = func(_ context.Context, _ api.CreateUserRequest) (*api.CreateUserResponse, error) {
					return nil, fmt.Errorf("user already exists")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should have Infof but not Successf
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
			mockClient := &mockClientInterfaceForUsers{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewUsersService(mockClient, mockOutput)

			err := service.CreateUser(context.Background(), tt.email, "viewer")

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

func TestUsersService_ListUsers(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockClientInterfaceForUsers)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name: "successfully lists users",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.listUsersFunc = func(_ context.Context) (*api.ListUsersResponse, error) {
					now := time.Now()
					return &api.ListUsersResponse{
						Users: []*api.User{
							{
								Email:     "alice@example.com",
								CreatedAt: time.Now(),
								Revoked:   false,
								LastUsed:  &now,
							},
							{
								Email:     "bob@example.com",
								CreatedAt: time.Now().Add(-24 * time.Hour),
								Revoked:   true,
								LastUsed:  nil,
							},
						},
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
			name: "handles empty user list",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.listUsersFunc = func(_ context.Context) (*api.ListUsersResponse, error) {
					return &api.ListUsersResponse{
						Users: []*api.User{},
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
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.listUsersFunc = func(_ context.Context) (*api.ListUsersResponse, error) {
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
		{
			name: "formats users correctly with revoked status",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.listUsersFunc = func(_ context.Context) (*api.ListUsersResponse, error) {
					return &api.ListUsersResponse{
						Users: []*api.User{
							{
								Email:     "revoked@example.com",
								CreatedAt: time.Now(),
								Revoked:   true,
								LastUsed:  nil,
							},
						},
					}, nil
				}
			},
			wantErr: false,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				for _, call := range m.calls {
					if call.method == "Table" && len(call.args) >= 2 {
						rows := call.args[1].([][]string)
						if len(rows) > 0 && len(rows[0]) >= 3 {
							status := rows[0][2] // Status column (Email=0, Role=1, Status=2)
							assert.Equal(t, "Revoked", status, "Revoked user should show Revoked status")
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForUsers{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewUsersService(mockClient, mockOutput)

			err := service.ListUsers(context.Background())

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

func TestUsersService_RevokeUser(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		setupMock    func(*mockClientInterfaceForUsers)
		wantErr      bool
		verifyOutput func(*testing.T, *mockOutputInterface)
	}{
		{
			name:  "successfully revokes user",
			email: "alice@example.com",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.revokeUserFunc = func(_ context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
					assert.Equal(t, "alice@example.com", req.Email)
					return &api.RevokeUserResponse{
						Email:   "alice@example.com",
						Message: "User revoked successfully",
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
						if call.args[0] == "Email" {
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
			name:  "handles user not found error",
			email: "nonexistent@example.com",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.revokeUserFunc = func(_ context.Context, _ api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
					return nil, fmt.Errorf("user not found")
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
			name:  "handles network error",
			email: "error@example.com",
			setupMock: func(m *mockClientInterfaceForUsers) {
				m.revokeUserFunc = func(_ context.Context, _ api.RevokeUserRequest) (*api.RevokeUserResponse, error) {
					return nil, fmt.Errorf("network error")
				}
			},
			wantErr: true,
			verifyOutput: func(t *testing.T, m *mockOutputInterface) {
				// Should have Infof but not Successf or KeyValue
				hasSuccess := false
				hasKeyValue := false
				for _, call := range m.calls {
					if call.method == "Successf" {
						hasSuccess = true
					}
					if call.method == "KeyValue" {
						hasKeyValue = true
					}
				}
				assert.False(t, hasSuccess, "Should not have Successf on error")
				assert.False(t, hasKeyValue, "Should not have KeyValue on error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientInterfaceForUsers{
				mockClientInterface: &mockClientInterface{},
			}
			tt.setupMock(mockClient)

			mockOutput := &mockOutputInterface{}
			service := NewUsersService(mockClient, mockOutput)

			err := service.RevokeUser(context.Background(), tt.email)

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
