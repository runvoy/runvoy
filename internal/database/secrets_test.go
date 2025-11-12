// Package database defines the repository interfaces for data persistence.
package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretsErrors(t *testing.T) {
	t.Run("ErrSecretNotFound", func(t *testing.T) {
		err := ErrSecretNotFound
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "secret not found")
	})

	t.Run("ErrSecretAlreadyExists", func(t *testing.T) {
		err := ErrSecretAlreadyExists
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "secret already exists")
	})
}
