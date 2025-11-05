# CLI Commands Testability Analysis

**Related to:** GitHub Issue #60  
**Date:** 2025-11-02  
**Objective:** Identify easy tests to add and propose refactorings to improve CLI command testability

---

## Executive Summary

The CLI commands in `cmd/cli/cmd/` currently have **zero test coverage**. Most commands are tightly coupled to external dependencies (HTTP client, file system, environment variables), making them difficult to test in their current form.

**Quick Wins Available:**
- ? `parseTimeout()` function can be tested immediately
- ? Helper functions can be extracted and tested independently

**Refactoring Needed:**
- Extract business logic from cobra command handlers
- Inject dependencies instead of creating them directly
- Create interfaces for testability
- Separate I/O operations from business logic

---

## Current State Analysis

### Commands Without Tests

All CLI commands lack tests:
- `logs.go` - No tests
- `run.go` - No tests  
- `status.go` - No tests
- `claim.go` - No tests
- `configure.go` - No tests
- `kill.go` - No tests
- `list.go` - No tests
- `users.go` - No tests
- `images.go` - No tests

### Testability Issues

**1. Direct Dependency Creation**
```go
// ? Hard to test - creates real client
c := client.New(cfg, slog.Default())
resp, err := c.GetLogs(cmd.Context(), executionID)
```

**2. Global I/O Operations**
```go
// ? Hard to verify output
output.Infof("Getting logs for execution: %s", output.Bold(executionID))
```

**3. Direct File System Access**
```go
// ? Hard to test - accesses real file system
cfg, err := getConfigFromContext(cmd)
```

**4. Environment Variable Access**
```go
// ? Hard to test - accesses real environment
for _, env := range os.Environ() {
    // ...
}
```

---

## Easy Tests to Add (Quick Wins)

### 1. Test `parseTimeout()` Function

**Location:** `cmd/cli/cmd/root.go:125`

This is a pure function that can be tested immediately without any refactoring.

**Test Cases:**
```go
// cmd/cli/cmd/root_test.go
func TestParseTimeout(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        want      time.Duration
        wantError bool
    }{
        {
            name:  "valid duration minutes",
            input: "10m",
            want:  10 * time.Minute,
        },
        {
            name:  "valid duration seconds",
            input: "30s",
            want:  30 * time.Second,
        },
        {
            name:  "valid duration hours",
            input: "1h",
            want:  time.Hour,
        },
        {
            name:  "valid seconds as integer",
            input: "600",
            want:  600 * time.Second,
        },
        {
            name:      "invalid format",
            input:     "invalid",
            wantError: true,
        },
        {
            name:      "empty string defaults to 10m",
            input:     "",
            want:      10 * time.Minute,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parseTimeout(tt.input)
            if tt.wantError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
```

**Impact:** ? Can be added immediately, no refactoring needed

---

## Refactoring Proposals

### Refactoring 1: Extract Helper Functions from `logs.go`

**Current Code:**
```go
func logsRun(cmd *cobra.Command, args []string) {
    executionID := args[0]
    cfg, err := getConfigFromContext(cmd)
    // ... all logic in one function
}
```

**Proposed Refactoring:**

1. **Create a service struct** that encapsulates the logic:
```go
// logs.go
type LogsService struct {
    client ClientInterface
    output OutputInterface
}

type ClientInterface interface {
    GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error)
}

type OutputInterface interface {
    Infof(format string, a ...interface{})
    Errorf(format string, a ...interface{})
    Table(headers []string, rows [][]string)
    Blank()
    Successf(format string, a ...interface{})
    Bold(text string) string
    Cyan(text string) string
}

func NewLogsService(client ClientInterface, output OutputInterface) *LogsService {
    return &LogsService{
        client: client,
        output: output,
    }
}

func (s *LogsService) DisplayLogs(ctx context.Context, executionID string, webURL string) error {
    resp, err := s.client.GetLogs(ctx, executionID)
    if err != nil {
        return fmt.Errorf("failed to get logs: %w", err)
    }

    s.output.Blank()
    rows := [][]string{}
    for _, log := range resp.Events {
        rows = append(rows, []string{
            s.output.Bold(fmt.Sprintf("%d", log.Line)),
            time.Unix(log.Timestamp/constants.MillisecondsPerSecond, 0).UTC().Format(time.DateTime),
            log.Message,
        })
    }
    s.output.Table([]string{"Line", "Timestamp (UTC)", "Message"}, rows)
    s.output.Blank()
    s.output.Successf("Logs retrieved successfully")
    s.output.Infof("View logs in web viewer: %s?execution_id=%s",
        webURL, s.output.Cyan(executionID))
    return nil
}

// Updated command handler
func logsRun(cmd *cobra.Command, args []string) {
    executionID := args[0]
    cfg, err := getConfigFromContext(cmd)
    if err != nil {
        output.Errorf("failed to load configuration: %v", err)
        return
    }

    c := client.New(cfg, slog.Default())
    service := NewLogsService(c, outputWrapper{})
    if err := service.DisplayLogs(cmd.Context(), executionID, cfg.WebURL); err != nil {
        output.Errorf(err.Error())
    }
}
```

2. **Create an output wrapper** that implements the interface:
```go
// output_wrapper.go
type outputWrapper struct{}

func (o outputWrapper) Infof(format string, a ...interface{}) {
    output.Infof(format, a...)
}
// ... implement other methods
```

3. **Now testable:**
```go
// logs_test.go
func TestLogsService_DisplayLogs(t *testing.T) {
    tests := []struct {
        name        string
        executionID string
        mockClient  func(*mocks.MockClientInterface)
        wantErr     bool
    }{
        {
            name:        "successfully displays logs",
            executionID: "exec-123",
            mockClient: func(m *mocks.MockClientInterface) {
                m.EXPECT().
                    GetLogs(gomock.Any(), "exec-123").
                    Return(&api.LogsResponse{
                        ExecutionID: "exec-123",
                        Events: []api.LogEvent{
                            {Line: 1, Timestamp: 1000, Message: "test log"},
                        },
                    }, nil)
            },
            wantErr: false,
        },
        {
            name:        "handles client error",
            executionID: "exec-123",
            mockClient: func(m *mocks.MockClientInterface) {
                m.EXPECT().
                    GetLogs(gomock.Any(), "exec-123").
                    Return(nil, errors.New("network error"))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockClient := mocks.NewMockClientInterface(ctrl)
            mockOutput := mocks.NewMockOutputInterface(ctrl)
            tt.mockClient(mockClient)

            // Set expectations on output
            if !tt.wantErr {
                mockOutput.EXPECT().Blank().Times(2)
                mockOutput.EXPECT().Table(gomock.Any(), gomock.Any())
                mockOutput.EXPECT().Successf(gomock.Any())
                mockOutput.EXPECT().Infof(gomock.Any(), gomock.Any(), gomock.Any())
            }

            service := NewLogsService(mockClient, mockOutput)
            err := service.DisplayLogs(context.Background(), tt.executionID, "https://example.com")
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Benefits:**
- ? Business logic separated from cobra integration
- ? Can test logic with mocks
- ? Easier to maintain and understand
- ? Can be reused in other contexts

---

### Refactoring 2: Extract Environment Variable Parsing from `run.go`

**Current Code:**
```go
envs := make(map[string]string)
for _, env := range os.Environ() {
    parts := strings.SplitN(env, "=", constants.EnvVarSplitLimit)
    if len(parts) == 2 && strings.HasPrefix(parts[0], "RUNVOY_USER_") {
        envs[strings.TrimPrefix(parts[0], "RUNVOY_USER_")] = parts[1]
    }
}
```

**Proposed Refactoring:**

Extract to a pure function:
```go
// run.go
func extractUserEnvVars(envVars []string) map[string]string {
    envs := make(map[string]string)
    for _, env := range envVars {
        parts := strings.SplitN(env, "=", constants.EnvVarSplitLimit)
        if len(parts) == 2 && strings.HasPrefix(parts[0], "RUNVOY_USER_") {
            envs[strings.TrimPrefix(parts[0], "RUNVOY_USER_")] = parts[1]
        }
    }
    return envs
}

// Updated command
envs := extractUserEnvVars(os.Environ())
```

**Test:**
```go
func TestExtractUserEnvVars(t *testing.T) {
    tests := []struct {
        name     string
        envVars  []string
        want     map[string]string
    }{
        {
            name: "extracts RUNVOY_USER_ prefixed vars",
            envVars: []string{
                "RUNVOY_USER_API_KEY=abc123",
                "RUNVOY_USER_TOKEN=xyz789",
                "PATH=/usr/bin",
            },
            want: map[string]string{
                "API_KEY": "abc123",
                "TOKEN":   "xyz789",
            },
        },
        {
            name:    "returns empty map when no matches",
            envVars: []string{"PATH=/usr/bin", "HOME=/home/user"},
            want:    map[string]string{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := extractUserEnvVars(tt.envVars)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

**Benefits:**
- ? Pure function, easy to test
- ? No dependency on `os.Environ()`
- ? Can test edge cases easily

---

### Refactoring 3: Create Client Interface for Dependency Injection

**Current Issue:** `client.New()` creates a concrete type that's hard to mock.

**Proposed Solution:**

1. **Create an interface** in the client package:
```go
// internal/client/interface.go
package client

type Interface interface {
    GetLogs(ctx context.Context, executionID string) (*api.LogsResponse, error)
    GetExecutionStatus(ctx context.Context, executionID string) (*api.ExecutionStatusResponse, error)
    RunCommand(ctx context.Context, req api.ExecutionRequest) (*api.ExecutionResponse, error)
    KillExecution(ctx context.Context, executionID string) (*api.KillExecutionResponse, error)
    ListExecutions(ctx context.Context) ([]api.Execution, error)
    ClaimAPIKey(ctx context.Context, token string) (*api.ClaimAPIKeyResponse, error)
    CreateUser(ctx context.Context, req api.CreateUserRequest) (*api.CreateUserResponse, error)
    RevokeUser(ctx context.Context, req api.RevokeUserRequest) (*api.RevokeUserResponse, error)
    ListUsers(ctx context.Context) (*api.ListUsersResponse, error)
    RegisterImage(ctx context.Context, image string, isDefault *bool) (*api.RegisterImageResponse, error)
    ListImages(ctx context.Context) (*api.ListImagesResponse, error)
    UnregisterImage(ctx context.Context, image string) (*api.RemoveImageResponse, error)
}
```

2. **Update Client to implement the interface** (it already does implicitly)

3. **Use interface in commands:**
```go
// logs.go
func logsRun(cmd *cobra.Command, args []string) {
    executionID := args[0]
    cfg, err := getConfigFromContext(cmd)
    if err != nil {
        output.Errorf("failed to load configuration: %v", err)
        return
    }

    var c client.Interface = client.New(cfg, slog.Default())
    // Now easy to substitute with mock in tests
}
```

**Benefits:**
- ? Allows dependency injection
- ? No breaking changes to existing code
- ? Can use `client.New()` in production, mocks in tests

---

### Refactoring 4: Extract Output Interface

**Current Issue:** Global `output.*` functions are hard to verify in tests.

**Proposed Solution:**

1. **Create an interface** (or use the existing package with a wrapper):
```go
// cmd/cli/cmd/output_interface.go
type OutputInterface interface {
    Infof(format string, a ...interface{})
    Errorf(format string, a ...interface{})
    Successf(format string, a ...interface{})
    Warningf(format string, a ...interface{})
    Table(headers []string, rows [][]string)
    Blank()
    Bold(text string) string
    Cyan(text string) string
    KeyValue(key, value string)
}
```

2. **Use dependency injection:**
```go
type LogsService struct {
    client client.Interface
    output OutputInterface  // Inject instead of using global
}
```

**Benefits:**
- ? Can capture output in tests
- ? Can verify formatting without capturing stdout
- ? Can use real output in production

---

## Implementation Priority

### Phase 1: Quick Wins (1-2 days)
1. ? Add tests for `parseTimeout()`
2. ? Extract and test `extractUserEnvVars()`

### Phase 2: Low-Risk Refactoring (3-5 days)
1. Extract helper functions from each command
2. Create client interface
3. Add unit tests for extracted functions

### Phase 3: Full Refactoring (1-2 weeks)
1. Refactor all commands to use dependency injection
2. Create comprehensive test suites
3. Achieve >80% coverage for CLI commands

---

## Testing Patterns

### Pattern 1: Pure Function Testing
```go
// Test pure functions with table-driven tests
func TestPureFunction(t *testing.T) {
    tests := []struct {
        name string
        input string
        want string
    }{
        // ...
    }
    // ...
}
```

### Pattern 2: Service Testing with Mocks
```go
// Test services with mocked dependencies
func TestService_Method(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockClient := mocks.NewMockClientInterface(ctrl)
    mockOutput := mocks.NewMockOutputInterface(ctrl)
    
    service := NewService(mockClient, mockOutput)
    // ...
}
```

### Pattern 3: Integration Testing
```go
//go:build integration

// Test full command execution with test server
func TestLogsCommand_Integration(t *testing.T) {
    // Start test HTTP server
    // Execute cobra command
    // Verify output
}
```

---

## Example: Complete Refactored `logs.go` with Tests

See `docs/TESTING_EXAMPLES.md` for a complete example implementation.

---

## Recommendations

1. **Start Small:** Begin with `parseTimeout()` test - it's pure and has no dependencies
2. **Extract Gradually:** Refactor one command at a time, starting with the simplest (`status.go`)
3. **Maintain Compatibility:** Keep existing command signatures working during refactoring
4. **Use Interfaces:** Create interfaces for all external dependencies
5. **Test Business Logic:** Focus on testing business logic, not cobra integration

---

## Success Metrics

- ? All pure functions have tests
- ? Business logic extracted from cobra handlers
- ? >80% test coverage for CLI commands
- ? All commands can be tested with mocks
- ? Integration tests for critical user flows

---

**Next Steps:**
1. Review this document with the team
2. Prioritize which commands to refactor first
3. Start with Phase 1 quick wins
4. Gradually move to Phase 2 and 3
