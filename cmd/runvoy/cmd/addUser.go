package cmd

import (
	"fmt"
	"log/slog"

	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/user"

	"github.com/spf13/cobra"
)

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
	RunE: runCreateUser,
	Args: cobra.ExactArgs(1),
}

func init() {
	usersCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(usersCmd)
}

func runCreateUser(cmd *cobra.Command, args []string) (err error) {
	email := args[0]
	fmt.Println("→ Creating user...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("❌ failed to load configuration: %w", err)
	}

	// Create user client and create user
	userClient := user.NewClient(cfg, slog.Default())
	return userClient.CreateUser(email)
}
