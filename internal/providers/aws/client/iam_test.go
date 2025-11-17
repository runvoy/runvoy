package client

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/stretchr/testify/assert"
)

func TestNewIAMClientAdapter(t *testing.T) {
	client := &iam.Client{}
	adapter := NewIAMClientAdapter(client)

	assert.NotNil(t, adapter)
}

func TestIAMClientAdapter_ImplementsInterface(_ *testing.T) {
	var _ IAMClient = (*IAMClientAdapter)(nil)
}
