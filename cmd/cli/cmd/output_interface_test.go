package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOutputWrapper(t *testing.T) {
	wrapper := NewOutputWrapper()
	assert.NotNil(t, wrapper)
}

func TestOutputWrapperImplementsInterface(_ *testing.T) {
	// This test verifies that outputWrapper properly implements OutputInterface
	var _ OutputInterface = &outputWrapper{}
	// NewOutputWrapper already returns OutputInterface, so type is inferred
	_ = NewOutputWrapper()
}

func TestOutputWrapper_Bold(t *testing.T) {
	wrapper := NewOutputWrapper()
	result := wrapper.Bold("test")
	// Bold should return a non-empty string containing the input
	assert.Contains(t, result, "test")
}

func TestOutputWrapper_Cyan(t *testing.T) {
	wrapper := NewOutputWrapper()
	result := wrapper.Cyan("test")
	// Cyan should return a non-empty string containing the input
	assert.Contains(t, result, "test")
}
