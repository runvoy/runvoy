package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"runvoy/internal/api"
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

	// Create request payload
	reqBody := api.CreateUserRequest{
		Email: email,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("❌ failed to marshal request: %w", err)
	}

	// Create HTTP request
	url, err := url.JoinPath(cfg.APIEndpoint, "users/create")
	if err != nil {
		return fmt.Errorf("❌ invalid API endpoint: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("❌ failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	slog.Debug("Making request", "url", url, "request", string(jsonData))

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("❌ failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusCreated {
		var errorResp struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return fmt.Errorf("❌ request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("❌ %s: %s", errorResp.Error, errorResp.Details)
	}

	// Parse successful response
	var createResp api.CreateUserResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		slog.Debug("Response body", "body", string(body))
		return fmt.Errorf("❌ failed to parse response: %w", err)
	}

	// Display result
	fmt.Printf("✅ User created successfully\n")
	if createResp.User != nil {
		fmt.Printf("	→ Email: %s\n", createResp.User.Email)
	}
	fmt.Printf("	→ API Key: %s\n\n", createResp.APIKey)
	fmt.Println("\n🔑  IMPORTANT: Save this API key now. It will not be shown again!")

	return nil
}
