package testing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"runvoy/internal/handlers"
	"runvoy/internal/services"
)

// TestServer provides a test HTTP server with mock services
type TestServer struct {
	Server *httptest.Server
	Mocks  *MockServices
}

// MockServices contains all mock services
type MockServices struct {
	Auth      *MockAuthService
	Execution *MockExecutionService
	Storage   *MockStorageService
	ECS       *MockECSService
	Lock      *MockLockService
	Log       *MockLogService
}

// NewTestServer creates a new test server with mocks
func NewTestServer(t *testing.T) *TestServer {
	mocks := &MockServices{
		Auth:      NewMockAuthService(t),
		Storage:   NewMockStorageService(t),
		ECS:       NewMockECSService(t),
		Lock:      NewMockLockService(t),
		Log:       NewMockLogService(t),
	}

	// Create execution service with mocks
	execution := services.NewExecutionService(mocks.Storage, mocks.ECS, mocks.Lock, mocks.Log)
	mocks.Execution = NewMockExecutionService(t)

	// Create handlers
	h := handlers.NewHandlers(mocks.Auth, execution)

	// Create test server
	mux := http.NewServeMux()
	mux.HandleFunc("/executions", h.ExecutionHandler)
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/status/", h.StatusHandler)

	server := httptest.NewServer(mux)

	return &TestServer{
		Server: server,
		Mocks:  mocks,
	}
}

// Close closes the test server
func (ts *TestServer) Close() {
	ts.Server.Close()
}

// Client returns an HTTP client for testing
func (ts *TestServer) Client() *http.Client {
	return ts.Server.Client()
}

// URL returns the server URL
func (ts *TestServer) URL() string {
	return ts.Server.URL
}