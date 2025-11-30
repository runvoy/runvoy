package constants

// DefaultWebURL is the default URL of the web application HTML site.
// This can be overridden via configuration (RUNVOY_WEB_URL env var or config file).
const DefaultWebURL = "https://web.runvoy.site/"

// DefaultDevWebURL is the default URL of the development web application HTML site.
const DefaultDevWebURL = "https://dev.web.runvoy.site/"

// LocalDevelopmentURL is the default URL of the local development server.
const LocalDevelopmentURL = "http://localhost:5173/"

// DefaultCORSAllowedOrigins is the default list of allowed CORS origins.
// Defaults to the web application URL and local development URL.
var DefaultCORSAllowedOrigins = []string{
	DefaultWebURL,
	DefaultDevWebURL,
	LocalDevelopmentURL,
}

// ConfigDirName is the name of the configuration directory in the user's home directory.
const ConfigDirName = "." + ProjectName

// ConfigFileName is the name of the global configuration file.
const ConfigFileName = "config.yaml"

// ConfigDirPath returns the full path to the global configuration directory.
func ConfigDirPath(homeDir string) string {
	return homeDir + "/" + ConfigDirName
}

// ConfigFilePath returns the full path to the global configuration file.
func ConfigFilePath(homeDir string) string {
	return ConfigDirPath(homeDir) + "/" + ConfigFileName
}

// ConfigDirPermissions is the file system permissions for config directory (0750).
const ConfigDirPermissions = 0o750

// ConfigFilePermissions is the file system permissions for config file (0600).
const ConfigFilePermissions = 0o600
