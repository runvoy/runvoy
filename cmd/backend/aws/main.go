package main

import (
	"context"
	"fmt"

	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/lambdaapi"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	cfg := config.MustLoadEnv()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.InitTimeout)
	defer cancel()

	svc, err := app.Initialize(ctx, constants.AWS, cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize service: %v", err))
	}

	handler := lambdaapi.NewHandler(svc)
	lambda.Start(handler.HandleRequest)
}
