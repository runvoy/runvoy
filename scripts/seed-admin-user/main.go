// TODO: this is a temporary script to seed the admin user into the database.
// Most probably overkill and needs some cleanup but not urgent for now, it does the job.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"runvoy/internal/auth"
	"runvoy/internal/config"
	"runvoy/internal/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type userItem struct {
	APIKeyHash string    `dynamodbav:"api_key_hash"`
	UserEmail  string    `dynamodbav:"user_email"`
	CreatedAt  time.Time `dynamodbav:"created_at"`
	Revoked    bool      `dynamodbav:"revoked"`
}

func setupAPIKeyAndConfig() (cfg *config.Config, apiKey, apiKeyHash string) {
	var err error
	apiKey, err = auth.GenerateSecretToken()
	if err != nil {
		log.Fatalf("error: failed to generate API key: %v", err)
	}

	cfg, err = config.Load()
	if err != nil {
		cfg = &config.Config{
			APIKey:      apiKey,
			APIEndpoint: "",
		}
	} else {
		cfg.APIKey = apiKey
	}

	apiKeyHash = auth.HashAPIKey(apiKey)
	return cfg, apiKey, apiKeyHash
}

func seedAdminUser(ctx context.Context, dynamoClient *dynamodb.Client, tableName, adminEmail, apiKeyHash string) {
	existingUser, err := checkUserExists(ctx, dynamoClient, tableName, adminEmail)
	if err != nil {
		log.Fatalf("error: failed to check if admin user exists: %v", err)
	}
	if existingUser {
		log.Fatalf("error: admin user %s already exists in DynamoDB", adminEmail)
	}

	item := userItem{
		APIKeyHash: apiKeyHash,
		UserEmail:  adminEmail,
		CreatedAt:  time.Now().UTC(),
		Revoked:    false,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Fatalf("error: failed to marshal DynamoDB item: %v", err)
	}

	log.Printf("seeding admin user %s into table %s...", adminEmail, tableName)

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(api_key_hash)"),
	})

	if err != nil {
		log.Fatalf("error: failed to seed admin user: %v", err)
	}

	log.Println("admin user created in DynamoDB")
}

func main() {
	if len(os.Args) != constants.ExpectedArgsSeedAdminUser {
		log.Fatalf("error: usage: %s <admin-email> <stack-name>", os.Args[0])
	}

	adminEmail := os.Args[1]
	stackName := os.Args[2]
	if adminEmail == "" || stackName == "" {
		log.Fatalf("error: admin email and stack name are required")
	}

	cfg, _, apiKeyHash := setupAPIKeyAndConfig()

	ctx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	cancel()
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	cfnClient := cloudformation.NewFromConfig(awsCfg)
	tableName, err := getTableNameFromStack(ctx2, cfnClient, stackName)
	cancel2()
	if err != nil {
		log.Fatalf("error: failed to resolve API keys table name from CloudFormation outputs: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	seedAdminUser(context.Background(), dynamoClient, tableName, adminEmail, apiKeyHash)

	if err = config.Save(cfg); err != nil {
		log.Fatalf(
			"error: failed to save config file: %v. "+
				"Please save the key manually or store it somewhere safe: %s",
			err, cfg.APIKey,
		)
	}
	log.Println("config file saved")
}

func checkUserExists(ctx context.Context, client *dynamodb.Client, tableName, email string) (bool, error) {
	result, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("failed to query user by email: %w", err)
	}

	return len(result.Items) > 0, nil
}

func getTableNameFromStack(ctx context.Context, client *cloudformation.Client, stackName string) (string, error) {
	output, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe stack: %w", err)
	}

	if len(output.Stacks) == 0 {
		return "", fmt.Errorf("stack %s not found", stackName)
	}

	stack := output.Stacks[0]
	for _, out := range stack.Outputs {
		if out.OutputKey != nil && *out.OutputKey == "APIKeysTableName" {
			if out.OutputValue != nil {
				return *out.OutputValue, nil
			}
		}
	}

	return "", fmt.Errorf("APIKeysTableName output not found in stack %s", stackName)
}
