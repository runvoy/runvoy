// Package aws contains AWS-specific configuration helpers for Runvoy services.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/spf13/viper"
)

// Config contains AWS-specific configuration.
// These settings are only used when the backend provider is AWS.
type Config struct {
	// DynamoDB Tables
	APIKeysTable              string `mapstructure:"api_keys_table"`
	ExecutionsTable           string `mapstructure:"executions_table"`
	ExecutionLogsTable        string `mapstructure:"execution_logs_table"`
	ImageTaskDefsTable        string `mapstructure:"image_taskdefs_table"`
	PendingAPIKeysTable       string `mapstructure:"pending_api_keys_table"`
	SecretsMetadataTable      string `mapstructure:"secrets_metadata_table"`
	WebSocketConnectionsTable string `mapstructure:"websocket_connections_table"`
	WebSocketTokensTable      string `mapstructure:"websocket_tokens_table"`

	// ECS Configuration
	DefaultTaskExecRoleARN string `mapstructure:"default_task_exec_role_arn"`
	DefaultTaskRoleARN     string `mapstructure:"default_task_role_arn"`
	ECSCluster             string `mapstructure:"ecs_cluster"`
	SecurityGroup          string `mapstructure:"security_group"`
	Subnet1                string `mapstructure:"subnet_1"`
	Subnet2                string `mapstructure:"subnet_2"`
	TaskDefinition         string `mapstructure:"task_definition"`

	// CloudWatch Logs
	LogGroup string `mapstructure:"log_group"`

	// API Gateway WebSocket
	WebSocketAPIEndpoint string `mapstructure:"websocket_api_endpoint"`

	// Secrets Management
	SecretsPrefix    string `mapstructure:"secrets_prefix"`
	SecretsKMSKeyARN string `mapstructure:"secrets_kms_key_arn"`

	// Infrastructure defaults
	InfraDefaultStackName string `mapstructure:"infra_default_stack_name" yaml:"infra_default_stack_name"`

	// AWS SDK Configuration (credentials, region, etc.)
	SDKConfig *aws.Config `mapstructure:"-"`
}

// BindEnvVars binds AWS-specific environment variables to the provided Viper instance.
func BindEnvVars(v *viper.Viper) {
	v.SetDefault("aws.secrets_prefix", awsConstants.SecretsPrefix)
	v.SetDefault("aws.infra_default_stack_name", awsConstants.DefaultInfraStackName)

	_ = v.BindEnv("aws.api_keys_table", "RUNVOY_AWS_API_KEYS_TABLE")
	_ = v.BindEnv("aws.default_task_exec_role_arn", "RUNVOY_AWS_DEFAULT_TASK_EXEC_ROLE_ARN")
	_ = v.BindEnv("aws.default_task_role_arn", "RUNVOY_AWS_DEFAULT_TASK_ROLE_ARN")
	_ = v.BindEnv("aws.ecs_cluster", "RUNVOY_AWS_ECS_CLUSTER")
	_ = v.BindEnv("aws.executions_table", "RUNVOY_AWS_EXECUTIONS_TABLE")
	_ = v.BindEnv("aws.execution_logs_table", "RUNVOY_AWS_EXECUTION_LOGS_TABLE")
	_ = v.BindEnv("aws.image_taskdefs_table", "RUNVOY_AWS_IMAGE_TASKDEFS_TABLE")
	_ = v.BindEnv("aws.log_group", "RUNVOY_AWS_LOG_GROUP")
	_ = v.BindEnv("aws.pending_api_keys_table", "RUNVOY_AWS_PENDING_API_KEYS_TABLE")
	_ = v.BindEnv("aws.secrets_kms_key_arn", "RUNVOY_AWS_SECRETS_KMS_KEY_ARN")
	_ = v.BindEnv("aws.secrets_metadata_table", "RUNVOY_AWS_SECRETS_METADATA_TABLE")
	_ = v.BindEnv("aws.secrets_prefix", "RUNVOY_AWS_SECRETS_PREFIX")
	_ = v.BindEnv("aws.security_group", "RUNVOY_AWS_SECURITY_GROUP")
	_ = v.BindEnv("aws.subnet_1", "RUNVOY_AWS_SUBNET_1")
	_ = v.BindEnv("aws.subnet_2", "RUNVOY_AWS_SUBNET_2")
	_ = v.BindEnv("aws.task_definition", "RUNVOY_AWS_TASK_DEFINITION")
	_ = v.BindEnv("aws.websocket_api_endpoint", "RUNVOY_AWS_WEBSOCKET_API_ENDPOINT")
	_ = v.BindEnv("aws.websocket_connections_table", "RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE")
	_ = v.BindEnv("aws.websocket_tokens_table", "RUNVOY_AWS_WEBSOCKET_TOKENS_TABLE")
	_ = v.BindEnv("aws.infra_default_stack_name", "RUNVOY_AWS_INFRA_DEFAULT_STACK_NAME")
}

// ValidateOrchestrator validates required AWS fields for the orchestrator service.
func ValidateOrchestrator(cfg *Config) error {
	if cfg == nil {
		return errors.New("AWS configuration is required when backend_provider is AWS")
	}

	required := map[string]string{
		"AWS.APIKeysTable":              cfg.APIKeysTable,
		"AWS.ECSCluster":                cfg.ECSCluster,
		"AWS.ExecutionsTable":           cfg.ExecutionsTable,
		"AWS.ExecutionLogsTable":        cfg.ExecutionLogsTable,
		"AWS.ImageTaskDefsTable":        cfg.ImageTaskDefsTable,
		"AWS.LogGroup":                  cfg.LogGroup,
		"AWS.PendingAPIKeysTable":       cfg.PendingAPIKeysTable,
		"AWS.SecretsKMSKeyARN":          cfg.SecretsKMSKeyARN,
		"AWS.SecretsMetadataTable":      cfg.SecretsMetadataTable,
		"AWS.SecretsPrefix":             cfg.SecretsPrefix,
		"AWS.SecurityGroup":             cfg.SecurityGroup,
		"AWS.Subnet1":                   cfg.Subnet1,
		"AWS.Subnet2":                   cfg.Subnet2,
		"AWS.WebSocketAPIEndpoint":      cfg.WebSocketAPIEndpoint,
		"AWS.WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"AWS.WebSocketTokensTable":      cfg.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	return nil
}

// ValidateEventProcessor validates required AWS fields for the event processor service.
func ValidateEventProcessor(cfg *Config) error {
	if cfg == nil {
		return errors.New("AWS configuration is required when backend_provider is AWS")
	}

	required := map[string]string{
		"AWS.APIKeysTable":              cfg.APIKeysTable,
		"AWS.DefaultTaskExecRoleARN":    cfg.DefaultTaskExecRoleARN,
		"AWS.DefaultTaskRoleARN":        cfg.DefaultTaskRoleARN,
		"AWS.ECSCluster":                cfg.ECSCluster,
		"AWS.ExecutionsTable":           cfg.ExecutionsTable,
		"AWS.ExecutionLogsTable":        cfg.ExecutionLogsTable,
		"AWS.ImageTaskDefsTable":        cfg.ImageTaskDefsTable,
		"AWS.LogGroup":                  cfg.LogGroup,
		"AWS.PendingAPIKeysTable":       cfg.PendingAPIKeysTable,
		"AWS.SecretsKMSKeyARN":          cfg.SecretsKMSKeyARN,
		"AWS.SecretsMetadataTable":      cfg.SecretsMetadataTable,
		"AWS.SecretsPrefix":             cfg.SecretsPrefix,
		"AWS.WebSocketAPIEndpoint":      cfg.WebSocketAPIEndpoint,
		"AWS.WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"AWS.WebSocketTokensTable":      cfg.WebSocketTokensTable,
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
	}

	cfg.WebSocketAPIEndpoint = "https://" + NormalizeWebSocketEndpoint(cfg.WebSocketAPIEndpoint)

	return nil
}

// NormalizeWebSocketEndpoint strips protocol prefixes from WebSocket endpoint URLs.
// Accepts: https://example.com, http://example.com, wss://example.com, ws://example.com, example.com
// Returns: example.com (without protocol).
func NormalizeWebSocketEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "wss://")
	endpoint = strings.TrimPrefix(endpoint, "ws://")
	return endpoint
}

// LoadSDKConfig loads the AWS SDK configuration from the environment.
func (c *Config) LoadSDKConfig(ctx context.Context) error {
	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS SDK configuration: %w", err)
	}
	c.SDKConfig = &awsCfg
	return nil
}

// NormalizeVersion strips any 'v' prefix from the version string.
// S3 paths use versions without the 'v' prefix (e.g., "0.1.0" not "v0.1.0").
func NormalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// BuildTemplateURL builds the S3 HTTPS URL for the CloudFormation template.
// The version is normalized to remove any 'v' prefix before building the URL.
// If region is empty, defaults to the ReleasesBucketRegion constant.
func BuildTemplateURL(version, region string) string {
	normalizedVersion := NormalizeVersion(version)
	if region == "" {
		region = awsConstants.ReleasesBucketRegion
	}
	bucketName := "runvoy-releases-" + region
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/%s",
		bucketName,
		region,
		normalizedVersion,
		awsConstants.CloudFormationTemplateFile)
}
