package cmd

import (
	"context"
	"fmt"
	"runvoy/internal/api"
	"runvoy/internal/client"
	"runvoy/internal/client/output"
	"runvoy/internal/constants"
	"strconv"

	"github.com/spf13/cobra"
)

var imagesCmd = &cobra.Command{
	Use:   "images",
	Short: "Docker images management commands",
}

var (
	registerImageIsDefault       bool
	registerImageTaskRole        string
	registerImageTaskExecRole    string
	registerImageCPU             string
	registerImageMemory          string
	registerImageRuntimePlatform string
)

var registerImageCmd = &cobra.Command{
	Use:   "register <image>",
	Short: "Register a new Docker image",
	Long: `Register a new Docker image.

NOTICE: The image will be registered with the current user as the owner
and there'll be no validation on the Docker image existence in the registry
nor the ability for the task executor to pull the image correctly.`,
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
	Short: "List all registered Docker images",
	Run:   listImagesRun,
}

var showImageCmd = &cobra.Command{
	Use:   "show <image>",
	Short: "Show detailed information about a Docker image",
	Example: fmt.Sprintf(`  - %s images show alpine:latest
  - %s images show alpine:latest-a1b2c3d4`, constants.ProjectName, constants.ProjectName),
	Run:  showImageRun,
	Args: cobra.ExactArgs(1),
}

var unregisterImageCmd = &cobra.Command{
	Use:     "unregister <image>",
	Short:   "Unregister a Docker image",
	Example: fmt.Sprintf(`  - %s images unregister alpine:latest`, constants.ProjectName),
	Run:     unregisterImageRun,
	Args:    cobra.ExactArgs(1),
}

func init() {
	registerImageCmd.Flags().BoolVar(&registerImageIsDefault,
		"set-default", false, "Set this image as the default image")
	registerImageCmd.Flags().StringVar(&registerImageTaskRole,
		"task-role", "", "Optional task role name for the image")
	registerImageCmd.Flags().StringVar(&registerImageTaskExecRole,
		"task-exec-role", "", "Optional task execution role name for the image")
	registerImageCmd.Flags().StringVar(&registerImageCPU,
		"cpu", "", "Optional CPU value (e.g., 256, 1024). Defaults to 256 if not specified")
	registerImageCmd.Flags().StringVar(&registerImageMemory,
		"memory", "", "Optional Memory value (e.g., 512, 2048). Defaults to 512 if not specified")
	registerImageCmd.Flags().StringVar(&registerImageRuntimePlatform,
		"runtime-platform", "",
		"Optional runtime platform (e.g., Linux/ARM64, Linux/X86_64). Defaults to Linux/ARM64 if not specified")
	imagesCmd.AddCommand(registerImageCmd)
	imagesCmd.AddCommand(listImagesCmd)
	imagesCmd.AddCommand(showImageCmd)
	imagesCmd.AddCommand(unregisterImageCmd)
	rootCmd.AddCommand(imagesCmd)
}

func registerImageRun(cmd *cobra.Command, args []string) {
	image := args[0]

	var isDefault *bool
	if cmd.Flags().Changed("set-default") {
		isDefault = &registerImageIsDefault
	}

	var taskRoleName *string
	if cmd.Flags().Changed("task-role") {
		taskRoleName = &registerImageTaskRole
	}

	var taskExecutionRoleName *string
	if cmd.Flags().Changed("task-exec-role") {
		taskExecutionRoleName = &registerImageTaskExecRole
	}

	var cpu *int
	if cmd.Flags().Changed("cpu") {
		cpuVal, parseErr := strconv.Atoi(registerImageCPU)
		if parseErr != nil {
			output.Errorf("invalid CPU value: %v (must be a number)", parseErr)
			return
		}
		cpu = &cpuVal
	}

	var memory *int
	if cmd.Flags().Changed("memory") {
		memoryVal, parseErr := strconv.Atoi(registerImageMemory)
		if parseErr != nil {
			output.Errorf("invalid Memory value: %v (must be a number)", parseErr)
			return
		}
		memory = &memoryVal
	}

	var runtimePlatform *string
	if cmd.Flags().Changed("runtime-platform") {
		runtimePlatform = &registerImageRuntimePlatform
	}

	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewImagesService(c, NewOutputWrapper())
		return service.RegisterImage(ctx, image, isDefault, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform)
	})
}

func listImagesRun(cmd *cobra.Command, _ []string) {
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewImagesService(c, NewOutputWrapper())
		return service.ListImages(ctx)
	})
}

func showImageRun(cmd *cobra.Command, args []string) {
	image := args[0]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewImagesService(c, NewOutputWrapper())
		return service.ShowImage(ctx, image)
	})
}

func unregisterImageRun(cmd *cobra.Command, args []string) {
	image := args[0]
	executeWithClient(cmd, func(ctx context.Context, c client.Interface) error {
		service := NewImagesService(c, NewOutputWrapper())
		return service.UnregisterImage(ctx, image)
	})
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
func (s *ImagesService) RegisterImage(
	ctx context.Context, image string, isDefault *bool, taskRoleName, taskExecutionRoleName *string,
	cpu, memory *int,
	runtimePlatform *string,
) error {
	resp, err := s.client.RegisterImage(
		ctx, image, isDefault, taskRoleName, taskExecutionRoleName, cpu, memory, runtimePlatform,
	)
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
			"Image ID",
			"Image",
			"CPU",
			"Memory",
			"Runtime Platform",
			"Is Default",
		},
		rows,
	)
	s.output.Blank()
	s.output.Successf("Images listed successfully")
	return nil
}

// ShowImage shows detailed information about a single image
func (s *ImagesService) ShowImage(ctx context.Context, image string) error {
	imageInfo, err := s.client.GetImage(ctx, image)
	if err != nil {
		return fmt.Errorf("failed to get image: %w", err)
	}

	s.output.Blank()
	s.output.KeyValue("Image ID", imageInfo.ImageID)
	s.output.KeyValue("Image", imageInfo.Image)
	if imageInfo.ImageRegistry != "" {
		s.output.KeyValue("Registry", imageInfo.ImageRegistry)
	}
	if imageInfo.ImageName != "" {
		s.output.KeyValue("Image Name", imageInfo.ImageName)
	}
	if imageInfo.ImageTag != "" {
		s.output.KeyValue("Tag", imageInfo.ImageTag)
	}
	if imageInfo.TaskDefinitionName != "" {
		s.output.KeyValue("Task Definition", imageInfo.TaskDefinitionName)
	}
	s.output.KeyValue("CPU", strconv.Itoa(imageInfo.CPU))
	s.output.KeyValue("Memory", strconv.Itoa(imageInfo.Memory))
	if imageInfo.RuntimePlatform != "" {
		s.output.KeyValue("Runtime Platform", imageInfo.RuntimePlatform)
	} else {
		s.output.KeyValue("Runtime Platform", "-")
	}
	if imageInfo.TaskRoleName != nil && *imageInfo.TaskRoleName != "" {
		s.output.KeyValue("Task Role", *imageInfo.TaskRoleName)
	} else {
		s.output.KeyValue("Task Role", "-")
	}
	if imageInfo.TaskExecutionRoleName != nil && *imageInfo.TaskExecutionRoleName != "" {
		s.output.KeyValue("Task Execution Role", *imageInfo.TaskExecutionRoleName)
	} else {
		s.output.KeyValue("Task Execution Role", "-")
	}
	defaultStr := strconv.FormatBool(false)
	if imageInfo.IsDefault != nil && *imageInfo.IsDefault {
		defaultStr = strconv.FormatBool(true)
	}
	s.output.KeyValue("Is Default", defaultStr)
	s.output.Blank()
	s.output.Successf("Image information retrieved successfully")
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
	for i := range images {
		image := &images[i]

		defaultStr := "false"
		if image.IsDefault != nil && *image.IsDefault {
			defaultStr = "true"
		}

		platformStr := image.RuntimePlatform
		if platformStr == "" {
			platformStr = "-"
		}

		rows = append(rows, []string{
			image.ImageID,
			image.Image,
			strconv.Itoa(image.CPU),
			strconv.Itoa(image.Memory),
			platformStr,
			defaultStr,
		})
	}
	return rows
}
