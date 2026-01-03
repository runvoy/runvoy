package gcp

// Package gcp provides GCP-specific infrastructure management for runvoy.
// This package implements a simple declarative infrastructure engine for GCP resources.
//
// Features:
// - Simple YAML-based resource definitions (deploy/providers/gcp/resources.yaml)
// - Environment variable substitution (${VAR})
// - Direct GCP SDK calls (no CloudFormation-style complexity)
// - Project creation with optional organization placement
// - Optional billing setup (skipped gracefully if not provided)
// - Progress logging per resource
// - Project deletion requires --force flag for safety
