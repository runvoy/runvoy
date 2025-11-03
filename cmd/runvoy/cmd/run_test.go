package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractUserEnvVars(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want map[string]string
	}{
		{
			name: "extracts prefixed variables",
			env: []string{
				"RUNVOY_USER_API_KEY=abc123",
				"RUNVOY_USER_TOKEN=xyz789",
				"PATH=/usr/bin",
			},
			want: map[string]string{
				"API_KEY": "abc123",
				"TOKEN":   "xyz789",
			},
		},
		{
			name: "ignores entries without separator",
			env: []string{
				"RUNVOY_USER_INVALID",
				"HOME=/home/user",
			},
			want: map[string]string{},
		},
		{
			name: "returns empty map when no matches",
			env: []string{
				"HOME=/home/user",
				"PATH=/usr/bin",
			},
			want: map[string]string{},
		},
		{
			name: "last value wins for duplicate keys",
			env: []string{
				"RUNVOY_USER_API_KEY=old",
				"RUNVOY_USER_API_KEY=new",
			},
			want: map[string]string{
				"API_KEY": "new",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserEnvVars(tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}
