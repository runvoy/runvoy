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
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
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
	Long: `Destroys the CloudFormation stack and all associated resources:
- Empties and deletes the S3 bucket
- Deletes the CloudFormation stack (Lambda, API Gateway, ECS, VPC, etc.)
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

	// Get AWS account ID for bucket name
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get AWS identity: %w", err)
	}
	accountID := *identity.Account
	bucketName := fmt.Sprintf("mycli-code-%s", accountID)

	// 1. Empty S3 bucket (required before CloudFormation can delete it)
	fmt.Println("→ Emptying S3 bucket...")
	if err := emptyBucket(ctx, cfg, bucketName, destroyRegion); err != nil {
		fmt.Printf("  Warning: Failed to empty bucket: %v\n", err)
		fmt.Println("  Continuing with stack deletion...")
	} else {
		fmt.Println("✓ Bucket emptied")
	}

	// 2. Delete Lambda function (created outside CloudFormation)
	fmt.Println("→ Deleting Lambda function...")
	lambdaClient := lambda.NewFromConfig(cfg)
	functionName := fmt.Sprintf("%s-orchestrator", destroyStackName)

	_, err = lambdaClient.DeleteFunction(ctx, &lambda.DeleteFunctionInput{
		FunctionName: &functionName,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "ResourceNotFoundException") {
			fmt.Printf("  Warning: Failed to delete Lambda: %v\n", err)
		} else {
			fmt.Println("  (Lambda function not found)")
		}
	} else {
		fmt.Println("✓ Lambda function deleted")
	}

	// 3. Delete CloudFormation stack
	cfnClient := cloudformation.NewFromConfig(cfg)

	fmt.Println("→ Deleting CloudFormation stack...")

	// Check if stack exists
	_, err = cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &destroyStackName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			fmt.Println("  Stack does not exist (already deleted)")
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

		fmt.Println("✓ Stack deleted successfully")
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

func emptyBucket(ctx context.Context, cfg aws.Config, bucketName, region string) error {
	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = region
		o.UsePathStyle = false
	})

	// Check if bucket exists
	_, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		// Bucket doesn't exist, nothing to empty
		return nil
	}

	// List all objects
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: &bucketName,
	})

	objectsDeleted := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if len(page.Contents) == 0 {
			continue
		}

		// Delete objects in batch
		var objectIds []s3Types.ObjectIdentifier
		for _, obj := range page.Contents {
			objectIds = append(objectIds, s3Types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		_, err = s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3Types.Delete{
				Objects: objectIds,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete objects: %w", err)
		}

		objectsDeleted += len(objectIds)
	}

	if objectsDeleted > 0 {
		fmt.Printf("  Deleted %d objects from bucket\n", objectsDeleted)
	}

	return nil
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/.mycli/config.yaml", home), nil
}
