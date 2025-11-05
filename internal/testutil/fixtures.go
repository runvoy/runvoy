// Package testutil provides shared testing utilities and helpers.
package testutil

import (
	"context"
	"log/slog"
	"os"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/constants"
)

// UserBuilder provides a fluent interface for building test users.
type UserBuilder struct {
	user *api.User
}

// NewUserBuilder creates a new UserBuilder with sensible defaults.
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: &api.User{
			Email:     "test@example.com",
			CreatedAt: time.Now().UTC(),
			Revoked:   false,
		},
	}
}

// WithEmail sets the user's email.
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

// WithCreatedAt sets the user's creation time.
func (b *UserBuilder) WithCreatedAt(t time.Time) *UserBuilder {
	b.user.CreatedAt = t
	return b
}

// WithLastUsed sets the user's last used time.
func (b *UserBuilder) WithLastUsed(t time.Time) *UserBuilder {
	b.user.LastUsed = &t
	return b
}

// Revoked marks the user as revoked.
func (b *UserBuilder) Revoked() *UserBuilder {
	b.user.Revoked = true
	return b
}

// Build returns the constructed User.
func (b *UserBuilder) Build() *api.User {
	return b.user
}

// ExecutionBuilder provides a fluent interface for building test executions.
type ExecutionBuilder struct {
	execution *api.Execution
}

// NewExecutionBuilder creates a new ExecutionBuilder with sensible defaults.
func NewExecutionBuilder() *ExecutionBuilder {
	return &ExecutionBuilder{
		execution: &api.Execution{
			ExecutionID: "exec-test-123",
			Command:     "echo 'test'",
			Status:      "pending",
			StartedAt:   time.Now().UTC(),
			UserEmail:   "test@example.com",
		},
	}
}

// WithExecutionID sets the execution ID.
func (b *ExecutionBuilder) WithExecutionID(id string) *ExecutionBuilder {
	b.execution.ExecutionID = id
	return b
}

// WithCommand sets the execution command.
func (b *ExecutionBuilder) WithCommand(cmd string) *ExecutionBuilder {
	b.execution.Command = cmd
	return b
}

// WithStatus sets the execution status.
func (b *ExecutionBuilder) WithStatus(status string) *ExecutionBuilder {
	b.execution.Status = status
	return b
}

// WithUserEmail sets the user email.
func (b *ExecutionBuilder) WithUserEmail(email string) *ExecutionBuilder {
	b.execution.UserEmail = email
	return b
}

// WithLogStreamName sets the log stream name.
func (b *ExecutionBuilder) WithLogStreamName(name string) *ExecutionBuilder {
	b.execution.LogStreamName = name
	return b
}

// Completed marks the execution as completed.
func (b *ExecutionBuilder) Completed() *ExecutionBuilder {
	now := time.Now().UTC()
	b.execution.Status = "completed"
	b.execution.CompletedAt = &now
	b.execution.ExitCode = 0
	return b
}

// Failed marks the execution as failed.
func (b *ExecutionBuilder) Failed() *ExecutionBuilder {
	now := time.Now().UTC()
	b.execution.Status = "failed"
	b.execution.CompletedAt = &now
	b.execution.ExitCode = 1
	return b
}

// Build returns the constructed Execution.
func (b *ExecutionBuilder) Build() *api.Execution {
	return b.execution
}

// TestContext creates a test context with a reasonable timeout.
// Note: The cancel function is intentionally not returned since test contexts
// are expected to be short-lived and will be cleaned up when the test completes.
func TestContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), constants.TestContextTimeout)
	_ = cancel // Silence unused warning - context will timeout automatically
	return ctx
}

// TestLogger creates a logger suitable for testing (outputs to stderr).
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// SilentLogger creates a logger that discards all output.
func SilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.NewFile(0, os.DevNull), &slog.HandlerOptions{
		Level: slog.LevelError + 1, // Suppress all logs
	}))
}
