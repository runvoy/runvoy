package lambdaapi

import (
	"context"
	"fmt"
	"net/http"
	"runvoy/internal/app"

	"github.com/aws/aws-lambda-go/events"
)

type LambdaHandler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) *LambdaHandler {
	return &LambdaHandler{svc: svc}
}

func (h *LambdaHandler) HandleRequest(
	ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	message := h.svc.Greet(req.RawPath)

	return events.LambdaFunctionURLResponse{
		StatusCode: http.StatusNotImplemented,
		Body:       fmt.Sprintf(`{"error": "Not implemented (%s)"}`, message),
	}, nil
}
