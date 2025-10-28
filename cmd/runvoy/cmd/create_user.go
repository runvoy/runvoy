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

func runCreateUser(_ *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		return
	}
	email := args[0]

	fmt.Printf("→ Creating user with email %s...\n", email)

	var resp api.CreateUserResponse
	req := client.Request{
		Method: "POST",
		Path:   "users/create",
		Body: api.CreateUserRequest{
			Email: email,
		},
	}

	client := client.New(cfg, slog.Default())
	if err := client.DoJSON(req, &resp); err != nil {
		fmt.Printf("❌ %s\n", err)
		return
	}

	fmt.Printf("✅ User created successfully\n")
	fmt.Printf("    → Email: %s\n", resp.User.Email)
	fmt.Printf("    → API Key: %s\n\n", resp.APIKey)
	fmt.Println("\n🔑  IMPORTANT: Save this API key now. It will not be shown again!")
}
