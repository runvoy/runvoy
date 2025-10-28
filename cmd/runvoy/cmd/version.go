package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🚀 " + constants.ProjectName)
		fmt.Printf(" → CLI version: %s\n", constants.Version)

		cfg, err := config.Load()
		if err != nil {
			slog.Error("failed to load configuration", "error", err)
			return
		}

		var resp api.HealthResponse
		req := client.Request{
			Method: "GET",
			Path:   "health",
		}

		client := client.New(cfg, slog.Default())
		if err := client.DoJSON(req, &resp); err != nil {
			fmt.Printf("❌ %s\n", err)
			return
		}

		fmt.Printf(" → Backend version: %s\n", resp.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
