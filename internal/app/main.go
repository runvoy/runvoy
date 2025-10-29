package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/database"
	apperrors "runvoy/internal/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type Service struct {
	userRepo      database.UserRepository
	executionRepo database.ExecutionRepository
	ecsClient     *ecs.Client
	cfg           *ServiceConfig
	Logger        *slog.Logger
}

// ServiceConfig holds configuration for the service
type ServiceConfig struct {
	ECSCluster      string
	TaskDefinition  string
	Subnet1         string
	Subnet2         string
	SecurityGroup   string
	LogGroup        string
	DefaultImage    string
	TaskRoleARN     string
	TaskExecRoleARN string
}

// NewService creates a new service instance.
// If userRepo is nil, user-related operations will not be available.
// This allows the service to work without database dependencies for simple operations.
func NewService(userRepo database.UserRepository, executionRepo database.ExecutionRepository, ecsClient *ecs.Client, cfg *ServiceConfig, logger *slog.Logger) *Service {
	return &Service{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		ecsClient:     ecsClient,
		cfg:           cfg,
		Logger:        logger,
	}
}

// CreateUser creates a new user with an API key.
// If no API key is provided in the request, one will be generated.
// The API key is only returned in the response and should be stored by the client.
func (s *Service) CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if req.Email == "" {
		return nil, apperrors.ErrBadRequest("email is required", nil)
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, apperrors.ErrBadRequest("invalid email address", err)
	}

	existingUser, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, apperrors.ErrConflict("user with this email already exists", nil)
	}

	apiKey := req.APIKey
	if apiKey == "" {
		apiKey, err = generateAPIKey()
		if err != nil {
			return nil, apperrors.ErrInternalError("failed to generate API key", err)
		}
	}

	apiKeyHash := hashAPIKey(apiKey)

	user := &api.User{
		Email:     req.Email,
		CreatedAt: time.Now().UTC(),
		Revoked:   false,
	}

	if err := s.userRepo.CreateUser(ctx, user, apiKeyHash); err != nil {
		return nil, apperrors.ErrDatabaseError("failed to create user", err)
	}

	return &api.CreateUserResponse{
		User:   user,
		APIKey: apiKey, // Return plain API key (only time it's available!)
	}, nil
}

// AuthenticateUser verifies an API key and returns the associated user.
// Returns appropriate errors for invalid API keys, revoked keys, or server errors.
func (s *Service) AuthenticateUser(ctx context.Context, apiKey string) (*api.User, error) {
	if s.userRepo == nil {
		return nil, apperrors.ErrInternalError("user repository not configured", nil)
	}

	if apiKey == "" {
		return nil, apperrors.ErrBadRequest("API key is required", nil)
	}

	apiKeyHash := hashAPIKey(apiKey)

	user, err := s.userRepo.GetUserByAPIKeyHash(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, apperrors.ErrInvalidAPIKey(nil)
	}

	if user.Revoked {
		return nil, apperrors.ErrAPIKeyRevoked(nil)
	}

	return user, nil
}

// RevokeUser marks a user's API key as revoked.
// Returns an error if the user does not exist or revocation fails.
func (s *Service) RevokeUser(ctx context.Context, email string) error {
	if s.userRepo == nil {
		return apperrors.ErrInternalError("user repository not configured", nil)
	}

	if email == "" {
		return apperrors.ErrBadRequest("email is required", nil)
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Propagate database errors as-is
		return err
	}
	if user == nil {
		return apperrors.ErrNotFound("user not found", nil)
	}

	if err := s.userRepo.RevokeUser(ctx, email); err != nil {
		// Propagate errors as-is (they already have proper status codes)
		return err
	}

	return nil
}

// generateAPIKey creates a cryptographically secure random API key.
// The key is base64-encoded and approximately 32 characters long.
func generateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// hashAPIKey creates a SHA-256 hash of the API key for secure storage.
// NOTICE: we never store plain API keys in the database.
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))

	return base64.StdEncoding.EncodeToString(hash[:])
}

// RunCommand starts an ECS Fargate task and records the execution.
func (s *Service) RunCommand(ctx context.Context, userEmail string, req api.ExecutionRequest) (*api.ExecutionResponse, error) {
	if s.executionRepo == nil {
		return nil, apperrors.ErrInternalError("execution repository not configured", nil)
	}

	if s.ecsClient == nil {
		return nil, apperrors.ErrInternalError("ECS client not configured", nil)
	}

	if req.Command == "" {
		return nil, apperrors.ErrBadRequest("command is required", nil)
	}

	// Note: Image override is not supported via task overrides
	// ECS container overrides don't support image changes
	// If a custom image is needed, a new task definition revision must be registered
	// For MVP, we use the image from the base task definition
	if req.Image != "" && req.Image != s.cfg.DefaultImage {
		s.Logger.Debug("custom image requested but not supported via overrides, using task definition image",
			"requested", req.Image, "using", s.cfg.DefaultImage)
	}

	envVars := []types.KeyValuePair{
		{
			Name:  aws.String("RUNVOY_COMMAND"),
			Value: aws.String(req.Command),
		},
	}
	for key, value := range req.Env {
		envVars = append(envVars, types.KeyValuePair{
			Name:  aws.String(key),
			Value: aws.String(value),
		})
	}

	containerCommand := []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("echo 'Execution starting'; %s", req.Command),
	}

	runTaskInput := &ecs.RunTaskInput{
		Cluster:        aws.String(s.cfg.ECSCluster),
		TaskDefinition: aws.String(s.cfg.TaskDefinition),
		LaunchType:     types.LaunchTypeFargate,
		Overrides: &types.TaskOverride{
			ContainerOverrides: []types.ContainerOverride{
				{
					Name:        aws.String("executor"),
					Command:     containerCommand,
					Environment: envVars,
				},
			},
		},
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets: []string{
					s.cfg.Subnet1,
					s.cfg.Subnet2,
				},
				SecurityGroups: []string{
					s.cfg.SecurityGroup,
				},
				AssignPublicIp: types.AssignPublicIpEnabled,
			},
		},
		Tags: []types.Tag{
			{
				Key:   aws.String("UserEmail"),
				Value: aws.String(userEmail),
			},
		},
	}

	runTaskOutput, err := s.ecsClient.RunTask(ctx, runTaskInput)
	if err != nil {
		return nil, apperrors.ErrInternalError("failed to start ECS task", err)
	}

	if len(runTaskOutput.Tasks) == 0 {
		return nil, apperrors.ErrInternalError("no tasks were started", nil)
	}

	task := runTaskOutput.Tasks[0]
	taskARN := aws.ToString(task.TaskArn)
	executionIDParts := strings.Split(taskARN, "/") // Format: arn:aws:ecs:region:account:task/cluster/task-id
	executionID := executionIDParts[len(executionIDParts)-1]

	// Add ExecutionID tag to the task for easier tracking
	_, err = s.ecsClient.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: aws.String(taskARN),
		Tags: []types.Tag{
			{
				Key:   aws.String("ExecutionID"),
				Value: aws.String(executionID),
			},
		},
	})
	if err != nil {
		// Log warning but continue - tag failure shouldn't block execution
		s.Logger.Warn("failed to add ExecutionID tag to task",
			"error", err,
			"taskARN", taskARN,
			"executionID", executionID,
		)
	}

	// Create execution record
	startedAt := time.Now().UTC()
	execution := &api.Execution{
		ExecutionID: executionID,
		UserEmail:   userEmail,
		Command:     req.Command,
		LockName:    req.Lock,
		TaskARN:     taskARN,
		StartedAt:   startedAt,
		Status:      "RUNNING",
	}

	if err := s.executionRepo.CreateExecution(ctx, execution); err != nil {
		s.Logger.Error("failed to create execution record, but task started",
			"error", err,
			"executionID", executionID,
			"taskARN", taskARN,
		)
		// Continue even if recording fails - the task is already running
	}

	// Build log URL
	// TODO: Implement log URL (simplified - in production this might need JWT or other auth)
	logURL := fmt.Sprintf("/api/v1/executions/%s/logs/notimplemented", executionID)

	return &api.ExecutionResponse{
		ExecutionID: executionID,
		LogURL:      logURL,
		Status:      "RUNNING",
	}, nil
}
