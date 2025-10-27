package constants

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
