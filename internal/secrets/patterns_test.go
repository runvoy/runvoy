package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSecretVariableNames(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected []string
	}{
		{
			name:     "empty environment",
			env:      map[string]string{},
			expected: []string{},
		},
		{
			name: "no secrets",
			env: map[string]string{
				"PATH":   "/usr/bin",
				"HOME":   "/home/user",
				"MY_VAR": "value",
			},
			expected: []string{},
		},
		{
			name: "detects GITHUB_TOKEN",
			env: map[string]string{
				"GITHUB_TOKEN": "ghp_token123",
			},
			expected: []string{"GITHUB_TOKEN"},
		},
		{
			name: "detects multiple secret patterns",
			env: map[string]string{
				"API_KEY":      "key123",
				"DB_PASSWORD":  "secret",
				"ACCESS_TOKEN": "tok",
			},
			expected: []string{"API_KEY", "DB_PASSWORD", "ACCESS_TOKEN"},
		},
		{
			name: "case insensitive detection",
			env: map[string]string{
				"my_api_key":    "key123",
				"My_Password":   "secret",
				"GITHUB_secret": "sec",
			},
			expected: []string{"my_api_key", "My_Password", "GITHUB_secret"},
		},
		{
			name: "detects all default patterns",
			env: map[string]string{
				"GITHUB_SECRET": "secret1",
				"GITHUB_TOKEN":  "token1",
				"MY_SECRET":     "secret2",
				"AUTH_TOKEN":    "token2",
				"DB_PASSWORD":   "pass",
				"API_KEY":       "key1",
				"API_SECRET":    "secret3",
				"PRIVATE_KEY":   "pk",
				"ACCESS_KEY":    "ak",
				"SECRET_KEY":    "sk",
			},
			expected: []string{
				"GITHUB_SECRET",
				"GITHUB_TOKEN",
				"MY_SECRET",
				"AUTH_TOKEN",
				"DB_PASSWORD",
				"API_KEY",
				"API_SECRET",
				"PRIVATE_KEY",
				"ACCESS_KEY",
				"SECRET_KEY",
			},
		},
		{
			name: "mixed environment with secrets and non-secrets",
			env: map[string]string{
				"PATH":        "/usr/bin",
				"API_KEY":     "key123",
				"DEBUG":       "true",
				"DB_PASSWORD": "secret",
				"LOG_LEVEL":   "info",
			},
			expected: []string{"API_KEY", "DB_PASSWORD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSecretVariableNames(tt.env)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestMergeSecretVarNames(t *testing.T) {
	tests := []struct {
		name     string
		known    []string
		detected []string
		expected []string
	}{
		{
			name:     "both empty",
			known:    []string{},
			detected: []string{},
			expected: []string{},
		},
		{
			name:     "only known",
			known:    []string{"KEY1", "KEY2"},
			detected: []string{},
			expected: []string{"KEY1", "KEY2"},
		},
		{
			name:     "only detected",
			known:    []string{},
			detected: []string{"KEY1", "KEY2"},
			expected: []string{"KEY1", "KEY2"},
		},
		{
			name:     "merge without duplicates",
			known:    []string{"KEY1", "KEY2"},
			detected: []string{"KEY3", "KEY4"},
			expected: []string{"KEY1", "KEY2", "KEY3", "KEY4"},
		},
		{
			name:     "merge with duplicates",
			known:    []string{"KEY1", "KEY2"},
			detected: []string{"KEY2", "KEY3"},
			expected: []string{"KEY1", "KEY2", "KEY3"},
		},
		{
			name:     "all duplicates",
			known:    []string{"KEY1", "KEY2"},
			detected: []string{"KEY1", "KEY2"},
			expected: []string{"KEY1", "KEY2"},
		},
		{
			name:     "preserves order from known first",
			known:    []string{"A", "B"},
			detected: []string{"C", "A"},
			expected: []string{"A", "B", "C"},
		},
		{
			name:     "nil known",
			known:    nil,
			detected: []string{"KEY1"},
			expected: []string{"KEY1"},
		},
		{
			name:     "nil detected",
			known:    []string{"KEY1"},
			detected: nil,
			expected: []string{"KEY1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeSecretVarNames(tt.known, tt.detected)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultSecretPatterns(t *testing.T) {
	// Verify all expected default patterns are present
	expectedPatterns := []string{
		"GITHUB_SECRET",
		"GITHUB_TOKEN",
		"SECRET",
		"TOKEN",
		"PASSWORD",
		"API_KEY",
		"API_SECRET",
		"PRIVATE_KEY",
		"ACCESS_KEY",
		"SECRET_KEY",
	}

	assert.ElementsMatch(t, expectedPatterns, DefaultSecretPatterns)
}
