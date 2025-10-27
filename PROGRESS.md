# Architecture Pivot Progress

## Date: 2025-10-26

## Summary

Pivoted from git-based single-user architecture to multi-user centralized execution platform with DynamoDB backend.

## Completed Tasks ✓

### 1. CloudFormation Infrastructure
- ✅ Added DynamoDB tables:
  - **APIKeysTable**: Stores user API keys (partition key: `api_key_hash`, GSI: `user_email-index`)
  - **ExecutionsTable**: Tracks all executions (partition key: `execution_id`, sort key: `started_at`, GSIs: `user_email-started_at`, `status-started_at`)
  - **LocksTable**: Manages distributed locks (partition key: `lock_name`, TTL enabled)
  - **CodeBucket**: S3 bucket for code uploads (7-day lifecycle)
- ✅ Updated Lambda environment variables (removed git tokens, added DynamoDB table names)
- ✅ Replaced API Gateway with Lambda Function URLs (simpler, cheaper)
- ✅ Added DynamoDB permissions to Lambda IAM role
- ✅ Updated outputs to export new resource names

**File**: `deploy/cloudformation.yaml`

## Notes

- API Gateway replaced with Lambda Function URLs (lower cost, simpler architecture)
- All routing logic in Lambda (easier to iterate)
- Git cloning removed from core architecture (can be added back later if needed)
- JWT tokens for web UI not yet implemented (placeholder in code)
- Lock TTL ensures automatic cleanup if tasks crash
