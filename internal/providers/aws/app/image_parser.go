// Package aws provides AWS-specific implementations for runvoy.
// This file contains Docker image reference parsing utilities.
package aws

import (
	"strings"
)

const (
	dockerHubRegistry = "docker.io"
)

// ImageReference represents a parsed Docker image reference.
type ImageReference struct {
	// Full is the complete image reference as provided
	Full string
	// Registry is the registry hostname (empty string = Docker Hub)
	// Examples: "", "123456.dkr.ecr.us-east-1.amazonaws.com", "gcr.io"
	Registry string
	// Name is the image name (may include org/repo structure)
	// Examples: "alpine", "hashicorp/terraform", "myapp"
	Name string
	// Tag is the image tag or digest
	// Examples: "latest", "1.6", "sha256:abc123..."
	Tag string
}

// ParseImageReference parses a Docker image reference into its components.
// Supports formats:
//   - alpine:latest → registry="", name="alpine", tag="latest"
//   - ubuntu → registry="", name="ubuntu", tag="latest"
//   - myorg/myapp:v1.0 → registry="", name="myorg/myapp", tag="v1.0"
//   - 123456.dkr.ecr.us-east-1.amazonaws.com/myapp:v1.0 →
//     registry="123456.dkr.ecr.us-east-1.amazonaws.com", name="myapp", tag="v1.0"
//   - gcr.io/project/image:tag → registry="gcr.io", name="project/image", tag="tag"
func ParseImageReference(image string) ImageReference {
	ref := ImageReference{
		Full: image,
		Tag:  "latest", // Default tag
	}

	// Split on '@' to handle digest references (e.g., image@sha256:...)
	var remainder string
	idx := strings.Index(image, "@")
	if idx != -1 {
		remainder = image[:idx]
		ref.Tag = image[idx+1:] // Everything after @ is the digest
	} else {
		remainder = image
		// Split on ':' to extract tag
		tagIdx := strings.LastIndex(remainder, ":")
		if tagIdx != -1 {
			// Check if this is a tag (not a port number in registry)
			// Port numbers appear before the first slash
			firstSlash := strings.Index(remainder, "/")
			if firstSlash == -1 || tagIdx > firstSlash {
				// This is a tag, not a port
				ref.Tag = remainder[tagIdx+1:]
				remainder = remainder[:tagIdx]
			}
		}
	}

	// Now remainder is registry/name or just name
	// A registry is identified by:
	// 1. Contains a dot (.) - e.g., gcr.io, ecr.amazonaws.com
	// 2. Contains a colon (:) - e.g., localhost:5000
	// 3. Is "localhost"

	const splitLimit = 2
	parts := strings.SplitN(remainder, "/", splitLimit)

	if len(parts) == 1 {
		// Just a name, no registry
		ref.Registry = ""
		ref.Name = parts[0]
	} else {
		// Check if first part is a registry
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") ||
			strings.Contains(firstPart, ":") ||
			firstPart == "localhost" {
			// This is a registry
			ref.Registry = firstPart
			ref.Name = parts[1]
		} else {
			// This is org/repo format (no registry)
			ref.Registry = ""
			ref.Name = remainder
		}
	}

	return ref
}

// NormalizeRegistry returns a normalized registry identifier.
// Returns "docker.io" for Docker Hub (empty registry), otherwise returns the registry as-is.
func (r ImageReference) NormalizeRegistry() string {
	if r.Registry == "" {
		return dockerHubRegistry
	}
	return r.Registry
}

// IsDockerHub returns true if the image is from Docker Hub.
func (r ImageReference) IsDockerHub() bool {
	return r.Registry == "" || r.Registry == dockerHubRegistry || r.Registry == "index.docker.io"
}

// IsECR returns true if the image is from AWS ECR.
func (r ImageReference) IsECR() bool {
	return strings.Contains(r.Registry, ".ecr.") && strings.Contains(r.Registry, ".amazonaws.com")
}

// ShortName returns the image name without registry or tag.
// Examples: "alpine", "hashicorp/terraform", "myapp"
func (r ImageReference) ShortName() string {
	return r.Name
}

// NameWithTag returns the image name with tag but without registry.
// Examples: "alpine:latest", "hashicorp/terraform:1.6"
func (r ImageReference) NameWithTag() string {
	if strings.HasPrefix(r.Tag, "sha256:") {
		return r.Name + "@" + r.Tag
	}
	return r.Name + ":" + r.Tag
}
