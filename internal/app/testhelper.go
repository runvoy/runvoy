package app

import (
	"log/slog"

	"runvoy/internal/constants"
	"runvoy/internal/database"
	"runvoy/internal/websocket"
)

// NewTestService constructs a Service with the provided dependencies for use in tests.
func NewTestService(
	userRepo database.UserRepository,
	executionRepo database.ExecutionRepository,
	connRepo database.ConnectionRepository,
	tokenRepo database.TokenRepository,
	runner Runner,
	log *slog.Logger,
	provider constants.BackendProvider,
	wsManager websocket.Manager,
) *Service {
	return &Service{
		userRepo:      userRepo,
		executionRepo: executionRepo,
		connRepo:      connRepo,
		tokenRepo:     tokenRepo,
		runner:        runner,
		Logger:        log,
		Provider:      provider,
		wsManager:     wsManager,
	}
}
