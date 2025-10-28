package main

import (
	"context"

	"runvoy/internal/app"
	"runvoy/internal/constants"
	"runvoy/internal/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	svc := app.MustInitialize(context.Background(), constants.AWS)
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
