# Preventing Duplicate Logs in DynamoDB

## The Race Condition Problem

**Note**: In practice, it should be rare if not impossible to have two Log Forwarder Lambdas processing logs concurrently for the same execution, as CloudWatch Logs subscription filters typically deliver events sequentially. However, we implement atomic counter protection to ensure correctness in all scenarios.

When multiple Log Forwarder Lambdas process CloudWatch log events simultaneously for the same execution, we need to ensure:

1. **No duplicate indexes**: Each log gets a unique, sequential index
2. **No gaps**: Indexes should be continuous (1, 2, 3, ... not 1, 2, 5, 6)
3. **No lost logs**: All logs must be stored successfully

### Example Scenario (Edge Case)

```
Execution: abc123
Current state: Logs 1-100 stored (max_index = 100)

T0: Log Forwarder A receives CloudWatch events [event1, event2, event3]
T1: Log Forwarder B receives CloudWatch events [event4, event5]
T2: Both forwarders read max_index = 100
T3: Forwarder A tries to write logs with indexes 101, 102, 103
T4: Forwarder B tries to write logs with indexes 101, 102  ❌ CONFLICT!
```

**Problem**: Both forwarders read the same max_index and assign overlapping indexes. The atomic counter solution prevents this even in rare concurrent scenarios.

## Solution 1: Atomic Counter with UpdateItem

### Architecture

Use a separate DynamoDB item as an atomic counter to track the current max_index per execution.

**Counter Item Structure**:
```
Partition Key: execution_id (same as logs)
Sort Key: "counter" (special value)
Attributes:
  - max_index (Number) - Current maximum log index
```

### Implementation

**Step 1: Atomically Increment Counter**

```go
func (r *LogRepository) GetAndIncrementMaxIndex(
    ctx context.Context, 
    executionID string, 
    incrementBy int64,
) (int64, error) {
    // Atomic update: get current value and increment
    result, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: aws.String(r.tableName),
        Key: map[string]types.AttributeValue{
            "execution_id": &types.AttributeValueMemberS{Value: executionID},
            "log_index":    &types.AttributeValueMemberN{Value: "0"}, // Special counter item
        },
        UpdateExpression: aws.String("ADD max_index :inc"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":inc": &types.AttributeValueMemberN{Value: strconv.FormatInt(incrementBy, 10)},
        },
        ReturnValues: types.ReturnValueUpdatedNew,
    })
    
    if err != nil {
        // If item doesn't exist, create it
        var notFound *types.ResourceNotFoundException
        if errors.As(err, &notFound) {
            // Initialize counter if it doesn't exist
            return r.initializeCounter(ctx, executionID, incrementBy)
        }
        return 0, err
    }
    
    // Extract new max_index from result
    newMaxIndex := extractMaxIndex(result.Attributes)
    return newMaxIndex, nil
}
```

**Step 2: Store Logs with Assigned Indexes**

```go
func (r *LogRepository) StoreLogs(
    ctx context.Context, 
    executionID string, 
    events []api.LogEvent,
) (int64, error) {
    if len(events) == 0 {
        return 0, nil
    }
    
    // Atomically get starting index
    startIndex, err := r.GetAndIncrementMaxIndex(ctx, executionID, int64(len(events)))
    if err != nil {
        return 0, err
    }
    
    // Calculate index range
    // If startIndex = 105 and we have 3 events, we get indexes 103, 104, 105
    // But we want 103, 104, 105 (starting from startIndex - len + 1)
    firstIndex := startIndex - int64(len(events)) + 1
    
    // Prepare batch write items
    writeRequests := make([]types.WriteRequest, 0, len(events))
    for i, event := range events {
        logIndex := firstIndex + int64(i)
        item := map[string]types.AttributeValue{
            "execution_id": &types.AttributeValueMemberS{Value: executionID},
            "log_index":    &types.AttributeValueMemberN{Value: strconv.FormatInt(logIndex, 10)},
            "timestamp":    &types.AttributeValueMemberN{Value: strconv.FormatInt(event.Timestamp, 10)},
            "message":      &types.AttributeValueMemberS{Value: event.Message},
        }
        writeRequests = append(writeRequests, types.WriteRequest{
            PutRequest: &types.PutRequest{Item: item},
        })
    }
    
    // Batch write (handle pagination if > 25 items)
    err = r.batchWriteItems(ctx, writeRequests)
    if err != nil {
        return 0, err
    }
    
    return startIndex, nil
}
```

### How It Works

**Example Flow**:

```
Execution: abc123
Current state: max_index = 100 (counter item: max_index = 100)

Forwarder A: GetAndIncrementMaxIndex(executionID, 3)
  → DynamoDB atomically: max_index = 100 + 3 = 103
  → Returns: 103
  → Assigns indexes: 101, 102, 103

Forwarder B: GetAndIncrementMaxIndex(executionID, 2)
  → DynamoDB atomically: max_index = 103 + 2 = 105
  → Returns: 105
  → Assigns indexes: 104, 105

Result: Logs stored with indexes 101, 102, 103, 104, 105 ✅
```

**Benefits**:
- ✅ **Atomic**: DynamoDB UpdateItem with ADD is atomic
- ✅ **No conflicts**: Each forwarder gets a unique range
- ✅ **No gaps**: Sequential indexes guaranteed
- ✅ **No duplicates**: Impossible due to primary key uniqueness

**Drawbacks**:
- ❌ Requires separate counter item per execution
- ❌ Slightly more complex (two operations: counter update + batch write)

## Solution 2: Conditional Writes with Retry

### Architecture

Store logs directly, but use conditional writes to ensure we're writing based on the expected current state.

### Implementation

**Step 1: Get Max Index**

```go
func (r *LogRepository) GetMaxIndex(ctx context.Context, executionID string) (int64, error) {
    result, err := r.client.Query(ctx, &dynamodb.QueryInput{
        TableName:              aws.String(r.tableName),
        KeyConditionExpression: aws.String("execution_id = :execution_id"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":execution_id": &types.AttributeValueMemberS{Value: executionID},
        },
        ScanIndexForward: aws.Bool(false), // Sort descending
        Limit:            aws.Int32(1),
    })
    
    if err != nil || len(result.Items) == 0 {
        return 0, nil // No logs exist
    }
    
    // Extract log_index from first item
    var logItem LogItem
    attributevalue.UnmarshalMap(result.Items[0], &logItem)
    return logItem.LogIndex, nil
}
```

**Step 2: Store Logs with Conditional Write**

```go
func (r *LogRepository) StoreLogs(
    ctx context.Context, 
    executionID string, 
    events []api.LogEvent,
    expectedMaxIndex int64, // Expected max_index before our write
) (int64, error) {
    if len(events) == 0 {
        return 0, nil
    }
    
    // Prepare batch write with conditions
    writeRequests := make([]types.WriteRequest, 0, len(events))
    for i, event := range events {
        logIndex := expectedMaxIndex + int64(i) + 1
        
        // Condition: Only write if no item exists with this key
        // OR: Condition on previous item existing (ensures sequential)
        item := map[string]types.AttributeValue{
            "execution_id": &types.AttributeValueMemberS{Value: executionID},
            "log_index":    &types.AttributeValueMemberN{Value: strconv.FormatInt(logIndex, 10)},
            "timestamp":    &types.AttributeValueMemberN{Value: strconv.FormatInt(event.Timestamp, 10)},
            "message":      &types.AttributeValueMemberS{Value: event.Message},
        }
        
        // Condition: previous index must exist (or this is index 1)
        conditionExpression := fmt.Sprintf(
            "attribute_not_exists(log_index) OR (log_index = :prev_index AND execution_id = :execution_id)",
        )
        
        writeRequests = append(writeRequests, types.WriteRequest{
            PutRequest: &types.PutRequest{
                Item: item,
                ConditionExpression: &conditionExpression, // Prevent overwrites
            },
        })
    }
    
    // Attempt batch write
    err := r.batchWriteItems(ctx, writeRequests)
    if err != nil {
        // Check if it's a conditional check failure
        var condCheckErr *types.ConditionalCheckFailedException
        if errors.As(err, &condCheckErr) {
            // Retry: max_index changed, recalculate
            return r.StoreLogsWithRetry(ctx, executionID, events, maxRetries)
        }
        return 0, err
    }
    
    return expectedMaxIndex + int64(len(events)), nil
}
```

**Step 3: Retry Logic**

```go
func (r *LogRepository) StoreLogsWithRetry(
    ctx context.Context,
    executionID string,
    events []api.LogEvent,
    maxRetries int,
) (int64, error) {
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Re-read max_index
        currentMaxIndex, err := r.GetMaxIndex(ctx, executionID)
        if err != nil {
            return 0, err
        }
        
        // Try to store with new max_index
        newMaxIndex, err := r.StoreLogs(ctx, executionID, events, currentMaxIndex)
        if err == nil {
            return newMaxIndex, nil
        }
        
        // Check if it's a conditional check failure
        var condCheckErr *types.ConditionalCheckFailedException
        if !errors.As(err, &condCheckErr) {
            return 0, err // Not a conflict, return error
        }
        
        // Conflict detected, retry after backoff
        time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
    }
    
    return 0, fmt.Errorf("failed to store logs after %d retries", maxRetries)
}
```

### How It Works

**Example Flow**:

```
Execution: abc123
Current state: max_index = 100

Forwarder A: GetMaxIndex() → 100
Forwarder B: GetMaxIndex() → 100

Forwarder A: StoreLogs(events, expectedMaxIndex=100)
  → Tries to write indexes 101, 102, 103
  → Condition: "attribute_not_exists(log_index)" ✅ Success

Forwarder B: StoreLogs(events, expectedMaxIndex=100)
  → Tries to write indexes 101, 102
  → Condition: "attribute_not_exists(log_index)" ❌ Fails (101 already exists)
  → Retry: GetMaxIndex() → 103
  → StoreLogs(events, expectedMaxIndex=103)
  → Tries to write indexes 104, 105 ✅ Success
```

**Benefits**:
- ✅ No extra counter item needed
- ✅ Simpler initial implementation
- ✅ Retry handles conflicts automatically

**Drawbacks**:
- ❌ Retries add latency (though usually 0-1 retries needed)
- ❌ More complex error handling
- ❌ Potential for more DynamoDB read operations

## Solution 3: Optimistic Locking with Version Numbers

### Architecture

Add a version number to track the "state" of log storage for an execution.

**Counter Item with Version**:
```
execution_id: "abc123"
log_index: 0 (counter item)
max_index: 100
version: 5
```

### Implementation

```go
func (r *LogRepository) StoreLogsOptimistic(
    ctx context.Context,
    executionID string,
    events []api.LogEvent,
) (int64, error) {
    const maxRetries = 5
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Read current state
        counter, err := r.getCounter(ctx, executionID)
        if err != nil {
            return 0, err
        }
        
        if counter == nil {
            // Initialize counter
            counter = &Counter{ExecutionID: executionID, MaxIndex: 0, Version: 0}
        }
        
        // Calculate new indexes
        startIndex := counter.MaxIndex + 1
        endIndex := counter.MaxIndex + int64(len(events))
        
        // Prepare batch write
        writeRequests := r.prepareLogWrites(executionID, events, startIndex)
        
        // Update counter with version check
        updateCounter := &types.UpdateItemInput{
            TableName: aws.String(r.tableName),
            Key: map[string]types.AttributeValue{
                "execution_id": &types.AttributeValueMemberS{Value: executionID},
                "log_index":    &types.AttributeValueMemberN{Value: "0"},
            },
            UpdateExpression: aws.String(
                "SET max_index = :end_index, version = :new_version",
            ),
            ConditionExpression: aws.String("version = :expected_version"),
            ExpressionAttributeValues: map[string]types.AttributeValue{
                ":end_index":       &types.AttributeValueMemberN{Value: strconv.FormatInt(endIndex, 10)},
                ":new_version":     &types.AttributeValueMemberN{Value: strconv.FormatInt(counter.Version+1, 10)},
                ":expected_version": &types.AttributeValueMemberN{Value: strconv.FormatInt(counter.Version, 10)},
            },
        }
        
        // Execute both operations (logs + counter update)
        err = r.executeBatchWithCounterUpdate(ctx, writeRequests, updateCounter)
        if err != nil {
            var condErr *types.ConditionalCheckFailedException
            if errors.As(err, &condErr) {
                // Version conflict, retry
                continue
            }
            return 0, err
        }
        
        return endIndex, nil
    }
    
    return 0, fmt.Errorf("failed after %d retries", maxRetries)
}
```

## Recommended Approach: Atomic Counter (Solution 1)

For the runvoy use case, **Solution 1 (Atomic Counter)** is recommended because:

1. **Simplest**: Single atomic operation per forwarder
2. **Most reliable**: No retries needed in normal operation
3. **Lowest latency**: No retry delays
4. **Predictable**: Deterministic behavior
5. **Future-proof**: Handles edge cases even though concurrent forwarders are rare

Even though concurrent forwarders for the same execution are rare in practice, the atomic counter provides a robust, simple solution that guarantees correctness.

### Simplified Implementation

```go
// Counter item structure
type CounterItem struct {
    ExecutionID string `dynamodbav:"execution_id"`
    LogIndex    int64  `dynamodbav:"log_index"`    // Always 0 for counter
    MaxIndex    int64  `dynamodbav:"max_index"`
}

// Get next index range atomically
func (r *LogRepository) ReserveIndexRange(
    ctx context.Context,
    executionID string,
    count int64,
) (startIndex int64, err error) {
    // Use UpdateItem with ADD to atomically increment
    result, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
        TableName: aws.String(r.tableName),
        Key: map[string]types.AttributeValue{
            "execution_id": &types.AttributeValueMemberS{Value: executionID},
            "log_index":    &types.AttributeValueMemberN{Value: "0"}, // Counter item
        },
        UpdateExpression: aws.String("ADD max_index :count"),
        ExpressionAttributeValues: map[string]types.AttributeValue{
            ":count": &types.AttributeValueMemberN{Value: strconv.FormatInt(count, 10)},
        },
        ReturnValues: types.ReturnValueUpdatedNew,
    })
    
    if err != nil {
        // Handle case where counter doesn't exist yet
        var notFoundErr *types.ResourceNotFoundException
        if errors.As(err, &notFoundErr) {
            // Initialize counter
            return r.initializeCounter(ctx, executionID, count)
        }
        return 0, err
    }
    
    // Extract new max_index
    maxIndexAttr := result.Attributes["max_index"]
    newMaxIndex := maxIndexAttr.(*types.AttributeValueMemberN).Value
    
    maxIndex, parseErr := strconv.ParseInt(newMaxIndex, 10, 64)
    if parseErr != nil {
        return 0, parseErr
    }
    
    // Return starting index (maxIndex - count + 1)
    return maxIndex - count + 1, nil
}

// Initialize counter if it doesn't exist
func (r *LogRepository) initializeCounter(
    ctx context.Context,
    executionID string,
    count int64,
) (int64, error) {
    // Try to create counter with initial value
    _, err := r.client.PutItem(ctx, &dynamodb.PutItemInput{
        TableName: aws.String(r.tableName),
        Item: map[string]types.AttributeValue{
            "execution_id": &types.AttributeValueMemberS{Value: executionID},
            "log_index":    &types.AttributeValueMemberN{Value: "0"},
            "max_index":    &types.AttributeValueMemberN{Value: strconv.FormatInt(count, 10)},
        },
        ConditionExpression: aws.String("attribute_not_exists(execution_id)"),
    })
    
    if err != nil {
        // Another forwarder created it, retry the update
        return r.ReserveIndexRange(ctx, executionID, count)
    }
    
    // Return starting index (count - count + 1 = 1)
    return 1, nil
}
```

### Usage

```go
func (r *LogRepository) StoreLogs(
    ctx context.Context,
    executionID string,
    events []api.LogEvent,
) (int64, error) {
    if len(events) == 0 {
        return 0, nil
    }
    
    // Atomically reserve index range
    startIndex, err := r.ReserveIndexRange(ctx, executionID, int64(len(events)))
    if err != nil {
        return 0, err
    }
    
    // Now we know our index range: [startIndex, startIndex+len(events)-1]
    // Store logs with these indexes
    writeRequests := make([]types.WriteRequest, 0, len(events))
    for i, event := range events {
        logIndex := startIndex + int64(i)
        // ... create PutRequest for this log ...
        writeRequests = append(writeRequests, types.WriteRequest{
            PutRequest: &types.PutRequest{Item: logItem},
        })
    }
    
    // Batch write (no conflicts possible since indexes are unique)
    err = r.batchWriteItems(ctx, writeRequests)
    if err != nil {
        return 0, err
    }
    
    return startIndex + int64(len(events)) - 1, nil
}
```

## Summary

**Atomic Counter (Recommended)**:
- ✅ Single atomic operation
- ✅ No retries needed
- ✅ Guaranteed no duplicates (primary key uniqueness)
- ✅ Guaranteed no gaps (atomic increment)

**Trade-off**: Requires a counter item per execution, but this is minimal overhead and provides the most reliable solution.
