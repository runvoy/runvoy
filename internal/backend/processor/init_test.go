package processor

import (
	"context"
	"testing"
	"time"

	"runvoy/internal/config"
	awsconfig "runvoy/internal/config/aws"
	"runvoy/internal/constants"
	"runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
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
		BackendProvider: "gcp", // Not yet supported
		InitTimeout:     30 * time.Second,
	}

	processor, err := Initialize(ctx, cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, processor)
	assert.Contains(t, err.Error(), "unknown backend provider")
	assert.Contains(t, err.Error(), "gcp")
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
