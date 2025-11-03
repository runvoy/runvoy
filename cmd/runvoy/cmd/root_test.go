package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      time.Duration
		wantError bool
	}{
		{
			name:  "valid duration minutes",
			input: "10m",
			want:  10 * time.Minute,
		},
		{
			name:  "valid duration seconds",
			input: "30s",
			want:  30 * time.Second,
		},
		{
			name:  "valid duration hours",
			input: "1h",
			want:  time.Hour,
		},
		{
			name:  "valid seconds as integer",
			input: "600",
			want:  600 * time.Second,
		},
		{
			name:  "empty string defaults to 10m",
			input: "",
			want:  10 * time.Minute,
		},
		{
			name:      "invalid format",
			input:     "invalid",
			wantError: true,
		},
		{
			name:  "negative number (parsed as valid duration)",
			input: "-10",
			want:  -10 * time.Second,
		},
		{
			name:  "zero seconds",
			input: "0",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeout(tt.input)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
