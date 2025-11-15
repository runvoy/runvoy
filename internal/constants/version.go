// Package constants defines global constants used throughout runvoy.
package constants

var version = "0.0.0-development" // Updated by CI/CD pipeline at build time

// GetVersion returns the current version of runvoy.
func GetVersion() *string {
	return &version
}

// ProjectName is the name of the CLI tool and application
const ProjectName = "runvoy"
