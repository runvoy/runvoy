package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds all configuration for the Lambda function
type Config struct {
	// AWS SDK configuration
	AWSConfig  aws.Config
	ECSClient  *ecs.Client
	LogsClient *cloudwatchlogs.Client

	// Environment variables from Lambda
	APIKeyHash    string `env:"API_KEY_HASH" env-description:"Bcrypt hash of API key"`
	GitHubToken   string `env:"GITHUB_TOKEN" env-description:"GitHub personal access token"`
	GitLabToken   string `env:"GITLAB_TOKEN" env-description:"GitLab personal access token"`
	SSHPrivateKey string `env:"SSH_PRIVATE_KEY" env-description:"Base64-encoded SSH private key"`

	// ECS configuration
	ECSCluster    string `env:"ECS_CLUSTER" env-description:"ECS cluster name" env-required:"true"`
	TaskDef       string `env:"TASK_DEFINITION" env-description:"ECS task definition" env-required:"true"`
	Subnet1       string `env:"SUBNET_1" env-description:"First subnet ID" env-required:"true"`
	Subnet2       string `env:"SUBNET_2" env-description:"Second subnet ID" env-required:"true"`
	SecurityGroup string `env:"SECURITY_GROUP" env-description:"Security group ID" env-required:"true"`
	LogGroup      string `env:"LOG_GROUP" env-description:"CloudWatch log group name" env-required:"true"`
}

// InitConfig initializes the configuration and AWS clients
func InitConfig() (*Config, error) {
	cfg := &Config{}

	// Read environment variables using cleanenv
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Load AWS SDK configuration
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	cfg.AWSConfig = awsConfig

	// Initialize AWS clients
	cfg.ECSClient = ecs.NewFromConfig(awsConfig)
	cfg.LogsClient = cloudwatchlogs.NewFromConfig(awsConfig)

	return cfg, nil
}
