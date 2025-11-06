# Log Streaming Optimization Strategy

## Execution Status-Based Strategy

The log fetching strategy is optimized based on execution status to improve performance and reduce costs.

## Strategy Overview

### RUNNING Executions

**Strategy**: DynamoDB storage + WebSocket streaming

**Flow**:
1. First `/logs` request: Fetch from CloudWatch → Store in DynamoDB with indexes
2. Subsequent requests: Read from DynamoDB (faster than CloudWatch)
3. WebSocket streaming: Real-time log delivery via DynamoDB queries
4. Log Forwarder: Stores new logs in DynamoDB as they arrive

**Benefits**:
- ✅ Real-time streaming support
- ✅ Fast subsequent reads (DynamoDB faster than CloudWatch)
- ✅ Consistent log ordering via indexes
- ✅ Gap-free streaming for late connections

**When to use**:
- Execution status is `RUNNING`
- Client wants to stream logs in real-time
- Multiple clients may tail the same execution

### COMPLETED Executions (SUCCEEDED, FAILED, STOPPED)

**Strategy**: Direct CloudWatch read (no DynamoDB, no streaming)

**Flow**:
1. `/logs` request: Fetch directly from CloudWatch Logs
2. Assign simple sequential indexes (for display only, not stored)
3. Return logs immediately
4. No WebSocket URL provided

**Benefits**:
- ✅ **No storage overhead**: No DynamoDB writes for completed executions
- ✅ **Faster one-off fetches**: Direct CloudWatch read (no DynamoDB query)
- ✅ **Cost-effective**: No DynamoDB storage costs
- ✅ **Simpler**: No streaming complexity for finished executions

**When to use**:
- Execution status is `SUCCEEDED`, `FAILED`, or `STOPPED`
- Client wants historical logs (execution is complete)
- No need for real-time streaming

## Implementation Details

### GetLogsByExecutionID Logic

```go
func (s *Service) GetLogsByExecutionID(ctx context.Context, executionID string) (*api.LogsResponse, error) {
    // 1. Fetch execution status
    execution, err := s.executionRepo.GetExecution(ctx, executionID)
    if err != nil {
        return nil, err
    }
    
    // 2. Route based on status
    if execution.Status == string(constants.ExecutionRunning) {
        return s.getLogsForRunningExecution(ctx, executionID)
    }
    
    // 3. Completed execution: direct CloudWatch read
    return s.getLogsForCompletedExecution(ctx, executionID, execution.Status)
}
```

### RUNNING Execution Flow

```go
func (s *Service) getLogsForRunningExecution(ctx context.Context, executionID string) (*api.LogsResponse, error) {
    // Check if logs already in DynamoDB
    maxIndex, err := s.logRepo.GetMaxIndex(ctx, executionID)
    if err != nil {
        return nil, err
    }
    
    var indexedEvents []api.IndexedLogEvent
    
    if maxIndex > 0 {
        // Logs exist: fetch from DynamoDB
        indexedEvents, err = s.logRepo.GetLogsSinceIndex(ctx, executionID, 0)
    } else {
        // First request: fetch from CloudWatch and store
        cloudWatchEvents, err := s.runner.FetchLogsByExecutionID(ctx, executionID)
        maxIndex, err = s.logRepo.StoreLogs(ctx, executionID, cloudWatchEvents)
        indexedEvents, err = s.logRepo.GetLogsSinceIndex(ctx, executionID, 0)
    }
    
    // Provide WebSocket URL for streaming
    websocketURL := fmt.Sprintf("wss://%s?execution_id=%s&last_index=%d", 
        s.websocketAPIBaseURL, executionID, maxIndex)
    
    return &api.LogsResponse{
        ExecutionID:  executionID,
        Events:       indexedEvents,
        Status:       string(constants.ExecutionRunning),
        LastIndex:    maxIndex,
        WebSocketURL: websocketURL, // ✅ Provided for RUNNING
    }, nil
}
```

### COMPLETED Execution Flow

```go
func (s *Service) getLogsForCompletedExecution(
    ctx context.Context,
    executionID string,
    status string,
) (*api.LogsResponse, error) {
    // Fetch directly from CloudWatch (no DynamoDB)
    cloudWatchEvents, err := s.runner.FetchLogsByExecutionID(ctx, executionID)
    if err != nil {
        return nil, err
    }
    
    // Simple sequential indexes (for display only)
    indexedEvents := make([]api.IndexedLogEvent, len(cloudWatchEvents))
    for i, event := range cloudWatchEvents {
        indexedEvents[i] = api.IndexedLogEvent{
            LogEvent: event,
            Index:    int64(i + 1),
        }
    }
    
    lastIndex := int64(len(indexedEvents))
    if lastIndex > 0 {
        lastIndex = indexedEvents[len(indexedEvents)-1].Index
    }
    
    return &api.LogsResponse{
        ExecutionID:  executionID,
        Events:       indexedEvents,
        Status:       status,
        LastIndex:    lastIndex,
        WebSocketURL: "", // ❌ No WebSocket for completed
    }, nil
}
```

### Log Forwarder Enhancement

The Log Forwarder should check execution status before processing:

```go
func (lf *LogForwarder) forwardLogsToConnections(
    ctx context.Context,
    executionID string,
    logEvents []events.CloudwatchLogsLogEvent,
) error {
    // Check execution status (skip if completed)
    execution, err := lf.executionRepo.GetExecution(ctx, executionID)
    if err != nil {
        return err
    }
    
    // Only process RUNNING executions
    if execution.Status != string(constants.ExecutionRunning) {
        lf.logger.Debug("skipping log forwarding for completed execution",
            "execution_id", executionID,
            "status", execution.Status,
        )
        return nil
    }
    
    // Continue with normal forwarding logic...
    // Store in DynamoDB, forward to WebSocket connections, etc.
}
```

## Cost Comparison

### RUNNING Execution (with DynamoDB)

**Storage Costs**:
- DynamoDB writes: ~$1.25 per million writes
- DynamoDB storage: ~$0.25 per GB-month
- Typical execution: 100-1000 logs = ~$0.001

**Benefits**:
- Fast subsequent reads
- Real-time streaming
- Gap-free delivery

### COMPLETED Execution (direct CloudWatch)

**Storage Costs**:
- DynamoDB: $0 (no storage)
- CloudWatch read: $0.50 per million requests
- Typical execution: 1 read = ~$0.0000005

**Benefits**:
- No storage overhead
- Simpler code path
- Faster for one-off fetches

## Decision Matrix

| Execution Status | DynamoDB Storage | WebSocket Streaming | Use Case |
|-----------------|------------------|---------------------|----------|
| RUNNING | ✅ Yes | ✅ Yes | Active execution, real-time tailing |
| SUCCEEDED | ❌ No | ❌ No | Historical logs, one-off fetch |
| FAILED | ❌ No | ❌ No | Debugging, historical logs |
| STOPPED | ❌ No | ❌ No | Manual termination, historical logs |

## Migration Considerations

### Existing Executions

When an execution transitions from RUNNING to COMPLETED:

1. **Logs in DynamoDB**: Can be left (TTL will clean up after 7 days)
2. **Future requests**: Will use direct CloudWatch read
3. **No cleanup needed**: TTL handles old logs automatically

### Client Behavior

**CLI**:
- RUNNING: Fetches logs, connects to WebSocket, streams new logs
- COMPLETED: Fetches logs once, displays all, exits

**Web App**:
- RUNNING: Fetches logs, connects to WebSocket, streams new logs
- COMPLETED: Fetches logs once, displays all, no WebSocket connection

## Testing Strategy

### Test Cases

1. **RUNNING execution**:
   - First `/logs` call → DynamoDB storage + WebSocket URL
   - Second `/logs` call → DynamoDB read (faster)
   - WebSocket connection → Real-time streaming works

2. **COMPLETED execution**:
   - `/logs` call → Direct CloudWatch read (no DynamoDB)
   - No WebSocket URL in response
   - Logs returned immediately

3. **Status transition**:
   - RUNNING → COMPLETED transition
   - Verify DynamoDB logs still accessible (for backward compatibility)
   - Verify new requests use CloudWatch direct read

## Summary

The status-based strategy optimizes for:
- **Performance**: Faster reads for completed executions
- **Cost**: No storage overhead for completed executions
- **Simplicity**: No streaming complexity for finished executions
- **Flexibility**: Real-time streaming only when needed (RUNNING)

This ensures the system is efficient and cost-effective while maintaining the full streaming capabilities for active executions.
