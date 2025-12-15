// Package constants provides GCP-specific constants for the runvoy GCP provider.
// It defines resource names, timeouts, and configuration defaults for:
//   - Firestore collections (equivalent to AWS DynamoDB tables)
//   - Cloud Run services for orchestrator and event processor
//   - Pub/Sub topics and subscriptions for event-driven architecture
//   - Cloud KMS for secrets encryption
//   - Cloud Scheduler for periodic health reconciliation
//   - VPC networking and Serverless VPC Access connectors
//   - IAM service accounts and role bindings
//   - Artifact Registry for container image storage
//   - Cloud Logging sinks for log routing
package constants
