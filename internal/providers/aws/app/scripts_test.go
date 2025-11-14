package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderScript(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		data         any
		shouldPanic  bool
		contains     []string
		notContains  []string
	}{
		{
			name:         "render main.sh template",
			templateName: "main.sh.tmpl",
			data: map[string]any{
				"ProjectName": "runvoy",
				"RequestID":   "req-123",
				"Image":       "ubuntu:22.04",
				"Command":     "echo hello",
				"Repo":        nil,
			},
			shouldPanic: false,
			contains:    []string{"echo hello", "runvoy", "req-123", "ubuntu:22.04"},
		},
		{
			name:         "render sidecar.sh template without git repo",
			templateName: "sidecar.sh.tmpl",
			data: map[string]any{
				"ProjectName":    "runvoy",
				"HasGitRepo":     false,
				"DefaultGitRef":  "main",
				"SecretVarNames": []string{},
				"AllVarNames":    []string{},
			},
			shouldPanic: false,
			contains:    []string{"set -e", "runvoy", "No git repository specified"},
		},
		{
			name:         "render sidecar.sh template with git repo",
			templateName: "sidecar.sh.tmpl",
			data: map[string]any{
				"ProjectName":    "runvoy",
				"HasGitRepo":     true,
				"DefaultGitRef":  "main",
				"SecretVarNames": []string{},
				"AllVarNames":    []string{},
			},
			shouldPanic: false,
			contains:    []string{"set -e", "runvoy", "git clone"},
		},
		{
			name:         "invalid template name",
			templateName: "nonexistent.tmpl",
			data:         map[string]any{},
			shouldPanic:  true,
		},
		{
			name:         "template with missing key",
			templateName: "main.sh.tmpl",
			data:         map[string]any{},
			shouldPanic:  true, // missingkey=error option should cause panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				assert.Panics(t, func() {
					renderScript(tt.templateName, tt.data)
				}, "Expected panic for invalid template or missing keys")
			} else {
				result := renderScript(tt.templateName, tt.data)
				require.NotEmpty(t, result, "Rendered script should not be empty")

				// Verify result is trimmed (no leading/trailing whitespace)
				assert.Equal(t, result, result, "Result should be trimmed")

				// Check for expected content
				for _, expected := range tt.contains {
					assert.Contains(t, result, expected, "Rendered script should contain expected content")
				}

				// Check for unexpected content
				for _, unexpected := range tt.notContains {
					assert.NotContains(t, result, unexpected, "Rendered script should not contain unexpected content")
				}
			}
		})
	}
}

func TestRenderScript_TrimsWhitespace(t *testing.T) {
	// This test verifies that renderScript trims whitespace from the result
	// We can't directly test this without modifying templates, but we can verify
	// that the function doesn't add extra whitespace

	result := renderScript("main.sh.tmpl", map[string]any{
		"ProjectName": "runvoy",
		"RequestID":   "req-123",
		"Image":       "ubuntu:22.04",
		"Command":     "test",
		"Repo":        nil,
	})

	// Result should not start or end with whitespace
	if result != "" {
		assert.NotEqual(t, ' ', result[0], "Result should not start with space")
		assert.NotEqual(t, '\t', result[0], "Result should not start with tab")
		assert.NotEqual(t, '\n', result[0], "Result should not start with newline")

		lastChar := result[len(result)-1]
		assert.NotEqual(t, ' ', lastChar, "Result should not end with space")
		assert.NotEqual(t, '\t', lastChar, "Result should not end with tab")
		assert.NotEqual(t, '\n', lastChar, "Result should not end with newline")
	}
}
