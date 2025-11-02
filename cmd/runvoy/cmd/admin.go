package cmd

import (
	"fmt"
	"log/slog"

	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/constants"
	"runvoy/internal/output"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin commands",
	Long:  "Administrative commands for managing runvoy",
}

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Image management commands",
	Long:  "Manage Docker images for task definitions",
}

var registerImageCmd = &cobra.Command{
	Use:   "register <image>",
	Short: "Register a Docker image for use in task definitions",
	Long:  `Register a Docker image so it can be used when running commands. Optionally specify --with-git to enable git repository cloning support.`,
	Example: fmt.Sprintf(`  - %s admin images register ubuntu:22.04
  - %s admin images register node:20 --with-git
  - %s admin images register public.ecr.aws/docker/library/python:3.11`, constants.ProjectName, constants.ProjectName, constants.ProjectName),
	Run:  runRegisterImage,
	Args: cobra.ExactArgs(1),
}

var withGit bool

func init() {
	registerImageCmd.Flags().BoolVar(&withGit, "with-git", false, "Enable git repository cloning support for this image")
	imagesCmd.AddCommand(registerImageCmd)
	adminCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(adminCmd)
}

func runRegisterImage(cmd *cobra.Command, args []string) {
	cfg, err := getConfigFromContext(cmd)
	if err != nil {
		output.Errorf("failed to load configuration: %v", err)
		return
	}
	image := args[0]

	output.Infof("Registering image %s...", image)
	if withGit {
		output.Infof("  Git repository support: enabled")
	}

	client := client.New(cfg, slog.Default())
	resp, err := client.RegisterImage(cmd.Context(), api.RegisterImageRequest{
		Image:   image,
		WithGit: withGit,
	})
	if err != nil {
		output.Errorf(err.Error())
		return
	}

	output.Successf("Image registered successfully")
	output.KeyValue("Image", resp.TaskDefinition.Image)
	output.KeyValue("Git Support", fmt.Sprintf("%v", resp.TaskDefinition.HasGit))
	output.KeyValue("Task Definition ARN", resp.TaskDefinition.TaskDefinitionARN)
	output.KeyValue("Task Key", resp.TaskDefinition.TaskKey)
}
