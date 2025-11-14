// Package api defines the API types and structures used across runvoy.
package api

// RegisterImageRequest represents the request to register a new Docker image
type RegisterImageRequest struct {
	Image                string  `json:"image"`
	IsDefault            *bool   `json:"is_default,omitempty"`
	TaskRoleName         *string `json:"task_role_name,omitempty"`
	TaskExecutionRoleName *string `json:"task_execution_role_name,omitempty"`
}

// RegisterImageResponse represents the response after registering an image
type RegisterImageResponse struct {
	Image   string `json:"image"`
	Message string `json:"message"`
}

// RemoveImageRequest represents the request to remove a Docker image
type RemoveImageRequest struct {
	Image string `json:"image"`
}

// RemoveImageResponse represents the response after removing an image
type RemoveImageResponse struct {
	Image   string `json:"image"`
	Message string `json:"message"`
}

// ImageInfo represents information about a registered image
type ImageInfo struct {
	Image                 string  `json:"image"`
	TaskDefinitionARN     string  `json:"task_definition_arn,omitempty"`
	TaskDefinitionName    string  `json:"task_definition_name,omitempty"`
	IsDefault             *bool   `json:"is_default,omitempty"`
	TaskRoleName          *string `json:"task_role_name,omitempty"`
	TaskExecutionRoleName *string `json:"task_execution_role_name,omitempty"`
	// Parsed image components
	ImageRegistry         string  `json:"image_registry,omitempty"`    // Empty string = Docker Hub
	ImageName             string  `json:"image_name,omitempty"`        // e.g., "alpine", "hashicorp/terraform"
	ImageTag              string  `json:"image_tag,omitempty"`         // e.g., "latest", "1.6"
}

// ListImagesResponse represents the response containing all registered images
type ListImagesResponse struct {
	Images []ImageInfo `json:"images"`
}
