// Package aws contains AWS-specific configuration helpers for Runvoy services.
package aws

import (
	"fmt"
	"strings"

	awsConstants "runvoy/internal/providers/aws/constants"

	"github.com/spf13/viper"
)

// Config contains AWS-specific configuration.
// These settings are only used when the backend provider is AWS.
type Config struct {
	// DynamoDB Tables
	APIKeysTable              string `mapstructure:"api_keys_table"`
	ExecutionsTable           string `mapstructure:"executions_table"`
	PendingAPIKeysTable       string `mapstructure:"pending_api_keys_table"`
	WebSocketConnectionsTable string `mapstructure:"websocket_connections_table"`
	WebSocketTokensTable      string `mapstructure:"websocket_tokens_table"`
	SecretsMetadataTable      string `mapstructure:"secrets_metadata_table"`

	// ECS Configuration
	ECSCluster      string `mapstructure:"ecs_cluster"`
	SecurityGroup   string `mapstructure:"security_group"`
	Subnet1         string `mapstructure:"subnet_1"`
	Subnet2         string `mapstructure:"subnet_2"`
	TaskDefinition  string `mapstructure:"task_definition"`
	TaskExecRoleARN string `mapstructure:"task_exec_role_arn"`
	TaskRoleARN     string `mapstructure:"task_role_arn"`

	// CloudWatch Logs
	LogGroup string `mapstructure:"log_group"`

	// API Gateway WebSocket
	WebSocketAPIEndpoint string `mapstructure:"websocket_api_endpoint"`

	// Secrets Management
	SecretsPrefix    string `mapstructure:"secrets_prefix"`
	SecretsKMSKeyARN string `mapstructure:"secrets_kms_key_arn"`
}

// BindEnvVars binds AWS-specific environment variables to the provided Viper instance.
func BindEnvVars(v *viper.Viper) {
	v.SetDefault("aws.secrets_prefix", awsConstants.SecretsPrefix)

	_ = v.BindEnv("aws.api_keys_table", "RUNVOY_AWS_API_KEYS_TABLE")
	_ = v.BindEnv("aws.ecs_cluster", "RUNVOY_AWS_ECS_CLUSTER")
	_ = v.BindEnv("aws.executions_table", "RUNVOY_AWS_EXECUTIONS_TABLE")
	_ = v.BindEnv("aws.log_group", "RUNVOY_AWS_LOG_GROUP")
	_ = v.BindEnv("aws.pending_api_keys_table", "RUNVOY_AWS_PENDING_API_KEYS_TABLE")
	_ = v.BindEnv("aws.security_group", "RUNVOY_AWS_SECURITY_GROUP")
	_ = v.BindEnv("aws.subnet_1", "RUNVOY_AWS_SUBNET_1")
	_ = v.BindEnv("aws.subnet_2", "RUNVOY_AWS_SUBNET_2")
	_ = v.BindEnv("aws.task_definition", "RUNVOY_AWS_TASK_DEFINITION")
	_ = v.BindEnv("aws.task_exec_role_arn", "RUNVOY_AWS_TASK_EXEC_ROLE_ARN")
	_ = v.BindEnv("aws.task_role_arn", "RUNVOY_AWS_TASK_ROLE_ARN")
	_ = v.BindEnv("aws.websocket_api_endpoint", "RUNVOY_AWS_WEBSOCKET_API_ENDPOINT")
	_ = v.BindEnv("aws.websocket_connections_table", "RUNVOY_AWS_WEBSOCKET_CONNECTIONS_TABLE")
	_ = v.BindEnv("aws.websocket_tokens_table", "RUNVOY_AWS_WEBSOCKET_TOKENS_TABLE")
	_ = v.BindEnv("aws.secrets_metadata_table", "RUNVOY_AWS_SECRETS_METADATA_TABLE")
	_ = v.BindEnv("aws.secrets_prefix", "RUNVOY_AWS_SECRETS_PREFIX")
	_ = v.BindEnv("aws.secrets_kms_key_arn", "RUNVOY_AWS_SECRETS_KMS_KEY_ARN")
}

// ValidateOrchestrator validates required AWS fields for the orchestrator service.
func ValidateOrchestrator(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("AWS configuration is required when backend_provider is AWS")
	}

	required := map[string]string{
		"AWS.APIKeysTable":              cfg.APIKeysTable,
		"AWS.ECSCluster":                cfg.ECSCluster,
		"AWS.ExecutionsTable":           cfg.ExecutionsTable,
		"AWS.LogGroup":                  cfg.LogGroup,
		"AWS.SecurityGroup":             cfg.SecurityGroup,
		"AWS.Subnet1":                   cfg.Subnet1,
		"AWS.Subnet2":                   cfg.Subnet2,
		"AWS.WebSocketAPIEndpoint":      cfg.WebSocketAPIEndpoint,
		"AWS.WebSocketConnectionsTable": cfg.WebSocketConnectionsTable,
		"AWS.WebSocketTokensTable":      cfg.WebSocketTokensTable,
		"AWS.SecretsMetadataTable":      cfg.SecretsMetadataTable,
		"AWS.SecretsPrefix":             cfg.SecretsPrefix,
		"AWS.SecretsKMSKeyARN":          cfg.SecretsKMSKeyARN,
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
		return fmt.Errorf("AWS configuration is required when backend_provider is AWS")
	}

	required := map[string]string{
		"AWS.ECSCluster":                cfg.ECSCluster,
		"AWS.ExecutionsTable":           cfg.ExecutionsTable,
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
// Returns: example.com (without protocol)
func NormalizeWebSocketEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "wss://")
	endpoint = strings.TrimPrefix(endpoint, "ws://")
	return endpoint
}
