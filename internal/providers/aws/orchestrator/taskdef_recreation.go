// Package orchestrator previously contained reusable task definition recreation
// logic for the health manager. That logic now lives in the ecsdefs package so
// it can be reused without introducing circular dependencies between packages.
package orchestrator
