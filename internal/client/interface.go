// Package client provides HTTP client functionality for the runvoy API.
package client

import (
	"context"

	"runvoy/internal/api"
)

// Interface defines the API client interface for dependency injection and testing
type Interface interface {
	// Health
	ReconcileHealth(ctx context.Context) (*api.HealthReconcileResponse, error)
	GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error)
	GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
	RunCommand(ctx context.Context, req *api.ExecutionRequest) (*api.ExecutionResponse, error)
	KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error)
	ListExecutions(ctx context.Context, limit int, statuses string) ([]api.Execution, error)
	ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error)
	CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error)
	RevokeUser(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error)
	ListUsers(ctx context.Context) (*api.ListUsersResponse, error)
	RegisterImage(
		ctx context.Context,
		image string,
		isDefault *bool,
		taskRoleName, taskExecutionRoleName *string,
		cpu, memory *int,
		runtimePlatform *string,
	) (*api.RegisterImageResponse, error)
	ListImages(ctx context.Context) (*api.ListImagesResponse, error)
	GetImage(ctx context.Context, image string) (*api.ImageInfo, error)
	UnregisterImage(ctx context.Context, image string) (*api.RemoveImageResponse, error)
	CreateSecret(ctx context.Context, req api.CreateSecretRequest) (*api.CreateSecretResponse, error)
	GetSecret(ctx context.Context, name string) (*api.GetSecretResponse, error)
	ListSecrets(ctx context.Context) (*api.ListSecretsResponse, error)
	UpdateSecret(ctx context.Context, name string, req api.UpdateSecretRequest) (*api.UpdateSecretResponse, error)
	DeleteSecret(ctx context.Context, name string) (*api.DeleteSecretResponse, error)
}

// Compile-time check to ensure Client implements Interface
var _ Interface = (*Client)(nil)
