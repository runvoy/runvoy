package dynamodb

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const executionIDIndexName = "execution_id-index"

// MockDynamoDBClient is a simple in-memory mock implementation of Client for testing.
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
	PutItemError        error
	GetItemError        error
	QueryError          error
	UpdateItemError     error
	DeleteItemError     error
	BatchWriteItemError error
	ScanError           error

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
func (m *MockDynamoDBClient) PutItem(
	_ context.Context,
	params *dynamodb.PutItemInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
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
	if m.Indexes[tableName] == nil {
		m.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
	}

	// Extract the partition key value
	// Try common partition key names first, then fall back to first string field
	partitionKey := ""
	if connID, ok := params.Item["connection_id"]; ok {
		partitionKey = getStringValue(connID)
	} else if secretName, hasSecretName := params.Item["secret_name"]; hasSecretName {
		partitionKey = getStringValue(secretName)
	} else {
		// Fallback: use first string value as partition key
		for _, v := range params.Item {
			partitionKey = getStringValue(v)
			break
		}
	}

	if partitionKey == "" {
		return nil, fmt.Errorf("failed to extract partition key from item")
	}

	if m.Tables[tableName][partitionKey] == nil {
		m.Tables[tableName][partitionKey] = make(map[string]map[string]types.AttributeValue)
	}

	// Get old item before replacing (to remove from indexes)
	var oldItem map[string]types.AttributeValue
	if m.Tables[tableName][partitionKey] != nil && m.Tables[tableName][partitionKey][""] != nil {
		oldItem = m.Tables[tableName][partitionKey][""]
	}

	// Store with empty sort key for simplicity
	m.Tables[tableName][partitionKey][""] = params.Item

	// Remove old item from indexes if it exists
	if oldItem != nil {
		m.removeItemFromIndexes(tableName, oldItem)
	}

	// Index items for GSI queries
	m.addItemToIndexes(tableName, params.Item)

	return &dynamodb.PutItemOutput{}, nil
}

// GetItem retrieves an item from the mock table.
func (m *MockDynamoDBClient) GetItem(
	_ context.Context,
	params *dynamodb.GetItemInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
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
func (m *MockDynamoDBClient) Query(
	_ context.Context,
	params *dynamodb.QueryInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.QueryOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.QueryCalls++

	if m.QueryError != nil {
		return nil, m.QueryError
	}

	tableName := *params.TableName
	var items []map[string]types.AttributeValue

	// If querying against an index, use the index
	if params.IndexName != nil {
		indexName := *params.IndexName
		if m.Indexes[tableName] != nil && m.Indexes[tableName][indexName] != nil {
			// Extract key value from ExpressionAttributeValues
			// For execution_id-index, look for :execution_id in ExpressionAttributeValues
			var keyValue string
			if params.ExpressionAttributeValues != nil {
				if execIDVal, ok := params.ExpressionAttributeValues[":execution_id"]; ok {
					keyValue = getStringValue(execIDVal)
				} else {
					// Try to find any string value as key
					for _, v := range params.ExpressionAttributeValues {
						keyValue = getStringValue(v)
						if keyValue != "" {
							break
						}
					}
				}
			}

			if keyValue != "" {
				if indexItems, exists := m.Indexes[tableName][indexName][keyValue]; exists {
					items = indexItems
				}
			}
		}
	} else {
		// Query against main table - return all items
		// This is a simplified implementation
		items = m.collectTableItems(tableName)
	}

	return &dynamodb.QueryOutput{
		Items: items,
		Count: safeInt32Count(len(items)),
	}, nil
}

// UpdateItem updates an item in the mock table.
func (m *MockDynamoDBClient) UpdateItem(
	_ context.Context,
	params *dynamodb.UpdateItemInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.UpdateItemOutput, error) {
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
func (m *MockDynamoDBClient) DeleteItem(
	_ context.Context,
	params *dynamodb.DeleteItemInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.DeleteItemOutput, error) {
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

	// Get the item before deleting to remove from indexes
	var item map[string]types.AttributeValue
	if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
		item = m.Tables[tableName][partitionKey][""]
		delete(m.Tables[tableName][partitionKey], "")
	}

	// Remove item from indexes
	if item != nil {
		m.removeItemFromIndexes(tableName, item)
	}

	return &dynamodb.DeleteItemOutput{}, nil
}

// BatchWriteItem performs batch write operations.
func (m *MockDynamoDBClient) BatchWriteItem(
	_ context.Context,
	params *dynamodb.BatchWriteItemInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.BatchWriteItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.BatchWriteItemCalls++

	if m.BatchWriteItemError != nil {
		return nil, m.BatchWriteItemError
	}

	// Process delete requests
	for tableName, requests := range params.RequestItems {
		for _, request := range requests {
			if request.DeleteRequest == nil {
				continue
			}

			var partitionKey string
			for _, v := range request.DeleteRequest.Key {
				partitionKey = getStringValue(v)
				break
			}

			// Get the item before deleting to remove from indexes
			var item map[string]types.AttributeValue
			if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
				item = m.Tables[tableName][partitionKey][""]
				delete(m.Tables[tableName][partitionKey], "")
			}

			// Remove item from indexes
			if item != nil {
				m.removeItemFromIndexes(tableName, item)
			}
		}
	}

	return &dynamodb.BatchWriteItemOutput{}, nil
}

// Scan scans all items in the mock table.
func (m *MockDynamoDBClient) Scan(
	_ context.Context,
	params *dynamodb.ScanInput,
	_ ...func(*dynamodb.Options),
) (*dynamodb.ScanOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.ScanCalls++

	if m.ScanError != nil {
		return nil, m.ScanError
	}

	// Simplified scan implementation - returns all items in table
	tableName := *params.TableName
	items := m.collectTableItems(tableName)

	return &dynamodb.ScanOutput{
		Items: items,
		Count: safeInt32Count(len(items)),
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

// safeInt32Count safely converts an int count to int32, clamping to max int32 if necessary.
func safeInt32Count(count int) int32 {
	const maxInt32 = int32(math.MaxInt32)
	if count > int(maxInt32) {
		return maxInt32
	}
	//nolint:gosec // Safe conversion: count is already checked to be <= maxInt32
	return int32(count)
}

// collectTableItems collects all items from a table for Query/Scan operations.
func (m *MockDynamoDBClient) collectTableItems(tableName string) []map[string]types.AttributeValue {
	var items []map[string]types.AttributeValue
	if m.Tables[tableName] != nil {
		for _, partitionItems := range m.Tables[tableName] {
			for _, item := range partitionItems {
				items = append(items, item)
			}
		}
	}
	return items
}

// addItemToIndexes adds an item to all relevant indexes for a table.
func (m *MockDynamoDBClient) addItemToIndexes(tableName string, item map[string]types.AttributeValue) {
	if m.Indexes[tableName] == nil {
		m.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
	}

	// Index items for GSI queries
	// For execution_id-index: index by execution_id
	execID, hasExecID := item["execution_id"]
	if !hasExecID {
		return
	}

	executionID := getStringValue(execID)
	if executionID == "" {
		return
	}

	if m.Indexes[tableName][executionIDIndexName] == nil {
		m.Indexes[tableName][executionIDIndexName] = make(map[string][]map[string]types.AttributeValue)
	}

	// Add item to index
	index := m.Indexes[tableName][executionIDIndexName]
	index[executionID] = append(index[executionID], item)
}

// removeItemFromIndexes removes an item from all indexes for a table.
// It identifies items by connection_id.
func (m *MockDynamoDBClient) removeItemFromIndexes(tableName string, item map[string]types.AttributeValue) {
	if m.Indexes[tableName] == nil {
		return
	}

	// Extract connection_id from item to identify it
	connID := ""
	if connIDVal, ok := item["connection_id"]; ok {
		connID = getStringValue(connIDVal)
	}
	if connID == "" {
		return
	}

	// Remove item from execution_id-index
	if m.Indexes[tableName][executionIDIndexName] == nil {
		return
	}

	execIDVal, hasExecID := item["execution_id"]
	if !hasExecID {
		return
	}

	executionID := getStringValue(execIDVal)
	if executionID == "" {
		return
	}

	indexItems, exists := m.Indexes[tableName][executionIDIndexName][executionID]
	if !exists {
		return
	}

	// Find and remove item with matching connection_id
	for i, indexItem := range indexItems {
		indexConnIDVal, hasConnID := indexItem["connection_id"]
		if !hasConnID {
			continue
		}

		if getStringValue(indexConnIDVal) == connID {
			// Remove this item from the slice
			m.Indexes[tableName][executionIDIndexName][executionID] = append(indexItems[:i], indexItems[i+1:]...)
			break
		}
	}
}
