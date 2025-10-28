package main

import (
	"runvoy/internal/app"
	"runvoy/internal/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	svc := app.NewService()
	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
