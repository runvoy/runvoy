// Package main provides a migration script to add the _all field to existing user records.
// This is required after adding the all-user_email GSI for sorted user queries.
//
// Usage:
//   go run scripts/migrate-users-add-all-field/main.go -table <table-name> [-dry-run]
//
// Flags:
//   -table string    DynamoDB table name (required)
//   -dry-run        Only show what would be updated without making changes (default: false)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type userItem struct {
	APIKeyHash string `dynamodbav:"api_key_hash"`
	UserEmail  string `dynamodbav:"user_email"`
}

func main() {
	tableName := flag.String("table", "", "DynamoDB table name (required)")
	dryRun := flag.Bool("dry-run", false, "Only show what would be updated without making changes")
	flag.Parse()

	if *tableName == "" {
		fmt.Println("Error: -table flag is required")
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	fmt.Printf("Migrating users in table: %s\n", *tableName)
	if *dryRun {
		fmt.Println("DRY RUN MODE - No changes will be made")
	}
	fmt.Println()

	// Scan the table to get all users
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(*tableName),
	}

	var updatedCount int
	var errorCount int

	paginator := dynamodb.NewScanPaginator(client, scanInput)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatalf("Failed to scan table: %v", err)
		}

		for _, item := range page.Items {
			// Check if _all field already exists
			if _, hasAll := item["_all"]; hasAll {
				continue // Skip items that already have the _all field
			}

			var user userItem
			if err := attributevalue.UnmarshalMap(item, &user); err != nil {
				log.Printf("Failed to unmarshal item: %v", err)
				errorCount++
				continue
			}

			fmt.Printf("Updating user: %s (api_key_hash: %s...)\n",
				user.UserEmail,
				truncate(user.APIKeyHash, 12))

			if !*dryRun {
				// Update the item to add the _all field
				_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
					TableName: aws.String(*tableName),
					Key: map[string]types.AttributeValue{
						"api_key_hash": &types.AttributeValueMemberS{Value: user.APIKeyHash},
					},
					UpdateExpression: aws.String("SET #all = :user"),
					ExpressionAttributeNames: map[string]string{
						"#all": "_all",
					},
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":user": &types.AttributeValueMemberS{Value: "USER"},
					},
				})
				if err != nil {
					log.Printf("  ERROR: Failed to update user %s: %v\n", user.UserEmail, err)
					errorCount++
					continue
				}
			}

			updatedCount++
		}
	}

	fmt.Println()
	fmt.Printf("Migration complete!\n")
	fmt.Printf("  Users updated: %d\n", updatedCount)
	if errorCount > 0 {
		fmt.Printf("  Errors: %d\n", errorCount)
	}
	if *dryRun {
		fmt.Println("\nRun without -dry-run to apply changes")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
