package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/stretchr/testify/assert"
)

func TestLambdaContextExtractor_ExtractRequestID(t *testing.T) {
	extractor := NewLambdaContextExtractor()

	t.Run("extracts AWS request ID from Lambda context", func(t *testing.T) {
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "test-lambda-request-123",
		}
		ctx := lambdacontext.NewContext(context.Background(), lc)

		requestID, found := extractor.ExtractRequestID(ctx)

		assert.True(t, found, "Should find Lambda context")
		assert.Equal(t, "test-lambda-request-123", requestID)
	})

	t.Run("returns false when no Lambda context present", func(t *testing.T) {
		ctx := context.Background()

		requestID, found := extractor.ExtractRequestID(ctx)

		assert.False(t, found, "Should not find Lambda context in plain context")
		assert.Empty(t, requestID)
	})

	t.Run("returns false when Lambda context has empty request ID", func(t *testing.T) {
		lc := &lambdacontext.LambdaContext{
			AwsRequestID: "",
		}
		ctx := lambdacontext.NewContext(context.Background(), lc)

		requestID, found := extractor.ExtractRequestID(ctx)

		assert.False(t, found, "Should return false for empty request ID")
		assert.Empty(t, requestID)
	})
}
