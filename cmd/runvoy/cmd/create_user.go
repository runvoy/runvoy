package cmd

import (
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

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
