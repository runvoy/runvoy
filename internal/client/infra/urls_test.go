package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildLogsURL(t *testing.T) {
	tests := []struct {
		name        string
		webURL      string
		executionID string
		want        string
	}{
		{
			name:        "standard HTTPS URL",
			webURL:      "https://example.com",
			executionID: "exec-123",
			want:        "https://example.com/logs?execution_id=exec-123",
		},
		{
			name:        "URL with trailing slash",
			webURL:      "https://example.com/",
			executionID: "exec-123",
			want:        "https://example.com/logs?execution_id=exec-123",
		},
		{
			name:        "URL with existing path",
			webURL:      "https://example.com/app",
			executionID: "exec-123",
			want:        "https://example.com/app/logs?execution_id=exec-123",
		},
		{
			name:        "URL with existing path and trailing slash",
			webURL:      "https://example.com/app/",
			executionID: "exec-123",
			want:        "https://example.com/app/logs?execution_id=exec-123",
		},
		{
			name:        "URL with port",
			webURL:      "https://example.com:8080",
			executionID: "exec-123",
			want:        "https://example.com:8080/logs?execution_id=exec-123",
		},
		{
			name:        "URL with query parameters",
			webURL:      "https://example.com?foo=bar",
			executionID: "exec-123",
			want:        "https://example.com/logs?execution_id=exec-123",
		},
		{
			name:        "execution ID with special characters",
			webURL:      "https://example.com",
			executionID: "exec-123?foo=bar&baz=qux",
			want:        "https://example.com/logs?execution_id=exec-123%3Ffoo%3Dbar%26baz%3Dqux",
		},
		{
			name:        "execution ID with spaces",
			webURL:      "https://example.com",
			executionID: "exec 123",
			want:        "https://example.com/logs?execution_id=exec+123",
		},
		{
			name:        "HTTP URL",
			webURL:      "http://localhost:3000",
			executionID: "exec-123",
			want:        "http://localhost:3000/logs?execution_id=exec-123",
		},
		{
			name:        "URL with subdomain",
			webURL:      "https://app.example.com",
			executionID: "exec-123",
			want:        "https://app.example.com/logs?execution_id=exec-123",
		},
		{
			name:        "empty execution ID",
			webURL:      "https://example.com",
			executionID: "",
			want:        "https://example.com/logs?execution_id=",
		},
		{
			name:        "URL with fragment",
			webURL:      "https://example.com#section",
			executionID: "exec-123",
			want:        "https://example.com/logs?execution_id=exec-123#section",
		},
		{
			name:        "complex execution ID",
			webURL:      "https://example.com",
			executionID: "exec-123-abc-def-456",
			want:        "https://example.com/logs?execution_id=exec-123-abc-def-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildLogsURL(tt.webURL, tt.executionID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildLogsURL_FallbackBehavior(t *testing.T) {
	tests := []struct {
		name        string
		webURL      string
		executionID string
		description string
	}{
		{
			name:        "invalid URL format",
			webURL:      "not-a-valid-url",
			executionID: "exec-123",
			description: "should fallback to string concatenation for invalid URLs",
		},
		{
			name:        "empty URL",
			webURL:      "",
			executionID: "exec-123",
			description: "should handle empty URL gracefully with fallback",
		},
		{
			name:        "URL with invalid characters",
			webURL:      "https://example.com/path with spaces",
			executionID: "exec-123",
			description: "should handle URLs with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildLogsURL(tt.webURL, tt.executionID)
			// The fallback should still produce a URL-like string
			assert.NotEmpty(t, got)
			// For empty URL, the fallback produces "logs?execution_id=..." without leading slash
			if tt.webURL != "" {
				assert.Contains(t, got, "/logs")
			} else {
				assert.Contains(t, got, "logs")
			}
			assert.Contains(t, got, "execution_id")
			assert.Contains(t, got, tt.executionID)
		})
	}
}

func TestBuildLogsURL_EdgeCases(t *testing.T) {
	t.Run("very long execution ID", func(t *testing.T) {
		longID := "exec-" + string(make([]byte, 1000))
		got := BuildLogsURL("https://example.com", longID)
		assert.Contains(t, got, "/logs")
		assert.Contains(t, got, "execution_id")
	})

	t.Run("execution ID with unicode characters", func(t *testing.T) {
		got := BuildLogsURL("https://example.com", "exec-测试-123")
		assert.Contains(t, got, "/logs")
		assert.Contains(t, got, "execution_id")
		// The unicode characters should be properly encoded
		assert.NotContains(t, got, "测试")
	})

	t.Run("URL with multiple path segments", func(t *testing.T) {
		got := BuildLogsURL("https://example.com/api/v1", "exec-123")
		assert.Equal(t, "https://example.com/api/v1/logs?execution_id=exec-123", got)
	})

	t.Run("URL with encoded path", func(t *testing.T) {
		got := BuildLogsURL("https://example.com/path%20with%20spaces", "exec-123")
		assert.Contains(t, got, "/logs")
		assert.Contains(t, got, "execution_id=exec-123")
	})
}
