package cmd

import (
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var revokeUserCmd = &cobra.Command{
	Use:   "revoke <email>",
	Short: "Revoke a user's API key",
	Run:   runRevokeUser,
	Args:  cobra.ExactArgs(1),
}

func runRevokeUser(cmd *cobra.Command, args []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}
	email := args[0]
	output.Infof("Revoking user with email %s...", email)

	client := client.New(cfg, slog.Default())
	resp, err := client.RevokeUser(cmd.Context(), api.RevokeUserRequest{
		Email: email,
	})
	if err != nil {
		output.Errorf(err.Error())
		return
	}

	output.Successf("User revoked successfully")
	output.KeyValue("Email", resp.Email)
}

func init() {
	usersCmd.AddCommand(revokeUserCmd)
}
