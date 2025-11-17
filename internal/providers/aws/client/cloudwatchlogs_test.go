package client

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
)

func TestNewCloudWatchLogsClientAdapter(t *testing.T) {
	client := &cloudwatchlogs.Client{}
	adapter := NewCloudWatchLogsClientAdapter(client)

	assert.NotNil(t, adapter)
}

func TestCloudWatchLogsClientAdapter_ImplementsInterface(_ *testing.T) {
	var _ CloudWatchLogsClient = (*CloudWatchLogsClientAdapter)(nil)
}
