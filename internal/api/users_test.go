// Package api defines the API types and structures used across runvoy.
package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		now := time.Now()
		user := User{
			Email:     "user@example.com",
			APIKey:    "key-123",
			CreatedAt: now,
			Revoked:   false,
			LastUsed:  &now,
		}

		data, err := json.Marshal(user)
		require.NoError(t, err)

		var unmarshaled User
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, user.Email, unmarshaled.Email)
		assert.Equal(t, user.APIKey, unmarshaled.APIKey)
		assert.Equal(t, user.Revoked, unmarshaled.Revoked)
		assert.NotNil(t, unmarshaled.LastUsed)
	})

	t.Run("omit empty optional fields", func(t *testing.T) {
		now := time.Now()
		user := User{
			Email:     "user@example.com",
			CreatedAt: now,
			Revoked:   false,
		}

		data, err := json.Marshal(user)
		require.NoError(t, err)

		// APIKey and LastUsed should be omitted when empty
		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "api_key")
		assert.NotContains(t, jsonStr, "last_used")
	})
}

func TestCreateUserRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with API key", func(t *testing.T) {
		req := CreateUserRequest{
			Email:  "user@example.com",
			APIKey: "custom-key",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CreateUserRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Email, unmarshaled.Email)
		assert.Equal(t, req.APIKey, unmarshaled.APIKey)
	})

	t.Run("marshal and unmarshal without API key", func(t *testing.T) {
		req := CreateUserRequest{
			Email: "user@example.com",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CreateUserRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Email, unmarshaled.Email)
		assert.Empty(t, unmarshaled.APIKey)
	})
}

func TestCreateUserResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now()
		resp := CreateUserResponse{
			User: &User{
				Email:     "user@example.com",
				CreatedAt: now,
				Revoked:   false,
			},
			ClaimToken: "token-123",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled CreateUserResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.NotNil(t, unmarshaled.User)
		assert.Equal(t, resp.User.Email, unmarshaled.User.Email)
		assert.Equal(t, resp.ClaimToken, unmarshaled.ClaimToken)
	})
}

func TestPendingAPIKeyJSON(t *testing.T) {
	t.Run("marshal and unmarshal viewed key", func(t *testing.T) {
		now := time.Now()
		key := PendingAPIKey{
			SecretToken:  "token-123",
			APIKey:       "key-123",
			UserEmail:    "user@example.com",
			CreatedBy:    "admin@example.com",
			CreatedAt:    now,
			ExpiresAt:    now.Unix() + 3600,
			Viewed:       true,
			ViewedAt:     &now,
			ViewedFromIP: "192.168.1.1",
		}

		data, err := json.Marshal(key)
		require.NoError(t, err)

		var unmarshaled PendingAPIKey
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, key.SecretToken, unmarshaled.SecretToken)
		assert.Equal(t, key.APIKey, unmarshaled.APIKey)
		assert.Equal(t, key.UserEmail, unmarshaled.UserEmail)
		assert.Equal(t, key.CreatedBy, unmarshaled.CreatedBy)
		assert.Equal(t, key.Viewed, unmarshaled.Viewed)
		assert.NotNil(t, unmarshaled.ViewedAt)
		assert.Equal(t, key.ViewedFromIP, unmarshaled.ViewedFromIP)
	})

	t.Run("marshal and unmarshal unviewed key", func(t *testing.T) {
		now := time.Now()
		key := PendingAPIKey{
			SecretToken: "token-123",
			APIKey:      "key-123",
			UserEmail:   "user@example.com",
			CreatedBy:   "admin@example.com",
			CreatedAt:   now,
			ExpiresAt:   now.Unix() + 3600,
			Viewed:      false,
		}

		data, err := json.Marshal(key)
		require.NoError(t, err)

		var unmarshaled PendingAPIKey
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, key.SecretToken, unmarshaled.SecretToken)
		assert.Equal(t, key.Viewed, unmarshaled.Viewed)
		assert.Nil(t, unmarshaled.ViewedAt)
	})
}

func TestClaimAPIKeyResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := ClaimAPIKeyResponse{
			APIKey:    "key-123",
			UserEmail: "user@example.com",
			Message:   "API key claimed successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ClaimAPIKeyResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.APIKey, unmarshaled.APIKey)
		assert.Equal(t, resp.UserEmail, unmarshaled.UserEmail)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestRevokeUserRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		req := RevokeUserRequest{
			Email: "user@example.com",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled RevokeUserRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Email, unmarshaled.Email)
	})
}

func TestRevokeUserResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := RevokeUserResponse{
			Message: "User revoked successfully",
			Email:   "user@example.com",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled RevokeUserResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Message, unmarshaled.Message)
		assert.Equal(t, resp.Email, unmarshaled.Email)
	})
}

func TestListUsersResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now()
		resp := ListUsersResponse{
			Users: []*User{
				{
					Email:     "user1@example.com",
					CreatedAt: now,
					Revoked:   false,
				},
				{
					Email:     "user2@example.com",
					CreatedAt: now,
					Revoked:   true,
				},
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ListUsersResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Users, 2)
		assert.Equal(t, resp.Users[0].Email, unmarshaled.Users[0].Email)
		assert.Equal(t, resp.Users[1].Email, unmarshaled.Users[1].Email)
	})
}
