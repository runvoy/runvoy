package aws

import (
	"testing"
)

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name             string
		image            string
		expectedRegistry string
		expectedName     string
		expectedTag      string
	}{
		{
			name:             "Simple image with tag",
			image:            "alpine:latest",
			expectedRegistry: "",
			expectedName:     "alpine",
			expectedTag:      "latest",
		},
		{
			name:             "Simple image without tag (default to latest)",
			image:            "ubuntu",
			expectedRegistry: "",
			expectedName:     "ubuntu",
			expectedTag:      "latest",
		},
		{
			name:             "Org/repo with tag",
			image:            "hashicorp/terraform:1.6",
			expectedRegistry: "",
			expectedName:     "hashicorp/terraform",
			expectedTag:      "1.6",
		},
		{
			name:             "ECR image with tag",
			image:            "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp:v1.0.0",
			expectedRegistry: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expectedName:     "myapp",
			expectedTag:      "v1.0.0",
		},
		{
			name:             "ECR image without tag",
			image:            "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp",
			expectedRegistry: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			expectedName:     "myapp",
			expectedTag:      "latest",
		},
		{
			name:             "GCR image",
			image:            "gcr.io/project-id/image-name:tag123",
			expectedRegistry: "gcr.io",
			expectedName:     "project-id/image-name",
			expectedTag:      "tag123",
		},
		{
			name:             "Docker Hub official with registry prefix",
			image:            "docker.io/library/alpine:3.18",
			expectedRegistry: "docker.io",
			expectedName:     "library/alpine",
			expectedTag:      "3.18",
		},
		{
			name:             "Localhost registry with port",
			image:            "localhost:5000/myimage:dev",
			expectedRegistry: "localhost:5000",
			expectedName:     "myimage",
			expectedTag:      "dev",
		},
		{
			name:             "Image with digest",
			image:            "alpine@sha256:abc123def456",
			expectedRegistry: "",
			expectedName:     "alpine",
			expectedTag:      "sha256:abc123def456",
		},
		{
			name:             "ECR with digest",
			image:            "123456.dkr.ecr.us-east-1.amazonaws.com/app@sha256:xyz789",
			expectedRegistry: "123456.dkr.ecr.us-east-1.amazonaws.com",
			expectedName:     "app",
			expectedTag:      "sha256:xyz789",
		},
		{
			name:             "Private registry with nested path",
			image:            "registry.example.com/team/project/app:v2.0",
			expectedRegistry: "registry.example.com",
			expectedName:     "team/project/app",
			expectedTag:      "v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := ParseImageReference(tt.image)

			if ref.Full != tt.image {
				t.Errorf("Full: got %q, want %q", ref.Full, tt.image)
			}
			if ref.Registry != tt.expectedRegistry {
				t.Errorf("Registry: got %q, want %q", ref.Registry, tt.expectedRegistry)
			}
			if ref.Name != tt.expectedName {
				t.Errorf("Name: got %q, want %q", ref.Name, tt.expectedName)
			}
			if ref.Tag != tt.expectedTag {
				t.Errorf("Tag: got %q, want %q", ref.Tag, tt.expectedTag)
			}
		})
	}
}

func TestImageReferenceHelpers(t *testing.T) {
	t.Run("IsDockerHub", func(t *testing.T) {
		tests := []struct {
			image    string
			expected bool
		}{
			{"alpine:latest", true},
			{"hashicorp/terraform:1.6", true},
			{"docker.io/library/alpine:latest", true},
			{"index.docker.io/library/alpine:latest", true},
			{"gcr.io/project/app:v1", false},
			{"123456.dkr.ecr.us-east-1.amazonaws.com/app:v1", false},
		}

		for _, tt := range tests {
			ref := ParseImageReference(tt.image)
			if got := ref.IsDockerHub(); got != tt.expected {
				t.Errorf("IsDockerHub(%q) = %v, want %v", tt.image, got, tt.expected)
			}
		}
	})

	t.Run("IsECR", func(t *testing.T) {
		tests := []struct {
			image    string
			expected bool
		}{
			{"alpine:latest", false},
			{"gcr.io/project/app:v1", false},
			{"123456.dkr.ecr.us-east-1.amazonaws.com/app:v1", true},
			{"987654.dkr.ecr.eu-west-1.amazonaws.com/myapp:latest", true},
		}

		for _, tt := range tests {
			ref := ParseImageReference(tt.image)
			if got := ref.IsECR(); got != tt.expected {
				t.Errorf("IsECR(%q) = %v, want %v", tt.image, got, tt.expected)
			}
		}
	})

	t.Run("NormalizeRegistry", func(t *testing.T) {
		tests := []struct {
			image    string
			expected string
		}{
			{"alpine:latest", "docker.io"},
			{"hashicorp/terraform:1.6", "docker.io"},
			{"gcr.io/project/app:v1", "gcr.io"},
			{"123456.dkr.ecr.us-east-1.amazonaws.com/app:v1", "123456.dkr.ecr.us-east-1.amazonaws.com"},
		}

		for _, tt := range tests {
			ref := ParseImageReference(tt.image)
			if got := ref.NormalizeRegistry(); got != tt.expected {
				t.Errorf("NormalizeRegistry(%q) = %v, want %v", tt.image, got, tt.expected)
			}
		}
	})

	t.Run("NameWithTag", func(t *testing.T) {
		tests := []struct {
			image    string
			expected string
		}{
			{"alpine:latest", "alpine:latest"},
			{"hashicorp/terraform:1.6", "hashicorp/terraform:1.6"},
			{"alpine@sha256:abc123", "alpine@sha256:abc123"},
			{"gcr.io/project/app:v1", "project/app:v1"},
		}

		for _, tt := range tests {
			ref := ParseImageReference(tt.image)
			if got := ref.NameWithTag(); got != tt.expected {
				t.Errorf("NameWithTag(%q) = %v, want %v", tt.image, got, tt.expected)
			}
		}
	})
}
