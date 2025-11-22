package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageConfig_ToLegacyParams(t *testing.T) {
	cpu := 512
	memory := 1024
	platform := "linux/arm64"
	taskRole := "my-task-role"
	execRole := "my-exec-role"
	isDefault := true

	config := &ImageConfig{
		Image:        "alpine:latest",
		IsDefault:    &isDefault,
		RegisteredBy: "test@example.com",
		Resources: &ResourceConfig{
			CPU:    &cpu,
			Memory: &memory,
		},
		Runtime: &RuntimeConfig{
			Platform: &platform,
		},
		Permissions: &PermissionConfig{
			TaskRole:      &taskRole,
			ExecutionRole: &execRole,
		},
	}

	image, def, taskRoleName, execRoleName, cpuVal, memVal, platformVal, createdBy := config.ToLegacyParams()

	assert.Equal(t, "alpine:latest", image)
	assert.NotNil(t, def)
	assert.True(t, *def)
	assert.NotNil(t, taskRoleName)
	assert.Equal(t, "my-task-role", *taskRoleName)
	assert.NotNil(t, execRoleName)
	assert.Equal(t, "my-exec-role", *execRoleName)
	assert.NotNil(t, cpuVal)
	assert.Equal(t, 512, *cpuVal)
	assert.NotNil(t, memVal)
	assert.Equal(t, 1024, *memVal)
	assert.NotNil(t, platformVal)
	assert.Equal(t, "linux/arm64", *platformVal)
	assert.Equal(t, "test@example.com", createdBy)
}

func TestImageConfig_ToLegacyParams_Minimal(t *testing.T) {
	config := &ImageConfig{
		Image:        "ubuntu:22.04",
		RegisteredBy: "admin@example.com",
	}

	image, def, taskRoleName, execRoleName, cpuVal, memVal, platformVal, createdBy := config.ToLegacyParams()

	assert.Equal(t, "ubuntu:22.04", image)
	assert.Nil(t, def)
	assert.Nil(t, taskRoleName)
	assert.Nil(t, execRoleName)
	assert.Nil(t, cpuVal)
	assert.Nil(t, memVal)
	assert.Nil(t, platformVal)
	assert.Equal(t, "admin@example.com", createdBy)
}

func TestFromLegacyParams_Full(t *testing.T) {
	cpu := 512
	memory := 1024
	platform := "linux/arm64"
	taskRole := "my-task-role"
	execRole := "my-exec-role"
	isDefault := true

	config := FromLegacyParams(
		"alpine:latest",
		&isDefault,
		&taskRole,
		&execRole,
		&cpu,
		&memory,
		&platform,
		"test@example.com",
	)

	assert.Equal(t, "alpine:latest", config.Image)
	assert.NotNil(t, config.IsDefault)
	assert.True(t, *config.IsDefault)
	assert.Equal(t, "test@example.com", config.RegisteredBy)

	assert.NotNil(t, config.Resources)
	assert.NotNil(t, config.Resources.CPU)
	assert.Equal(t, 512, *config.Resources.CPU)
	assert.NotNil(t, config.Resources.Memory)
	assert.Equal(t, 1024, *config.Resources.Memory)

	assert.NotNil(t, config.Runtime)
	assert.NotNil(t, config.Runtime.Platform)
	assert.Equal(t, "linux/arm64", *config.Runtime.Platform)

	assert.NotNil(t, config.Permissions)
	assert.NotNil(t, config.Permissions.TaskRole)
	assert.Equal(t, "my-task-role", *config.Permissions.TaskRole)
	assert.NotNil(t, config.Permissions.ExecutionRole)
	assert.Equal(t, "my-exec-role", *config.Permissions.ExecutionRole)
}

func TestFromLegacyParams_Minimal(t *testing.T) {
	config := FromLegacyParams(
		"ubuntu:22.04",
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"admin@example.com",
	)

	assert.Equal(t, "ubuntu:22.04", config.Image)
	assert.Nil(t, config.IsDefault)
	assert.Equal(t, "admin@example.com", config.RegisteredBy)
	assert.Nil(t, config.Resources)
	assert.Nil(t, config.Runtime)
	assert.Nil(t, config.Permissions)
}

func TestRoundTrip(t *testing.T) {
	cpu := 256
	memory := 512
	platform := "linux/amd64"
	taskRole := "app-role"
	execRole := "ecs-role"
	isDefault := false

	original := FromLegacyParams(
		"node:18",
		&isDefault,
		&taskRole,
		&execRole,
		&cpu,
		&memory,
		&platform,
		"dev@example.com",
	)

	image, def, tRole, eRole, c, m, p, createdBy := original.ToLegacyParams()

	roundtrip := FromLegacyParams(image, def, tRole, eRole, c, m, p, createdBy)

	assert.Equal(t, original.Image, roundtrip.Image)
	assert.Equal(t, original.IsDefault, roundtrip.IsDefault)
	assert.Equal(t, original.RegisteredBy, roundtrip.RegisteredBy)
	assert.Equal(t, original.Resources, roundtrip.Resources)
	assert.Equal(t, original.Runtime, roundtrip.Runtime)
	assert.Equal(t, original.Permissions, roundtrip.Permissions)
}
