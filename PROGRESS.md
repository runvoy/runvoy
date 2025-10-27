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
- ✅ Simplified API Gateway to single `/{proxy+}` catch-all route (routing in Lambda)
- ✅ Added DynamoDB permissions to Lambda IAM role
- ✅ Updated outputs to export new resource names

**File**: `deploy/cloudformation.yaml`

### 2. Lambda Orchestrator (Go)
- ✅ Created `dynamodb.go` with full CRUD operations:
  - Lock management: `tryAcquireLock()`, `releaseLock()`, `getLockHolder()`, `listLocks()`
  - Execution tracking: `recordExecution()`, `getExecution()`, `listExecutions()`
- ✅ Updated `config.go`:
  - Added DynamoDB client
  - New env vars: `API_KEYS_TABLE`, `EXECUTIONS_TABLE`, `LOCKS_TABLE`, `CODE_BUCKET`, `JWT_SECRET`, `WEB_UI_URL`
  - Removed: `GitHubToken`, `GitLabToken`, `SSHPrivateKey`, `APIKeyHash`
- ✅ Rewrote `auth.go`:
  - DynamoDB-based authentication (SHA256 hash lookup)
  - Returns `*User` struct with email and metadata
  - Auto-updates `last_used` timestamp
  - Supports revocation
- ✅ Updated `main.go`:
  - HTTP path-based routing (replaces JSON action field)
  - Routes: `POST /executions`, `GET /executions`, `GET /executions/{id}`, `GET /executions/{id}/logs`, `GET /locks`
  - Strips `/prod` stage prefix
- ✅ Rewrote `handlers.go`:
  - `handleCreateExecution()`: Creates execution with lock acquisition
  - `handleGetExecution()`: Gets status with ECS sync
  - `handleListExecutions()`: Lists user's executions
  - `handleGetExecutionLogs()`: Streams logs from CloudWatch
  - `handleListLocks()`: Lists active locks
- ✅ Simplified `shell.go` (removed git cloning logic)
- ✅ Added Go dependencies: `github.com/aws/aws-sdk-go-v2/service/dynamodb`, `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`
- ✅ **Build verified**: Lambda compiles successfully

**Directory**: `lambda/orchestrator/`

## Pending Tasks 🚧

### 3. CLI Admin Commands
- ⏸️ `mycli admin add-user <email>` - Generate API key for new user
- ⏸️ `mycli admin revoke-user <email>` - Disable user's API key
- ⏸️ `mycli admin list-users` - Show all users and their status

### 4. CLI Execution Commands
- ⏸️ Update `mycli configure` to store API key (not AWS credentials)
- ⏸️ Update `mycli exec` to use API key authentication
- ⏸️ Update `mycli status` for new API
- ⏸️ Update `mycli logs` for new API
- ⏸️ Add `mycli list` command
- ⏸️ Add `mycli locks list` command

### 5. Deployment & Testing
- ⏸️ Update `mycli init` command for new architecture
- ⏸️ Update `Makefile` build scripts
- ⏸️ Test end-to-end flow
- ⏸️ Update README.md with new usage

## Architecture Changes

### Before (Git-based)
```
User → CLI → API Gateway → Lambda → ECS (git clone + exec)
                                    ↓
                            CloudWatch Logs
```
- Single API key (bcrypt hash)
- Git cloning required
- JSON action routing
- No execution tracking

### After (DynamoDB-based)
```
User → CLI → API Gateway → Lambda → DynamoDB (keys, executions, locks)
              (API key)              ↓
                                   ECS (direct exec)
                                    ↓
                            CloudWatch Logs
```
- Multiple users with individual API keys
- Direct command execution
- HTTP path routing
- Full execution tracking
- Distributed locking

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/executions` | Create new execution |
| GET | `/executions` | List user's executions |
| GET | `/executions/{id}` | Get execution status |
| GET | `/executions/{id}/logs` | Get execution logs |
| GET | `/locks` | List active locks |

## Data Models

### APIKeysTable
```
{
  "api_key_hash": "sha256(...)",  // Partition key
  "user_email": "alice@acme.com",
  "created_at": "2025-10-26T...",
  "revoked": false,
  "last_used": "2025-10-26T..."
}
```

### ExecutionsTable
```
{
  "execution_id": "67344a1e8f3c",  // Partition key
  "started_at": "2025-10-26T...",   // Sort key
  "user_email": "alice@acme.com",
  "command": "ls -la",
  "lock_name": "infra-prod",
  "task_arn": "arn:aws:ecs:...",
  "status": "running",
  "log_stream_name": "task/executor/..."
}
```

### LocksTable
```
{
  "lock_name": "infra-prod",       // Partition key
  "execution_id": "67344a1e8f3c",
  "user_email": "alice@acme.com",
  "acquired_at": "2025-10-26T...",
  "ttl": 1730000000                // Auto-expire
}
```

## Next Steps

1. **CLI Admin Commands** - Implement user management
2. **CLI Execution Updates** - Update exec flow to use API keys
3. **Testing** - Deploy and test end-to-end
4. **Documentation** - Update README and ARCHITECTURE.md

## Build Status

- ✅ CloudFormation validates successfully
- ✅ Lambda builds successfully (Linux ARM64)
- ⏸️ CLI builds (not tested yet)

## Files Modified

- `deploy/cloudformation.yaml` - Infrastructure
- `deploy/cloudformation-bucket.yaml` - No changes needed
- `lambda/orchestrator/main.go` - HTTP routing
- `lambda/orchestrator/config.go` - DynamoDB config
- `lambda/orchestrator/auth.go` - DynamoDB auth
- `lambda/orchestrator/handlers.go` - New handlers
- `lambda/orchestrator/dynamodb.go` - **NEW** DynamoDB operations
- `lambda/orchestrator/shell.go` - Simplified
- `lambda/orchestrator/util.go` - Added extractTaskID
- `go.mod` - Added DynamoDB dependencies

## Notes

- API Gateway now uses catch-all `/{proxy+}` route for flexibility
- All routing logic moved to Lambda (easier to iterate)
- Git cloning removed from core architecture (can be added back later if needed)
- JWT tokens for web UI not yet implemented (placeholder in code)
- Lock TTL ensures automatic cleanup if tasks crash
