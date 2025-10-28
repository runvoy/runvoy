package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"runvoy/internal/app"
	"runvoy/internal/lambdaapi"
)

func main() {
	svc := app.MustInitialize(context.Background(), "aws")
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
