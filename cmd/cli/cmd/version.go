package cmd

import (
	"log/slog"

	"code.l3x.in/runvoy/internal/client"
	"code.l3x.in/runvoy/internal/client/output"
	"code.l3x.in/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of the CLI",
	Run: func(cmd *cobra.Command, _ []string) {
		output.KeyValue("CLI version", *constants.GetVersion())

		cfg, err := getConfigFromContext(cmd)
		if err != nil {
			output.Fatalf("failed to load configuration: %v", err)
		}

		client := client.New(cfg, slog.Default())
		health, err := client.GetHealth(cmd.Context())
		if err != nil {
			output.Errorf(err.Error())
			return
		}

		output.KeyValue("Backend version", health.Version)
		output.KeyValue("Backend provider", string(health.Provider))
		if health.Region != "" {
			output.KeyValue("Backend region", health.Region)
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
