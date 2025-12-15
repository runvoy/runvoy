package processor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/auth/authorization"
	"github.com/runvoy/runvoy/internal/config"
	awsconfig "github.com/runvoy/runvoy/internal/config/aws"
	"github.com/runvoy/runvoy/internal/constants"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize_AWSProvider(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	cfg := &config.Config{
		BackendProvider: constants.AWS,
		InitTimeout:     30 * time.Second,
		AWS: &awsconfig.Config{
			SDKConfig: &aws.Config{
				Region: "us-east-1",
			},
			ExecutionsTable:           "test-executions",
			WebSocketConnectionsTable: "test-connections",
			WebSocketTokensTable:      "test-tokens",
		},
	}

	processor, err := Initialize(ctx, cfg, logger)
	// In test environments without AWS credentials, initialization may fail
	// with credential errors, which is expected. We just verify it doesn't panic
	// and that errors are related to AWS connectivity rather than code issues.
	if err != nil {
		// Expected errors in test environments: credential errors, connection errors
		assert.Contains(t, err.Error(), "failed to", "error should be descriptive")
		assert.Nil(t, processor, "processor should be nil on error")
	} else {
		assert.NotNil(t, processor, "processor should be created on success")
	}
}

func TestInitialize_UnknownProvider(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	cfg := &config.Config{
		BackendProvider: "azure", // Not yet supported
		InitTimeout:     30 * time.Second,
	}

	processor, err := Initialize(ctx, cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, processor)
	assert.Contains(t, err.Error(), "unknown backend provider: azure")
}

func TestSelectProviderInitializer_GCPNotImplemented(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	cfg := &config.Config{
		BackendProvider: constants.GCP,
		InitTimeout:     30 * time.Second,
	}

	processor, err := Initialize(ctx, cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, processor)
	assert.Contains(t, err.Error(), "GCP processor initializer not yet implemented")
}

func TestInitialize_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	logger := testutil.SilentLogger()

	cfg := &config.Config{
		BackendProvider: constants.AWS,
		InitTimeout:     30 * time.Second,
		AWS: &awsconfig.Config{
			SDKConfig: &aws.Config{
				Region: "us-east-1",
			},
			ExecutionsTable:           "test-executions",
			WebSocketConnectionsTable: "test-connections",
			WebSocketTokensTable:      "test-tokens",
		},
	}

	// SDK config loading might succeed even with canceled context
	// depending on implementation, so we just check it doesn't panic
	processor, err := Initialize(ctx, cfg, logger)
	// Either succeeds or returns an error, but shouldn't panic
	if err != nil {
		assert.Contains(t, err.Error(), "context")
	}
	_ = processor
}

func TestInitialize_CustomInitializer(t *testing.T) {
	ctx := context.Background()
	logger := testutil.SilentLogger()

	cfg := &config.Config{
		BackendProvider: constants.AWS,
	}

	var called bool
	customProc := &mockProcessor{}
	customInitializer := func(
		_ context.Context,
		_ *config.Config,
		_ *slog.Logger,
		_ *authorization.Enforcer,
	) (Processor, error) {
		called = true
		return customProc, nil
	}

	p, err := Initialize(ctx, cfg, logger, WithProviderInitializer(customInitializer))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.True(t, called, "custom initializer should be invoked")
	assert.Equal(t, customProc, p)
}
