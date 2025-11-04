// Package lambdaapi provides Lambda handler creation for AWS Lambda Function URLs,
// integrating the application service with the HTTP router through algnhsa adapter.
package lambdaapi

import (
	"runvoy/internal/app"
	"runvoy/internal/config"
	"runvoy/internal/server"
	"time"

	"github.com/akrylysov/algnhsa"
	"github.com/aws/aws-lambda-go/lambda"
)

// NewHandler creates a new Lambda handler with the given service.
// The request timeout is passed to the router to configure the timeout middleware.
// It uses algnhsa to adapt the chi router to work with Lambda Function URLs.
func NewHandler(svc *app.Service, cfg *config.Config, requestTimeout time.Duration) lambda.Handler {
	router := server.NewRouter(svc, cfg, requestTimeout)
	return algnhsa.New(router.Handler(), nil)
}
