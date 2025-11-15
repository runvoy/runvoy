// Package app provides the core application orchestrator and related components.
// This package acts as a container for the application's main components:
// - orchestrator: Command execution and API orchestration
// - processor: Event processing (CloudWatch Logs, ECS task completions, WebSocket lifecycle)
// - websocket: WebSocket connection management
package app

// Re-export orchestrator types for backward compatibility
import (
	"runvoy/internal/app/orchestrator"
)

// Service is re-exported from the orchestrator package for backward compatibility.
// It represents the main application service that orchestrates command execution and API requests.
type Service = orchestrator.Service

// NewService is re-exported from the orchestrator package for backward compatibility.
var NewService = orchestrator.NewService

// Initialize is re-exported from the orchestrator package for backward compatibility.
var Initialize = orchestrator.Initialize
