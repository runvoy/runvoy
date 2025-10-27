package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runvoy/internal/assets"
	"runvoy/internal/constants"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfnTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const defaultMaxWaitTime = 15 * time.Minute
const defaultMinWaitTime = 10 * time.Second

// AWSProvider implements Provider for AWS
type AWSProvider struct {
	cfg aws.Config
}

// NewAWSProvider creates a new AWS provider
func NewAWSProvider() (Provider, error) {
	return &AWSProvider{}, nil
}

// InitializeInfrastructure deploys the AWS infrastructure
func (p *AWSProvider) InitializeInfrastructure(ctx context.Context, cfg *Config) (*InfrastructureOutput, error) {
	if cfg.StackPrefix == "" {
		return nil, fmt.Errorf("stack prefix is required")
	}
	// Derive stack names from the prefix (implementation detail)
	mainStackName := getMainStackName(cfg.StackPrefix)
	bucketStackName := getBucketStackName(cfg.StackPrefix)

	fmt.Println("→ Loading AWS configuration...")
	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return nil, fmt.Errorf("region is required")
	}
	p.cfg = awsCfg
	fmt.Println("✓ AWS configuration loaded")

	// 1. Generate API key
	fmt.Println("→ Generating API key...")
	apiKey, apiKeyHash, err := p.generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	fmt.Println("✓ API key generated")

	// 2. Build Lambda function
	fmt.Println("→ Building Lambda function...")
	lambdaZip, err := p.buildFunction()
	if err != nil {
		return nil, fmt.Errorf("failed to build Lambda: %w", err)
	}
	fmt.Printf("✓ Lambda function built (size: %.1f KB)\n", float64(len(lambdaZip))/1024)

	// 3. Create bucket stack for Lambda code
	fmt.Printf("→ Creating S3 bucket stack '%s'...\n", bucketStackName)
	bucketName, err := p.createBucketStack(ctx, bucketStackName, cfg.StackPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket stack: %w", err)
	}
	fmt.Printf("✓ S3 bucket created: %s\n", bucketName)

	// 4. Upload Lambda code to S3
	fmt.Println("→ Uploading Lambda code to S3...")
	if err := p.uploadLambdaCode(ctx, bucketName, lambdaZip); err != nil {
		return nil, fmt.Errorf("failed to upload Lambda code: %w", err)
	}
	fmt.Println("✓ Lambda code uploaded to S3")

	// 5. Create main CloudFormation stack
	fmt.Printf("→ Creating main CloudFormation stack '%s' (this may take 5-10 minutes)...\n", mainStackName)
	if err := p.createMainStack(ctx, mainStackName, bucketName, cfg.StackPrefix); err != nil {
		return nil, fmt.Errorf("failed to create main stack: %w", err)
	}
	fmt.Printf("✓ Main stack '%s' created successfully\n", mainStackName)

	// 6. Get outputs from main stack
	fmt.Println("→ Retrieving stack outputs...")
	outputs, err := p.getStackOutputs(ctx, mainStackName)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack outputs: %w", err)
	}
	fmt.Println("✓ Stack outputs retrieved")

	// 7. Insert API key into DynamoDB
	fmt.Println("→ Storing API key in database...")
	apiKeysTable := outputs["APIKeysTableName"]
	if apiKeysTable == "" {
		return nil, fmt.Errorf("API keys table name not found in stack outputs")
	}
	if err := p.insertAPIKey(ctx, apiKeysTable, apiKey, apiKeyHash); err != nil {
		return nil, fmt.Errorf("failed to insert API key: %w", err)
	}
	fmt.Println("✓ API key stored in database")

	return &InfrastructureOutput{
		APIEndpoint:  outputs["APIEndpoint"],
		APIKey:       apiKey,
		Region:       cfg.Region,
		StackPrefix:  cfg.StackPrefix, // Return the prefix, not the individual stack names
		APIKeysTable: apiKeysTable,
		CreatedAt:    time.Now(),
	}, nil
}

func (p *AWSProvider) UpdateInfrastructure(ctx context.Context, cfg *Config) error {
	// TODO: Implement update logic
	return fmt.Errorf("update not yet implemented")
}

func (p *AWSProvider) DestroyInfrastructure(ctx context.Context, cfg *Config) error {
	// Load AWS config
	cfgOpts := []func(*config.LoadOptions) error{}
	if cfg.Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if cfg.Region == "" {
		return fmt.Errorf("region is required")
	}
	p.cfg = awsCfg

	var (
		cfnClient       *cloudformation.Client = cloudformation.NewFromConfig(p.cfg)
		s3Client        *s3.Client             = s3.NewFromConfig(p.cfg)
		mainStackName   string                 = getMainStackName(cfg.StackPrefix)
		bucketStackName string                 = getBucketStackName(cfg.StackPrefix)
	)

	// 1. Delete main stack first
	fmt.Println("→ Deleting main CloudFormation stack...")
	if err := p.deleteStack(ctx, cfnClient, mainStackName); err != nil {
		return fmt.Errorf("failed to delete main stack: %w", err)
	}
	fmt.Printf("✓ Main stack '%s' deleted\n", mainStackName)

	// 2. Empty the S3 bucket
	fmt.Println("→ Emptying S3 bucket...")
	bucketName, err := p.getBucketNameFromStack(ctx, cfnClient, bucketStackName)
	if err != nil {
		// Bucket stack might not exist anymore, try to construct bucket name
		// Bucket name format: {StackName}-lambda-code-{AccountId}-{Region}
		accountId, _ := p.getAccountID(ctx)
		cfgLower := strings.ToLower(strings.TrimSuffix(bucketStackName, "-lambda-bucket"))
		bucketName = fmt.Sprintf("%s-lambda-code-%s-%s", cfgLower, accountId, cfg.Region)
		fmt.Printf("  Attempting to find bucket with constructed name: %s\n", bucketName)
	} else {
		fmt.Printf("  Found bucket: %s\n", bucketName)
	}

	if err := p.emptyBucket(ctx, s3Client, bucketName); err != nil {
		// Log warning but continue - bucket might be empty or not exist
		fmt.Printf("⚠️  Warning: failed to empty bucket: %v\n", err)
	} else {
		fmt.Println("✓ S3 bucket emptied")
	}

	// 3. Delete bucket stack
	fmt.Println("→ Deleting bucket stack...")
	if err := p.deleteStack(ctx, cfnClient, bucketStackName); err != nil {
		// Log warning but continue - stack might not exist
		fmt.Printf("⚠️  Warning: failed to delete bucket stack: %v\n", err)
	} else {
		fmt.Printf("✓ Bucket stack '%s' deleted\n", bucketStackName)
	}

	return nil
}

func (p *AWSProvider) GetEndpoint(ctx context.Context, cfg *Config) (string, error) {
	if cfg.StackPrefix == "" {
		return "", fmt.Errorf("stack prefix is required")
	}
	// Use the main stack name (derived from prefix) to get outputs
	mainStackName := getMainStackName(cfg.StackPrefix)
	outputs, err := p.getStackOutputs(ctx, mainStackName)
	if err != nil {
		return "", err
	}
	return outputs["APIEndpoint"], nil
}

func (p *AWSProvider) ValidateConfig(cfg *Config) error {
	if cfg.StackPrefix == "" {
		return &ValidationError{Message: "stack prefix is required", Field: "stack_prefix"}
	}
	if cfg.Region == "" {
		return &ValidationError{Message: "region is required", Field: "region"}
	}
	return nil
}

func (p *AWSProvider) GetName() string {
	return "aws"
}

// Helper functions for stack naming

func getMainStackName(prefix string) string {
	return fmt.Sprintf("%s-backend", prefix)
}

func getBucketStackName(prefix string) string {
	return fmt.Sprintf("%s-lambda-bucket", prefix)
}

// Helper methods

func (p *AWSProvider) generateAPIKey() (string, string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", err
	}
	apiKey := fmt.Sprintf("sk_live_%s", hex.EncodeToString(randomBytes))

	// Hash with SHA256 for DynamoDB lookup
	hash := sha256.Sum256([]byte(apiKey))
	apiKeyHash := hex.EncodeToString(hash[:])

	return apiKey, apiKeyHash, nil
}

func (p *AWSProvider) buildFunction() ([]byte, error) {
	// Create temporary directory for building
	tmpDir, err := os.MkdirTemp("", "lambda-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract embedded Lambda source to temp directory
	lambdaFS := assets.GetLambdaSourceFS()
	entries, err := lambdaFS.ReadDir("lambda/orchestrator")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded lambda source: %w", err)
	}

	// Copy all .go files from embedded FS to temp directory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		srcPath := filepath.Join("lambda", "orchestrator", entry.Name())
		content, err := lambdaFS.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", srcPath, err)
		}

		dstPath := filepath.Join(tmpDir, entry.Name())
		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s: %w", dstPath, err)
		}
	}

	// Create go.mod file
	goModContent := `module orchestrator

go 1.25.3

require github.com/aws/aws-lambda-go v1.47.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Run go mod download to fetch dependencies
	modDownloadCmd := exec.Command("go", "mod", "download")
	modDownloadCmd.Dir = tmpDir
	if output, err := modDownloadCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("go mod download failed: %w\n%s", err, string(output))
	}

	// Build the Go binary
	buildCmd := exec.Command("go", "build", "-o", "bootstrap")
	buildCmd.Dir = tmpDir
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
	bootstrapPath := filepath.Join(tmpDir, "bootstrap")
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

	return buf.Bytes(), nil
}

func (p *AWSProvider) createBucketStack(ctx context.Context, stackName string, projectName string) (string, error) {
	cfnClient := cloudformation.NewFromConfig(p.cfg)

	// Get embedded bucket template
	bucketTemplateStr, err := assets.GetCloudFormationLambdaBucketTemplate()
	if err != nil {
		return "", fmt.Errorf("failed to load bucket template: %w", err)
	}

	// Create bucket stack
	fmt.Println("   Starting CloudFormation stack creation...")
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: &bucketTemplateStr,
		Parameters: []cfnTypes.Parameter{
			{
				ParameterKey:   aws.String("ProjectName"),
				ParameterValue: aws.String(projectName),
			},
		},
		Tags: []cfnTypes.Tag{
			{Key: strPtr("Project"), Value: strPtr(constants.ProjectName)},
			{Key: strPtr("Stack"), Value: strPtr("Lambda-Bucket")},
		},
	})
	if err != nil {
		return "", err
	}

	// Wait for stack creation
	fmt.Println("   Waiting for stack to become ready (this may take a minute)...")
	bucketWaiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)

	err = bucketWaiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	}, maxWaitForContext(ctx))
	if err != nil {
		return "", err
	}

	// Get bucket name from outputs
	fmt.Println("   Retrieving bucket name from stack outputs...")
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil || len(resp.Stacks) == 0 {
		return "", err
	}

	outputs := parseStackOutputs(resp.Stacks[0].Outputs)
	bucketName := outputs["BucketName"]
	if bucketName == "" {
		return "", fmt.Errorf("bucket name not found in stack outputs")
	}

	return bucketName, nil
}

func (p *AWSProvider) uploadLambdaCode(ctx context.Context, bucketName string, lambdaZip []byte) error {
	s3Client := s3.NewFromConfig(p.cfg)
	lambdaKey := "bootstrap.zip"

	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &lambdaKey,
		Body:   bytes.NewReader(lambdaZip),
	})
	return err
}

func (p *AWSProvider) createMainStack(ctx context.Context, stackName string, bucketName string, projectName string) error {
	cfnClient := cloudformation.NewFromConfig(p.cfg)

	lambdaKey := "bootstrap.zip"
	cfnParams := []cfnTypes.Parameter{
		{
			ParameterKey:   aws.String("LambdaCodeBucket"),
			ParameterValue: aws.String(bucketName),
		},
		{
			ParameterKey:   aws.String("LambdaCodeKey"),
			ParameterValue: aws.String(lambdaKey),
		},
		{
			ParameterKey:   aws.String("ProjectName"),
			ParameterValue: aws.String(projectName),
		},
	}

	// Get embedded main CloudFormation template
	templateStr, err := assets.GetCloudFormationBackendTemplate()
	if err != nil {
		return fmt.Errorf("failed to load backend template: %w", err)
	}

	fmt.Println("   Starting CloudFormation stack creation...")
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: &templateStr,
		Capabilities: []cfnTypes.Capability{cfnTypes.CapabilityCapabilityNamedIam},
		Parameters:   cfnParams,
		Tags: []cfnTypes.Tag{
			{Key: strPtr("Project"), Value: strPtr(constants.ProjectName)},
		},
	})
	if err != nil {
		return err
	}

	// Wait for stack creation
	fmt.Println("   Waiting for stack to become ready...")
	waiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)

	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	}, maxWaitForContext(ctx))
	return err
}

func (p *AWSProvider) getStackOutputs(ctx context.Context, stackName string) (map[string]string, error) {
	cfnClient := cloudformation.NewFromConfig(p.cfg)
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil || len(resp.Stacks) == 0 {
		return nil, err
	}
	return parseStackOutputs(resp.Stacks[0].Outputs), nil
}

func (p *AWSProvider) insertAPIKey(ctx context.Context, tableName, apiKey, apiKeyHash string) error {
	dynamoDBClient := dynamodb.NewFromConfig(p.cfg)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := dynamoDBClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]dynamodbTypes.AttributeValue{
			"api_key_hash": &dynamodbTypes.AttributeValueMemberS{Value: apiKeyHash},
			"user_email":   &dynamodbTypes.AttributeValueMemberS{Value: fmt.Sprintf("admin@%s.local", constants.ProjectName)},
			"created_at":   &dynamodbTypes.AttributeValueMemberS{Value: now},
			"revoked":      &dynamodbTypes.AttributeValueMemberBOOL{Value: false},
			"last_used":    &dynamodbTypes.AttributeValueMemberS{Value: now},
		},
	})
	return err
}

func parseStackOutputs(outputs []cfnTypes.Output) map[string]string {
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

func maxWaitForContext(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		// Use 10 seconds less than context deadline to avoid timeout
		maxwait := remaining - defaultMinWaitTime
		return maxwait
	}

	return defaultMaxWaitTime
}

func (p *AWSProvider) deleteStack(ctx context.Context, cfnClient *cloudformation.Client, stackName string) error {
	// Delete the stack
	_, err := cfnClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
		StackName: &stackName,
	})
	if err != nil {
		return fmt.Errorf("failed to initiate stack deletion: %w", err)
	}

	// Wait for stack deletion to complete
	fmt.Printf("   Waiting for stack '%s' to be deleted (this may take several minutes)...\n", stackName)

	// Calculate maxwait based on context deadline
	maxwait := maxWaitForContext(ctx)

	waiter := cloudformation.NewStackDeleteCompleteWaiter(cfnClient)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	}, maxwait)
	if err != nil {
		return fmt.Errorf("failed to wait for stack deletion: %w", err)
	}

	return nil
}

func (p *AWSProvider) getBucketNameFromStack(ctx context.Context, cfnClient *cloudformation.Client, stackName string) (string, error) {
	resp, err := cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Stacks) == 0 {
		return "", fmt.Errorf("stack not found")
	}

	outputs := parseStackOutputs(resp.Stacks[0].Outputs)
	bucketName := outputs["BucketName"]
	if bucketName == "" {
		return "", fmt.Errorf("bucket name not found in stack outputs")
	}

	return bucketName, nil
}

func (p *AWSProvider) emptyBucket(ctx context.Context, s3Client *s3.Client, bucketName string) error {
	// List all objects including versions
	paginator := s3.NewListObjectVersionsPaginator(s3Client, &s3.ListObjectVersionsInput{
		Bucket: &bucketName,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list bucket objects: %w", err)
		}

		// Delete all versions
		var objectsToDelete []s3Types.ObjectIdentifier
		for _, version := range page.Versions {
			if version.Key != nil {
				objectsToDelete = append(objectsToDelete, s3Types.ObjectIdentifier{
					Key:       version.Key,
					VersionId: version.VersionId,
				})
			}
		}

		// Delete all delete markers
		for _, marker := range page.DeleteMarkers {
			if marker.Key != nil {
				objectsToDelete = append(objectsToDelete, s3Types.ObjectIdentifier{
					Key:       marker.Key,
					VersionId: marker.VersionId,
				})
			}
		}

		// Delete objects in batches
		if len(objectsToDelete) > 0 {
			_, err := s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: &bucketName,
				Delete: &s3Types.Delete{
					Objects: objectsToDelete,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to delete objects: %w", err)
			}
		}
	}

	return nil
}

func (p *AWSProvider) getAccountID(ctx context.Context) (string, error) {
	// Use STS to get account ID
	stsClient := sts.NewFromConfig(p.cfg)
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if result.Account == nil {
		return "", fmt.Errorf("account ID not found")
	}
	return *result.Account, nil
}
