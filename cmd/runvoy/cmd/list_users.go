package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var listUsersCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	Long:  `List all users in the system with their basic information`,
	Example: fmt.Sprintf(`  - %s users list`, constants.ProjectName),
	Run:   runListUsers,
	PostRun: func(cmd *cobra.Command, _ []string) {
		output.Blank()
	},
}

func init() {
	usersCmd.AddCommand(listUsersCmd)
}

func runListUsers(cmd *cobra.Command, _ []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	output.Infof("Listing users…")

	client := client.New(cfg, slog.Default())
	resp, err := client.ListUsers(cmd.Context())
	if err != nil {
		output.Errorf("failed to list users: %v", err)
		return
	}

	if len(resp.Users) == 0 {
		output.Blank()
		output.Warningf("No users found")
		return
	}

	rows := make([][]string, 0, len(resp.Users))
	for _, u := range resp.Users {
		createdAt := u.CreatedAt.UTC().Format(time.DateTime)

		lastUsed := "Never"
		if !u.LastUsed.IsZero() {
			lastUsed = u.LastUsed.UTC().Format(time.DateTime)
		}

		status := "Active"
		if u.Revoked {
			status = "Revoked"
		}

		rows = append(rows, []string{
			output.Bold(u.Email),
			status,
			createdAt,
			lastUsed,
		})
	}

	output.Blank()
	output.Table(
		[]string{
			"Email",
			"Status",
			"Created (UTC)",
			"Last Used (UTC)",
		},
		rows,
	)
	output.Blank()
	output.Successf("Users listed successfully")
}
