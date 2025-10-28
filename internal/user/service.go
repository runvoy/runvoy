package user

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
)

// Service provides user management operations
type Service struct {
	config *config.Config
	logger *slog.Logger
}

// NewService creates a new user service instance
func NewService(cfg *config.Config, logger *slog.Logger) *Service {
	return &Service{
		config: cfg,
		logger: logger,
	}
}

// CreateUserRequest represents the parameters for creating a user
type CreateUserRequest struct {
	Email string
}

// CreateUserResponse represents the result of creating a user
type CreateUserResponse struct {
	User   *api.User
	APIKey string
}

// CreateUser creates a new user with the given email
func (s *Service) CreateUser(req CreateUserRequest) (*CreateUserResponse, error) {
	// Create request payload
	apiReq := api.CreateUserRequest{
		Email: req.Email,
	}
	jsonData, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url, err := url.JoinPath(s.config.APIEndpoint, "users/create")
	if err != nil {
		return nil, fmt.Errorf("invalid API endpoint: %w", err)
	}
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", s.config.APIKey)

	s.logger.Debug("Making request", "url", url, "request", string(jsonData))

	// Make request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusCreated {
		var errorResp struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errorResp.Error, errorResp.Details)
	}

	// Parse successful response
	var createResp api.CreateUserResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		s.logger.Debug("Response body", "body", string(body))
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &CreateUserResponse{
		User:   createResp.User,
		APIKey: createResp.APIKey,
	}, nil
}
