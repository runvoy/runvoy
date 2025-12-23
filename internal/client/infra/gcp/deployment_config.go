package gcp

import (
	"errors"
	"fmt"

	"google.golang.org/api/deploymentmanager/v2"
	"gopkg.in/yaml.v3"

	gcpdeploy "github.com/runvoy/runvoy/deploy/providers/gcp"
	"github.com/runvoy/runvoy/internal/providers/gcp/constants"
)

const (
	runvoyDeploymentName     = "runvoy-backend"
	runvoyDeploymentTemplate = gcpdeploy.TemplateName
)

type deploymentConfig struct {
	Imports   []deploymentImport   `yaml:"imports"`
	Resources []deploymentResource `yaml:"resources"`
}

type deploymentImport struct {
	Path string `yaml:"path"`
}

type deploymentResource struct {
	Name       string         `yaml:"name"`
	Type       string         `yaml:"type"`
	Properties map[string]any `yaml:"properties,omitempty"`
}

func buildDeploymentTarget(config *ResourceConfig) (*deploymentmanager.TargetConfiguration, error) {
	configContent, err := buildDeploymentConfig(config)
	if err != nil {
		return nil, err
	}

	return &deploymentmanager.TargetConfiguration{
		Config: &deploymentmanager.ConfigFile{
			Content: configContent,
		},
		Imports: []*deploymentmanager.ImportFile{
			{
				Name:    runvoyDeploymentTemplate,
				Content: gcpdeploy.Template,
			},
		},
	}, nil
}

func buildDeploymentConfig(config *ResourceConfig) (string, error) {
	if config == nil {
		return "", errors.New("deployment config is required")
	}

	dmConfig := deploymentConfig{
		Imports: []deploymentImport{
			{Path: runvoyDeploymentTemplate},
		},
		Resources: []deploymentResource{
			{
				Name:       runvoyDeploymentName,
				Type:       runvoyDeploymentTemplate,
				Properties: buildDeploymentProperties(config),
			},
		},
	}

	content, err := yaml.Marshal(dmConfig)
	if err != nil {
		return "", fmt.Errorf("marshal deployment config: %w", err)
	}

	return string(content), nil
}

func buildDeploymentProperties(config *ResourceConfig) map[string]any {
	return map[string]any{
		"projectId":                      config.ProjectID,
		"region":                         config.Region,
		"vpcName":                        config.VPCName,
		"subnetName":                     config.SubnetName,
		"vpcConnectorName":               config.VPCConnectorName,
		"useDirectVPCEgress":             config.UseDirectVPCEgress,
		"vpcCidrRange":                   config.VPCCIDRRange,
		"vpcConnectorIpRange":            constants.VPCConnectorIPRange,
		"vpcConnectorMinInstances":       constants.VPCConnectorMinInstances,
		"vpcConnectorMaxInstances":       constants.VPCConnectorMaxInstances,
		"firewallRuleEgress":             constants.FirewallRuleEgress,
		"firestoreLocationId":            config.FirestoreLocationID,
		"orchestratorImage":              config.OrchestratorImage,
		"eventProcessorImage":            config.EventProcessorImage,
		"minInstances":                   config.MinInstances,
		"maxInstances":                   config.MaxInstances,
		"timeoutSeconds":                 config.TimeoutSeconds,
		"taskEventsTopic":                config.TaskEventsTopic,
		"logEventsTopic":                 config.LogEventsTopic,
		"webSocketEventsTopic":           constants.TopicWebSocketEvents,
		"processorSubscription":          config.ProcessorSub,
		"keyRingName":                    config.KeyRingName,
		"cryptoKeyName":                  config.CryptoKeyName,
		"schedulerJobName":               constants.SchedulerHealthReconcile,
		"healthSchedule":                 config.HealthSchedule,
		"logRetentionDays":               config.LogRetentionDays,
		"artifactRegistryRepo":           constants.ArtifactRegistryRepo,
		"serviceAccountOrchestrator":     constants.ServiceAccountOrchestrator,
		"serviceAccountEventProcessor":   constants.ServiceAccountEventProcessor,
		"serviceAccountRunner":           constants.ServiceAccountRunner,
		"serviceOrchestrator":            constants.ServiceOrchestrator,
		"serviceEventProcessor":          constants.ServiceEventProcessor,
		"serviceRunner":                  constants.ServiceRunner,
		"logSinkRunner":                  constants.LogSinkRunner,
		"collectionApiKeys":              constants.CollectionAPIKeys,
		"collectionExecutions":           constants.CollectionExecutions,
		"collectionPendingApiKeys":       constants.CollectionPendingAPIKeys,
		"collectionSecretsMetadata":      constants.CollectionSecretsMetadata,
		"collectionImageConfigs":         constants.CollectionImageConfigs,
		"collectionExecutionLogs":        constants.CollectionExecutionLogs,
		"collectionWebsocketTokens":      constants.CollectionWebSocketTokens,
		"collectionWebsocketConnections": constants.CollectionWebSocketConnection,
	}
}
