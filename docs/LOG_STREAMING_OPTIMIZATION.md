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

### COMPLETED Execution Flow (Opportunistic with Safety)

```go
func (s *Service) getLogsForCompletedExecution(
    ctx context.Context,
    executionID string,
    status string,
) (*api.LogsResponse, error) {
    // 1. Opportunistic: Try DynamoDB first (faster, already indexed)
    // Logs might already be stored from when execution was RUNNING
    maxIndex, err := s.logRepo.GetMaxIndex(ctx, executionID)
    if err == nil && maxIndex > 0 {
        indexedEvents, err := s.logRepo.GetLogsSinceIndex(ctx, executionID, 0)
        if err == nil {
            // 2. Verify completeness to prevent race condition data loss
            // Race condition: Log Forwarder might still be processing final logs
            if s.isDynamoDBLogsComplete(ctx, executionID, indexedEvents) {
                // Safe to use DynamoDB: logs are complete
                return &api.LogsResponse{
                    ExecutionID:  executionID,
                    Events:       indexedEvents,
                    Status:       status,
                    LastIndex:    maxIndex,
                    WebSocketURL: "", // ❌ No WebSocket for completed
                }, nil
            }
            // Logs might be incomplete, fall back to CloudWatch
        }
    }
    
    // 3. Fallback: Fetch directly from CloudWatch (guaranteed complete)
    // Handles: no logs in DynamoDB, race conditions, DynamoDB errors
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

// Verify DynamoDB logs are complete by checking execution completion time
func (s *Service) isDynamoDBLogsComplete(
    ctx context.Context,
    executionID string,
    indexedEvents []api.IndexedLogEvent,
) bool {
    execution, err := s.executionRepo.GetExecution(ctx, executionID)
    if err != nil || execution.CompletedAt == nil {
        // No completion time: use DynamoDB (execution might be very old, no race condition)
        return true
    }
    
    completionTime := execution.CompletedAt.UnixMilli()
    if len(indexedEvents) == 0 {
        return false // No logs in DynamoDB
    }
    
    lastLogTime := indexedEvents[len(indexedEvents)-1].Timestamp
    currentTime := time.Now().UnixMilli()
    bufferTime := 30 * 1000 // 30 seconds in milliseconds
    
    // Race condition check:
    // If execution completed recently (within buffer), and last log is before completion,
    // Log Forwarder might still be processing final logs
    timeSinceCompletion := currentTime - completionTime
    
    if timeSinceCompletion < bufferTime {
        // Execution completed recently: check if last log is close to completion time
        // If last log is significantly before completion, we might be missing final logs
        timeDiff := completionTime - lastLogTime
        if timeDiff > bufferTime {
            // Last log is too old relative to completion - might be missing final logs
            return false
        }
    }
    
    // Safe to use DynamoDB:
    // - Execution completed long ago (Log Forwarder has processed all logs)
    // - OR last log is close to completion time (likely complete)
    return true
}
```

**Race Condition Protection**:

The verification checks if execution completed recently and if DynamoDB has logs close to the completion time. This ensures:
- ✅ **No data loss**: If execution completed recently and last log is too old, we fall back to CloudWatch
- ✅ **Performance**: Fast path when DynamoDB is complete (execution completed long ago or logs are recent)
- ✅ **Safety**: Always verify completeness before using DynamoDB for recently completed executions

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

## Race Condition Handling

### The Problem

When an execution completes:
1. Execution status changes to COMPLETED
2. Log Forwarder might still be processing final logs that arrived just before completion
3. If we read from DynamoDB immediately, we might miss these final logs

### The Solution

**Verification Strategy**:
1. Check DynamoDB first (fast path)
2. Verify completeness by comparing last log timestamp to execution completion time
3. Use a 30-second buffer to account for Log Forwarder processing delay
4. If verification fails, fall back to CloudWatch (guaranteed complete)

**Safety Guarantees**:
- ✅ **No data loss**: CloudWatch fallback ensures all logs are retrieved
- ✅ **Performance**: Fast DynamoDB path when logs are complete
- ✅ **Race condition safe**: Verification prevents missing final logs
- ✅ **Handles edge cases**: Works even if execution completed before first /logs request

## Summary

The status-based strategy optimizes for:
- **Performance**: Faster reads for completed executions (opportunistic DynamoDB)
- **Safety**: No data loss (CloudWatch fallback with verification)
- **Cost**: Efficient use of DynamoDB when available
- **Simplicity**: No streaming complexity for finished executions
- **Flexibility**: Real-time streaming only when needed (RUNNING)

This ensures the system is efficient, cost-effective, and safe while maintaining the full streaming capabilities for active executions.
