package user

import (
	"fmt"
	"log/slog"

	"runvoy/internal/config"
)

// Client provides a high-level interface for user operations
type Client struct {
	service *Service
}

// NewClient creates a new user client
func NewClient(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		service: NewService(cfg, logger),
	}
}

// CreateUser creates a new user and returns a formatted response
func (c *Client) CreateUser(email string) error {
	req := CreateUserRequest{
		Email: email,
	}

	resp, err := c.service.CreateUser(req)
	if err != nil {
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
