package dynamodb

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const executionIDIndexName = "execution_id-index"

// MockDynamoDBClient is a simple in-memory mock implementation of Client for testing.
// It provides basic support for Put, Get, Query, Update, Delete, and BatchWrite operations.
type MockDynamoDBClient struct {
	mu sync.RWMutex

	// partitionKeys maps table name to its partition key attribute name
	partitionKeys map[string]string

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
		// Partition keys for known tables. For unknown tables, will infer from item.
		partitionKeys: map[string]string{
			"api_key_hash":  "api_key_hash",
			"secret_token":  "secret_token",
			"connection_id": "connection_id",
			"execution_id":  "execution_id",
			"token":         "token",
			"secret_name":   "secret_name",
			"image_id":      "image_id",
		},
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

	partitionKey := m.getPartitionKeyFromAttributes(params.Item)
	if partitionKey == "" {
		return nil, errors.New("failed to extract partition key from item")
	}

	sortKey := getSortKeyFromAttributes(params.Item)

	if m.Tables[tableName][partitionKey] == nil {
		m.Tables[tableName][partitionKey] = make(map[string]map[string]types.AttributeValue)
	}

	// Get old item before replacing (to remove from indexes)
	var oldItem map[string]types.AttributeValue
	if m.Tables[tableName][partitionKey] != nil && m.Tables[tableName][partitionKey][sortKey] != nil {
		oldItem = m.Tables[tableName][partitionKey][sortKey]
	}

	m.Tables[tableName][partitionKey][sortKey] = params.Item

	if oldItem != nil {
		m.removeItemFromIndexes(tableName, oldItem)
	}

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
		items = m.queryIndex(tableName, *params.IndexName, params.ExpressionAttributeValues)
	} else {
		items = m.queryMainTable(tableName, params.ExpressionAttributeValues)
	}

	if params.ScanIndexForward != nil && len(items) > 1 {
		ascending := aws.ToBool(params.ScanIndexForward)
		sort.SliceStable(items, func(i, j int) bool {
			left := getSortKeyFromAttributes(items[i])
			right := getSortKeyFromAttributes(items[j])
			if ascending {
				return left < right
			}
			return left > right
		})
	}

	return &dynamodb.QueryOutput{
		Items: items,
		Count: safeInt32Count(len(items)),
	}, nil
}

// queryIndex queries items from an index.
func (m *MockDynamoDBClient) queryIndex(
	tableName, indexName string,
	expressionAttributeValues map[string]types.AttributeValue,
) []map[string]types.AttributeValue {
	if m.Indexes[tableName] == nil || m.Indexes[tableName][indexName] == nil {
		return nil
	}

	keyValue := m.extractKeyValue(expressionAttributeValues)
	if keyValue == "" {
		return nil
	}

	if indexItems, exists := m.Indexes[tableName][indexName][keyValue]; exists {
		return indexItems
	}
	return nil
}

// queryMainTable queries items from the main table.
func (m *MockDynamoDBClient) queryMainTable(
	tableName string,
	expressionAttributeValues map[string]types.AttributeValue,
) []map[string]types.AttributeValue {
	if expressionAttributeValues != nil {
		if execIDVal, ok := expressionAttributeValues[":execution_id"]; ok {
			executionID := getStringValue(execIDVal)
			if partition, exists := m.Tables[tableName][executionID]; exists {
				items := make([]map[string]types.AttributeValue, 0, len(partition))
				for _, item := range partition {
					items = append(items, item)
				}
				return items
			}
		}
	}

	// Query against main table - return all items
	// This is a simplified implementation
	return m.collectTableItems(tableName)
}

// extractKeyValue extracts the key value from ExpressionAttributeValues.
func (m *MockDynamoDBClient) extractKeyValue(
	expressionAttributeValues map[string]types.AttributeValue,
) string {
	if expressionAttributeValues == nil {
		return ""
	}

	// For execution_id-index, look for :execution_id in ExpressionAttributeValues
	if execIDVal, ok := expressionAttributeValues[":execution_id"]; ok {
		return getStringValue(execIDVal)
	}

	// Try to find any string value as key
	for _, v := range expressionAttributeValues {
		keyValue := getStringValue(v)
		if keyValue != "" {
			return keyValue
		}
	}
	return ""
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

	partitionKey := m.getPartitionKeyFromAttributes(params.Key)
	if partitionKey == "" {
		return nil, errors.New("item not found")
	}

	sortKey := getSortKeyFromAttributes(params.Key)

	// Check if item exists
	if m.Tables[tableName] == nil || m.Tables[tableName][partitionKey] == nil {
		return nil, errors.New("item not found")
	}

	// For simplicity, just mark the item as updated by adding a field
	// In a real mock, you'd parse and apply the update expression
	item := m.Tables[tableName][partitionKey][sortKey]
	if item == nil {
		return nil, errors.New("item not found")
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

	partitionKey := m.getPartitionKeyFromAttributes(params.Key)
	sortKey := getSortKeyFromAttributes(params.Key)

	// Get the item before deleting to remove from indexes
	var item map[string]types.AttributeValue
	if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
		item = m.Tables[tableName][partitionKey][sortKey]
		delete(m.Tables[tableName][partitionKey], sortKey)
	}

	// Remove item from indexes
	if item != nil {
		m.removeItemFromIndexes(tableName, item)
	}

	return &dynamodb.DeleteItemOutput{}, nil
}

// BatchWriteItem performs batch write operations.
//
//nolint:funlen // Mimics DynamoDB batch semantics for tests; splitting would add noise.
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

	for tableName, requests := range params.RequestItems {
		if m.Tables[tableName] == nil {
			m.Tables[tableName] = make(map[string]map[string]map[string]types.AttributeValue)
		}
		if m.Indexes[tableName] == nil {
			m.Indexes[tableName] = make(map[string]map[string][]map[string]types.AttributeValue)
		}

		for _, request := range requests {
			switch {
			case request.PutRequest != nil:
				item := request.PutRequest.Item
				partitionKey := m.getPartitionKeyFromAttributes(item)
				if partitionKey == "" {
					return nil, errors.New("failed to extract partition key from item")
				}
				sortKey := getSortKeyFromAttributes(item)

				if m.Tables[tableName][partitionKey] == nil {
					m.Tables[tableName][partitionKey] = make(map[string]map[string]types.AttributeValue)
				}

				var oldItem map[string]types.AttributeValue
				if existing := m.Tables[tableName][partitionKey][sortKey]; existing != nil {
					oldItem = existing
				}

				m.Tables[tableName][partitionKey][sortKey] = item

				if oldItem != nil {
					m.removeItemFromIndexes(tableName, oldItem)
				}

				m.addItemToIndexes(tableName, item)

			case request.DeleteRequest != nil:
				partitionKey := m.getPartitionKeyFromAttributes(request.DeleteRequest.Key)
				sortKey := getSortKeyFromAttributes(request.DeleteRequest.Key)

				// Get the item before deleting to remove from indexes
				var item map[string]types.AttributeValue
				if m.Tables[tableName] != nil && m.Tables[tableName][partitionKey] != nil {
					item = m.Tables[tableName][partitionKey][sortKey]
					delete(m.Tables[tableName][partitionKey], sortKey)
				}

				// Remove item from indexes
				if item != nil {
					m.removeItemFromIndexes(tableName, item)
				}
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

// getPartitionKeyFromAttributes extracts the first known partition key value from the provided attributes.
// Falls back to any string attribute if no known keys are present.
func (m *MockDynamoDBClient) getPartitionKeyFromAttributes(attrs map[string]types.AttributeValue) string {
	for knownKey := range m.partitionKeys {
		if keyVal, ok := attrs[knownKey]; ok {
			if partitionKey := getStringValue(keyVal); partitionKey != "" {
				return partitionKey
			}
		}
	}

	for _, v := range attrs {
		if partitionKey := getStringValue(v); partitionKey != "" {
			return partitionKey
		}
	}

	return ""
}

func getSortKeyFromAttributes(attrs map[string]types.AttributeValue) string {
	if sortVal, ok := attrs["event_key"]; ok {
		return getStringValue(sortVal)
	}

	return ""
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

	// For created_by_request_id-index: index by created_by_request_id (sparse index)
	if createdByRequestIDVal, hasCreatedByRequestID := item["created_by_request_id"]; hasCreatedByRequestID {
		createdByRequestID := getStringValue(createdByRequestIDVal)
		if createdByRequestID != "" {
			indexName := "created_by_request_id-index"
			if m.Indexes[tableName][indexName] == nil {
				m.Indexes[tableName][indexName] = make(map[string][]map[string]types.AttributeValue)
			}
			createdIndex := m.Indexes[tableName][indexName]
			createdIndex[createdByRequestID] = append(createdIndex[createdByRequestID], item)
		}
	}

	// For modified_by_request_id-index: index by modified_by_request_id (sparse index)
	if modifiedByRequestIDVal, hasModifiedByRequestID := item["modified_by_request_id"]; hasModifiedByRequestID {
		modifiedByRequestID := getStringValue(modifiedByRequestIDVal)
		if modifiedByRequestID != "" {
			indexName := "modified_by_request_id-index"
			if m.Indexes[tableName][indexName] == nil {
				m.Indexes[tableName][indexName] = make(map[string][]map[string]types.AttributeValue)
			}
			modifiedIndex := m.Indexes[tableName][indexName]
			modifiedIndex[modifiedByRequestID] = append(modifiedIndex[modifiedByRequestID], item)
		}
	}
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
