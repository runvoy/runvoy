package constants

var version string = "0.0.0-development" // Updated by CI/CD pipeline at build time

func GetVersion() *string {
	return &version
}

// ProjectName is the name of the CLI tool and application
const ProjectName = "runvoy"

// ConfigDirName is the name of the configuration directory in the user's home directory
const ConfigDirName = ".runvoy"

// ConfigFileName is the name of the global configuration file
const ConfigFileName = "config.yaml"

// ConfigPath returns the full path to the global configuration directory
func ConfigDirPath(homeDir string) string {
	return homeDir + "/" + ConfigDirName
}

// ConfigFilePath returns the full path to the global configuration file
func ConfigFilePath(homeDir string) string {
	return ConfigDirPath(homeDir) + "/" + ConfigFileName
}

type BackendProvider string

const (
	AWS BackendProvider = "AWS"
	// Example: GCP BackendProvider = "GCP"
)

type Environment string

// Environment types for logger configuration
const (
	Development Environment = "development"
	Production  Environment = "production"
	CLI         Environment = "cli"
)

const ApiKeyHeader = "X-API-Key"
const ContentTypeHeader = "Content-Type"

type Service string

const (
	OrchestratorService   Service = "orchestrator"
	EventProcessorService Service = "event-processor"
)
