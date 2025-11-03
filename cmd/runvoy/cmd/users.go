package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var listUsersCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all users",
	Long:    `List all users in the system with their basic information`,
	Example: fmt.Sprintf(`  - %s users list`, constants.ProjectName),
	Run:     runListUsers,
	PostRun: func(_ *cobra.Command, _ []string) {
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

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "User management commands",
}

var createUserCmd = &cobra.Command{
	Use:   "create <email>",
	Short: "Create a new user",
	Long:  `Create a new user with the given email`,
	Example: fmt.Sprintf(`  - %s users create alice@example.com
  - %s users create bob@another-example.com`, constants.ProjectName, constants.ProjectName),
	Run:  runCreateUser,
	Args: cobra.ExactArgs(1),
}

func init() {
	usersCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(usersCmd)
}

func runCreateUser(cmd *cobra.Command, args []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}
	email := args[0]

	output.Infof("Creating user with email %s...", email)

	client := client.New(cfg, slog.Default())
	resp, err := client.CreateUser(cmd.Context(), api.CreateUserRequest{
		Email: email,
	})
	if err != nil {
		output.Errorf(err.Error())
		return
	}

	output.Successf("User created successfully")
	output.KeyValue("Email", resp.User.Email)
	output.KeyValue("Claim Token", resp.ClaimToken)
	output.Blank()
	output.Infof("Share this command with the user => %s claim %s", output.Bold(constants.ProjectName), output.Bold(resp.ClaimToken))
	output.Blank()
	output.Warningf("⏱  Token expires in 15 minutes")
	output.Warningf("👁  Can only be viewed once")
}
