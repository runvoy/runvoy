// Package constants defines global constants used throughout runvoy.
package constants

import "slices"

// ExecutionStatus represents the business-level status of a command execution.
// This is distinct from EcsStatus, which reflects the AWS ECS task lifecycle.
// Execution statuses are used throughout the API and stored in the database.
type ExecutionStatus string

const (
	// ExecutionStarting indicates the command has been accepted and is being scheduled
	ExecutionStarting ExecutionStatus = "STARTING"
	// ExecutionRunning indicates the command is currently executing
	ExecutionRunning ExecutionStatus = "RUNNING"
	// ExecutionSucceeded indicates the command completed successfully
	ExecutionSucceeded ExecutionStatus = "SUCCEEDED"
	// ExecutionFailed indicates the command failed with an error
	ExecutionFailed ExecutionStatus = "FAILED"
	// ExecutionStopped indicates the command was manually terminated
	ExecutionStopped ExecutionStatus = "STOPPED"
	// ExecutionTerminating indicates a stop request is in progress
	ExecutionTerminating ExecutionStatus = "TERMINATING"
)

// TerminalExecutionStatuses returns all statuses that represent completed executions
func TerminalExecutionStatuses() []ExecutionStatus {
	return []ExecutionStatus{
		ExecutionFailed,
		ExecutionStopped,
		ExecutionSucceeded,
		ExecutionTerminating,
	}
}

// validTransitions defines the allowed state transitions for execution statuses.
// Each key represents a source status, and the value is a slice of allowed destination statuses.
var validTransitions = map[ExecutionStatus][]ExecutionStatus{
	ExecutionStarting:    {ExecutionRunning, ExecutionFailed, ExecutionTerminating},
	ExecutionRunning:     {ExecutionSucceeded, ExecutionFailed, ExecutionStopped, ExecutionTerminating},
	ExecutionTerminating: {ExecutionStopped},
	// Terminal states (SUCCEEDED, FAILED, STOPPED) have no valid transitions
	ExecutionSucceeded: {},
	ExecutionFailed:    {},
	ExecutionStopped:   {},
}

// CanTransition checks if a status transition from 'from' to 'to' is valid.
// Returns true if the transition is allowed, false otherwise.
// If the source status is not in the validTransitions map, returns false.
func CanTransition(from, to ExecutionStatus) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(allowed, to)
}
