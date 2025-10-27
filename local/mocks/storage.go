package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"runvoy/internal/api"
)

// MockStorage implements StorageService for local testing
type MockStorage struct {
	mu          sync.RWMutex
	users       map[string]*api.User
	executions  map[string]*api.Execution
	locks       map[string]*api.Lock
	apiKeyIndex map[string]string // apiKeyHash -> email
}

// NewMockStorage creates a new mock storage
func NewMockStorage() *MockStorage {
	return &MockStorage{
		users:       make(map[string]*api.User),
		executions:  make(map[string]*api.Execution),
		locks:       make(map[string]*api.Lock),
		apiKeyIndex: make(map[string]string),
	}
}

// User operations
func (m *MockStorage) GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*api.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	email, exists := m.apiKeyIndex[apiKeyHash]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	user, exists := m.users[email]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

func (m *MockStorage) CreateUser(ctx context.Context, user *api.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users[user.Email] = user
	if user.APIKey != "" {
		// In real implementation, this would be the hash
		m.apiKeyIndex[user.APIKey] = user.Email
	}

	return nil
}

func (m *MockStorage) UpdateUser(ctx context.Context, user *api.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.users[user.Email] = user
	return nil
}

func (m *MockStorage) DeleteUser(ctx context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.users, email)
	return nil
}

// Execution operations
func (m *MockStorage) CreateExecution(ctx context.Context, execution *api.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executions[execution.ExecutionID] = execution
	return nil
}

func (m *MockStorage) GetExecution(ctx context.Context, executionID string) (*api.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	execution, exists := m.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution not found")
	}

	return execution, nil
}

func (m *MockStorage) UpdateExecution(ctx context.Context, execution *api.Execution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executions[execution.ExecutionID] = execution
	return nil
}

func (m *MockStorage) ListExecutions(ctx context.Context, userEmail string, limit int) ([]*api.Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*api.Execution
	for _, execution := range m.executions {
		if execution.UserEmail == userEmail {
			results = append(results, execution)
		}
	}

	// Simple limit implementation
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Lock operations
func (m *MockStorage) CreateLock(ctx context.Context, lock *api.Lock) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.locks[lock.LockName] = lock
	return nil
}

func (m *MockStorage) GetLock(ctx context.Context, lockName string) (*api.Lock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, exists := m.locks[lockName]
	if !exists {
		return nil, fmt.Errorf("lock not found")
	}

	return lock, nil
}

func (m *MockStorage) DeleteLock(ctx context.Context, lockName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.locks, lockName)
	return nil
}