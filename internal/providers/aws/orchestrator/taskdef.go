package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	awsClient "runvoy/internal/providers/aws/client"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/providers/aws/ecsdefs"

	awsStd "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// SanitizeImageNameForTaskDef converts a Docker image name to a valid ECS task definition family name.
// ECS task definition family names must match: [a-zA-Z0-9_-]+
// Examples:
//   - "hashicorp/terraform:1.6" -> "hashicorp-terraform-1-6"
//   - "myregistry.com/my-image:latest" -> "myregistry-com-my-image-latest"
func SanitizeImageNameForTaskDef(image string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(image, "-")
	re2 := regexp.MustCompile(`-+`)
	sanitized = re2.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, "-")
	return sanitized
}

// TaskDefinitionFamilyName returns the ECS task definition family name for a given image.
// Format: {TaskDefinitionFamilyPrefix}-{sanitized-image-name}
func TaskDefinitionFamilyName(image string) string {
	sanitized := SanitizeImageNameForTaskDef(image)
	return fmt.Sprintf("%s-%s", awsConstants.TaskDefinitionFamilyPrefix, sanitized)
}

// ExtractImageFromTaskDefFamily extracts the Docker image name from a task definition family name.
// Returns empty string if the family name doesn't match the expected format.
// NOTE: This is approximate - images should be read from container definitions or tags, not family names.
func ExtractImageFromTaskDefFamily(familyName string) string {
	prefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	if !strings.HasPrefix(familyName, prefix) {
		return ""
	}
	imagePart := strings.TrimPrefix(familyName, prefix)
	return imagePart
}

// listTaskDefinitionsByPrefix lists all active task definitions whose family name starts with the given prefix.
// It handles pagination internally and filters by checking the task definition family name (extracted from ARN).
// This is necessary because the FamilyPrefix parameter in ListTaskDefinitions doesn't work as expected
// for prefix matching - it requires exact family match rather than prefix matching.
func listTaskDefinitionsByPrefix(ctx context.Context, ecsClient awsClient.ECSClient, prefix string) ([]string, error) {
	nextToken := ""
	var taskDefArns []string

	for {
		listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			Status:    ecsTypes.TaskDefinitionStatusActive,
			NextToken: awsStd.String(nextToken),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list task definitions: %w", err)
		}

		// taskDefARN format example:
		// arn:aws:ecs:us-east-2:123456789012:task-definition/runvoy-image-alpine-latest:1
		// Extract the family name (last part after "/") and filter by prefix
		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			parts := strings.Split(taskDefARN, "/")
			if len(parts) > 0 &&
				strings.HasPrefix(parts[len(parts)-1], prefix) {
				taskDefArns = append(taskDefArns, taskDefARN)
			}
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	return taskDefArns, nil
}

// GetDefaultImage returns the Docker image marked as default (via IsDefault tag).
// Returns empty string if no default image is found.
func GetDefaultImage(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	log *slog.Logger,
) (string, error) {
	familyPrefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		return "", err
	}

	log.Debug("calling external service", "context", map[string]string{
		"operation":     "ECS.ListTagsForResource",
		"resource_arns": strings.Join(taskDefArns, ", "),
	})

	for _, taskDefARN := range taskDefArns {
		var tagsOutput *ecs.ListTagsForResourceOutput
		tagsOutput, err = ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if err != nil {
			log.Debug("failed to list tags for task definition", "context", map[string]string{
				"arn":   taskDefARN,
				"error": err.Error(),
			})
			continue
		}

		isDefault := false
		var dockerImage string
		for _, tag := range tagsOutput.Tags {
			if tag.Key != nil && tag.Value != nil {
				if *tag.Key == awsConstants.TaskDefinitionIsDefaultTagKey &&
					*tag.Value == awsConstants.TaskDefinitionIsDefaultTagValue {
					isDefault = true
				} else if *tag.Key == awsConstants.TaskDefinitionDockerImageTagKey {
					dockerImage = *tag.Value
				}
			}
		}

		if isDefault && dockerImage != "" {
			return dockerImage, nil
		}
	}

	return "", nil
}

// GetTaskDefinitionForImage looks up an existing task definition for the given Docker image.
// Returns an error if the task definition doesn't exist (does not auto-register).
func GetTaskDefinitionForImage(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	image string,
	log *slog.Logger,
) (string, error) {
	family := TaskDefinitionFamilyName(image)

	listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
		FamilyPrefix: awsStd.String(family),
		Status:       ecsTypes.TaskDefinitionStatusActive,
		MaxResults:   awsStd.Int32(1),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list task definitions: %w", err)
	}

	if len(listOutput.TaskDefinitionArns) > 0 {
		latestARN := listOutput.TaskDefinitionArns[len(listOutput.TaskDefinitionArns)-1]
		log.Debug("task definition found", "context", map[string]string{
			"family": family,
			"arn":    latestARN,
		})
		return latestARN, nil
	}

	return "", fmt.Errorf("task definition for image %q not found (family: %s). "+
		"Image must be registered via /api/v1/images/register",
		image, family)
}

// BuildTaskDefinitionInput creates the ECS RegisterTaskDefinitionInput for a new task definition.
func BuildTaskDefinitionInput(
	ctx context.Context,
	family, image, taskExecRoleARN, taskRoleARN, region string,
	cpu, memory int,
	runtimePlatform string,
	cfg *Config,
) *ecs.RegisterTaskDefinitionInput {
	cpuStr := fmt.Sprintf("%d", cpu)
	memoryStr := fmt.Sprintf("%d", memory)

	return ecsdefs.BuildTaskDefinitionInputForConfig(
		ctx,
		family,
		image,
		taskExecRoleARN,
		taskRoleARN,
		cfg.LogGroup,
		region,
		cpuStr,
		memoryStr,
		runtimePlatform,
	)
}

// checkIfImageIsDefault checks if the image being removed is marked as default.
func checkIfImageIsDefault(ctx context.Context, ecsClient awsClient.ECSClient, family string, log *slog.Logger) bool {
	taskDefArns, err := listTaskDefinitionsByPrefix(ctx, ecsClient, family)
	if err != nil {
		log.Warn("failed to check if image is default before removal", "error", err)
		return false
	}

	for _, taskDefARN := range taskDefArns {
		tagsOutput, listErr := ecsClient.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{
			ResourceArn: awsStd.String(taskDefARN),
		})
		if listErr == nil && tagsOutput != nil {
			for _, tag := range tagsOutput.Tags {
				if tag.Key != nil && *tag.Key == awsConstants.TaskDefinitionIsDefaultTagKey &&
					tag.Value != nil && *tag.Value == awsConstants.TaskDefinitionIsDefaultTagValue {
					return true
				}
			}
		}
	}
	return false
}

// deregisterAllTaskDefRevisions deregisters all active task definition revisions for a given family.
func deregisterAllTaskDefRevisions(
	ctx context.Context, ecsClient awsClient.ECSClient, family, image string, log *slog.Logger,
) error {
	nextToken := ""
	log.Debug("calling external service", "context", map[string]string{
		"operation": "ECS.ListTaskDefinitions",
		"family":    family,
		"image":     image,
		"status":    string(ecsTypes.TaskDefinitionStatusActive),
		"paginated": "true",
	})

	for {
		listOutput, err := ecsClient.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: awsStd.String(family),
			Status:       ecsTypes.TaskDefinitionStatusActive,
			MaxResults:   awsStd.Int32(awsConstants.ECSTaskDefinitionMaxResults),
			NextToken:    awsStd.String(nextToken),
		})
		if err != nil {
			return fmt.Errorf("failed to list task definitions: %w", err)
		}

		for _, taskDefARN := range listOutput.TaskDefinitionArns {
			_, deregErr := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: awsStd.String(taskDefARN),
			})
			if deregErr != nil {
				log.Error("failed to deregister task definition revision", "context", map[string]string{
					"family": family,
					"image":  image,
					"arn":    taskDefARN,
					"error":  deregErr.Error(),
				})
				return fmt.Errorf("failed to deregister task definition revision: %w", deregErr)
			}

			log.Info("deregistered task definition revision", "context", map[string]string{
				"family": family,
				"image":  image,
				"arn":    taskDefARN,
			})
		}

		if listOutput.NextToken == nil {
			break
		}
		nextToken = *listOutput.NextToken
	}

	log.Info("deregistered all task definition revisions", "context", map[string]string{
		"family": family,
		"image":  image,
	})
	return nil
}

// markLastRemainingImageAsDefault marks the last remaining image as default if needed.
func markLastRemainingImageAsDefault(
	ctx context.Context, ecsClient awsClient.ECSClient, family string, log *slog.Logger,
) error {
	familyPrefix := awsConstants.TaskDefinitionFamilyPrefix + "-"
	remainingTaskDefs, err := listTaskDefinitionsByPrefix(ctx, ecsClient, familyPrefix)
	if err != nil {
		log.Warn("failed to list remaining task definitions after removal", "error", err)
		return nil
	}

	remainingImages := extractRemainingImages(ctx, ecsClient, remainingTaskDefs, family, log)
	if len(remainingImages) == 1 {
		var lastImage string
		var lastTaskDefARN string
		for img, arn := range remainingImages {
			lastImage = img
			lastTaskDefARN = arn
		}

		log.Info("only one image remaining after removing default, marking it as default",
			"image", lastImage)

		tagImageAsDefault(ctx, ecsClient, lastImage, lastTaskDefARN, log)
	}
	return nil
}

// extractRemainingImages extracts image-to-task-definition mappings from the given task definition ARNs.
// Returns a map of image names to task definition ARNs.
func extractRemainingImages(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	taskDefARNs []string,
	family string,
	log *slog.Logger,
) map[string]string {
	remainingImages := make(map[string]string)
	for _, taskDefARN := range taskDefARNs {
		image, err := extractImageFromTaskDefinition(ctx, ecsClient, taskDefARN, family, log)
		if err != nil {
			continue
		}
		if image != "" {
			remainingImages[image] = taskDefARN
		}
	}
	return remainingImages
}

// extractImageFromTaskDefinition extracts the runner container image from a task definition.
// Returns the image name and nil error on success, or empty string and error on failure.
func extractImageFromTaskDefinition(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	taskDefARN string,
	family string,
	log *slog.Logger,
) (string, error) {
	descOutput, err := ecsClient.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: awsStd.String(taskDefARN),
	})
	if err != nil {
		log.Error("failed to describe task definition", "context", map[string]string{
			"family": family,
			"arn":    taskDefARN,
			"error":  err.Error(),
		})
		return "", err
	}

	if descOutput.TaskDefinition == nil {
		return "", nil
	}

	for i := range descOutput.TaskDefinition.ContainerDefinitions {
		container := &descOutput.TaskDefinition.ContainerDefinitions[i]
		if container.Name != nil && *container.Name == awsConstants.RunnerContainerName && container.Image != nil {
			return *container.Image, nil
		}
	}
	return "", nil
}

// tagImageAsDefault tags a task definition as the default image.
func tagImageAsDefault(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	image string,
	taskDefARN string,
	log *slog.Logger,
) {
	tags := []ecsTypes.Tag{
		{
			Key:   awsStd.String(awsConstants.TaskDefinitionIsDefaultTagKey),
			Value: awsStd.String(awsConstants.TaskDefinitionIsDefaultTagValue),
		},
		{
			Key:   awsStd.String(awsConstants.TaskDefinitionDockerImageTagKey),
			Value: awsStd.String(image),
		},
	}

	_, err := ecsClient.TagResource(ctx, &ecs.TagResourceInput{
		ResourceArn: awsStd.String(taskDefARN),
		Tags:        tags,
	})
	if err != nil {
		log.Warn("failed to tag last remaining image as default", "context", map[string]string{
			"image": image,
			"arn":   taskDefARN,
			"error": err.Error(),
		})
	} else {
		log.Info("marked last remaining image as default", "context", map[string]string{
			"image": image,
			"arn":   taskDefARN,
		})
	}
}

// DeregisterTaskDefinitionsForImage deregisters all task definition revisions for a given image.
// If the removed image was the default and only one image remains, that image becomes the new default.
func DeregisterTaskDefinitionsForImage(
	ctx context.Context,
	ecsClient awsClient.ECSClient,
	image string,
	log *slog.Logger,
) error {
	family := TaskDefinitionFamilyName(image)

	wasDefault := checkIfImageIsDefault(ctx, ecsClient, family, log)

	if err := deregisterAllTaskDefRevisions(ctx, ecsClient, family, image, log); err != nil {
		return err
	}

	if wasDefault {
		if err := markLastRemainingImageAsDefault(ctx, ecsClient, family, log); err != nil {
			return err
		}
	}

	return nil
}
