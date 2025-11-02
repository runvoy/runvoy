package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"runvoy/internal/api"
	apperrors "runvoy/internal/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticateUser(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		apiKey          string
		mockUser        *api.User
		mockErr         error
		expectErr       bool
		expectedErrCode string
	}{
		{
			name:            "empty API key",
			apiKey:          "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:            "user not found",
			apiKey:          "non-existent-key",
			mockUser:        nil,
			mockErr:         nil,
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidAPIKey,
		},
		{
			name:    "revoked API key",
			apiKey:  "revoked-key",
			mockUser: &api.User{
				Email:     "user@example.com",
				CreatedAt: time.Now(),
				Revoked:   true,
			},
			mockErr:         nil,
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeAPIKeyRevoked,
		},
		{
			name:    "successful authentication",
			apiKey:  "valid-key",
			mockUser: &api.User{
				Email:     "user@example.com",
				CreatedAt: time.Now(),
				Revoked:   false,
			},
			mockErr:   nil,
			expectErr: false,
		},
		{
			name:            "repository error",
			apiKey:          "test-key",
			mockUser:        nil,
			mockErr:         errors.New("database connection failed"),
			expectErr:       true,
			expectedErrCode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{
				getUserByAPIKeyHashFunc: func(ctx context.Context, apiKeyHash string) (*api.User, error) {
					return tt.mockUser, tt.mockErr
				},
			}

			svc := newTestService(userRepo, nil, nil)
			user, err := svc.AuthenticateUser(ctx, tt.apiKey)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode != "" {
					assert.Equal(t, tt.expectedErrCode, apperrors.GetErrorCode(err))
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				require.NotNil(t, user)
				assert.Equal(t, tt.mockUser.Email, user.Email)
				assert.Equal(t, tt.mockUser.Revoked, user.Revoked)
			}
		})
	}
}

func TestUpdateUserLastUsed(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		email           string
		mockTime        *time.Time
		mockErr         error
		expectErr       bool
		expectedErrCode string
	}{
		{
			name:            "empty email",
			email:           "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:      "successful update",
			email:     "user@example.com",
			mockTime:  timePtr(time.Now()),
			mockErr:   nil,
			expectErr: false,
		},
		{
			name:      "repository error",
			email:     "user@example.com",
			mockTime:  nil,
			mockErr:   errors.New("update failed"),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{
				updateLastUsedFunc: func(ctx context.Context, email string) (*time.Time, error) {
					return tt.mockTime, tt.mockErr
				},
			}

			svc := newTestService(userRepo, nil, nil)
			timestamp, err := svc.UpdateUserLastUsed(ctx, tt.email)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode != "" {
					assert.Equal(t, tt.expectedErrCode, apperrors.GetErrorCode(err))
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, timestamp)
			}
		})
	}
}

func TestRevokeUser(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		email           string
		mockUser        *api.User
		getUserErr      error
		revokeErr       error
		expectErr       bool
		expectedErrCode string
	}{
		{
			name:            "empty email",
			email:           "",
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeInvalidRequest,
		},
		{
			name:            "user not found",
			email:           "nonexistent@example.com",
			mockUser:        nil,
			getUserErr:      nil,
			expectErr:       true,
			expectedErrCode: apperrors.ErrCodeNotFound,
		},
		{
			name:  "successful revocation",
			email: "user@example.com",
			mockUser: &api.User{
				Email:   "user@example.com",
				Revoked: false,
			},
			getUserErr: nil,
			revokeErr:  nil,
			expectErr:  false,
		},
		{
			name:       "database error on get",
			email:      "user@example.com",
			mockUser:   nil,
			getUserErr: errors.New("database connection failed"),
			expectErr:  true,
		},
		{
			name:  "database error on revoke",
			email: "user@example.com",
			mockUser: &api.User{
				Email:   "user@example.com",
				Revoked: false,
			},
			getUserErr: nil,
			revokeErr:  errors.New("update failed"),
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{
				getUserByEmailFunc: func(ctx context.Context, email string) (*api.User, error) {
					return tt.mockUser, tt.getUserErr
				},
				revokeUserFunc: func(ctx context.Context, email string) error {
					return tt.revokeErr
				},
			}

			svc := newTestService(userRepo, nil, nil)
			err := svc.RevokeUser(ctx, tt.email)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectedErrCode != "" {
					assert.Equal(t, tt.expectedErrCode, apperrors.GetErrorCode(err))
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
