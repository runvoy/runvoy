package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds all configuration for the Lambda function
type Config struct {
	// AWS SDK configuration
	AWSConfig   aws.Config
	ECSClient   *ecs.Client
	LogsClient  *cloudwatchlogs.Client
	DynamoDBClient *dynamodb.Client

	// DynamoDB tables
	APIKeysTable    string `env:"API_KEYS_TABLE" env-description:"DynamoDB table for API keys" env-required:"true"`
	ExecutionsTable string `env:"EXECUTIONS_TABLE" env-description:"DynamoDB table for executions" env-required:"true"`
	LocksTable      string `env:"LOCKS_TABLE" env-description:"DynamoDB table for locks" env-required:"true"`

	// S3 configuration
	CodeBucket string `env:"CODE_BUCKET" env-description:"S3 bucket for code uploads" env-required:"true"`

	// JWT configuration
	JWTSecret string `env:"JWT_SECRET" env-description:"Secret for signing JWT tokens"`
	WebUIURL  string `env:"WEB_UI_URL" env-description:"Base URL for web UI"`

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
	cfg.DynamoDBClient = dynamodb.NewFromConfig(awsConfig)

	return cfg, nil
}
