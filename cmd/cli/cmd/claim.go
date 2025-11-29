package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runvoy/runvoy/internal/client"
	"github.com/runvoy/runvoy/internal/client/output"
	"github.com/runvoy/runvoy/internal/config"
	"github.com/runvoy/runvoy/internal/constants"

	"github.com/spf13/cobra"
)

var claimCmd = &cobra.Command{
	Use:     "claim <token>",
	Short:   "Claim a user's API key",
	Long:    `Claim a user's API key using the given token`,
	Example: fmt.Sprintf(`  - %s claim 1234567890`, constants.ProjectName),
	Run:     runClaim,
	Args:    cobra.ExactArgs(1),
}

func init() {
	rootCmd.AddCommand(claimCmd)
}

func runClaim(cmd *cobra.Command, args []string) {
	token := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewClaimService(c, NewOutputWrapper(), NewConfigSaver())
	if err = service.ClaimAPIKey(cmd.Context(), token, cfg); err != nil {
		output.Errorf(err.Error())
	}
}

// ClaimService handles API key claiming logic
type ClaimService struct {
	client      client.Interface
	output      OutputInterface
	configSaver ConfigSaver
}

// NewClaimService creates a new ClaimService with the provided dependencies
func NewClaimService(apiClient client.Interface, outputter OutputInterface, configSaver ConfigSaver) *ClaimService {
	return &ClaimService{
		client:      apiClient,
		output:      outputter,
		configSaver: configSaver,
	}
}

// ClaimAPIKey claims an API key using the provided token and saves it to the config
func (s *ClaimService) ClaimAPIKey(ctx context.Context, token string, cfg *config.Config) error {
	resp, err := s.client.ClaimAPIKey(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to claim API key: %w", err)
	}

	cfg.APIKey = resp.APIKey
	if err = s.configSaver.Save(cfg); err != nil {
		s.output.Errorf("failed to save API key to config: %v", err)
		s.output.Warningf("API Key => %s", s.output.Bold(resp.APIKey))
		return fmt.Errorf("failed to save API key to config: %w", err)
	}

	s.output.Successf("API key claimed successfully and saved to config")
	return nil
}
