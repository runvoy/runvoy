package client

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/stretchr/testify/assert"
)

func TestNewECSClientAdapter(t *testing.T) {
	client := &ecs.Client{}
	adapter := NewECSClientAdapter(client)

	assert.NotNil(t, adapter)
}

func TestECSClientAdapter_ImplementsInterface(_ *testing.T) {
	var _ ECSClient = (*ECSClientAdapter)(nil)
}
