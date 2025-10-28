# AddUser Feature Implementation

## Overview
This document describes the implementation of the `addUser` feature for the Runvoy backend, which allows creating API keys for users (identified by email) and storing them in DynamoDB.

## Design Principles
The implementation follows these key principles:
1. **Cloud Provider Abstraction**: Repository pattern separates database operations from business logic
2. **Security**: API keys are hashed using SHA-256 before storage
3. **Extensibility**: Easy to add support for other cloud providers (PostgreSQL, MongoDB, etc.)
4. **Flexibility**: The service can operate without a database for simple operations

## Architecture

### Layer Structure
```
┌─────────────────────────────────────────┐
│  HTTP Handler (router.go)              │  ← API endpoints
├─────────────────────────────────────────┤
│  Service Layer (app/main.go)           │  ← Business logic
├─────────────────────────────────────────┤
│  Repository Interface (database/)      │  ← Abstraction layer
├─────────────────────────────────────────┤
│  DynamoDB Implementation (dynamodb/)   │  ← AWS-specific code
└─────────────────────────────────────────┘
```

## Components

### 1. Repository Interface
**File**: `internal/database/repository.go`

Defines the `UserRepository` interface with methods:
- `CreateUser(ctx, user, apiKeyHash)` - Store new user with hashed API key
- `GetUserByEmail(ctx, email)` - Retrieve user by email (uses GSI)
- `GetUserByAPIKeyHash(ctx, hash)` - Retrieve user for authentication
- `UpdateLastUsed(ctx, email)` - Track API key usage
- `RevokeUser(ctx, email)` - Disable API key without deletion

**Benefit**: Allows swapping DynamoDB for other databases without changing business logic.

### 2. DynamoDB Implementation
**File**: `internal/database/dynamodb/users.go`

Implements `UserRepository` using AWS SDK v2:
- Uses `runvoy-api-keys` table with `api_key_hash` as primary key
- Leverages `user_email-index` GSI for email lookups
- Handles conditional writes to prevent duplicate keys
- Maps between internal types and DynamoDB attributes

**Key Features**:
- Graceful handling of conditional check failures
- Proper error wrapping for debugging
- Separation of storage schema from API types

### 3. Service Layer
**File**: `internal/app/main.go`

Enhanced with user management methods:
- `CreateUser(ctx, req)` - Full user creation flow with validation
  - Email validation using `net/mail`
  - Duplicate user detection
  - Automatic API key generation (24 random bytes → base64)
  - Returns plain API key only once
- `AuthenticateUser(ctx, apiKey)` - API key verification
  - Checks key validity and revocation status

**Security**:
- API keys are 32 characters (base64-encoded)
- Keys are hashed with SHA-256 before storage
- Plain keys are never logged or persisted

### 4. HTTP Endpoint
**File**: `internal/server/router.go`

New endpoint: `POST /api/v1/users`

**Request**:
```json
{
  "email": "user@example.com",
  "api_key": "optional-custom-key"
}
```

**Response** (201 Created):
```json
{
  "user": {
    "email": "user@example.com",
    "created_at": "2025-10-28T...",
    "revoked": false
  },
  "api_key": "base64-encoded-api-key-here"
}
```

**Error Responses**:
- `400 Bad Request` - Invalid email or missing required fields
- `409 Conflict` - User with email already exists
- `500 Internal Server Error` - Database or system errors

### 5. Initialization
**Files**: `cmd/backend/aws/main.go`, `cmd/local/main.go`

Both entry points now:
1. Load AWS configuration (if available)
2. Create DynamoDB client
3. Read `API_KEYS_TABLE` environment variable
4. Initialize UserRepository (or nil if not configured)
5. Pass repository to service layer

**Local Development**: Can run without AWS credentials for basic operations.

## Dependencies Added

```go
github.com/aws/aws-sdk-go-v2 v1.32.7
github.com/aws/aws-sdk-go-v2/config v1.28.7
github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.24
github.com/aws/aws-sdk-go-v2/service/dynamodb v1.39.1
```

## Usage Examples

### Creating a User via API
```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com"}'
```

Response:
```json
{
  "user": {
    "email": "alice@example.com",
    "created_at": "2025-10-28T10:30:00Z",
    "revoked": false
  },
  "api_key": "dGhpc2lzYW5leGFtcGxla2V5MTIzNDU2"
}
```

### Using the API Key
Store the returned API key in `~/.runvoy/config.yaml`:
```yaml
api_endpoint: https://api.runvoy.io
api_key: dGhpc2lzYW5leGFtcGxla2V5MTIzNDU2
```

## Environment Variables

**Lambda/Production**:
- `API_KEYS_TABLE` - DynamoDB table name (default: `runvoy-api-keys`)

**Local Development**:
- `API_KEYS_TABLE` - Optional, enables user operations
- `PORT` - HTTP server port (default: `8080`)
- AWS credentials via standard AWS SDK environment variables

## Future Extensions

### Supporting Other Databases
To add PostgreSQL support:

1. Create `internal/database/postgres/users.go`:
```go
type UserRepository struct {
    db *sql.DB
}

func (r *UserRepository) CreateUser(ctx, user, apiKeyHash) error {
    _, err := r.db.ExecContext(ctx,
        "INSERT INTO users (email, api_key_hash, created_at) VALUES ($1, $2, $3)",
        user.Email, apiKeyHash, user.CreatedAt)
    return err
}
// ... implement other interface methods
```

2. Update initialization code to use Postgres repository when configured

The service layer requires zero changes!

### Additional Features
The abstraction supports adding:
- User quotas and rate limiting
- Multiple API keys per user
- API key expiration
- Audit logging of key usage
- Admin endpoints for key management

## Testing

### Unit Tests (TODO)
```bash
go test ./internal/app/...
go test ./internal/database/...
```

### Integration Tests (TODO)
Use DynamoDB Local or LocalStack for testing

### Manual Testing
```bash
# Start local server
export API_KEYS_TABLE=runvoy-api-keys
go run ./cmd/local

# Create user
curl -X POST localhost:8080/api/v1/users \
  -d '{"email": "test@example.com"}'
```

## Security Considerations

1. **API Key Storage**: Only SHA-256 hashes are stored
2. **Key Generation**: Uses `crypto/rand` for cryptographic security
3. **Transport**: HTTPS required in production (Lambda Function URL supports TLS)
4. **Revocation**: Soft delete via `revoked` flag preserves audit trail
5. **Rate Limiting**: Should be added in middleware (TODO)

## CloudFormation Integration

The existing `infra/cloudformation-backend.yaml` already defines:
- `APIKeysTable` with correct schema
- Lambda IAM permissions for DynamoDB operations
- Environment variable mapping for `API_KEYS_TABLE`

No infrastructure changes needed!

## Summary

This implementation provides:
- ✅ Clean separation of concerns
- ✅ Cloud provider abstraction
- ✅ Secure API key handling
- ✅ RESTful API endpoint
- ✅ Zero infrastructure changes
- ✅ Easy to extend and test

The codebase is now ready for user onboarding and API key management.
