package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/client/output"

	"github.com/spf13/cobra"
)

var listUsersCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all users",
	Long:    `List all users in the system with their basic information`,
	Example: fmt.Sprintf(`  - %s users list`, constants.ProjectName),
	Run:     runListUsers,
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

	c := client.New(cfg, slog.Default())
	service := NewUsersService(c, NewOutputWrapper())
	if err = service.ListUsers(cmd.Context()); err != nil {
		output.Errorf(err.Error())
	}
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

	c := client.New(cfg, slog.Default())
	service := NewUsersService(c, NewOutputWrapper())
	if err = service.RevokeUser(cmd.Context(), email); err != nil {
		output.Errorf(err.Error())
	}
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

	c := client.New(cfg, slog.Default())
	service := NewUsersService(c, NewOutputWrapper())
	if err = service.CreateUser(cmd.Context(), email); err != nil {
		output.Errorf(err.Error())
	}
}

// UsersService handles user management logic
type UsersService struct {
	client client.Interface
	output OutputInterface
}

// NewUsersService creates a new UsersService with the provided dependencies
func NewUsersService(apiClient client.Interface, outputter OutputInterface) *UsersService {
	return &UsersService{
		client: apiClient,
		output: outputter,
	}
}

// CreateUser creates a new user with the given email
func (s *UsersService) CreateUser(ctx context.Context, email string) error {
	s.output.Infof("Creating user with email %s...", email)

	resp, err := s.client.CreateUser(ctx, api.CreateUserRequest{
		Email: email,
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	s.output.Successf("User created successfully")
	s.output.KeyValue("Email", resp.User.Email)
	s.output.KeyValue("Claim Token", resp.ClaimToken)
	s.output.Blank()
	s.output.Infof(
		"Share this command with the user => %s claim %s",
		s.output.Bold(constants.ProjectName),
		s.output.Bold(resp.ClaimToken),
	)
	s.output.Blank()
	s.output.Warningf("⏱  Token expires in 15 minutes")
	s.output.Warningf("👁  Can only be viewed once")
	return nil
}

// ListUsers lists all users and displays them in a table format
func (s *UsersService) ListUsers(ctx context.Context) error {
	s.output.Infof("Listing users…")

	resp, err := s.client.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(resp.Users) == 0 {
		s.output.Blank()
		s.output.Warningf("No users found")
		return nil
	}

	rows := s.formatUsers(resp.Users)

	s.output.Blank()
	s.output.Table(
		[]string{
			"Email",
			"Status",
			"Created (UTC)",
			"Last Used (UTC)",
		},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Users listed successfully")
	return nil
}

// RevokeUser revokes a user's API key
func (s *UsersService) RevokeUser(ctx context.Context, email string) error {
	s.output.Infof("Revoking user with email %s...", email)

	resp, err := s.client.RevokeUser(ctx, api.RevokeUserRequest{
		Email: email,
	})
	if err != nil {
		return fmt.Errorf("failed to revoke user: %w", err)
	}

	s.output.Successf("User revoked successfully")
	s.output.KeyValue("Email", resp.Email)
	return nil
}

// formatUsers formats user data into table rows
func (s *UsersService) formatUsers(users []*api.User) [][]string {
	rows := make([][]string, 0, len(users))
	for _, u := range users {
		createdAt := u.CreatedAt.UTC().Format(time.DateTime)

		lastUsed := "Never"
		if u.LastUsed != nil && !u.LastUsed.IsZero() {
			lastUsed = u.LastUsed.UTC().Format(time.DateTime)
		}

		status := "Active"
		if u.Revoked {
			status = "Revoked"
		}

		rows = append(rows, []string{
			s.output.Bold(u.Email),
			status,
			createdAt,
			lastUsed,
		})
	}
	return rows
}
