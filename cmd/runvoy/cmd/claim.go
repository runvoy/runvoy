package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/config"
	"runvoy/internal/constants"
	"runvoy/internal/output"

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
	if err := service.ClaimAPIKey(cmd.Context(), token, cfg); err != nil {
		output.Errorf(err.Error())
	}
}

// ConfigSaver defines an interface for saving configuration
type ConfigSaver interface {
	Save(config *config.Config) error
}

// configSaverWrapper wraps the global config.Save function to implement ConfigSaver
type configSaverWrapper struct{}

func (c *configSaverWrapper) Save(cfg *config.Config) error {
	return config.Save(cfg)
}

// NewConfigSaver creates a new ConfigSaver that uses the global config.Save function
func NewConfigSaver() ConfigSaver {
	return &configSaverWrapper{}
}

// ClaimService handles API key claiming logic
type ClaimService struct {
	client     client.Interface
	output     OutputInterface
	configSaver ConfigSaver
}

// NewClaimService creates a new ClaimService with the provided dependencies
func NewClaimService(client client.Interface, output OutputInterface, configSaver ConfigSaver) *ClaimService {
	return &ClaimService{
		client:     client,
		output:     output,
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
