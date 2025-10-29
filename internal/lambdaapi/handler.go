package lambdaapi

import (
	"context"
	"runvoy/internal/app"
	"runvoy/internal/server"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

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

func (h *LambdaHandler) HandleRequest(
	ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {

	return h.adapter(ctx, req)
}
