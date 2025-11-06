# DynamoDB-Based Log Streaming Design

## Overview

This document details the design for using DynamoDB as temporary storage for indexed logs, enabling reliable streaming for CLI and web applications.

## DynamoDB Table Design

### Table: `{project-name}-execution-logs`

**Primary Key**:
- **Partition Key**: `execution_id` (String)
- **Sort Key**: `log_index` (Number) - Sequential index starting from 1

**Attributes**:
- `timestamp` (Number) - Unix timestamp in milliseconds
- `message` (String) - Log message content
- `expires_at` (Number) - TTL timestamp for automatic cleanup (Unix timestamp in seconds)

**Global Secondary Indexes**: None required
- All queries are by `execution_id` with `log_index` range conditions

**TTL Configuration**:
- DynamoDB TTL enabled on `expires_at` attribute
- Set to 7 days after execution completion
- Automatic cleanup of old logs

## Data Model

### Log Item Structure

```go
type LogItem struct {
    ExecutionID string `dynamodbav:"execution_id"`
    LogIndex    int64  `dynamodbav:"log_index"`
    Timestamp   int64  `dynamodbav:"timestamp"`
    Message     string `dynamodbav:"message"`
    ExpiresAt   int64  `dynamodbav:"expires_at"`
}
```

### API Types

```go
// IndexedLogEvent extends LogEvent with sequential index
type IndexedLogEvent struct {
    LogEvent
    Index int64 `json:"index"` // Sequential index (1, 2, 3, ...)
}

// Enhanced LogsResponse
type LogsResponse struct {
    ExecutionID  string            `json:"execution_id"`
    Events       []IndexedLogEvent `json:"events"`
    Status       string            `json:"status"`
    WebSocketURL string            `json:"websocket_url,omitempty"`
    LastIndex    int64             `json:"last_index,omitempty"` // Highest index in response
}
```

## Repository Interface

```go
// LogRepository defines the interface for log storage operations
type LogRepository interface {
    // StoreLogs stores log events in DynamoDB with sequential indexes
    // Returns the highest index stored
    StoreLogs(ctx context.Context, executionID string, events []api.LogEvent) (int64, error)
    
    // GetLogsSinceIndex retrieves logs starting from a specific index (exclusive)
    // Returns logs sorted by log_index ascending
    // lastIndex: the highest index the client has already seen (logs with index > lastIndex will be returned)
    GetLogsSinceIndex(ctx context.Context, executionID string, lastIndex int64) ([]api.IndexedLogEvent, error)
    
    // GetMaxIndex returns the highest index for an execution (or 0 if none exist)
    GetMaxIndex(ctx context.Context, executionID string) (int64, error)
    
    // SetExpiration sets TTL for all logs of an execution
    // Updates expires_at attribute for all log items
    SetExpiration(ctx context.Context, executionID string, expiresAt int64) error
}
```

## Implementation Flow

### 1. Initial Log Fetch (`GET /api/v1/executions/{id}/logs`)

**Flow**:
1. Check if logs exist in DynamoDB for execution_id
2. **If logs exist**: Query all logs, return with indexes
3. **If no logs exist**:
   - Fetch from CloudWatch Logs
   - Assign sequential indexes (1, 2, 3, ...)
   - Store in DynamoDB using batch write
   - Return indexed logs with `last_index`

**Pseudo-code**:
```go
func (s *Service) GetLogsByExecutionID(ctx context.Context, executionID string) (*api.LogsResponse, error) {
    // Check DynamoDB first
    maxIndex, err := s.logRepo.GetMaxIndex(ctx, executionID)
    if err != nil {
        return nil, err
    }
    
    if maxIndex > 0 {
        // Logs exist in DynamoDB, fetch all
        events, err := s.logRepo.GetLogsSinceIndex(ctx, executionID, 0)
        if err != nil {
            return nil, err
        }
        return &api.LogsResponse{
            ExecutionID: executionID,
            Events: events,
            LastIndex: maxIndex,
        }, nil
    }
    
    // No logs in DynamoDB, fetch from CloudWatch
    cloudWatchEvents, err := s.runner.FetchLogsByExecutionID(ctx, executionID)
    if err != nil {
        return nil, err
    }
    
    // Store in DynamoDB with indexes
    maxIndex, err = s.logRepo.StoreLogs(ctx, executionID, cloudWatchEvents)
    if err != nil {
        return nil, err
    }
    
    // Fetch back from DynamoDB to get indexed events
    indexedEvents, err := s.logRepo.GetLogsSinceIndex(ctx, executionID, 0)
    if err != nil {
        return nil, err
    }
    
    return &api.LogsResponse{
        ExecutionID: executionID,
        Events: indexedEvents,
        LastIndex: maxIndex,
    }, nil
}
```

### 2. Log Forwarder (CloudWatch → DynamoDB → WebSocket)

**When new logs arrive from CloudWatch subscription filter**:

1. **Get current max index** from DynamoDB
2. **Store new logs** with indexes starting from `maxIndex + 1`
3. **Query for all active connections** for this execution
4. **For each connection**:
   - Get connection's `last_index` (from query param or stored metadata)
   - Query DynamoDB: `execution_id = X AND log_index > last_index`
   - Forward logs in index order via WebSocket

**Pseudo-code**:
```go
func (lf *LogForwarder) forwardLogsToConnections(
    ctx context.Context,
    executionID string,
    logEvents []events.CloudwatchLogsLogEvent,
) error {
    // Get current max index
    maxIndex, err := lf.logRepo.GetMaxIndex(ctx, executionID)
    if err != nil {
        return err
    }
    
    // Convert CloudWatch events to API LogEvents
    apiEvents := convertToLogEvents(logEvents)
    
    // Store with sequential indexes
    newMaxIndex, err := lf.logRepo.StoreLogs(ctx, executionID, apiEvents)
    if err != nil {
        return err
    }
    
    // Get all connections for this execution
    connectionIDs, err := lf.connRepo.GetConnectionsByExecutionID(ctx, executionID)
    if err != nil {
        return err
    }
    
    // Forward to each connection
    for _, connectionID := range connectionIDs {
        // Get last_index for this connection (from query param or metadata)
        lastIndex := lf.getLastIndexForConnection(connectionID)
        
        // Query DynamoDB for logs after last_index
        logsToForward, err := lf.logRepo.GetLogsSinceIndex(ctx, executionID, lastIndex)
        if err != nil {
            continue // Log error but continue with other connections
        }
        
        // Forward in index order
        for _, log := range logsToForward {
            lf.sendToConnection(ctx, connectionID, log)
        }
    }
    
    return nil
}
```

### 3. WebSocket Connection

**Connection URL**:
```
wss://{api-endpoint}?execution_id={executionID}&last_index={lastIndex}
```

**WebSocket Manager**:
- Extract `last_index` from query parameter
- Store in connection record or pass to Log Forwarder each time
- Log Forwarder uses this to query DynamoDB

### 4. Client Implementation (CLI)

**Flow**:
1. **Initial fetch**: Call `GET /logs`, receive logs with indexes
2. **Track last_index**: Store highest index from response
3. **Connect to WebSocket**: Include `last_index=lastIndex` in URL
4. **Stream logs**: Receive logs with indexes, output immediately
5. **Update last_index**: Track highest index received
6. **Reconnection**: Use last seen index for `last_index` parameter

**Example**:
```go
func (s *LogsService) DisplayLogs(ctx context.Context, executionID string) error {
    // Initial fetch
    resp, err := s.client.GetLogs(ctx, executionID)
    if err != nil {
        return err
    }
    
    // Display initial logs
    s.displayLogEvents(resp.Events)
    
    // Track last index
    lastIndex := resp.LastIndex
    
    // Connect to WebSocket with last_index
    wsURL := fmt.Sprintf("%s?execution_id=%s&last_index=%d", 
        resp.WebSocketURL, executionID, lastIndex)
    
    // Stream logs
    s.streamLogsViaWebSocket(wsURL, lastIndex)
    
    return nil
}
```

## DynamoDB Operations

### StoreLogs

**Batch Write**:
- Use `BatchWriteItem` for efficient bulk writes
- DynamoDB limit: 25 items per batch
- Assign sequential indexes: `maxIndex + 1, maxIndex + 2, ...`

**Atomicity**:
- Use conditional write to ensure no race conditions when multiple Log Forwarders process simultaneously
- Or use `GetMaxIndex` → `StoreLogs` with retry on conflict

### GetLogsSinceIndex

**Query**:
```go
KeyConditionExpression: "execution_id = :execution_id AND log_index > :last_index"
ExpressionAttributeValues: {
    ":execution_id": executionID,
    ":last_index": lastIndex,
}
ScanIndexForward: true  // Sort ascending by log_index
```

**Pagination**:
- Use `Limit` and `LastEvaluatedKey` for large result sets
- Typically won't need pagination (execution logs are finite)

### GetMaxIndex

**Query**:
```go
KeyConditionExpression: "execution_id = :execution_id"
ScanIndexForward: false  // Sort descending
Limit: 1
```

**Return**: `log_index` from first (highest) result, or 0 if no logs exist

### SetExpiration

**Update**:
- Use `UpdateItem` for each log item
- Or use batch update for efficiency
- Sets `expires_at` attribute for TTL

**Alternative**: Use `UpdateItem` with `UpdateExpression` to update all items:
```go
// Note: This requires scanning or querying all items first
// More efficient: Set expiration when storing logs initially
```

## CloudFormation Resources

### DynamoDB Table

```yaml
ExecutionLogsTable:
  Type: AWS::DynamoDB::Table
  Properties:
    TableName: !Sub "${ProjectName}-execution-logs"
    BillingMode: PAY_PER_REQUEST
    AttributeDefinitions:
      - AttributeName: execution_id
        AttributeType: S
      - AttributeName: log_index
        AttributeType: N
    KeySchema:
      - AttributeName: execution_id
        KeyType: HASH
      - AttributeName: log_index
        KeyType: RANGE
    TimeToLiveSpecification:
      AttributeName: expires_at
      Enabled: true
```

### IAM Permissions

**For Orchestrator Lambda** (handles `/logs` endpoint):
```yaml
- Effect: Allow
  Action:
    - dynamodb:PutItem
    - dynamodb:GetItem
    - dynamodb:Query
    - dynamodb:BatchWriteItem
  Resource: !GetAtt ExecutionLogsTable.Arn
```

**For Log Forwarder Lambda**:
```yaml
- Effect: Allow
  Action:
    - dynamodb:PutItem
    - dynamodb:Query
    - dynamodb:BatchWriteItem
  Resource: !GetAtt ExecutionLogsTable.Arn
```

## Cost Considerations

### Storage Costs
- **On-Demand Pricing**: ~$0.25 per GB-month
- **TTL**: Automatic cleanup after 7 days limits storage

### Read/Write Costs
- **Write Units**: ~$1.25 per million
- **Read Units**: ~$0.25 per million
- **Typical Execution**: 100-1000 log lines
- **Cost per Execution**: < $0.001

### Optimization
- Use batch operations for bulk writes
- TTL reduces storage costs automatically
- Query only when needed (not on every log forward)

## Error Handling

### Race Conditions

**Scenario**: Multiple Log Forwarders process logs simultaneously
- **Solution**: Use conditional writes or atomic increment
- **Fallback**: Query and deduplicate on read

### Missing Logs

**Scenario**: Logs not in DynamoDB but execution is running
- **Solution**: Log Forwarder always stores logs
- **Fallback**: `/logs` endpoint can fetch from CloudWatch if DynamoDB is empty

### Connection Failures

**Scenario**: WebSocket connection fails during log forwarding
- **Solution**: Log Forwarder continues for other connections
- **Client**: Reconnects with last seen index

## Testing Strategy

### Unit Tests
- Log Repository: Store, query, get max index
- Index assignment: Sequential, no gaps
- TTL management: Expiration set correctly

### Integration Tests
- End-to-end: CloudWatch → DynamoDB → WebSocket → Client
- Late connection: Start execution, wait, connect → receive all logs
- Reconnection: Disconnect, reconnect → receive missed logs
- Concurrent writes: Multiple log forwarders → no duplicates

### Performance Tests
- Large executions: 10,000+ log lines
- Concurrent connections: Multiple clients per execution
- Batch writes: Efficiency of bulk operations

## Migration Strategy

### Phase 1: Add DynamoDB Table
- Create table via CloudFormation
- Deploy with read-only access initially

### Phase 2: Dual-Write
- Write to both CloudWatch (existing) and DynamoDB (new)
- Read from CloudWatch (existing behavior)
- Validate DynamoDB writes

### Phase 3: Switch Reads
- Read from DynamoDB first
- Fallback to CloudWatch if DynamoDB empty
- Monitor for issues

### Phase 4: Remove CloudWatch Dependency
- Remove CloudWatch read path
- Keep CloudWatch write (subscription filter)
- Full DynamoDB-based reads

## Monitoring

### Metrics
- DynamoDB read/write counts
- Log storage latency
- Log retrieval latency
- TTL cleanup rate

### Alarms
- High write latency (> 100ms)
- High read latency (> 50ms)
- Storage growth rate

### Logging
- Log Forwarder: Store operations
- Repository: Query operations
- Errors: Failed writes, missing logs
