// Package aws provides AWS-specific event processing implementations.
package aws

import "time"

// ECSTaskStateChangeEvent represents the detail structure of an ECS Task State Change event.
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

// ContainerDetail represents a container within an ECS task.
type ContainerDetail struct {
	ContainerArn string `json:"containerArn"`
	Name         string `json:"name"`
	ExitCode     *int   `json:"exitCode,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// ParseTime parses an RFC3339 timestamp string.
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}
