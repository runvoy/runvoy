package constants

import "time"

// Resource naming and prefixes.
const (
	// DefaultRegion is the default GCP region used when no region is specified.
	DefaultRegion = "us-central1"

	// ProjectPollInterval is the interval at which to poll for project status changes.
	ProjectPollInterval = 5 * time.Second

	// ProjectOperationTimeout is the maximum time to wait for a project operation.
	ProjectOperationTimeout = 5 * time.Minute

	// StatusDeleteRequested is the lifecycle state of a project after deletion is requested.
	StatusDeleteRequested = "DELETE_REQUESTED"

	// ResourcePrefix is the prefix for all runvoy GCP resources.
	ResourcePrefix = "runvoy"
)

// Firestore collection names (equivalent to AWS DynamoDB tables).
const (
	// CollectionAPIKeys stores API key hashes and user mappings.
	CollectionAPIKeys = ResourcePrefix + "-api-keys"
	// CollectionExecutions stores execution records.
	CollectionExecutions = ResourcePrefix + "-executions"
	// CollectionPendingAPIKeys stores pending API key requests.
	CollectionPendingAPIKeys = ResourcePrefix + "-pending-api-keys"
	// CollectionSecretsMetadata stores metadata for secrets.
	CollectionSecretsMetadata = ResourcePrefix + "-secrets-metadata"
	// CollectionImageConfigs stores registered image configurations.
	CollectionImageConfigs = ResourcePrefix + "-image-configs"
	// CollectionWebSocketTokens stores WebSocket authentication tokens.
	CollectionWebSocketTokens = ResourcePrefix + "-websocket-tokens"
	// CollectionWebSocketConnection stores active WebSocket connections.
	CollectionWebSocketConnection = ResourcePrefix + "-websocket-connections"
	// CollectionExecutionLogs stores buffered execution log events.
	CollectionExecutionLogs = ResourcePrefix + "-execution-logs"
)

// Cloud Run and Cloud Functions service names.
const (
	// ServiceOrchestrator is the Cloud Run service for the orchestrator.
	ServiceOrchestrator = ResourcePrefix + "-orchestrator"
	// ServiceEventProcessor is the Cloud Run service for the event processor.
	ServiceEventProcessor = ResourcePrefix + "-event-processor"
	// ServiceRunner is the Cloud Run service for task execution.
	ServiceRunner = ResourcePrefix + "-runner"
	// FunctionOrchestrator is the Cloud Function name for the orchestrator.
	FunctionOrchestrator = ResourcePrefix + "-orchestrator"
	// FunctionEventProcessor is the Cloud Function name for event processing.
	FunctionEventProcessor = ResourcePrefix + "-event-processor"
)

// Pub/Sub topic and subscription names.
const (
	// TopicTaskEvents is the topic for task lifecycle events.
	TopicTaskEvents = ResourcePrefix + "-task-events"
	// TopicLogEvents is the topic for log streaming events.
	TopicLogEvents = ResourcePrefix + "-log-events"
	// TopicWebSocketEvents is the topic for WebSocket events.
	TopicWebSocketEvents = ResourcePrefix + "-websocket-events"
	// SubscriptionProcessor is the subscription for the event processor.
	SubscriptionProcessor = ResourcePrefix + "-processor-subscription"
)

// Cloud Scheduler and periodic job names.
const (
	// SchedulerHealthReconcile is the scheduler job for health reconciliation.
	SchedulerHealthReconcile = ResourcePrefix + "-health-reconcile"
	// HealthReconcileSchedule is the cron expression for health reconciliation.
	HealthReconcileSchedule = "0 * * * *"
)

// Secret Manager and Cloud KMS resource names.
const (
	// SecretPrefix is the prefix for secrets in Secret Manager.
	SecretPrefix = ResourcePrefix + "/secrets"
	// KeyRingName is the Cloud KMS key ring name.
	KeyRingName = ResourcePrefix + "-keyring"
	// CryptoKeyName is the Cloud KMS crypto key name.
	CryptoKeyName = ResourcePrefix + "-secrets-key"
	// KeyPurposeDesc describes the purpose of the encryption key.
	KeyPurposeDesc = "Encryption key for runvoy secrets"
)

// VPC and network resource names.
const (
	// VPCName is the VPC network name.
	VPCName = ResourcePrefix + "-vpc"
	// SubnetName is the subnet name.
	SubnetName = ResourcePrefix + "-subnet"
	// VPCConnectorName is the Serverless VPC Access connector name.
	VPCConnectorName = ResourcePrefix + "-connector"
	// FirewallRuleEgress is the firewall rule for egress traffic.
	FirewallRuleEgress = ResourcePrefix + "-allow-egress"
	// FirewallRuleIngress is the firewall rule for ingress traffic.
	FirewallRuleIngress = ResourcePrefix + "-allow-ingress"
)

// Cloud Logging sink names.
const (
	// LogSinkRunner routes runner logs to Pub/Sub.
	LogSinkRunner = ResourcePrefix + "-runner-logs-sink"
	// LogSinkOrchestrator routes orchestrator logs to Pub/Sub.
	LogSinkOrchestrator = ResourcePrefix + "-orchestrator-logs-sink"
	// LogSinkEventProcessor routes event processor logs to Pub/Sub.
	LogSinkEventProcessor = ResourcePrefix + "-event-processor-logs-sink"
)

// Service account names.
const (
	// ServiceAccountOrchestrator is the service account for the orchestrator.
	ServiceAccountOrchestrator = ResourcePrefix + "-orchestrator-sa"
	// ServiceAccountEventProcessor is the service account for the event processor.
	ServiceAccountEventProcessor = ResourcePrefix + "-event-processor-sa"
	// ServiceAccountRunner is the service account for task runners.
	ServiceAccountRunner = ResourcePrefix + "-runner-sa"
)

// Artifact Registry repository names.
const (
	// ArtifactRegistryRepo is the container image repository name.
	ArtifactRegistryRepo = ResourcePrefix + "-images"
)

// Cloud Run configuration defaults.
const (
	// DefaultCPU is the default CPU allocation for Cloud Run services.
	DefaultCPU = "1000m"
	// DefaultMemory is the default memory allocation for Cloud Run services.
	DefaultMemory = "512Mi"
	// DefaultMaxInstances is the default maximum number of instances.
	DefaultMaxInstances = 100
	// DefaultMinInstances is the default minimum number of instances.
	DefaultMinInstances = 0
	// DefaultTimeoutSeconds is the default request timeout.
	DefaultTimeoutSeconds = 300
	// DefaultConcurrency is the default maximum concurrent requests per instance.
	DefaultConcurrency = 80
)

// Firestore configuration.
const (
	// FirestoreLocationID is the Firestore database location (multi-region US).
	FirestoreLocationID = "nam5"
	// FirestoreDatabaseID is the default Firestore database ID.
	FirestoreDatabaseID = "(default)"
)

// Operation timeouts for GCP resources.
const (
	// CloudRunOperationTimeout is the timeout for Cloud Run operations.
	CloudRunOperationTimeout = 10 * time.Minute
	// PubSubOperationTimeout is the timeout for Pub/Sub operations.
	PubSubOperationTimeout = 2 * time.Minute
	// FirestoreOperationTimeout is the timeout for Firestore operations.
	FirestoreOperationTimeout = 5 * time.Minute
	// KMSOperationTimeout is the timeout for Cloud KMS operations.
	KMSOperationTimeout = 2 * time.Minute
	// VPCOperationTimeout is the timeout for VPC operations.
	VPCOperationTimeout = 5 * time.Minute
	// SchedulerOperationTimeout is the timeout for Cloud Scheduler operations.
	SchedulerOperationTimeout = 1 * time.Minute
	// SecretManagerTimeout is the timeout for Secret Manager operations.
	SecretManagerTimeout = 1 * time.Minute
	// ArtifactRegistryTimeout is the timeout for Artifact Registry operations.
	ArtifactRegistryTimeout = 2 * time.Minute
	// ServiceAccountTimeout is the timeout for service account operations.
	ServiceAccountTimeout = 1 * time.Minute
	// IAMBindingTimeout is the timeout for IAM binding operations.
	IAMBindingTimeout = 2 * time.Minute
	// LoggingSinkTimeout is the timeout for logging sink operations.
	LoggingSinkTimeout = 1 * time.Minute
	// VPCConnectorTimeout is the timeout for VPC connector operations.
	VPCConnectorTimeout = 5 * time.Minute
	// FirewallOperationTimeout is the timeout for firewall operations.
	FirewallOperationTimeout = 2 * time.Minute
	// CloudFunctionTimeout is the timeout for Cloud Function operations.
	CloudFunctionTimeout = 10 * time.Minute
	// ResourcePollInterval is the interval for polling resource status.
	ResourcePollInterval = 5 * time.Second
	// ServiceUsageOperationTimeout is the timeout for enabling service APIs.
	ServiceUsageOperationTimeout = 10 * time.Minute
)

// Logging configuration.
const (
	// DefaultLogRetentionDays is the default log retention period.
	DefaultLogRetentionDays = 365
)

// VPC CIDR configuration.
const (
	// VPCCIDRRange is the CIDR range for the VPC.
	VPCCIDRRange = "10.8.0.0/28"
	// VPCConnectorIPRange is the IP range for the VPC connector.
	VPCConnectorIPRange = "10.8.0.0/28"
	// VPCConnectorMachineType is the machine type for VPC connector instances.
	VPCConnectorMachineType = "e2-micro"
	// VPCConnectorMinInstances is the minimum number of VPC connector instances.
	VPCConnectorMinInstances = 2
	// VPCConnectorMaxInstances is the maximum number of VPC connector instances.
	VPCConnectorMaxInstances = 3
)

// RequiredServices lists the GCP APIs that must be enabled for Runvoy.
var RequiredServices = []string{
	"run.googleapis.com",
	"compute.googleapis.com",
	"vpcaccess.googleapis.com",
	"firestore.googleapis.com",
	"pubsub.googleapis.com",
	"cloudscheduler.googleapis.com",
	"secretmanager.googleapis.com",
	"cloudkms.googleapis.com",
	"artifactregistry.googleapis.com",
}
