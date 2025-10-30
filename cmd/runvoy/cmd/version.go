package cmd

import (
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of the CLI",
	Run: func(cmd *cobra.Command, _ []string) {
		output.KeyValue("CLI version", *constants.GetVersion())

		cfg, err := getConfigFromContext(cmd)
		if err != nil {
			slog.Error("failed to load configuration", "error", err)
			return
		}

		client := client.New(cfg, slog.Default())
		health, err := client.GetHealth(cmd.Context())
		if err != nil {
			output.Error(err.Error())
			return
		}

		output.KeyValue("Backend version", health.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
