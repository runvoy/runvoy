package dynamodb

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// MockDynamoDBClient is a simple in-memory mock implementation of DynamoDBClient for testing.
// It provides basic support for Put, Get, Query, Update, Delete, and BatchWrite operations.
type MockDynamoDBClient struct {
	mu sync.RWMutex

	// Tables maps table name -> partition key -> sort key -> item
	// For tables without sort key, use empty string as sort key
	Tables map[string]map[string]map[string]map[string]types.AttributeValue

	// Index stores items by index name for Query operations
	// Format: tableName -> indexName -> keyValue -> list of items
	Indexes map[string]map[string]map[string][]map[string]types.AttributeValue

	// Error injection for testing error scenarios
	PutItemError         error
	GetItemError         error
	QueryError           error
	UpdateItemError      error
	DeleteItemError      error
	BatchWriteItemError  error
	ScanError            error

	// Call tracking for test assertions
	PutItemCalls        int
	GetItemCalls        int
	QueryCalls          int
	UpdateItemCalls     int
	DeleteItemCalls     int
	BatchWriteItemCalls int
	ScanCalls           int
}

// NewMockDynamoDBClient creates a new mock DynamoDB client for testing.
func NewMockDynamoDBClient() *MockDynamoDBClient {
	return &MockDynamoDBClient{
		Tables:  make(map[string]map[string]map[string]map[string]types.AttributeValue),
		Indexes: make(map[string]map[string]map[string][]map[string]types.AttributeValue),
	}
}

// PutItem stores an item in the mock table.
func (m *MockDynamoDBClient) PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PutItemCalls++

	if m.PutItemError != nil {
		return nil, m.PutItemError
	}

	tableName := *params.TableName
	if m.Tables[tableName] == nil {
		m.Tables[tableName] = make(map[string]map[string]map[string]types.AttributeValue)
	}

	// Extract the partition key value (simplified - assumes first key is partition key)
	var partitionKey string
	for _, v := range params.Item {
		partitionKey = getStringValue(v)
		break
	}

	if m.Tables[tableName][partitionKey] == nil {
		m.Tables[tableName][partitionKey] = make(map[string]map[string]types.AttributeValue)
	}

	// Store with empty sort key for simplicity
	m.Tables[tableName][partitionKey][""] = params.Item

	return &dynamodb.PutItemOutput{}, nil
}

// GetItem retrieves an item from the mock table.
func (m *MockDynamoDBClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetItemCalls++

	if m.GetItemError != nil {
		return nil, m.GetItemError
	}

	tableName := *params.TableName

	// Extract the partition key value
	var partitionKey string
	for _, v := range params.Key {
		partitionKey = getStringValue(v)
		break
	}

	var item map[string]types.AttributeValue
	if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
		item = m.Tables[tableName][partitionKey][""]
	}

	return &dynamodb.GetItemOutput{
		Item: item,
	}, nil
}

// Query searches for items in the mock table.
func (m *MockDynamoDBClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.QueryCalls++

	if m.QueryError != nil {
		return nil, m.QueryError
	}

	// Simplified query implementation - returns all items in table
	tableName := *params.TableName
	var items []map[string]types.AttributeValue

	if m.Tables[tableName] != nil {
		for _, partitionItems := range m.Tables[tableName] {
			for _, item := range partitionItems {
				items = append(items, item)
			}
		}
	}

	return &dynamodb.QueryOutput{
		Items: items,
		Count: int32(len(items)),
	}, nil
}

// UpdateItem updates an item in the mock table.
func (m *MockDynamoDBClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateItemCalls++

	if m.UpdateItemError != nil {
		return nil, m.UpdateItemError
	}

	tableName := *params.TableName

	// Extract the partition key value
	var partitionKey string
	for _, v := range params.Key {
		partitionKey = getStringValue(v)
		break
	}

	// Check if item exists
	if m.Tables[tableName] == nil || m.Tables[tableName][partitionKey] == nil {
		return nil, fmt.Errorf("item not found")
	}

	// For simplicity, just mark the item as updated by adding a field
	// In a real mock, you'd parse and apply the update expression
	item := m.Tables[tableName][partitionKey][""]
	if item == nil {
		return nil, fmt.Errorf("item not found")
	}

	return &dynamodb.UpdateItemOutput{}, nil
}

// DeleteItem removes an item from the mock table.
func (m *MockDynamoDBClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteItemCalls++

	if m.DeleteItemError != nil {
		return nil, m.DeleteItemError
	}

	tableName := *params.TableName

	// Extract the partition key value
	var partitionKey string
	for _, v := range params.Key {
		partitionKey = getStringValue(v)
		break
	}

	if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
		delete(m.Tables[tableName][partitionKey], "")
	}

	return &dynamodb.DeleteItemOutput{}, nil
}

// BatchWriteItem performs batch write operations.
func (m *MockDynamoDBClient) BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.BatchWriteItemCalls++

	if m.BatchWriteItemError != nil {
		return nil, m.BatchWriteItemError
	}

	// Process delete requests
	for tableName, requests := range params.RequestItems {
		for _, request := range requests {
			if request.DeleteRequest != nil {
				var partitionKey string
				for _, v := range request.DeleteRequest.Key {
					partitionKey = getStringValue(v)
					break
				}

				if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
					delete(m.Tables[tableName][partitionKey], "")
				}
			}
		}
	}

	return &dynamodb.BatchWriteItemOutput{}, nil
}

// Scan scans all items in the mock table.
func (m *MockDynamoDBClient) Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ScanCalls++

	if m.ScanError != nil {
		return nil, m.ScanError
	}

	// Simplified scan implementation - returns all items in table
	tableName := *params.TableName
	var items []map[string]types.AttributeValue

	if m.Tables[tableName] != nil {
		for _, partitionItems := range m.Tables[tableName] {
			for _, item := range partitionItems {
				items = append(items, item)
			}
		}
	}

	return &dynamodb.ScanOutput{
		Items: items,
		Count: int32(len(items)),
	}, nil
}

// ResetCallCounts resets all call counters to zero.
func (m *MockDynamoDBClient) ResetCallCounts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PutItemCalls = 0
	m.GetItemCalls = 0
	m.QueryCalls = 0
	m.UpdateItemCalls = 0
	m.DeleteItemCalls = 0
	m.BatchWriteItemCalls = 0
	m.ScanCalls = 0
}

// ClearTables removes all data from the mock tables.
func (m *MockDynamoDBClient) ClearTables() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Tables = make(map[string]map[string]map[string]map[string]types.AttributeValue)
	m.Indexes = make(map[string]map[string]map[string][]map[string]types.AttributeValue)
}

// getStringValue extracts a string value from an AttributeValue.
// This is a simplified helper for the mock implementation.
func getStringValue(av types.AttributeValue) string {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	default:
		return ""
	}
}
