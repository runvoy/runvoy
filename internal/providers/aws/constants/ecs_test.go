package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportedRuntimePlatforms(t *testing.T) {
	platforms := SupportedRuntimePlatforms()

	expected := []string{
		DefaultRuntimePlatformOSFamily + "/" + RuntimePlatformArchX8664,
		DefaultRuntimePlatformOSFamily + "/" + RuntimePlatformArchARM64,
	}

	assert.Equal(t, expected, platforms)
	assert.Len(t, platforms, 2)
	assert.Contains(t, platforms, "Linux/X86_64")
	assert.Contains(t, platforms, "Linux/ARM64")
}
