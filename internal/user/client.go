package user

import (
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/config"
)

// UserClient provides user management operations
type UserClient struct {
	client *client.Client
}

// New creates a new user client
func New(cfg *config.Config, logger *slog.Logger) *UserClient {
	return &UserClient{
		client: client.New(cfg, logger),
	}
}

// CreateUser creates a new user and returns a formatted response
func (u *UserClient) CreateUser(email string) error {
	req := client.Request{
		Method: "POST",
		Path:   "users/create",
		Body: api.CreateUserRequest{
			Email: email,
		},
	}

	var resp api.CreateUserResponse
	if err := u.client.DoJSON(req, &resp); err != nil {
		return fmt.Errorf("❌ %w", err)
	}

	// Display result
	fmt.Printf("✅ User created successfully\n")
	if resp.User != nil {
		fmt.Printf("\t→ Email: %s\n", resp.User.Email)
	}
	fmt.Printf("\t→ API Key: %s\n\n", resp.APIKey)
	fmt.Println("\n🔑  IMPORTANT: Save this API key now. It will not be shown again!")

	return nil
}
