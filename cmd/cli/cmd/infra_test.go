package cmd

import (
	"testing"

	"github.com/runvoy/runvoy/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInfraApply_UsesProjectNameFlag(t *testing.T) {
	t.Helper()

	// Ensure init() has run and flags are bound.
	require.NotNil(t, infraApplyCmd)

	flag := infraApplyCmd.Flags().Lookup("project-name")
	require.NotNil(t, flag, "project-name flag should be defined on infra apply command")
}

func TestInfraDestroy_UsesProjectNameFlag(t *testing.T) {
	t.Helper()

	require.NotNil(t, infraDestroyCmd)

	flag := infraDestroyCmd.Flags().Lookup("project-name")
	require.NotNil(t, flag, "project-name flag should be defined on infra destroy command")
}

func TestInfraDefaultProjectNameComesFromConfig(t *testing.T) {
	t.Helper()

	cfg, err := config.Load()
	require.NoError(t, err)

	applyFlag := infraApplyCmd.Flags().Lookup("project-name")
	require.NotNil(t, applyFlag)

	// The default value for the flag should match GetDefaultStackName().
	require.Equal(t, cfg.GetDefaultStackName(), applyFlag.DefValue)
}
