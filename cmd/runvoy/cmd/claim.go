package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var claimCmd = &cobra.Command{
	Use:     "claim <token>",
	Short:   "Claim a user's API key",
	Long:    `Claim a user's API key using the given token`,
	Example: fmt.Sprintf(`  - %s claim 1234567890`, constants.ProjectName),
	Run:     runClaim,
	Args:    cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(claimCmd)
}

func runClaim(cmd *cobra.Command, args []string) {
	token := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	resp, err := c.ClaimAPIKey(cmd.Context(), token)
	if err != nil {
		output.Errorf(err.Error())
		return
	}

	cfg.APIKey = resp.APIKey
	if err := config.Save(cfg); err != nil {
		output.Errorf("failed to save API key to config: %v", err)
		output.Warningf("API Key => %s", output.Bold(resp.APIKey))
		return
	}

	output.Successf("API key claimed successfully and saved to config")
}
