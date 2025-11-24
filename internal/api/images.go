package api

import (
	"time"
)

// RegisterImageRequest represents the request to register a new Docker image
type RegisterImageRequest struct {
	Image                 string  `json:"image"`
	IsDefault             *bool   `json:"is_default,omitempty"`
	TaskRoleName          *string `json:"task_role_name,omitempty"`
	TaskExecutionRoleName *string `json:"task_execution_role_name,omitempty"`
	CPU                   *int    `json:"cpu,omitempty"`
	Memory                *int    `json:"memory,omitempty"`
	RuntimePlatform       *string `json:"runtime_platform,omitempty"`
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
	ImageID               string    `json:"image_id"`
	Image                 string    `json:"image"`
	TaskDefinitionName    string    `json:"task_definition_name,omitempty"`
	IsDefault             *bool     `json:"is_default,omitempty"`
	TaskRoleName          *string   `json:"task_role_name,omitempty"`
	TaskExecutionRoleName *string   `json:"task_execution_role_name,omitempty"`
	CPU                   int       `json:"cpu,omitempty"`
	Memory                int       `json:"memory,omitempty"`
	RuntimePlatform       string    `json:"runtime_platform,omitempty"`
	ImageRegistry         string    `json:"image_registry,omitempty"`
	ImageName             string    `json:"image_name,omitempty"`
	ImageTag              string    `json:"image_tag,omitempty"`
	CreatedBy             string    `json:"created_by,omitempty"`
	OwnedBy               []string  `json:"owned_by"`
	CreatedAt             time.Time `json:"created_at"`
	CreatedByRequestID    string    `json:"created_by_request_id"`
	ModifiedByRequestID   string    `json:"modified_by_request_id"`
}

// ListImagesResponse represents the response containing all registered images
type ListImagesResponse struct {
	Images []ImageInfo `json:"images"`
}
