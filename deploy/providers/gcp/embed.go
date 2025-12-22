// Package gcpdeploy provides embedded GCP Deployment Manager templates.
package gcpdeploy

import _ "embed"

// TemplateName is the filename used in Deployment Manager imports.
const TemplateName = "runvoy-deployment.jinja"

// Template is the embedded Deployment Manager template content.
//
//go:embed runvoy-deployment.jinja
var Template string
