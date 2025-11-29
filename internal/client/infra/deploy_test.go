package infra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runvoy/runvoy/internal/constants"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeployer(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		region   string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "AWS provider lowercase",
			provider: "aws",
			region:   "us-east-1",
			wantErr:  false,
		},
		{
			name:     "AWS provider uppercase",
			provider: "AWS",
			region:   "us-west-2",
			wantErr:  false,
		},
		{
			name:     "unsupported provider",
			provider: "gcp",
			region:   "us-central1",
			wantErr:  true,
			errMsg:   "unsupported provider: gcp",
		},
		{
			name:     "empty provider",
			provider: "",
			region:   "us-east-1",
			wantErr:  true,
			errMsg:   "unsupported provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			deployer, err := NewDeployer(ctx, tt.provider, tt.region)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, deployer)
			} else {
				require.NoError(t, err)
				require.NotNil(t, deployer)
				assert.Equal(t, tt.region, deployer.GetRegion())
			}
		})
	}
}

func TestResolveTemplate(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		template  string
		version   string
		region    string
		wantURL   bool
		wantBody  bool
		wantErr   bool
		errMsg    string
		checkFunc func(*testing.T, *TemplateSource)
	}{
		{
			name:     "AWS default template",
			provider: "aws",
			template: "",
			version:  "v1.0.0",
			region:   "us-east-1",
			wantURL:  true,
			wantBody: false,
			wantErr:  false,
			checkFunc: func(t *testing.T, ts *TemplateSource) {
				assert.NotEmpty(t, ts.URL)
				assert.Contains(t, ts.URL, "runvoy-releases-us-east-1")
				assert.Contains(t, ts.URL, "1.0.0") // version gets normalized (v prefix removed)
				assert.Empty(t, ts.Body)
			},
		},
		{
			name:     "AWS HTTPS URL",
			provider: "aws",
			template: "https://example.com/template.yaml",
			version:  "v1.0.0",
			region:   "us-west-2",
			wantURL:  true,
			wantBody: false,
			wantErr:  false,
			checkFunc: func(t *testing.T, ts *TemplateSource) {
				assert.Equal(t, "https://example.com/template.yaml", ts.URL)
				assert.Empty(t, ts.Body)
			},
		},
		{
			name:     "AWS HTTP URL",
			provider: "aws",
			template: "http://example.com/template.yaml",
			version:  "v1.0.0",
			region:   "us-west-2",
			wantURL:  true,
			wantBody: false,
			wantErr:  false,
			checkFunc: func(t *testing.T, ts *TemplateSource) {
				assert.Equal(t, "http://example.com/template.yaml", ts.URL)
				assert.Empty(t, ts.Body)
			},
		},
		{
			name:     "AWS S3 URI",
			provider: "aws",
			template: "s3://my-bucket/path/to/template.yaml",
			version:  "v1.0.0",
			region:   "us-east-1",
			wantURL:  true,
			wantBody: false,
			wantErr:  false,
			checkFunc: func(t *testing.T, ts *TemplateSource) {
				assert.Equal(t, "https://my-bucket.s3.amazonaws.com/path/to/template.yaml", ts.URL)
				assert.Empty(t, ts.Body)
			},
		},
		{
			name:     "AWS invalid S3 URI",
			provider: "aws",
			template: "s3://just-bucket",
			version:  "v1.0.0",
			region:   "us-east-1",
			wantErr:  true,
			errMsg:   "invalid S3 URI",
		},
		{
			name:     "unsupported provider",
			provider: "gcp",
			template: "",
			version:  "v1.0.0",
			region:   "us-central1",
			wantErr:  true,
			errMsg:   "unsupported provider: gcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveTemplate(tt.provider, tt.template, tt.version, tt.region)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.checkFunc != nil {
					tt.checkFunc(t, result)
				}
			}
		})
	}
}

func TestResolveTemplate_LocalFile(t *testing.T) {
	t.Run("local file template", func(t *testing.T) {
		// Create a temporary file
		tmpDir := t.TempDir()
		templatePath := filepath.Join(tmpDir, "template.yaml")
		templateContent := "AWSTemplateFormatVersion: '2010-09-09'\nDescription: Test template"

		err := os.WriteFile(templatePath, []byte(templateContent), 0600)
		require.NoError(t, err)

		result, err := ResolveTemplate(string(constants.AWS), templatePath, "v1.0.0", "us-east-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.URL)
		assert.Equal(t, templateContent, result.Body)
	})

	t.Run("local file not found", func(t *testing.T) {
		result, err := ResolveTemplate(string(constants.AWS), "/nonexistent/template.yaml", "v1.0.0", "us-east-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read template file")
		assert.Nil(t, result)
	})
}

func TestParseParameters(t *testing.T) {
	tests := []struct {
		name    string
		params  []string
		want    map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid parameters",
			params: []string{
				"Key1=Value1",
				"Key2=Value2",
				"Key3=Value3",
			},
			want: map[string]string{
				"Key1": "Value1",
				"Key2": "Value2",
				"Key3": "Value3",
			},
			wantErr: false,
		},
		{
			name: "parameter with equals in value",
			params: []string{
				"Key1=Value=WithEquals",
			},
			want: map[string]string{
				"Key1": "Value=WithEquals",
			},
			wantErr: false,
		},
		{
			name:    "empty parameters",
			params:  []string{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "invalid parameter format - no equals",
			params: []string{
				"InvalidParameter",
			},
			wantErr: true,
			errMsg:  "invalid parameter format",
		},
		{
			name: "invalid parameter format - only key",
			params: []string{
				"Key=",
			},
			want: map[string]string{
				"Key": "",
			},
			wantErr: false,
		},
		{
			name: "parameter with spaces in value",
			params: []string{
				"Key1=Value with spaces",
			},
			want: map[string]string{
				"Key1": "Value with spaces",
			},
			wantErr: false,
		},
		{
			name: "mixed valid and invalid parameters",
			params: []string{
				"Key1=Value1",
				"InvalidParameter",
			},
			wantErr: true,
			errMsg:  "invalid parameter format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseParameters(tt.params)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestResolveAWSTemplate(t *testing.T) {
	t.Run("default template URL construction", func(t *testing.T) {
		result, err := resolveAWSTemplate("", "v1.2.3", "us-west-2")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.URL)
		assert.Contains(t, result.URL, "runvoy-releases-us-west-2")
		assert.Contains(t, result.URL, "1.2.3") // version gets normalized (v prefix removed)
		assert.Empty(t, result.Body)
	})

	t.Run("HTTPS URL passthrough", func(t *testing.T) {
		url := "https://example.com/my-template.yaml"
		result, err := resolveAWSTemplate(url, "v1.0.0", "us-east-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, url, result.URL)
		assert.Empty(t, result.Body)
	})

	t.Run("S3 URI conversion", func(t *testing.T) {
		result, err := resolveAWSTemplate("s3://my-bucket/templates/stack.yaml", "v1.0.0", "us-east-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "https://my-bucket.s3.amazonaws.com/templates/stack.yaml", result.URL)
		assert.Empty(t, result.Body)
	})

	t.Run("S3 URI with nested path", func(t *testing.T) {
		result, err := resolveAWSTemplate("s3://bucket/path/to/deep/template.yaml", "v1.0.0", "us-east-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "https://bucket.s3.amazonaws.com/path/to/deep/template.yaml", result.URL)
		assert.Empty(t, result.Body)
	})

	t.Run("invalid S3 URI - bucket only", func(t *testing.T) {
		result, err := resolveAWSTemplate("s3://bucket-only", "v1.0.0", "us-east-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid S3 URI")
		assert.Nil(t, result)
	})

	t.Run("local file with relative path", func(t *testing.T) {
		tmpDir := t.TempDir()
		templatePath := filepath.Join(tmpDir, "template.yaml")
		templateContent := "Resources:\n  MyResource:\n    Type: AWS::S3::Bucket"

		err := os.WriteFile(templatePath, []byte(templateContent), 0600)
		require.NoError(t, err)

		result, err := resolveAWSTemplate(templatePath, "v1.0.0", "us-east-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.URL)
		assert.Equal(t, templateContent, result.Body)
	})

	t.Run("local file error - file does not exist", func(t *testing.T) {
		result, err := resolveAWSTemplate("/path/to/nonexistent.yaml", "v1.0.0", "us-east-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read template file")
		assert.Nil(t, result)
	})
}

func TestDeployOptions(t *testing.T) {
	t.Run("deploy options with all fields", func(t *testing.T) {
		opts := &DeployOptions{
			StackName:  "my-stack",
			Template:   "https://example.com/template.yaml",
			Version:    "v1.0.0",
			Parameters: []string{"Key1=Value1", "Key2=Value2"},
			Wait:       true,
			Region:     "us-east-1",
		}

		assert.Equal(t, "my-stack", opts.StackName)
		assert.Equal(t, "https://example.com/template.yaml", opts.Template)
		assert.Equal(t, "v1.0.0", opts.Version)
		assert.Len(t, opts.Parameters, 2)
		assert.True(t, opts.Wait)
		assert.Equal(t, "us-east-1", opts.Region)
	})
}

func TestDestroyOptions(t *testing.T) {
	t.Run("destroy options with all fields", func(t *testing.T) {
		opts := &DestroyOptions{
			StackName: "my-stack",
			Wait:      true,
			Region:    "us-west-2",
		}

		assert.Equal(t, "my-stack", opts.StackName)
		assert.True(t, opts.Wait)
		assert.Equal(t, "us-west-2", opts.Region)
	})
}

func TestTemplateSource(t *testing.T) {
	t.Run("template source with URL", func(t *testing.T) {
		ts := &TemplateSource{
			URL: "https://example.com/template.yaml",
		}

		assert.NotEmpty(t, ts.URL)
		assert.Empty(t, ts.Body)
	})

	t.Run("template source with body", func(t *testing.T) {
		ts := &TemplateSource{
			Body: "template content here",
		}

		assert.Empty(t, ts.URL)
		assert.NotEmpty(t, ts.Body)
	})
}

func TestDeployResult(t *testing.T) {
	t.Run("deploy result fields", func(t *testing.T) {
		result := &DeployResult{
			StackName:     "test-stack",
			OperationType: "CREATE",
			Status:        "CREATE_COMPLETE",
			Outputs: map[string]string{
				"ApiEndpoint": "https://api.example.com",
			},
			NoChanges: false,
		}

		assert.Equal(t, "test-stack", result.StackName)
		assert.Equal(t, "CREATE", result.OperationType)
		assert.Equal(t, "CREATE_COMPLETE", result.Status)
		assert.Len(t, result.Outputs, 1)
		assert.False(t, result.NoChanges)
	})

	t.Run("deploy result with no changes", func(t *testing.T) {
		result := &DeployResult{
			StackName:     "test-stack",
			OperationType: "UPDATE",
			Status:        "NO_CHANGES",
			Outputs:       map[string]string{},
			NoChanges:     true,
		}

		assert.True(t, result.NoChanges)
		assert.Equal(t, "NO_CHANGES", result.Status)
	})
}

func TestDestroyResult(t *testing.T) {
	t.Run("destroy result fields", func(t *testing.T) {
		result := &DestroyResult{
			StackName: "test-stack",
			Status:    "DELETE_COMPLETE",
			NotFound:  false,
		}

		assert.Equal(t, "test-stack", result.StackName)
		assert.Equal(t, "DELETE_COMPLETE", result.Status)
		assert.False(t, result.NotFound)
	})

	t.Run("destroy result for non-existent stack", func(t *testing.T) {
		result := &DestroyResult{
			StackName: "nonexistent-stack",
			Status:    "NOT_FOUND",
			NotFound:  true,
		}

		assert.True(t, result.NotFound)
		assert.Equal(t, "NOT_FOUND", result.Status)
	})
}

func TestProviderCaseInsensitive(t *testing.T) {
	testCases := []string{"aws", "AWS", "Aws", "aWs"}

	for _, provider := range testCases {
		t.Run("provider: "+provider, func(t *testing.T) {
			ctx := context.Background()
			deployer, err := NewDeployer(ctx, provider, "us-east-1")

			// Note: This might fail in test environments without AWS credentials
			// but we're mainly testing the case-insensitive matching logic
			if err != nil && !strings.Contains(err.Error(), "failed to load AWS configuration") {
				t.Errorf("Expected AWS configuration error or success, got: %v", err)
			}
			if err == nil {
				require.NotNil(t, deployer)
			}
		})
	}
}
