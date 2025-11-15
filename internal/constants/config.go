// Package constants defines global constants used throughout runvoy.
package constants

// DefaultWebURL is the default URL of the web application HTML file.
// This can be overridden via configuration (RUNVOY_WEB_URL env var or config file).
const DefaultWebURL = "https://runvoy.site/"

// ConfigDirName is the name of the configuration directory in the user's home directory
const ConfigDirName = "." + ProjectName

// ConfigFileName is the name of the global configuration file
const ConfigFileName = "config.yaml"

// ConfigDirPath returns the full path to the global configuration directory.
func ConfigDirPath(homeDir string) string {
	return homeDir + "/" + ConfigDirName
}

// ConfigFilePath returns the full path to the global configuration file
func ConfigFilePath(homeDir string) string {
	return ConfigDirPath(homeDir) + "/" + ConfigFileName
}

// ConfigDirPermissions is the file system permissions for config directory (0750)
const ConfigDirPermissions = 0750

// ConfigFilePermissions is the file system permissions for config file (0600)
const ConfigFilePermissions = 0600
