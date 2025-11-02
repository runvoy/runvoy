package cmd

import (
	"log/slog"
	"runvoy/internal/client"
	"runvoy/internal/output"
	"strconv"

	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Images management commands",
}

var registerImageCmd = &cobra.Command{
	Use:   "register <image>",
	Short: "Register a new image",
	Run:   registerImageRun,
	Args:  cobra.ExactArgs(1),
}

var listImagesCmd = &cobra.Command{
	Use:   "list",
	Short: "List all images",
	Run:   listImagesRun,
}

var unregisterImageCmd = &cobra.Command{
	Use:   "unregister <image>",
	Short: "Unregister an image",
	Run:   unregisterImageRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
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
	resp, err := client.RegisterImage(cmd.Context(), image)
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
			strconv.FormatBool(image.IsDefault),
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
