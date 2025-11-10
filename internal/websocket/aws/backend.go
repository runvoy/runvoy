// Package aws provides AWS-specific WebSocket backend implementation using API Gateway.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"runvoy/internal/websocket"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

// Backend implements websocket.Backend for AWS API Gateway WebSockets.
type Backend struct {
	apiGwClient *apigatewaymanagementapi.Client
	logger      *slog.Logger
}

// NewBackend creates a new AWS WebSocket backend.
func NewBackend(
	awsCfg *aws.Config,
	apiGatewayEndpoint string,
	logger *slog.Logger,
) *Backend {
	apiGwClient := apigatewaymanagementapi.NewFromConfig(*awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(apiGatewayEndpoint)
	})

	return &Backend{
		apiGwClient: apiGwClient,
		logger:      logger,
	}
}

// ParseEvent parses an AWS API Gateway WebSocket event into a provider-agnostic format.
func (b *Backend) ParseEvent(
	ctx context.Context,
	rawEvent *json.RawMessage,
	reqLogger *slog.Logger,
) (*websocket.WebSocketEvent, error) {
	if rawEvent == nil {
		return nil, nil
	}

	var req events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(*rawEvent, &req); err != nil {
		return nil, nil // Not a WebSocket event
	}

	if req.RequestContext.RouteKey == "" {
		return nil, nil // Not a valid WebSocket event
	}

	// Extract client IP from AWS-specific location
	clientIP := ""
	if req.RequestContext.Identity.SourceIP != "" {
		clientIP = req.RequestContext.Identity.SourceIP
	}

	return &websocket.WebSocketEvent{
		RouteKey:     req.RequestContext.RouteKey,
		ConnectionID: req.RequestContext.ConnectionID,
		QueryParams:  req.QueryStringParameters,
		ClientIP:     clientIP,
	}, nil
}

// SendToConnection sends data to a WebSocket connection via API Gateway.
func (b *Backend) SendToConnection(ctx context.Context, connectionID string, data []byte) error {
	_, err := b.apiGwClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	})

	if err != nil {
		return fmt.Errorf("failed to send to connection %s: %w", connectionID, err)
	}

	return nil
}

// BuildResponse creates an API Gateway response for a WebSocket event.
func (b *Backend) BuildResponse(statusCode int, body string) any {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Body:       body,
	}
}
