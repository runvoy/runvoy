package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	internalConfig "mycli/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
)

var (
	destroyStackName string
	destroyRegion    string
	forceDestroy     bool
	keepConfig       bool
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy mycli infrastructure and clean up AWS resources",
	Long: `Destroys both CloudFormation stacks and all associated resources:
- Deletes the main CloudFormation stack (Lambda Function URL, ECS, VPC, etc.)
- Empties and deletes the S3 bucket for Lambda code
- Deletes the Lambda bucket CloudFormation stack
- Cleans up ECS task definitions
- Optionally removes local configuration

This is useful for cleaning up during development or completely removing mycli.`,
	RunE: runDestroy,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().StringVar(&destroyStackName, "stack-name", "mycli", "CloudFormation stack name")
	destroyCmd.Flags().StringVar(&destroyRegion, "region", "", "AWS region (default: from config or AWS profile)")
	destroyCmd.Flags().BoolVar(&forceDestroy, "force", false, "Skip confirmation prompt")
	destroyCmd.Flags().BoolVar(&keepConfig, "keep-config", false, "Keep local config file after destruction")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Safety confirmation
	if !forceDestroy {
		fmt.Printf("⚠️  WARNING: This will destroy the CloudFormation stack '%s' and all resources.\n", destroyStackName)
		fmt.Println("   This action cannot be undone.")
		fmt.Print("\nType 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Destroy cancelled.")
			return nil
		}
	}

	fmt.Println("\nDestroying mycli infrastructure...")

	// Try to load region from config if not specified
	if destroyRegion == "" {
		cfg, err := internalConfig.Load()
		if err == nil && cfg.Region != "" {
			destroyRegion = cfg.Region
		}
	}

	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if destroyRegion != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(destroyRegion))
	}
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if destroyRegion == "" {
		destroyRegion = cfg.Region
	}

	fmt.Printf("   Region: %s\n", destroyRegion)
	fmt.Printf("   Stack: %s\n\n", destroyStackName)

	// 1. Delete ECS task definitions (created dynamically by Lambda)
	fmt.Println("→ Deleting ECS task definitions...")
	ecsClient := ecs.NewFromConfig(cfg)
	if err := deleteTaskDefinitions(ctx, ecsClient, destroyStackName); err != nil {
		fmt.Printf("  Warning: Failed to delete task definitions: %v\n", err)
	}

	// 2. Delete main CloudFormation stack
	cfnClient := cloudformation.NewFromConfig(cfg)

	fmt.Println("→ Deleting main CloudFormation stack...")

	// Check if stack exists
	_, err = cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &destroyStackName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fmt.Println("  Main stack does not exist (already deleted)")
		} else {
			return fmt.Errorf("failed to describe stack: %w", err)
		}
	} else {
		// Delete stack
		_, err = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &destroyStackName,
		})
		if err != nil {
			return fmt.Errorf("failed to delete stack: %w", err)
		}

		fmt.Println("  Waiting for stack deletion (this may take a few minutes)...")

		// Wait for stack deletion
		waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
		err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
			StackName: &destroyStackName,
		}, 15*time.Minute)
		if err != nil {
			return fmt.Errorf("stack deletion failed: %w", err)
		}

		fmt.Println("✓ Main stack deleted successfully")
	}

	// 3. Delete Lambda bucket stack
	bucketStackName := fmt.Sprintf("%s-lambda-bucket", destroyStackName)

	fmt.Println("→ Deleting Lambda bucket stack...")

	// First, get the bucket name from stack outputs
	bucketResp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &bucketStackName,
	})

	var bucketName string
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fmt.Println("  Lambda bucket stack does not exist (already deleted)")
		} else {
			fmt.Printf("  Warning: Failed to describe bucket stack: %v\n", err)
		}
	} else {
		// Get bucket name from outputs
		if len(bucketResp.Stacks) > 0 {
			outputs := parseStackOutputs(bucketResp.Stacks[0].Outputs)
			bucketName = outputs["BucketName"]
		}

		// Empty the S3 bucket before deleting the stack
		if bucketName != "" {
			fmt.Println("  Emptying S3 bucket...")
			s3Client := s3.NewFromConfig(cfg)
			if err := emptyS3Bucket(ctx, s3Client, bucketName); err != nil {
				fmt.Printf("  Warning: Failed to empty bucket: %v\n", err)
			} else {
				fmt.Println("  ✓ S3 bucket emptied")
			}
		}

		// Delete bucket stack
		_, err = cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: &bucketStackName,
		})
		if err != nil {
			fmt.Printf("  Warning: Failed to delete bucket stack: %v\n", err)
		} else {
			fmt.Println("  Waiting for bucket stack deletion...")

			// Wait for bucket stack deletion
			bucketWaiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
			err = bucketWaiter.Wait(ctx, &cloudformation.DescribeStacksInput{
				StackName: &bucketStackName,
			}, 5*time.Minute)
			if err != nil {
				fmt.Printf("  Warning: Bucket stack deletion timed out or failed: %v\n", err)
			} else {
				fmt.Println("✓ Lambda bucket stack deleted successfully")
			}
		}
	}

	// 4. Remove local config
	if !keepConfig {
		fmt.Println("→ Removing local configuration...")
		configPath, err := getConfigPath()
		if err == nil {
			if err := os.Remove(configPath); err != nil {
				if !os.IsNotExist(err) {
					fmt.Printf("  Warning: Failed to remove config: %v\n", err)
				}
			} else {
				fmt.Println("✓ Config removed")
			}
		}
	}

	fmt.Println("\n✅ Destruction complete!")
	fmt.Println("   All AWS resources have been removed.")

	return nil
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/.mycli/config.yaml", home), nil
}

// deleteTaskDefinitions lists and deletes all ECS task definitions with the mycli prefix
func deleteTaskDefinitions(ctx context.Context, ecsClient *ecs.Client, stackName string) error {
	// Collect all mycli task definitions directly from ListTaskDefinitions
	// We don't use ListTaskDefinitionFamilies because it only lists ACTIVE families
	// and we need to delete both ACTIVE and INACTIVE task definitions
	fmt.Println("  Collecting task definitions...")

	// Deregister all revisions of each family (both active and inactive)
	deletedCount := 0

	// Build a map of all task definitions to avoid listing all of them multiple times
	// This is more efficient when we have multiple families
	type statusType string
	const (
		statusActive   statusType = "ACTIVE"
		statusInactive statusType = "INACTIVE"
	)

	taskDefsByFamily := make(map[string]map[statusType][]string)

	// Collect all mycli task definitions once (both active and inactive)
	fmt.Println("  Collecting task definitions...")

	// Collect active task definitions
	activePaginator := ecs.NewListTaskDefinitionsPaginator(ecsClient, &ecs.ListTaskDefinitionsInput{
		Status: ecsTypes.TaskDefinitionStatusActive,
	})
	activeCount := 0
	for activePaginator.HasMorePages() {
		page, err := activePaginator.NextPage(ctx)
		if err != nil {
			fmt.Printf("  Warning: Failed to list active task definitions: %v\n", err)
			break
		}
		for _, arn := range page.TaskDefinitionArns {
			// Extract family name from ARN: arn:aws:ecs:region:account:task-definition/family:revision
			// Check for mycli prefix (case-insensitive)
			lowerArn := strings.ToLower(arn)
			if strings.Contains(lowerArn, ":task-definition/mycli") {
				// Split on :task-definition/ instead of /task-definition/
				parts := strings.Split(arn, ":task-definition/")
				if len(parts) == 2 {
					family := strings.Split(parts[1], ":")[0]
					if taskDefsByFamily[family] == nil {
						taskDefsByFamily[family] = make(map[statusType][]string)
					}
					taskDefsByFamily[family][statusActive] = append(taskDefsByFamily[family][statusActive], arn)
					activeCount++
				}
			}
		}
	}

	// Collect inactive task definitions
	inactivePaginator := ecs.NewListTaskDefinitionsPaginator(ecsClient, &ecs.ListTaskDefinitionsInput{
		Status: ecsTypes.TaskDefinitionStatusInactive,
	})
	inactiveCount := 0
	for inactivePaginator.HasMorePages() {
		page, err := inactivePaginator.NextPage(ctx)
		if err != nil {
			fmt.Printf("  Warning: Failed to list inactive task definitions: %v\n", err)
			break
		}
		for _, arn := range page.TaskDefinitionArns {
			// Extract family name from ARN - check for mycli prefix (case-insensitive)
			lowerArn := strings.ToLower(arn)
			if strings.Contains(lowerArn, ":task-definition/mycli") {
				// Split on :task-definition/ instead of /task-definition/
				parts := strings.Split(arn, ":task-definition/")
				if len(parts) == 2 {
					family := strings.Split(parts[1], ":")[0]
					if taskDefsByFamily[family] == nil {
						taskDefsByFamily[family] = make(map[statusType][]string)
					}
					taskDefsByFamily[family][statusInactive] = append(taskDefsByFamily[family][statusInactive], arn)
					inactiveCount++
				}
			}
		}
	}

	fmt.Printf("  Found %d active and %d inactive mycli task definitions across %d families\n",
		activeCount, inactiveCount, len(taskDefsByFamily))

	if len(taskDefsByFamily) == 0 {
		fmt.Println("  (No task definitions found with mycli prefix)")
		return nil
	}

	// First, deregister all ACTIVE task definitions
	fmt.Println("  Deregistering active task definitions...")
	for _, families := range taskDefsByFamily {
		for _, taskDefArn := range families[statusActive] {
			_, err := ecsClient.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
				TaskDefinition: aws.String(taskDefArn),
			})
			if err != nil {
				fmt.Printf("  Warning: Failed to deregister %s: %v\n", taskDefArn, err)
			} else {
				fmt.Printf("  Deregistered: %s\n", taskDefArn)
				deletedCount++
			}
		}
	}

	// Then, delete all INACTIVE task definitions (now including those we just deregistered)
	fmt.Println("  Deleting inactive task definitions...")
	var inactiveTaskDefs []string
	for _, families := range taskDefsByFamily {
		inactiveTaskDefs = append(inactiveTaskDefs, families[statusInactive]...)
	}

	// Delete all inactive task definitions
	if len(inactiveTaskDefs) > 0 {
		result, err := ecsClient.DeleteTaskDefinitions(ctx, &ecs.DeleteTaskDefinitionsInput{
			TaskDefinitions: inactiveTaskDefs,
		})
		if err != nil {
			fmt.Printf("  Warning: Failed to delete task definitions: %v\n", err)
		} else {
			// TaskDefinitions contains successfully deleted task definitions
			if len(result.TaskDefinitions) > 0 {
				for _, taskDef := range result.TaskDefinitions {
					fmt.Printf("  Deleted: %s\n", *taskDef.TaskDefinitionArn)
					deletedCount++
				}
			}
			// Failures contains any failures
			if len(result.Failures) > 0 {
				for _, failure := range result.Failures {
					reason := "unknown error"
					if failure.Reason != nil {
						reason = *failure.Reason
					}
					arn := "unknown"
					if failure.Arn != nil {
						arn = *failure.Arn
					}
					fmt.Printf("  Warning: Failed to delete %s: %s\n", arn, reason)
				}
			}
		}
	}

	if deletedCount > 0 {
		fmt.Printf("✓ Deleted %d task definitions\n", deletedCount)
	} else {
		fmt.Println("✓ Task definitions cleaned up")
	}

	return nil
}

// emptyS3Bucket empties all objects and versions from an S3 bucket
func emptyS3Bucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	// List and delete all object versions
	paginator := s3.NewListObjectVersionsPaginator(s3Client, &s3.ListObjectVersionsInput{
		Bucket: &bucketName,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list object versions: %w", err)
		}

		// Delete objects in batches
		if len(page.Versions) > 0 || len(page.DeleteMarkers) > 0 {
			var objectsToDelete []s3Types.ObjectIdentifier

			// Add versions
			for _, version := range page.Versions {
				objectsToDelete = append(objectsToDelete, s3Types.ObjectIdentifier{
					Key:       version.Key,
					VersionId: version.VersionId,
				})
			}

			// Add delete markers
			for _, marker := range page.DeleteMarkers {
				objectsToDelete = append(objectsToDelete, s3Types.ObjectIdentifier{
					Key:       marker.Key,
					VersionId: marker.VersionId,
				})
			}

			if len(objectsToDelete) > 0 {
				_, err = s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
					Bucket: &bucketName,
					Delete: &s3Types.Delete{
						Objects: objectsToDelete,
						Quiet:   aws.Bool(true),
					},
				})
				if err != nil {
					return fmt.Errorf("failed to delete objects: %w", err)
				}
			}
		}
	}

	return nil
}
