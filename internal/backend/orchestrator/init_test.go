package orchestrator

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/database"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize_UsesCustomInitializer(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	cfg := &config.Config{BackendProvider: constants.AWS}
	log := testutil.SilentLogger()
	runner := &mockRunner{}

	deps := &ProviderDependencies{
		Repositories: database.Repositories{
			User:       &mockUserRepository{},
			Execution:  &mockExecutionRepository{},
			Connection: &mockConnectionRepository{},
			Token:      &mockTokenRepository{},
			Image:      stubImageRepository{},
			Secrets:    &mockSecretsRepository{},
		},
		TaskManager:          runner,
		ImageRegistry:        runner,
		LogManager:           runner,
		ObservabilityManager: runner,
		WebSocketManager:     &mockWebSocketManager{},
		HealthManager:        &stubHealthManager{},
	}

	var called bool
	initializer := func(
		_ context.Context,
		_ *config.Config,
		_ *slog.Logger,
		_ *authorization.Enforcer,
	) (*ProviderDependencies, error) {
		called = true
		return deps, nil
	}

	svc, err := Initialize(ctx, cfg, log, WithProviderInitializer(initializer))
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.True(t, called, "custom initializer should be invoked")
}

func TestSelectProviderInitializer_DefaultAWS(t *testing.T) {
	initializer, err := selectProviderInitializer(constants.AWS, nil)

	require.NoError(t, err)
	require.NotNil(t, initializer)
	assert.Equal(t,
		reflect.ValueOf(awsProviderInitializer).Pointer(),
		reflect.ValueOf(initializer).Pointer(),
	)
}

func TestSelectProviderInitializer_UnknownProvider(t *testing.T) {
	initializer, err := selectProviderInitializer("gcp", nil)

	require.Error(t, err)
	assert.Nil(t, initializer)
	assert.Contains(t, err.Error(), "unknown backend provider: gcp")
}

// stubImageRepository satisfies both database.ImageRepository and authorization.ImageRepository for tests.
type stubImageRepository struct{}

func (stubImageRepository) GetImagesByRequestID(_ context.Context, _ string) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
}

func (stubImageRepository) ListImages(_ context.Context) ([]api.ImageInfo, error) {
	return []api.ImageInfo{}, nil
}
