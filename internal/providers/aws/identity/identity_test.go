package identity

import (
	"context"
	"log/slog"
	"testing"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestGetAccountID(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("function can be called", func(t *testing.T) {
		awsCfg := &awsStd.Config{
			Region: "us-east-1",
		}

		output, err := GetAccountID(ctx, awsCfg, logger)

		if err != nil {
			assert.Empty(t, output, "Output should be empty on error")
			assert.Error(t, err)
			t.Logf("Function executed but requires AWS credentials: %v", err)
		} else {
			assert.NotEmpty(t, output, "Account ID should not be empty when successful")
			assert.Regexp(t, `^\d{12}$`, output, "Account ID should be 12 digits")
		}
	})
}
