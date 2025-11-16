// Package orchestrator provides AWS-specific implementations for runvoy orchestrator.
// This file provides an adapter for the health manager to use orchestrator functions.
package orchestrator

// Package orchestrator previously provided an adapter to expose task
// definition recreation logic to the health manager. That logic has been
// moved to the ecsdefs package, and the health manager now depends on it
// directly, so this adapter is no longer needed.
