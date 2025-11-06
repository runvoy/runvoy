package events

import (
	"encoding/json"
	"time"
)

// ECSTaskStateChangeEvent represents the detail structure of an ECS Task State Change event
type ECSTaskStateChangeEvent struct {
	ClusterArn    string            `json:"clusterArn"`
	TaskArn       string            `json:"taskArn"`
	LastStatus    string            `json:"lastStatus"`
	DesiredStatus string            `json:"desiredStatus"`
	Containers    []ContainerDetail `json:"containers"`
	StartedAt     string            `json:"startedAt"`
	StoppedAt     string            `json:"stoppedAt"`
	StoppedReason string            `json:"stoppedReason"`
	StopCode      string            `json:"stopCode"`
	CPU           string            `json:"cpu"`
	Memory        string            `json:"memory"`
}

// ContainerDetail represents a container within an ECS task
type ContainerDetail struct {
	ContainerArn string `json:"containerArn"`
	Name         string `json:"name"`
	ExitCode     *int   `json:"exitCode,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// ParseTime parses an RFC3339 timestamp string
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

// CloudWatchLogsEvent represents a CloudWatch Logs event from a subscription filter
// See: https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/SubscriptionFilters.html
type CloudWatchLogsEvent struct {
	MessageType         string               `json:"messageType"`
	Owner               string               `json:"owner"`
	LogGroup            string               `json:"logGroup"`
	LogStream           string               `json:"logStream"`
	SubscriptionFilters []string             `json:"subscriptionFilters"`
	LogEvents           []CloudWatchLogEvent `json:"logEvents"`
}

// CloudWatchLogEvent represents a single log event from CloudWatch Logs
type CloudWatchLogEvent struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// ParseCloudWatchLogsEvent parses the detail field from a CloudWatch event
// that contains a compressed and base64-encoded CloudWatch Logs subscription event
func ParseCloudWatchLogsEvent(detailData []byte) (*CloudWatchLogsEvent, error) {
	var event CloudWatchLogsEvent
	if err := json.Unmarshal(detailData, &event); err != nil {
		return nil, err
	}
	return &event, nil
}
