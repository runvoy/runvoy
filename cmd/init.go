package cmd

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	internalConfig "mycli/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

//go:embed cloudformation.yaml
var cfnTemplate string

var (
	initStackName string
	initRegion    string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mycli infrastructure in your AWS account",
	Long: `Deploys the complete mycli infrastructure to your AWS account:
- Creates CloudFormation stack with all required resources
- Generates and stores a secure API key
- Configures the CLI automatically

This is a one-time setup command.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initStackName, "stack-name", "mycli", "CloudFormation stack name")
	initCmd.Flags().StringVar(&initRegion, "region", "", "AWS region (default: us-east-2)")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Println("Initializing mycli infrastructure...")
	fmt.Printf("   Stack name: %s\n", initStackName)

	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if initRegion != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(initRegion))
	}
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if initRegion == "" {
		if cfg.Region != "" {
			initRegion = cfg.Region
		} else {
			initRegion = "us-east-2" // Default to us-east-2 (cheap, no S3 quirks)
		}
	}

	// Ensure cfg uses the selected region
	cfg.Region = initRegion
	fmt.Printf("   Region: %s\n\n", initRegion)

	// Get AWS account ID
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get AWS identity: %w", err)
	}
	accountID := *identity.Account

	// 1. Generate API key
	fmt.Println("→ Generating API key...")
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	apiKey := fmt.Sprintf("sk_live_%s", hex.EncodeToString(randomBytes))

	// 2. Hash with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash key: %w", err)
	}
	apiKeyHash := string(hash)

	// 3. Build Lambda function
	fmt.Println("→ Building Lambda function...")
	lambdaZip, err := buildLambda()
	if err != nil {
		return fmt.Errorf("failed to build Lambda: %w", err)
	}

	// 4. Create S3 bucket for code storage
	bucketName := fmt.Sprintf("mycli-code-%s", accountID)
	fmt.Println("→ Creating S3 bucket...")
	s3Client := s3.NewFromConfig(cfg)

	createBucketInput := &s3.CreateBucketInput{
		Bucket: &bucketName,
	}

	// us-east-1 doesn't need (and doesn't accept) LocationConstraint
	if initRegion != "us-east-1" {
		createBucketInput.CreateBucketConfiguration = &s3Types.CreateBucketConfiguration{
			LocationConstraint: s3Types.BucketLocationConstraint(initRegion),
		}
	}

	_, err = s3Client.CreateBucket(ctx, createBucketInput)
	if err != nil {
		// Ignore error if bucket already exists
		if !strings.Contains(err.Error(), "BucketAlreadyOwnedByYou") &&
			!strings.Contains(err.Error(), "BucketAlreadyExists") {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		fmt.Println("  (bucket already exists)")
	}

	// 5. Create CloudFormation stack
	cfnClient := cloudformation.NewFromConfig(cfg)

	fmt.Println("→ Creating CloudFormation stack...")
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &initStackName,
		TemplateBody: &cfnTemplate,
		Capabilities: []types.Capability{types.CapabilityCapabilityNamedIam},
		Parameters: []types.Parameter{
			{
				ParameterKey:   aws.String("APIKeyHash"),
				ParameterValue: aws.String(apiKeyHash),
			},
			{
				ParameterKey:   aws.String("CodeBucketName"),
				ParameterValue: aws.String(bucketName),
			},
		},
		Tags: []types.Tag{
			{Key: strPtr("Project"), Value: strPtr("mycli")},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	fmt.Println("  Waiting for stack creation (this may take a few minutes)...")

	// Wait for stack creation
	waiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &initStackName,
	}, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("stack creation failed: %w", err)
	}

	fmt.Println("✓ Stack created successfully")

	// 6. Get stack outputs
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &initStackName,
	})
	if err != nil || len(resp.Stacks) == 0 {
		return fmt.Errorf("failed to describe stack: %w", err)
	}

	stack := resp.Stacks[0]
	outputs := parseStackOutputs(stack.Outputs)

	apiEndpoint := outputs["APIEndpoint"]
	codeBucket := outputs["CodeBucket"]
	lambdaRoleArn := outputs["LambdaExecutionRoleArn"]

	// 7. Create Lambda function with direct zip upload
	fmt.Println("→ Creating Lambda function...")
	lambdaClient := lambda.NewFromConfig(cfg)
	functionName := fmt.Sprintf("%s-orchestrator", initStackName)

	_, err = lambdaClient.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName: &functionName,
		Runtime:      lambdaTypes.RuntimeProvidedal2023,
		Role:         &lambdaRoleArn,
		Handler:      aws.String("bootstrap"),
		Code: &lambdaTypes.FunctionCode{
			ZipFile: lambdaZip,
		},
		Timeout:       aws.Int32(60),
		Architectures: []lambdaTypes.Architecture{lambdaTypes.ArchitectureArm64},
		Environment: &lambdaTypes.Environment{
			Variables: map[string]string{
				"API_KEY_HASH":    apiKeyHash,
				"CODE_BUCKET":     bucketName,
				"ECS_CLUSTER":     outputs["ECSClusterName"],
				"TASK_DEFINITION": outputs["TaskDefinitionArn"],
				"SUBNET_1":        outputs["Subnet1"],
				"SUBNET_2":        outputs["Subnet2"],
				"SECURITY_GROUP":  outputs["FargateSecurityGroup"],
				"LOG_GROUP":       outputs["LogGroup"],
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Lambda function: %w", err)
	}

	fmt.Println("✓ Lambda function created")

	// 8. Configure API Gateway integration
	fmt.Println("→ Configuring API Gateway...")
	restAPIId := outputs["RestAPIId"]
	resourceId := outputs["APIResourceId"]
	lambdaArn := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", initRegion, accountID, functionName)

	apigwClient := apigateway.NewFromConfig(cfg)

	// Create API Gateway method
	_, err = apigwClient.PutMethod(ctx, &apigateway.PutMethodInput{
		RestApiId:         &restAPIId,
		ResourceId:        &resourceId,
		HttpMethod:        aws.String("POST"),
		AuthorizationType: aws.String("NONE"),
	})
	if err != nil {
		return fmt.Errorf("failed to create API Gateway method: %w", err)
	}

	// Create Lambda integration
	integrationUri := fmt.Sprintf("arn:aws:apigateway:%s:lambda:path/2015-03-31/functions/%s/invocations", initRegion, lambdaArn)
	_, err = apigwClient.PutIntegration(ctx, &apigateway.PutIntegrationInput{
		RestApiId:             &restAPIId,
		ResourceId:            &resourceId,
		HttpMethod:            aws.String("POST"),
		Type:                  "AWS_PROXY",
		IntegrationHttpMethod: aws.String("POST"),
		Uri:                   &integrationUri,
	})
	if err != nil {
		return fmt.Errorf("failed to create API Gateway integration: %w", err)
	}

	// Add Lambda permission for API Gateway
	sourceArn := fmt.Sprintf("arn:aws:execute-api:%s:%s:%s/*/*", initRegion, accountID, restAPIId)
	_, err = lambdaClient.AddPermission(ctx, &lambda.AddPermissionInput{
		FunctionName: &functionName,
		StatementId:  aws.String("AllowAPIGatewayInvoke"),
		Action:       aws.String("lambda:InvokeFunction"),
		Principal:    aws.String("apigateway.amazonaws.com"),
		SourceArn:    &sourceArn,
	})
	if err != nil {
		return fmt.Errorf("failed to add Lambda permission: %w", err)
	}

	// Deploy API
	_, err = apigwClient.CreateDeployment(ctx, &apigateway.CreateDeploymentInput{
		RestApiId: &restAPIId,
		StageName: aws.String("prod"),
	})
	if err != nil {
		return fmt.Errorf("failed to deploy API Gateway: %w", err)
	}

	fmt.Println("✓ API Gateway configured")

	// 9. Save to config file
	fmt.Println("→ Saving configuration...")
	cliConfig := &internalConfig.Config{
		APIEndpoint: apiEndpoint,
		APIKey:      apiKey,
		CodeBucket:  codeBucket,
		Region:      initRegion,
	}
	if err := internalConfig.Save(cliConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// 10. Success!
	fmt.Println("\n✅ Setup complete!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Configuration saved to ~/.mycli/config.yaml")
	fmt.Printf("  API Endpoint: %s\n", apiEndpoint)
	fmt.Printf("  Code Bucket:  %s\n", codeBucket)
	fmt.Printf("  Region:       %s\n", initRegion)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\n🔑 Your API key: %s\n", apiKey)
	fmt.Println("   (Also saved to config file)")
	fmt.Println("\nTest it with: mycli exec \"echo hello\"")

	return nil
}

func buildLambda() ([]byte, error) {
	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Navigate to lambda/orchestrator directory
	lambdaDir := filepath.Join(cwd, "lambda", "orchestrator")
	if _, err := os.Stat(lambdaDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("lambda directory not found: %s", lambdaDir)
	}

	// Build the Go binary
	buildCmd := exec.Command("go", "build", "-tags", "lambda.norpc", "-o", "bootstrap", "main.go")
	buildCmd.Dir = lambdaDir
	buildCmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=arm64",
		"CGO_ENABLED=0",
	)

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("build failed: %w\n%s", err, string(output))
	}

	// Create zip file
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	// Add bootstrap file
	bootstrapPath := filepath.Join(lambdaDir, "bootstrap")
	bootstrapFile, err := os.Open(bootstrapPath)
	if err != nil {
		return nil, err
	}
	defer bootstrapFile.Close()

	info, err := bootstrapFile.Stat()
	if err != nil {
		return nil, err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, err
	}
	header.Name = "bootstrap"
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(writer, bootstrapFile)
	if err != nil {
		return nil, err
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	// Clean up
	os.Remove(bootstrapPath)

	return buf.Bytes(), nil
}

func parseStackOutputs(outputs []types.Output) map[string]string {
	result := make(map[string]string)
	for _, output := range outputs {
		if output.OutputKey != nil && output.OutputValue != nil {
			result[*output.OutputKey] = *output.OutputValue
		}
	}
	return result
}

func strPtr(s string) *string {
	return &s
}
