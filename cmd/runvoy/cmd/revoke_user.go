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
		output.Error("failed to load configuration: %v", err)
		return
	}
	email := args[0]
	output.Info("Revoking user with email %s...", email)

	client := client.New(cfg, slog.Default())
	resp, err := client.RevokeUser(cmd.Context(), api.RevokeUserRequest{
		Email: email,
	})
	if err != nil {
		output.Error(err.Error())
		return
	}

	output.Success("User revoked successfully")
	output.KeyValue("Email", resp.Email)
}

func init() {
	usersCmd.AddCommand(revokeUserCmd)
}
