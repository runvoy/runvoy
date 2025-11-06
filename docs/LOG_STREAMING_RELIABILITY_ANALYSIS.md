# Log Streaming Reliability Analysis

## Current State Analysis

### Architecture Overview

The current log streaming implementation uses a two-phase approach:

1. **Initial Fetch**: Client calls `GET /api/v1/executions/{id}/logs` which:
   - Fetches all existing logs from CloudWatch Logs using `GetLogEvents`
   - Returns logs sorted by timestamp
   - Provides WebSocket URL if execution is RUNNING

2. **Real-time Streaming**: Client connects to WebSocket and receives:
   - New log events forwarded from CloudWatch Logs subscription filter
   - Events are sorted by timestamp before forwarding
   - Disconnect notification when execution completes

### Identified Problems

#### 1. **Race Condition Between Initial Fetch and WebSocket Connection**

**Problem**: When a client connects to the WebSocket, there's a time gap between:
- The initial `GET /api/v1/executions/{id}/logs` call
- The WebSocket connection establishment

During this gap, new log events may be generated and forwarded via the subscription filter, but the client hasn't connected yet and misses them.

**Example Scenario**:
```
T0: Client calls GET /api/v1/executions/{id}/logs → receives logs 1-100
T1: Log events 101-105 are generated and forwarded
T2: Client connects to WebSocket (misses logs 101-105)
T3: Log events 106+ are received via WebSocket
```

#### 2. **No Gap Detection or Replay Mechanism**

**Problem**: If a client:
- Connects late (after execution has been running)
- Experiences a temporary disconnection
- Connects after some logs have already been streamed

There's no mechanism to detect missing logs or replay them. The client only receives logs forwarded after connection.

#### 3. **CloudWatch Logs Subscription Filter Delay**

**Problem**: CloudWatch Logs subscription filters have inherent latency:
- Logs are written to CloudWatch Logs
- Subscription filter triggers (may have delay)
- Lambda processes and forwards logs
- Client receives logs

This delay can cause logs to be missed if the client connects during this processing window.

#### 4. **Duplicate Log Detection is Fragile**

**Current Implementation**: Client uses a `map[int64]api.LogEvent` keyed by timestamp to deduplicate:
```go
if _, exists := logMap[logEvent.Timestamp]; !exists {
    logMap[logEvent.Timestamp] = logEvent
    s.printLogLine(len(logMap), logEvent)
}
```

**Problems**:
- Multiple log events can have the same timestamp (millisecond precision)
- Events might be duplicated between initial fetch and WebSocket stream
- No sequence numbers or unique IDs to truly deduplicate

#### 5. **No Backfill After Reconnection**

**Problem**: If a client disconnects and reconnects:
- No mechanism to fetch logs that were generated during the disconnection
- Client only receives new logs after reconnection
- Historical logs are lost

#### 6. **Missing Logs During Execution Completion**

**Problem**: When an execution completes:
- Final log events might be generated
- WebSocket disconnect notification is sent
- But there's a race condition where final logs might arrive after the disconnect notification, or disconnect might be sent before final logs are forwarded

## Proposed Solutions

### Solution 1: Timestamp-Based Backfill (Recommended)

**Approach**: Use the last received log timestamp as a cursor to fetch missed logs.

#### Implementation

1. **Enhanced Initial Fetch**:
   - Client calls `GET /api/v1/executions/{id}/logs`
   - Response includes `last_timestamp` (timestamp of the most recent log)

2. **WebSocket Connection with Cursor**:
   - Client connects to WebSocket with `last_timestamp` as query parameter: `wss://...?execution_id=xxx&since=1234567890`
   - Server stores this cursor when connection is established

3. **Log Forwarder Enhancement**:
   - When forwarding logs, check if there are any logs between the cursor and the first log in the batch
   - If gap detected, fetch and forward missed logs from CloudWatch Logs before forwarding new logs
   - Update cursor after forwarding

4. **Reconnection Handling**:
   - Client tracks last received timestamp
   - On reconnection, includes `since` parameter
   - Server backfills missing logs

#### Benefits
- ✅ Ensures no logs are missed
- ✅ Handles late connections gracefully
- ✅ Works for reconnections
- ✅ Minimal changes to existing architecture

#### Code Changes Required

1. **`internal/websocket/websocket_manager.go`**:
   - Store `since` timestamp in connection record
   - Pass to Log Forwarder when forwarding logs

2. **`internal/websocket/log_forwarder.go`**:
   - Before forwarding new logs, check for gaps
   - Fetch missing logs from CloudWatch Logs
   - Forward backfilled logs first, then new logs

3. **`cmd/cli/cmd/logs.go`**:
   - Track last received timestamp
   - Include in WebSocket connection URL on reconnection

4. **`internal/app/aws/logs.go`**:
   - Add `FetchLogsSinceTimestamp()` method to fetch logs after a specific timestamp

### Solution 2: Sequence Number-Based System

**Approach**: Add sequence numbers to log events for reliable ordering and deduplication.

#### Implementation

1. **Log Event Enhancement**:
   - Add `sequence_number` field to `api.LogEvent`
   - Sequence numbers are monotonically increasing per execution

2. **Backend Tracking**:
   - Store last sequence number per execution in DynamoDB
   - Increment sequence number for each log event
   - Include sequence number in WebSocket messages

3. **Gap Detection**:
   - Client tracks expected sequence number
   - If gap detected, request backfill for missing sequence numbers

#### Benefits
- ✅ Guaranteed ordering
- ✅ True deduplication (no timestamp collisions)
- ✅ Easy gap detection
- ✅ More robust than timestamp-based

#### Drawbacks
- ❌ Requires state management (sequence numbers)
- ❌ More complex implementation
- ❌ Requires DynamoDB writes for sequence tracking

### Solution 3: Polling Fallback with Hybrid Approach

**Approach**: Combine WebSocket streaming with periodic polling to catch missed logs.

#### Implementation

1. **WebSocket Primary, Polling Backup**:
   - Client connects to WebSocket for real-time streaming
   - Simultaneously polls `GET /api/v1/executions/{id}/logs` every 5-10 seconds
   - Compare received logs to detect gaps
   - Backfill missing logs

2. **Smart Polling**:
   - Only poll if WebSocket is connected (to catch missed logs)
   - Stop polling if WebSocket is disconnected (rely on reconnection backfill)
   - Use `If-None-Match` or timestamp-based conditional requests

#### Benefits
- ✅ Redundant mechanism catches missed logs
- ✅ Works with existing infrastructure
- ✅ No backend changes required (client-side only)

#### Drawbacks
- ❌ Increased API calls (higher cost)
- ❌ Not truly real-time (polling delay)
- ❌ Client-side complexity

### Solution 4: CloudWatch Logs Query-Based Backfill

**Approach**: Use CloudWatch Logs Insights or GetLogEvents with time range to fetch missed logs.

#### Implementation

1. **Connection Metadata**:
   - Store connection timestamp in DynamoDB connection record
   - When forwarding logs, check connection timestamp vs. log timestamps

2. **Backfill Logic**:
   - Before forwarding new logs, query CloudWatch Logs for logs between connection timestamp and first new log timestamp
   - Forward backfilled logs in chronological order
   - Then forward new logs

3. **Periodic Backfill Check**:
   - Periodically check for gaps (e.g., every 30 seconds)
   - Fetch and forward any missed logs

#### Benefits
- ✅ Leverages existing CloudWatch Logs API
- ✅ No sequence number tracking needed
- ✅ Works with existing connection management

#### Drawbacks
- ❌ CloudWatch Logs API calls add latency
- ❌ More complex gap detection logic
- ❌ Cost implications (GetLogEvents API calls)

## Recommended Solution: DynamoDB-Based Indexed Logs (For CLI Streaming)

**Note**: This solution is designed specifically to support CLI streaming and piping scenarios where logs must be output in strict chronological order without buffering.

### Architecture Overview

Use DynamoDB as temporary storage for logs with sequential indexes, enabling reliable streaming and gap-free log delivery.

#### Key Design Principles

1. **Indexed Logs**: Each log event gets a sequential index number (1, 2, 3, ...) per execution
2. **DynamoDB Storage**: Logs stored temporarily in DynamoDB with execution_id + index as composite key
3. **Lazy Population**: First `/logs` request triggers CloudWatch fetch and DynamoDB storage
4. **Index-Based Streaming**: WebSocket uses `last_index` parameter instead of timestamp
5. **Streaming-Friendly**: Client requests logs starting from last seen index, enabling true streaming

### Implementation Strategy

#### Phase 1: DynamoDB Schema Design

**Table**: `{project-name}-execution-logs`

**Schema**:
```
Partition Key: execution_id (String)
Sort Key: log_index (Number)
Attributes:
  - timestamp (Number) - Unix timestamp in milliseconds
  - message (String) - Log message content
  - expires_at (Number) - TTL for automatic cleanup (e.g., 7 days after execution completion)
```

**Global Secondary Index**: None required (queries are by execution_id + index range)

**TTL**: Set `expires_at` to 7 days after execution completion to auto-cleanup old logs

#### Phase 2: Log Repository Interface

Add new repository interface for log storage:

```go
// LogRepository defines the interface for log storage operations
type LogRepository interface {
    // StoreLogs stores log events in DynamoDB with sequential indexes
    // Returns the highest index stored
    StoreLogs(ctx context.Context, executionID string, events []api.LogEvent) (int64, error)
    
    // GetLogsSinceIndex retrieves logs starting from a specific index (exclusive)
    // lastIndex: the highest index the client has already seen
    GetLogsSinceIndex(ctx context.Context, executionID string, lastIndex int64) ([]api.IndexedLogEvent, error)
    
    // GetMaxIndex returns the highest index for an execution (or 0 if none)
    GetMaxIndex(ctx context.Context, executionID string) (int64, error)
    
    // SetExpiration sets TTL for all logs of an execution
    SetExpiration(ctx context.Context, executionID string, expiresAt int64) error
}
```

#### Phase 3: API Type Enhancements

```go
// IndexedLogEvent extends LogEvent with index for reliable ordering
type IndexedLogEvent struct {
    LogEvent
    Index int64 `json:"index"` // Sequential index (1, 2, 3, ...)
}

// LogsResponse enhancement
type LogsResponse struct {
    ExecutionID string            `json:"execution_id"`
    Events      []IndexedLogEvent `json:"events"` // Now includes indexes
    Status      string            `json:"status"`
    WebSocketURL string           `json:"websocket_url,omitempty"`
    LastIndex   int64             `json:"last_index,omitempty"` // NEW: highest index in response
}
```

#### Phase 4: Log Storage Flow

**Strategy Based on Execution Status**:

**For RUNNING Executions**:
1. **Check DynamoDB**: Query for existing logs for execution_id
2. **If logs exist**: Return indexed logs from DynamoDB + WebSocket URL
3. **If no logs exist**: 
   - Fetch from CloudWatch Logs
   - Store in DynamoDB with sequential indexes (1, 2, 3, ...)
   - Return indexed logs + WebSocket URL
4. **Response includes**: All logs with indexes + `last_index` + WebSocket URL

**For COMPLETED Executions** (SUCCEEDED, FAILED, STOPPED):
1. **Fetch directly from CloudWatch**: No DynamoDB storage needed
2. **Return logs**: Simple indexed logs (indexes for display only, not stored)
3. **No WebSocket URL**: Execution is complete, no streaming needed
4. **Response includes**: All logs with indexes + `last_index` (no WebSocket URL)

**Benefits**:
- ✅ **Optimized for completed executions**: No unnecessary DynamoDB writes
- ✅ **Faster for one-off fetches**: Direct CloudWatch read
- ✅ **Cost-effective**: No storage for completed executions
- ✅ **Simpler**: No streaming complexity for finished executions

#### Phase 5: WebSocket Connection Enhancement

**Connection with index cursor**:
```
wss://...?execution_id=xxx&last_index=100
```

**WebSocket Manager**:
- Store `last_index` in connection record (or read from query param each time)
- Pass to Log Forwarder when forwarding logs

#### Phase 6: Log Forwarder Enhancement

**When new logs arrive from CloudWatch**:

1. **Get current max index** from DynamoDB for execution_id
2. **Store new logs** in DynamoDB with indexes starting from max_index + 1
3. **Query for logs after connection's last_index**:
   - Query DynamoDB: `execution_id = X AND log_index > last_index`
   - Sort by log_index
4. **Forward logs in index order** via WebSocket
5. **Update connection cursor** (optional: store in connection record)

**Gap Detection**:
- Before forwarding, check if there are any logs between `last_index` and first new log
- If gap detected, forward all missing logs first

#### Phase 7: Client Implementation

**CLI (`cmd/cli/cmd/logs.go`)**:

1. **Initial fetch**: Get logs with indexes, track `last_index`
2. **WebSocket connection**: Include `last_index={value}` in URL
3. **Streaming**: Receive logs with indexes, output immediately (no buffering needed)
4. **Track last_index**: Update on each received log event
5. **Reconnection**: Use last seen index for `last_index` parameter

**Benefits for CLI**:
- ✅ True streaming: Logs output in order without buffering
- ✅ Works with piping: `runvoy logs <id> | grep error`
- ✅ No duplicates: Index-based deduplication
- ✅ Gap-free: DynamoDB query ensures no missing logs

### Implementation Plan

#### Step 1: Create Log Repository

**File**: `internal/database/repository.go`
- Add `LogRepository` interface

**File**: `internal/database/dynamodb/logs.go` (new)
- Implement `LogRepository` using DynamoDB
- Methods: `StoreLogs`, `GetLogsSinceIndex`, `GetMaxIndex`, `SetExpiration`

#### Step 2: Update API Types

**File**: `internal/api/types.go`
- Add `IndexedLogEvent` type
- Add `Index` field to `LogEvent` (or new `IndexedLogEvent`)
- Add `LastIndex` to `LogsResponse`

#### Step 3: Enhance Logs Endpoint

**File**: `internal/app/main.go` - `GetLogsByExecutionID`
- **Check execution status** first
- **If RUNNING**:
  - Check DynamoDB for existing logs
  - If not found, fetch from CloudWatch and store in DynamoDB
  - Return indexed logs with `last_index` + WebSocket URL
- **If COMPLETED** (SUCCEEDED, FAILED, STOPPED):
  - Fetch directly from CloudWatch (no DynamoDB)
  - Return indexed logs with `last_index` (no WebSocket URL)

#### Step 4: Enhance Log Forwarder

**File**: `internal/websocket/log_forwarder.go`
- **Only process RUNNING executions**: Check execution status before processing
- Store incoming logs in DynamoDB with indexes
- Query DynamoDB for logs after connection's `last_index`
- Forward in index order
- **Note**: Completed executions are handled by `/logs` endpoint, not Log Forwarder

#### Step 5: Update WebSocket Manager

**File**: `internal/websocket/websocket_manager.go`
- Read `last_index` from query parameter
- Store in connection metadata (or pass to Log Forwarder)

#### Step 6: Update CLI Client

**File**: `cmd/cli/cmd/logs.go`
- Track `last_index` from initial response
- Include `last_index` parameter in WebSocket URL
- Update `last_index` on each received log
- Use for reconnection

#### Step 7: CloudFormation Updates

**File**: `deployments/cloudformation-backend.yaml`
- Add DynamoDB table: `{project-name}-execution-logs`
- Add IAM permissions for Log Forwarder and API to read/write logs table
- Configure TTL on `expires_at` attribute

### Success Metrics

- ✅ **Streaming-friendly**: Logs output in strict order without buffering
- ✅ **CLI piping works**: `runvoy logs <id> | grep error` functions correctly
- ✅ **No gaps**: All logs delivered in sequence
- ✅ **No duplicates**: Index-based deduplication
- ✅ **Late connections**: Works regardless of when client connects
- ✅ **Reconnection resilient**: Automatic catch-up using last index
- ✅ **Cost-effective**: TTL auto-cleans old logs after 7 days

### Alternative: Timestamp-Based Backfill (For Web App Only)

**Note**: The timestamp-based approach (Solution 1) may still be suitable for the web app where buffering/reordering is acceptable. The DynamoDB indexed approach is recommended for CLI/streaming scenarios.

## Alternative: Simpler Quick Win (Timestamp Query Parameter)

For a faster implementation, start with a simpler approach:

1. **Client includes last timestamp in WebSocket URL**: `wss://...?execution_id=xxx&since=1234567890`
2. **Log Forwarder checks for gaps** before forwarding each batch
3. **Fetch and forward missing logs** if gap detected
4. **Client-side polling** (every 10s) as safety net

This provides most of the benefits with minimal complexity.
