package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"runvoy/internal/constants"
	"strconv"

	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Images management commands",
}

var (
	registerImageIsDefault bool
)

var registerImageCmd = &cobra.Command{
	Use:   "register <image>",
	Short: "Register a new image",
	Example: fmt.Sprintf(`  - %s images register alpine:latest
  - %s images register ecr-public.us-east-1.amazonaws.com/docker/library/ubuntu:22.04
  - %s images register ubuntu:22.04 --set-default`,
		constants.ProjectName,
		constants.ProjectName,
		constants.ProjectName,
	),
	Run:  registerImageRun,
	Args: cobra.ExactArgs(1),
}

var listImagesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all images",
	Run:   listImagesRun,
}

var unregisterImageCmd = &cobra.Command{
	Use:     "unregister <image>",
	Short:   "Unregister an image",
	Example: fmt.Sprintf(`  - %s images unregister alpine:latest`, constants.ProjectName),
	Run:     unregisterImageRun,
	Args:    cobra.ExactArgs(1),
}

func init() {
	registerImageCmd.Flags().BoolVar(&registerImageIsDefault,
		"set-default", false, "Set this image as the default image")
	imagesCmd.AddCommand(registerImageCmd)
	imagesCmd.AddCommand(listImagesCmd)
	imagesCmd.AddCommand(unregisterImageCmd)
	rootCmd.AddCommand(imagesCmd)
}

func registerImageRun(cmd *cobra.Command, args []string) {
	image := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	var isDefault *bool
	if cmd.Flags().Changed("set-default") {
		isDefault = &registerImageIsDefault
	}

	c := client.New(cfg, slog.Default())
	service := NewImagesService(c, NewOutputWrapper())
	if err = service.RegisterImage(cmd.Context(), image, isDefault); err != nil {
		output.Errorf(err.Error())
	}
}

func listImagesRun(cmd *cobra.Command, _ []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewImagesService(c, NewOutputWrapper())
	if err = service.ListImages(cmd.Context()); err != nil {
		output.Errorf(err.Error())
	}
}

func unregisterImageRun(cmd *cobra.Command, args []string) {
	image := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	c := client.New(cfg, slog.Default())
	service := NewImagesService(c, NewOutputWrapper())
	if err = service.UnregisterImage(cmd.Context(), image); err != nil {
		output.Errorf(err.Error())
	}
}

// ImagesService handles image management logic
type ImagesService struct {
	client client.Interface
	output OutputInterface
}

// NewImagesService creates a new ImagesService with the provided dependencies
func NewImagesService(apiClient client.Interface, outputter OutputInterface) *ImagesService {
	return &ImagesService{
		client: apiClient,
		output: outputter,
	}
}

// RegisterImage registers a new image
func (s *ImagesService) RegisterImage(ctx context.Context, image string, isDefault *bool) error {
	resp, err := s.client.RegisterImage(ctx, image, isDefault)
	if err != nil {
		return fmt.Errorf("failed to register image: %w", err)
	}

	s.output.Successf("Image registered successfully")
	s.output.KeyValue("Image", resp.Image)
	if resp.Message != "" {
		s.output.KeyValue("Message", resp.Message)
	}
	return nil
}

// ListImages lists all registered images
func (s *ImagesService) ListImages(ctx context.Context) error {
	resp, err := s.client.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	rows := s.formatImages(resp.Images)

	s.output.Blank()
	s.output.Table(
		[]string{
			"Image",
			"Is Default",
		},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Images listed successfully")
	return nil
}

// UnregisterImage unregisters an image
func (s *ImagesService) UnregisterImage(ctx context.Context, image string) error {
	resp, err := s.client.UnregisterImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	s.output.Successf("Image removed successfully")
	s.output.KeyValue("Image", resp.Image)
	s.output.KeyValue("Message", resp.Message)
	return nil
}

// formatImages formats image data into table rows
func (s *ImagesService) formatImages(images []api.ImageInfo) [][]string {
	rows := make([][]string, 0, len(images))
	for _, image := range images {
		rows = append(rows, []string{
			image.Image,
			strconv.FormatBool(*image.IsDefault),
		})
	}
	return rows
}
