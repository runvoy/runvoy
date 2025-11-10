# Testing Examples: Before and After

This document shows practical examples of how to refactor existing code for better testability.

## Example 1: Database Repository with Mocking

### Before: Untestable Code

```go
// internal/providers/aws/database/dynamodb/users.go
package dynamodb

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"runvoy/internal/api"
)

type UserRepository struct {
	client    *dynamodb.Client  // Concrete type, can't mock
	tableName string
}

func (r *UserRepository) CreateUser(ctx context.Context, user *api.User) error {
	// Direct call to AWS - requires real DynamoDB or complex test setup
	_, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
		// ... setup
	})
	return err
}
```

**Problems:**
- Uses concrete `*dynamodb.Client` type
- Can't mock AWS calls
- Tests require real DynamoDB or expensive test harness
- No way to test error conditions

### After: Testable Code

```go
// internal/database/repository.go
package database

// Define interface for DynamoDB operations we need
type DynamoDBClient interface {
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// internal/providers/aws/database/dynamodb/users.go
type UserRepository struct {
	client    DynamoDBClient  // Use interface
	tableName string
	logger    *slog.Logger
}

func NewUserRepository(client DynamoDBClient, tableName string, logger *slog.Logger) *UserRepository {
	return &UserRepository{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}
}
```

**Test with Mock:**

```go
// internal/providers/aws/database/dynamodb/users_test.go
package dynamodb

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"runvoy/internal/api"
	"runvoy/internal/testutil"
)

// MockDynamoDBClient implements DynamoDBClient interface
type MockDynamoDBClient struct {
	mock.Mock
}

func (m *MockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

func TestUserRepository_CreateUser(t *testing.T) {
	tests := []struct {
		name      string
		user      *api.User
		setupMock func(*MockDynamoDBClient)
		wantErr   bool
	}{
		{
			name: "successfully creates user",
			user: testutil.NewUserBuilder().WithEmail("test@example.com").Build(),
			setupMock: func(m *MockDynamoDBClient) {
				m.On("PutItem", mock.Anything, mock.Anything).
					Return(&dynamodb.PutItemOutput{}, nil)
			},
			wantErr: false,
		},
		{
			name: "handles duplicate user error",
			user: testutil.NewUserBuilder().WithEmail("existing@example.com").Build(),
			setupMock: func(m *MockDynamoDBClient) {
				m.On("PutItem", mock.Anything, mock.Anything).
					Return(nil, &types.ConditionalCheckFailedException{})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDynamoDBClient)
			tt.setupMock(mockClient)

			repo := NewUserRepository(mockClient, "test-table", testutil.SilentLogger())

			err := repo.CreateUser(context.Background(), tt.user, "test-hash")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
```

---

## Example 2: HTTP Handler Testing

### Before: Untestable Handler

```go
// internal/server/handlers.go
func (router *Router) handleRun(w http.ResponseWriter, r *http.Request) {
	// Tightly coupled to concrete implementations
	var req api.RunRequest
	json.NewDecoder(r.Body).Decode(&req)

	// Direct service call - hard to test
	execution, err := router.svc.Run(r.Context(), &req)
	if err != nil {
		// Error handling
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(execution)
}
```

**Problems:**
- No input validation testing
- No error path testing
- Requires real service implementation

### After: Testable Handler

```go
// internal/server/handlers.go (improved)
func (router *Router) handleRun(w http.ResponseWriter, r *http.Request) {
	var req api.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		router.respondError(w, apperrors.ErrBadRequest("invalid request body", err))
		return
	}

	// Validate request
	if req.Command == "" {
		router.respondError(w, apperrors.ErrBadRequest("command is required", nil))
		return
	}

	execution, err := router.svc.Run(r.Context(), &req)
	if err != nil {
		router.respondError(w, err)
		return
	}

	router.respondJSON(w, http.StatusOK, execution)
}

func (router *Router) respondError(w http.ResponseWriter, err error) {
	var statusCode int
	switch {
	case errors.Is(err, apperrors.ErrBadRequest("", nil)):
		statusCode = http.StatusBadRequest
	case errors.Is(err, apperrors.ErrUnauthorized("", nil)):
		statusCode = http.StatusUnauthorized
	case errors.Is(err, apperrors.ErrNotFound("", nil)):
		statusCode = http.StatusNotFound
	default:
		statusCode = http.StatusInternalServerError
	}

	router.respondJSON(w, statusCode, map[string]string{"error": err.Error()})
}
```

**Test:**

```go
// internal/server/handlers_test.go
func TestRouter_HandleRun(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupMock      func(*MockService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "successfully starts execution",
			requestBody: `{"command": "echo hello"}`,
			setupMock: func(m *MockService) {
				m.On("Run", mock.Anything, mock.MatchedBy(func(req *api.RunRequest) bool {
					return req.Command == "echo hello"
				})).Return(&api.Execution{
					ID:      "exec-123",
					Status:  "pending",
					Command: "echo hello",
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var response api.Execution
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "exec-123", response.ID)
				assert.Equal(t, "pending", response.Status)
			},
		},
		{
			name:           "returns 400 for empty command",
			requestBody:    `{"command": ""}`,
			setupMock:      func(m *MockService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder) {
				assert.Contains(t, rr.Body.String(), "command is required")
			},
		},
		{
			name:           "returns 400 for invalid JSON",
			requestBody:    `{invalid json}`,
			setupMock:      func(m *MockService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "handles service error",
			requestBody: `{"command": "test"}`,
			setupMock: func(m *MockService) {
				m.On("Run", mock.Anything, mock.Anything).
					Return(nil, apperrors.ErrUnauthorized("invalid API key", nil))
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockService)
			tt.setupMock(mockSvc)

			router := NewRouter(mockSvc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/run",
				strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rr)
			}

			mockSvc.AssertExpectations(t)
		})
	}
}
```

---

## Example 3: Testing with Test Fixtures

### Before: Repetitive Test Setup

```go
func TestSomething(t *testing.T) {
	user := &api.User{
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	// Use user...
}

func TestSomethingElse(t *testing.T) {
	user := &api.User{
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		Revoked:   false,
	}
	// Use user...
}
```

**Problems:**
- Duplicated setup code
- Hard to maintain when User struct changes
- No clear intent of what's important in each test

### After: Using Fixtures and Builders

```go
// internal/testutil/fixtures.go
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &api.User{
			Email:     "test@example.com",
			CreatedAt: time.Now().UTC(),
			Revoked:   false,
		},
	}
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

func (b *UserBuilder) Revoked() *UserBuilder {
	b.user.Revoked = true
	return b
}

func (b *UserBuilder) Build() *api.User {
	return b.user
}
```

**Tests become clearer:**

```go
func TestCreateUser_Success(t *testing.T) {
	user := testutil.NewUserBuilder().Build()
	// Only default values, test focuses on happy path
}

func TestCreateUser_WithCustomEmail(t *testing.T) {
	user := testutil.NewUserBuilder().
		WithEmail("custom@example.com").
		Build()
	// Intent is clear: we're testing with a custom email
}

func TestGetUser_RevokedUser(t *testing.T) {
	revokedUser := testutil.NewUserBuilder().
		Revoked().
		Build()
	// Intent is clear: testing revoked user behavior
}
```

---

## Example 4: Error Path Testing

### Before: Only Happy Path Tested

```go
func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig("config.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
}
```

**Problems:**
- Only tests successful parsing
- No validation of error conditions
- Missing edge cases

### After: Comprehensive Error Testing

```go
func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errType   error
	}{
		{
			name: "successfully parses valid config",
			content: `
api_endpoint: https://api.example.com
api_key: test-key
`,
			wantErr: false,
		},
		{
			name:    "returns error for invalid YAML",
			content: `invalid: yaml: content:`,
			wantErr: true,
		},
		{
			name: "returns error for missing required field",
			content: `
api_endpoint: https://api.example.com
# api_key missing
`,
			wantErr: true,
			errType: apperrors.ErrValidation("", nil),
		},
		{
			name: "returns error for invalid URL",
			content: `
api_endpoint: not-a-url
api_key: test-key
`,
			wantErr: true,
		},
		{
			name:    "returns error for empty file",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with test content
			tmpFile := createTempFile(t, tt.content)
			defer os.Remove(tmpFile)

			cfg, err := ParseConfig(tmpFile)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				assert.Nil(t, cfg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				assert.NotEmpty(t, cfg.APIEndpoint)
				assert.NotEmpty(t, cfg.APIKey)
			}
		})
	}
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	return tmpFile.Name()
}
```

---

## Example 5: Table-Driven Tests

### Before: Repetitive Test Functions

```go
func TestFormatDuration_Seconds(t *testing.T) {
	result := FormatDuration(5 * time.Second)
	assert.Equal(t, "5s", result)
}

func TestFormatDuration_Minutes(t *testing.T) {
	result := FormatDuration(2 * time.Minute)
	assert.Equal(t, "2m0s", result)
}

func TestFormatDuration_Hours(t *testing.T) {
	result := FormatDuration(1 * time.Hour)
	assert.Equal(t, "1h0m0s", result)
}
```

**Problems:**
- Lots of repetitive code
- Hard to add new cases
- Each test needs to be run separately

### After: Table-Driven Test

```go
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "seconds only",
			duration: 5 * time.Second,
			want:     "5s",
		},
		{
			name:     "minutes and seconds",
			duration: 2*time.Minute + 30*time.Second,
			want:     "2m30s",
		},
		{
			name:     "hours, minutes, and seconds",
			duration: 1*time.Hour + 30*time.Minute + 45*time.Second,
			want:     "1h30m45s",
		},
		{
			name:     "large duration",
			duration: 24 * time.Hour,
			want:     "24h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

**Benefits:**
- Easy to add new test cases
- All cases run together
- Clear test structure
- Subtests provide isolation

---

## Example 6: Integration Test Setup

```go
//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"

	dynamodbrepo "runvoy/internal/providers/aws/database/dynamodb"
	"runvoy/internal/testutil"
)

// setupTestDB creates a DynamoDB client pointing to DynamoDB Local
func setupTestDB(t *testing.T) *dynamodb.Client {
	t.Helper()

	endpoint := "http://localhost:8000"

	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		},
	)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-west-2"),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	require.NoError(t, err)

	return dynamodb.NewFromConfig(cfg)
}

func TestUserRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := setupTestDB(t)
	tableName := "test-users"

	// Create table (in real setup, this would be in TestMain)
	createTestTable(t, client, tableName)
	defer deleteTestTable(t, client, tableName)

	repo := dynamodbrepo.NewUserRepository(client, tableName, testutil.TestLogger())

	t.Run("create and retrieve user", func(t *testing.T) {
		user := testutil.NewUserBuilder().
			WithEmail("integration-test@example.com").
			Build()

		err := repo.CreateUser(context.Background(), user, "test-hash-123")
		require.NoError(t, err)

		retrieved, err := repo.GetUserByEmail(context.Background(), user.Email)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, user.Email, retrieved.Email)
	})

	t.Run("handles duplicate user creation", func(t *testing.T) {
		user := testutil.NewUserBuilder().
			WithEmail("duplicate@example.com").
			Build()

		// First creation should succeed
		err := repo.CreateUser(context.Background(), user, "hash-1")
		require.NoError(t, err)

		// Second creation with same hash should fail
		err = repo.CreateUser(context.Background(), user, "hash-1")
		assert.Error(t, err)
		assert.ErrorIs(t, err, apperrors.ErrConflict("", nil))
	})
}
```

---

## Running Tests

```bash
# Unit tests only (fast)
go test ./...
just test

# Integration tests
go test -tags=integration ./...
just test-integration

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package
go test ./internal/auth/...

# Run specific test
go test -run TestGenerateAPIKey ./internal/auth/

# Verbose output
go test -v ./...

# Short mode (skip slow tests)
go test -short ./...
```

---

## Key Takeaways

1. **Use interfaces** for external dependencies (databases, APIs, etc.)
2. **Use test fixtures** and builders to reduce boilerplate
3. **Test error paths** as thoroughly as happy paths
4. **Use table-driven tests** for multiple similar scenarios
5. **Write integration tests** for critical workflows
6. **Keep tests fast** - unit tests should run in milliseconds
7. **Make tests readable** - they serve as documentation
8. **Use mocks appropriately** - don't mock everything
