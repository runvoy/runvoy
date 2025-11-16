package authorization

import (
	"embed"
)

// CasbinFS embeds the Casbin model and policy files into the binary.
// This allows the application to run without requiring these files to be present on the filesystem.
//
//go:embed casbin/model.conf casbin/policy.csv
var CasbinFS embed.FS
