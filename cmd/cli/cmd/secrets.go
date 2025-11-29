package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

const createSecretArgCount = 3

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Secrets management commands",
}

var createSecretCmd = &cobra.Command{
	Use:   "create <name> <key-name> <value>",
	Short: "Create a new secret",
	Long:  `Create a new secret with the given name, key name (environment variable name), and value`,
	Example: fmt.Sprintf(
		"  - %s secrets create github-token GITHUB_TOKEN \"ghp_xxxxx\"\n"+
			"  - %s secrets create db-password DB_PASSWORD \"secret123\" --description \"Database password\"",
		constants.ProjectName,
		constants.ProjectName,
	),
	Run:  runCreateSecret,
	Args: cobra.ExactArgs(createSecretArgCount),
}

var createSecretDescription string

func init() {
	secretsCmd.AddCommand(createSecretCmd)
	createSecretCmd.Flags().StringVar(&createSecretDescription, "description", "", "Description for the secret")
	rootCmd.AddCommand(secretsCmd)
}

func runCreateSecret(cmd *cobra.Command, args []string) {
	name := args[0]
	keyName := args[1]
	value := args[2]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewSecretsService(c, NewOutputWrapper())
		return service.CreateSecret(ctx, name, keyName, value, createSecretDescription)
	})
}

var getSecretCmd = &cobra.Command{
	Use:     "get <name>",
	Short:   "Get a secret by name",
	Long:    `Retrieve a secret by its name, including its value`,
	Example: fmt.Sprintf(`  - %s secrets get github-token`, constants.ProjectName),
	Run:     runGetSecret,
	Args:    cobra.ExactArgs(1),
}

func init() {
	secretsCmd.AddCommand(getSecretCmd)
}

func runGetSecret(cmd *cobra.Command, args []string) {
	name := args[0]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewSecretsService(c, NewOutputWrapper())
		return service.GetSecret(ctx, name)
	})
}

var listSecretsCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all secrets",
	Long:    `List all secrets in the system with their basic information`,
	Example: fmt.Sprintf(`  - %s secrets list`, constants.ProjectName),
	Run:     runListSecrets,
}

func init() {
	secretsCmd.AddCommand(listSecretsCmd)
}

func runListSecrets(cmd *cobra.Command, _ []string) {
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewSecretsService(c, NewOutputWrapper())
		return service.ListSecrets(ctx)
	})
}

var updateSecretCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a secret",
	Long:  `Update a secret's metadata (description, key_name) and/or value`,
	Example: fmt.Sprintf(
		"  - %s secrets update github-token --key-name GITHUB_API_TOKEN --value \"new-token\"\n"+
			"  - %s secrets update db-password --description \"Updated database password\"",
		constants.ProjectName,
		constants.ProjectName,
	),
	Run:  runUpdateSecret,
	Args: cobra.ExactArgs(1),
}

var updateSecretKeyName string
var updateSecretValue string
var updateSecretDescription string

func init() {
	secretsCmd.AddCommand(updateSecretCmd)
	updateSecretCmd.Flags().StringVar(
		&updateSecretKeyName,
		"key-name",
		"",
		"Environment variable name (e.g., GITHUB_TOKEN)",
	)
	updateSecretCmd.Flags().StringVar(&updateSecretValue, "value", "", "Secret value to update")
	updateSecretCmd.Flags().StringVar(
		&updateSecretDescription,
		"description",
		"",
		"Description for the secret",
	)
}

func runUpdateSecret(cmd *cobra.Command, args []string) {
	name := args[0]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewSecretsService(c, NewOutputWrapper())
		return service.UpdateSecret(ctx, name, updateSecretKeyName, updateSecretValue, updateSecretDescription)
	})
}

var deleteSecretCmd = &cobra.Command{
	Use:     "delete <name>",
	Short:   "Delete a secret",
	Long:    `Delete a secret by its name`,
	Example: fmt.Sprintf(`  - %s secrets delete github-token`, constants.ProjectName),
	Run:     runDeleteSecret,
	Args:    cobra.ExactArgs(1),
}

func init() {
	secretsCmd.AddCommand(deleteSecretCmd)
}

func runDeleteSecret(cmd *cobra.Command, args []string) {
	name := args[0]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewSecretsService(c, NewOutputWrapper())
		return service.DeleteSecret(ctx, name)
	})
}

// SecretsService handles secrets management logic
type SecretsService struct {
	client client.Interface
	output OutputInterface
}

// NewSecretsService creates a new SecretsService with the provided dependencies
func NewSecretsService(apiClient client.Interface, outputter OutputInterface) *SecretsService {
	return &SecretsService{
		client: apiClient,
		output: outputter,
	}
}

// CreateSecret creates a new secret with the given name, key name, value, and optional description
func (s *SecretsService) CreateSecret(ctx context.Context, name, keyName, value, description string) error {
	s.output.Infof("Creating secret %s...", name)

	req := api.CreateSecretRequest{
		Name:    name,
		KeyName: keyName,
		Value:   value,
	}
	if description != "" {
		req.Description = description
	}

	resp, err := s.client.CreateSecret(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	s.output.Successf("Secret created successfully")
	s.output.KeyValue("Name", name)
	s.output.KeyValue("Key Name", keyName)
	if description != "" {
		s.output.KeyValue("Description", description)
	}
	s.output.Blank()
	s.output.Infof(resp.Message)
	return nil
}

// GetSecret retrieves a secret by name
func (s *SecretsService) GetSecret(ctx context.Context, name string) error {
	s.output.Infof("Retrieving secret %s...", name)

	resp, err := s.client.GetSecret(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if resp.Secret == nil {
		s.output.Warningf("Secret not found")
		return nil
	}

	secret := resp.Secret
	s.output.Blank()
	s.output.KeyValue("Name", secret.Name)
	s.output.KeyValue("Key Name", secret.KeyName)
	if secret.Description != "" {
		s.output.KeyValue("Description", secret.Description)
	}
	if secret.Value != "" {
		s.output.KeyValue("Value", secret.Value)
	}
	s.output.KeyValue("Created By", secret.CreatedBy)
	if len(secret.OwnedBy) > 0 {
		s.output.KeyValue("Owned By", strings.Join(secret.OwnedBy, ", "))
	} else {
		s.output.KeyValue("Owned By", "-")
	}
	s.output.KeyValue("Created At", secret.CreatedAt.UTC().Format(time.DateTime))
	s.output.KeyValue("Updated By", secret.UpdatedBy)
	s.output.KeyValue("Updated At", secret.UpdatedAt.UTC().Format(time.DateTime))
	s.output.Blank()
	s.output.Successf("Secret retrieved successfully")
	return nil
}

// ListSecrets lists all secrets and displays them in a table format
func (s *SecretsService) ListSecrets(ctx context.Context) error {
	s.output.Infof("Listing secrets…")

	resp, err := s.client.ListSecrets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(resp.Secrets) == 0 {
		s.output.Blank()
		s.output.Warningf("No secrets found")
		return nil
	}

	rows := s.formatSecrets(resp.Secrets)

	s.output.Blank()
	s.output.Table(
		[]string{
			"Name",
			"Key Name",
			"Description",
			"Created By",
			"Created At (UTC)",
		},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Secrets listed successfully")
	return nil
}

// UpdateSecret updates a secret's metadata and/or value
func (s *SecretsService) UpdateSecret(ctx context.Context, name, keyName, value, description string) error {
	s.output.Infof("Updating secret %s...", name)

	req := api.UpdateSecretRequest{}
	if keyName != "" {
		req.KeyName = keyName
	}
	if value != "" {
		req.Value = value
	}
	if description != "" {
		req.Description = description
	}

	if req.KeyName == "" && req.Value == "" && req.Description == "" {
		return fmt.Errorf("at least one field (--key-name, --value, or --description) must be provided")
	}

	resp, err := s.client.UpdateSecret(ctx, name, req)
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	s.output.Successf("Secret updated successfully")
	s.output.KeyValue("Name", name)
	s.output.Blank()
	s.output.Infof(resp.Message)
	return nil
}

// DeleteSecret deletes a secret by name
func (s *SecretsService) DeleteSecret(ctx context.Context, name string) error {
	s.output.Infof("Deleting secret %s...", name)

	resp, err := s.client.DeleteSecret(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	s.output.Successf("Secret deleted successfully")
	s.output.KeyValue("Name", resp.Name)
	s.output.Blank()
	s.output.Infof(resp.Message)
	return nil
}

// formatSecrets formats secret data into table rows
func (s *SecretsService) formatSecrets(secrets []*api.Secret) [][]string {
	rows := make([][]string, 0, len(secrets))
	for _, secret := range secrets {
		description := secret.Description
		if description == "" {
			description = "-"
		}
		rows = append(rows, []string{
			s.output.Bold(secret.Name),
			secret.KeyName,
			description,
			secret.CreatedBy,
			secret.CreatedAt.UTC().Format(time.DateTime),
		})
	}
	return rows
}
