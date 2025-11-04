// Package client provides HTTP client functionality for the runvoy API.
package client

import (
	"context"

	"runvoy/internal/api"
)

// Interface defines the API client interface for dependency injection and testing
type Interface interface {
	GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error)
	GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
	RunCommand(ctx context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error)
	KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error)
	ListExecutions(ctx context.Context) ([]api.Execution, error)
	ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error)
	CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error)
	RevokeUser(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error)
	ListUsers(ctx context.Context) (*api.ListUsersResponse, error)
	RegisterImage(ctx context.Context, image string, isDefault *bool) (*api.RegisterImageResponse, error)
	ListImages(ctx context.Context) (*api.ListImagesResponse, error)
	UnregisterImage(ctx context.Context, image string) (*api.RemoveImageResponse, error)
}

// Compile-time check to ensure Client implements Interface
var _ Interface = (*Client)(nil)
