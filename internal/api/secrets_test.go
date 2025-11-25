package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretJSON(t *testing.T) {
	t.Run("marshal and unmarshal with value", func(t *testing.T) {
		now := time.Now()
		secret := Secret{
			Name:        "db-password",
			KeyName:     "DATABASE_PASSWORD",
			Description: "Database password for production",
			Value:       "super-secret",
			CreatedBy:   "admin@example.com",
			CreatedAt:   now,
			UpdatedAt:   now,
			UpdatedBy:   "admin@example.com",
		}

		data, err := json.Marshal(secret)
		require.NoError(t, err)

		var unmarshaled Secret
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, secret.Name, unmarshaled.Name)
		assert.Equal(t, secret.KeyName, unmarshaled.KeyName)
		assert.Equal(t, secret.Description, unmarshaled.Description)
		assert.Equal(t, secret.Value, unmarshaled.Value)
		assert.Equal(t, secret.CreatedBy, unmarshaled.CreatedBy)
		assert.Equal(t, secret.UpdatedBy, unmarshaled.UpdatedBy)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		now := time.Now()
		secret := Secret{
			Name:      "db-password",
			KeyName:   "DATABASE_PASSWORD",
			CreatedBy: "admin@example.com",
			CreatedAt: now,
			UpdatedAt: now,
			UpdatedBy: "admin@example.com",
		}

		data, err := json.Marshal(secret)
		require.NoError(t, err)

		// Description and Value should be omitted when empty
		jsonStr := string(data)
		assert.NotContains(t, jsonStr, "description")
		assert.NotContains(t, jsonStr, "value")
	})
}

func TestCreateSecretRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		req := CreateSecretRequest{
			Name:        "db-password",
			KeyName:     "DATABASE_PASSWORD",
			Description: "Database password",
			Value:       "super-secret",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CreateSecretRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Name, unmarshaled.Name)
		assert.Equal(t, req.KeyName, unmarshaled.KeyName)
		assert.Equal(t, req.Description, unmarshaled.Description)
		assert.Equal(t, req.Value, unmarshaled.Value)
	})

	t.Run("omit optional description", func(t *testing.T) {
		req := CreateSecretRequest{
			Name:    "db-password",
			KeyName: "DATABASE_PASSWORD",
			Value:   "super-secret",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled CreateSecretRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Name, unmarshaled.Name)
		assert.Empty(t, unmarshaled.Description)
	})
}

func TestCreateSecretResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := CreateSecretResponse{
			Message: "Secret created successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled CreateSecretResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestUpdateSecretRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with all fields", func(t *testing.T) {
		req := UpdateSecretRequest{
			Description: "Updated description",
			KeyName:     "UPDATED_KEY",
			Value:       "new-value",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled UpdateSecretRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Description, unmarshaled.Description)
		assert.Equal(t, req.KeyName, unmarshaled.KeyName)
		assert.Equal(t, req.Value, unmarshaled.Value)
	})

	t.Run("omit optional fields", func(t *testing.T) {
		req := UpdateSecretRequest{
			Description: "Updated description",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		jsonStr := string(data)
		assert.Contains(t, jsonStr, "description")
		assert.NotContains(t, jsonStr, "key_name")
		assert.NotContains(t, jsonStr, "value")
	})
}

func TestUpdateSecretResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := UpdateSecretResponse{
			Message: "Secret updated successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled UpdateSecretResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}

func TestGetSecretRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		req := GetSecretRequest{
			Name: "db-password",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled GetSecretRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Name, unmarshaled.Name)
	})
}

func TestGetSecretResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now()
		resp := GetSecretResponse{
			Secret: &Secret{
				Name:      "db-password",
				KeyName:   "DATABASE_PASSWORD",
				Value:     "super-secret",
				CreatedBy: "admin@example.com",
				CreatedAt: now,
				UpdatedAt: now,
				UpdatedBy: "admin@example.com",
			},
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled GetSecretResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.NotNil(t, unmarshaled.Secret)
		assert.Equal(t, resp.Secret.Name, unmarshaled.Secret.Name)
		assert.Equal(t, resp.Secret.Value, unmarshaled.Secret.Value)
	})
}

func TestListSecretsRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal with filter", func(t *testing.T) {
		req := ListSecretsRequest{
			CreatedBy: "user@example.com",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled ListSecretsRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.CreatedBy, unmarshaled.CreatedBy)
	})
}

func TestListSecretsResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		now := time.Now()
		resp := ListSecretsResponse{
			Secrets: []*Secret{
				{
					Name:      "secret1",
					KeyName:   "KEY1",
					CreatedBy: "user@example.com",
					CreatedAt: now,
					UpdatedAt: now,
					UpdatedBy: "user@example.com",
				},
				{
					Name:      "secret2",
					KeyName:   "KEY2",
					CreatedBy: "user@example.com",
					CreatedAt: now,
					UpdatedAt: now,
					UpdatedBy: "user@example.com",
				},
			},
			Total: 2,
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ListSecretsResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Len(t, unmarshaled.Secrets, 2)
		assert.Equal(t, resp.Total, unmarshaled.Total)
		assert.Equal(t, resp.Secrets[0].Name, unmarshaled.Secrets[0].Name)
	})
}

func TestDeleteSecretRequestJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		req := DeleteSecretRequest{
			Name: "db-password",
		}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var unmarshaled DeleteSecretRequest
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, req.Name, unmarshaled.Name)
	})
}

func TestDeleteSecretResponseJSON(t *testing.T) {
	t.Run("marshal and unmarshal", func(t *testing.T) {
		resp := DeleteSecretResponse{
			Name:    "db-password",
			Message: "Secret deleted successfully",
		}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled DeleteSecretResponse
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Name, unmarshaled.Name)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}
