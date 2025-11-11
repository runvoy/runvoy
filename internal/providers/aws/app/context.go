package aws

import (
	"context"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

// LambdaContextExtractor extracts request IDs from AWS Lambda contexts.
type LambdaContextExtractor struct{}

// NewLambdaContextExtractor creates a new AWS Lambda context extractor.
func NewLambdaContextExtractor() *LambdaContextExtractor {
	return &LambdaContextExtractor{}
}

// ExtractRequestID extracts the AWS request ID from a Lambda context.
func (e *LambdaContextExtractor) ExtractRequestID(ctx context.Context) (string, bool) {
	lc, ok := lambdacontext.FromContext(ctx)
	if !ok {
		return "", false
	}

	if lc.AwsRequestID == "" {
		return "", false
	}

	return lc.AwsRequestID, true
}
