package lambdaapi

import (
	"context"
	"runvoy/internal/app"
	"runvoy/internal/server"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// LambdaHandler handles AWS Lambda requests for the runvoy API.
type LambdaHandler struct {
	adapter func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)
}

// NewHandler creates a new Lambda handler with the given service.
// The request timeout is passed to the router to configure the timeout middleware.
func NewHandler(svc *app.Service, requestTimeout time.Duration) *LambdaHandler {
	router := server.NewRouter(svc, requestTimeout)
	adapter := ChiRouterToLambdaAdapter(router.ChiMux())

	return &LambdaHandler{adapter: adapter}
}

// HandleRequest processes an incoming Lambda function URL request.
func (h *LambdaHandler) HandleRequest(
	ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return h.adapter(ctx, req)
}
