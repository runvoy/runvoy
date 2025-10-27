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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfnTypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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
	lambdaZip, err := p.buildLambda()
	if err != nil {
		return nil, fmt.Errorf("failed to build Lambda: %w", err)
	}
	fmt.Printf("✓ Lambda function built (size: %.1f KB)\n", float64(len(lambdaZip))/1024)

	// 3. Create bucket stack for Lambda code
	fmt.Printf("→ Creating S3 bucket stack '%s'...\n", fmt.Sprintf("%s-lambda-bucket", cfg.StackName))
	bucketStackName := fmt.Sprintf("%s-lambda-bucket", cfg.StackName)
	bucketName, err := p.createBucketStack(ctx, bucketStackName, cfg)
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
	fmt.Printf("→ Creating main CloudFormation stack '%s' (this may take 5-10 minutes)...\n", cfg.StackName)
	if err := p.createMainStack(ctx, cfg, bucketName); err != nil {
		return nil, fmt.Errorf("failed to create main stack: %w", err)
	}
	fmt.Printf("✓ Main stack '%s' created successfully\n", cfg.StackName)

	// 6. Get outputs from main stack
	fmt.Println("→ Retrieving stack outputs...")
	outputs, err := p.getStackOutputs(ctx, cfg.StackName)
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
		StackName:    cfg.StackName,
		APIKeysTable: apiKeysTable,
		CreatedAt:    time.Now(),
	}, nil
}

func (p *AWSProvider) UpdateInfrastructure(ctx context.Context, cfg *Config) error {
	// TODO: Implement update logic
	return fmt.Errorf("update not yet implemented")
}

func (p *AWSProvider) DestroyInfrastructure(ctx context.Context, cfg *Config) error {
	// TODO: Implement destroy logic
	return fmt.Errorf("destroy not yet implemented")
}

func (p *AWSProvider) GetEndpoint(ctx context.Context, cfg *Config) (string, error) {
	outputs, err := p.getStackOutputs(ctx, cfg.StackName)
	if err != nil {
		return "", err
	}
	return outputs["APIEndpoint"], nil
}

func (p *AWSProvider) ValidateConfig(cfg *Config) error {
	if cfg.StackName == "" {
		return &ValidationError{Message: "stack name is required", Field: "stack_name"}
	}
	if cfg.Region == "" {
		return &ValidationError{Message: "region is required", Field: "region"}
	}
	return nil
}

func (p *AWSProvider) GetName() string {
	return "aws"
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

func (p *AWSProvider) buildLambda() ([]byte, error) {
	// Find project root
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Navigate to backend/orchestrator directory
	lambdaDir := filepath.Join(cwd, "backend", "orchestrator")
	if _, err := os.Stat(lambdaDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("orchestrator directory not found: %s", lambdaDir)
	}

	// Build the Go binary
	buildCmd := exec.Command("go", "build", "-o", "bootstrap")
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

func (p *AWSProvider) createBucketStack(ctx context.Context, stackName string, cfg *Config) (string, error) {
	cfnClient := cloudformation.NewFromConfig(p.cfg)

	// Read bucket template
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	bucketTemplatePath := filepath.Join(cwd, "deploy", "cloudformation-bucket.yaml")
	bucketTemplateBody, err := os.ReadFile(bucketTemplatePath)
	if err != nil {
		return "", err
	}
	bucketTemplateStr := string(bucketTemplateBody)

	// Create bucket stack
	fmt.Println("   Starting CloudFormation stack creation...")
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &stackName,
		TemplateBody: &bucketTemplateStr,
		Parameters: []cfnTypes.Parameter{
			{
				ParameterKey:   aws.String("ProjectName"),
				ParameterValue: aws.String(cfg.StackName),
			},
		},
		Tags: []cfnTypes.Tag{
			{Key: strPtr("Project"), Value: strPtr("mycli")},
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
	}, 5*time.Minute)
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

func (p *AWSProvider) createMainStack(ctx context.Context, cfg *Config, bucketName string) error {
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
	}

	// Read main CloudFormation template
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	templatePath := filepath.Join(cwd, "deploy", "cloudformation.yaml")
	templateBody, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	templateStr := string(templateBody)

	fmt.Println("   Starting CloudFormation stack creation...")
	_, err = cfnClient.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    &cfg.StackName,
		TemplateBody: &templateStr,
		Capabilities: []cfnTypes.Capability{cfnTypes.CapabilityCapabilityNamedIam},
		Parameters:   cfnParams,
		Tags: []cfnTypes.Tag{
			{Key: strPtr("Project"), Value: strPtr("mycli")},
		},
	})
	if err != nil {
		return err
	}

	// Wait for stack creation
	fmt.Println("   Waiting for stack to become ready...")
	fmt.Println("   Progress: Creating Lambda function, API Gateway, and DynamoDB table...")
	waiter := cloudformation.NewStackCreateCompleteWaiter(cfnClient)
	err = waiter.Wait(ctx, &cloudformation.DescribeStacksInput{
		StackName: &cfg.StackName,
	}, 10*time.Minute)
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
			"user_email":   &dynamodbTypes.AttributeValueMemberS{Value: "admin@mycli.local"},
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
