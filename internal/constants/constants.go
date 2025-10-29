package constants

var version string = "0.0.0-development" // Updated by CI/CD pipeline at build time

func GetVersion() *string {
	return &version
}

// ProjectName is the name of the CLI tool and application
const ProjectName = "runvoy"

// ConfigDirName is the name of the configuration directory in the user's home directory
const ConfigDirName = ".runvoy"

// ConfigFileName is the name of the configuration file
const ConfigFileName = "config.yaml"

// ConfigPath returns the full path to the configuration directory
func ConfigDirPath(homeDir string) string {
	return homeDir + "/" + ConfigDirName
}

// ConfigFilePath returns the full path to the configuration file
func ConfigFilePath(homeDir string) string {
	return ConfigDirPath(homeDir) + "/" + ConfigFileName
}

type BackendProvider string

const (
	AWS BackendProvider = "AWS"
)

type Environment string

// Environment types for logger configuration
const (
	Development Environment = "development"
	Production  Environment = "production"
	CLI         Environment = "cli"
)
