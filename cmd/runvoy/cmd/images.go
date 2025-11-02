package cmd

import (
	"fmt"
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"
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

	client := client.New(cfg, slog.Default())
	var isDefault *bool
	if cmd.Flags().Changed("set-default") {
		isDefault = &registerImageIsDefault
	}
	resp, err := client.RegisterImage(cmd.Context(), image, isDefault)
	if err != nil {
		output.Errorf("failed to register image: %v", err)
		return
	}
	output.Successf("Image registered successfully")
	output.KeyValue("Image", resp.Image)
	if resp.Message != "" {
		output.KeyValue("Message", resp.Message)
	}
}

func listImagesRun(cmd *cobra.Command, args []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.ListImages(cmd.Context())
	if err != nil {
		output.Errorf("failed to list images: %v", err)
		return
	}

	rows := make([][]string, 0, len(resp.Images))
	for _, image := range resp.Images {
		rows = append(rows, []string{
			image.Image,
			strconv.FormatBool(*image.IsDefault),
		})
	}

	output.Blank()
	output.Table(
		[]string{
			"Image",
			"Is Default",
		},
		rows,
	)
	output.Blank()
	output.Successf("Images listed successfully")
}

func unregisterImageRun(cmd *cobra.Command, args []string) {
	image := args[0]
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.UnregisterImage(cmd.Context(), image)
	if err != nil {
		output.Errorf("failed to remove image: %v", err)
		return
	}
	output.Successf("Image removed successfully")
	output.KeyValue("Image", resp.Image)
	output.KeyValue("Message", resp.Message)
}
