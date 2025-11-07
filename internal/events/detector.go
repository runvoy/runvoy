package events

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

// EventType represents the type of Lambda event
type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeCloudWatch
	EventTypeAPIGatewayWebSocket
	EventTypeAPIGatewayProxy
	EventTypeCloudWatchLogs
	EventTypeSNS
	EventTypeSQS
)

// String returns the string representation of the EventType
func (e EventType) String() string {
	switch e {
	case EventTypeCloudWatch:
		return "CloudWatch"
	case EventTypeAPIGatewayWebSocket:
		return "APIGatewayWebSocket"
	case EventTypeAPIGatewayProxy:
		return "APIGatewayProxy"
	case EventTypeCloudWatchLogs:
		return "CloudWatchLogs"
	case EventTypeSNS:
		return "SNS"
	case EventTypeSQS:
		return "SQS"
	default:
		return "Unknown"
	}
}

// DetectEventType determines the type of Lambda event by attempting to unmarshal
// into known AWS Lambda event types and validating required fields
func DetectEventType(rawEvent json.RawMessage) EventType {
	// Try CloudWatch Event
	var cwEvent events.CloudWatchEvent
	if err := json.Unmarshal(rawEvent, &cwEvent); err == nil {
		if cwEvent.Source != "" && cwEvent.DetailType != "" {
			return EventTypeCloudWatch
		}
	}

	// Try API Gateway WebSocket
	var wsEvent events.APIGatewayWebsocketProxyRequest
	if err := json.Unmarshal(rawEvent, &wsEvent); err == nil {
		if wsEvent.RequestContext.ConnectionID != "" && wsEvent.RequestContext.RouteKey != "" {
			return EventTypeAPIGatewayWebSocket
		}
	}

	// Try API Gateway Proxy (REST API)
	var proxyEvent events.APIGatewayProxyRequest
	if err := json.Unmarshal(rawEvent, &proxyEvent); err == nil {
		if proxyEvent.RequestContext.RequestID != "" && proxyEvent.HTTPMethod != "" {
			return EventTypeAPIGatewayProxy
		}
	}

	// Try API Gateway V2 (HTTP API)
	var proxyV2Event events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(rawEvent, &proxyV2Event); err == nil {
		if proxyV2Event.RequestContext.HTTP.Method != "" {
			return EventTypeAPIGatewayProxy
		}
	}

	// Try CloudWatch Logs
	var logsEvent events.CloudwatchLogsEvent
	if err := json.Unmarshal(rawEvent, &logsEvent); err == nil {
		if logsEvent.AWSLogs.Data != "" {
			return EventTypeCloudWatchLogs
		}
	}

	// Try SNS
	var snsEvent events.SNSEvent
	if err := json.Unmarshal(rawEvent, &snsEvent); err == nil {
		if len(snsEvent.Records) > 0 && snsEvent.Records[0].SNS.MessageID != "" {
			return EventTypeSNS
		}
	}

	// Try SQS
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(rawEvent, &sqsEvent); err == nil {
		if len(sqsEvent.Records) > 0 && sqsEvent.Records[0].MessageId != "" {
			return EventTypeSQS
		}
	}

	return EventTypeUnknown
}
