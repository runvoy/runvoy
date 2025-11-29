// Package main provides a utility script to truncate (delete all records from) a DynamoDB table.
// It scans all items from the table and deletes them in batches.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/runvoy/runvoy/internal/constants"
	awsconstants "github.com/runvoy/runvoy/internal/providers/aws/constants"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	if len(os.Args) != constants.ExpectedArgsTruncateDynamoDBTable {
		log.Fatalf("error: usage: %s <table-name>", os.Args[0])
	}

	tableName := os.Args[1]
	if tableName == "" {
		log.Fatalf("error: table name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	cancel()
	if err != nil {
		log.Fatalf("error: failed to load AWS configuration: %v", err)
	}

	client := dynamodb.NewFromConfig(awsCfg)

	truncateCtx := context.Background()
	if truncateErr := truncateTable(truncateCtx, client, tableName); truncateErr != nil {
		log.Fatalf("error: failed to truncate table: %v", truncateErr)
	}

	log.Printf("successfully truncated table: %s", tableName)
}

type tableKeySchema struct {
	hashKeyName  string
	rangeKeyName string
}

func truncateTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	schema, err := getTableKeySchema(ctx, client, tableName)
	if err != nil {
		return fmt.Errorf("failed to get table key schema: %w", err)
	}

	projectionExpr, exprAttrNames := buildProjectionExpression(schema)

	return scanAndDelete(ctx, client, tableName, schema, projectionExpr, exprAttrNames)
}

func getTableKeySchema(ctx context.Context, client *dynamodb.Client, tableName string) (*tableKeySchema, error) {
	log.Printf("describing table: %s", tableName)

	describeOutput, describeErr := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if describeErr != nil {
		return nil, fmt.Errorf("failed to describe table: %w", describeErr)
	}

	table := describeOutput.Table
	if table == nil {
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	keySchema := table.KeySchema
	if len(keySchema) == 0 {
		return nil, fmt.Errorf("table %s has no key schema", tableName)
	}

	schema := &tableKeySchema{}
	for _, key := range keySchema {
		switch key.KeyType {
		case types.KeyTypeHash:
			schema.hashKeyName = *key.AttributeName
		case types.KeyTypeRange:
			schema.rangeKeyName = *key.AttributeName
		}
	}

	if schema.hashKeyName == "" {
		return nil, fmt.Errorf("table %s has no hash key", tableName)
	}

	log.Printf("table key schema: hash=%s", schema.hashKeyName)
	if schema.rangeKeyName != "" {
		log.Printf("table key schema: range=%s", schema.rangeKeyName)
	}

	return schema, nil
}

func buildProjectionExpression(
	schema *tableKeySchema,
) (projectionExpression string, expressionAttributeNames map[string]string) {
	projectionExpression = "#hash"
	expressionAttributeNames = map[string]string{
		"#hash": schema.hashKeyName,
	}
	if schema.rangeKeyName != "" {
		projectionExpression = "#hash, #range"
		expressionAttributeNames["#range"] = schema.rangeKeyName
	}
	return projectionExpression, expressionAttributeNames
}

func scanAndDelete(
	ctx context.Context,
	client *dynamodb.Client,
	tableName string,
	schema *tableKeySchema,
	projectionExpression string,
	expressionAttributeNames map[string]string,
) error {
	log.Printf("scanning table: %s", tableName)

	var totalScanned int
	var totalDeleted int
	var lastEvaluatedKey map[string]types.AttributeValue

	batchSize := awsconstants.DynamoDBBatchWriteLimit

	for {
		scanInput := &dynamodb.ScanInput{
			TableName:                aws.String(tableName),
			ProjectionExpression:     aws.String(projectionExpression),
			ExpressionAttributeNames: expressionAttributeNames,
			ExclusiveStartKey:        lastEvaluatedKey,
			Limit:                    aws.Int32(100),
		}

		scanOutput, scanErr := client.Scan(ctx, scanInput)
		if scanErr != nil {
			return fmt.Errorf("failed to scan table: %w", scanErr)
		}

		items := scanOutput.Items
		totalScanned += len(items)

		if len(items) == 0 {
			if len(scanOutput.LastEvaluatedKey) == 0 {
				break
			}
			lastEvaluatedKey = scanOutput.LastEvaluatedKey
			continue
		}

		log.Printf("scanned %d items, processing batch for deletion...", totalScanned)

		deletedCount, deleteErr := processBatch(ctx, client, tableName, items, schema, batchSize)
		if deleteErr != nil {
			return deleteErr
		}

		totalDeleted += deletedCount
		log.Printf("deleted %d items (total: %d)", deletedCount, totalDeleted)

		if len(scanOutput.LastEvaluatedKey) == 0 {
			break
		}

		lastEvaluatedKey = scanOutput.LastEvaluatedKey
	}

	log.Printf("truncation complete: scanned %d items, deleted %d items", totalScanned, totalDeleted)

	return nil
}

func processBatch(
	ctx context.Context,
	client *dynamodb.Client,
	tableName string,
	items []map[string]types.AttributeValue,
	schema *tableKeySchema,
	batchSize int,
) (int, error) {
	var totalDeleted int

	for i := 0; i < len(items); i += batchSize {
		end := min(i+batchSize, len(items))
		batch := items[i:end]

		deleteRequests, buildErr := buildDeleteRequests(batch, schema)
		if buildErr != nil {
			return 0, buildErr
		}

		if deleteErr := batchDelete(ctx, client, tableName, deleteRequests); deleteErr != nil {
			return 0, fmt.Errorf("failed to delete batch: %w", deleteErr)
		}

		totalDeleted += len(deleteRequests)
	}

	return totalDeleted, nil
}

func buildDeleteRequests(
	items []map[string]types.AttributeValue,
	schema *tableKeySchema,
) ([]types.WriteRequest, error) {
	deleteRequests := make([]types.WriteRequest, 0, len(items))

	for _, item := range items {
		itemKey, keyErr := extractItemKey(item, schema)
		if keyErr != nil {
			return nil, keyErr
		}

		deleteRequests = append(deleteRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: itemKey,
			},
		})
	}

	return deleteRequests, nil
}

func extractItemKey(
	item map[string]types.AttributeValue,
	schema *tableKeySchema,
) (map[string]types.AttributeValue, error) {
	key := make(map[string]types.AttributeValue)

	hashKeyAttr, hashKeyExists := item[schema.hashKeyName]
	if !hashKeyExists {
		return nil, fmt.Errorf("item missing hash key attribute: %s", schema.hashKeyName)
	}
	key[schema.hashKeyName] = hashKeyAttr

	if schema.rangeKeyName != "" {
		rangeKeyAttr, rangeKeyExists := item[schema.rangeKeyName]
		if !rangeKeyExists {
			return nil, fmt.Errorf("item missing range key attribute: %s", schema.rangeKeyName)
		}
		key[schema.rangeKeyName] = rangeKeyAttr
	}

	return key, nil
}

func batchDelete(ctx context.Context, client *dynamodb.Client, tableName string, requests []types.WriteRequest) error {
	unprocessedRequests := requests

	for len(unprocessedRequests) > 0 {
		batchWriteOutput, err := client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				tableName: unprocessedRequests,
			},
		})
		if err != nil {
			return fmt.Errorf("batch write failed: %w", err)
		}

		unprocessed, ok := batchWriteOutput.UnprocessedItems[tableName]
		if !ok || len(unprocessed) == 0 {
			break
		}

		unprocessedRequests = unprocessed

		if len(unprocessedRequests) > 0 {
			log.Printf("retrying %d unprocessed items...", len(unprocessedRequests))
		}
	}

	return nil
}
