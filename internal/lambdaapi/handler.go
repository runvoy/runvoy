package lambdaapi

import (
	"context"
	"runvoy/internal/app"
	"runvoy/internal/server"

	"github.com/aws/aws-lambda-go/events"
)

type LambdaHandler struct {
	adapter func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)
}

func NewHandler(svc *app.Service) *LambdaHandler {
	router := server.NewRouter(svc)
	adapter := ChiRouterToLambdaAdapter(router.ChiMux())
	return &LambdaHandler{adapter: adapter}
}

func (h *LambdaHandler) HandleRequest(
	ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return h.adapter(ctx, req)
}
