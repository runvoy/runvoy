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

## Recommended Solution: Hybrid Approach (Solution 1 + Solution 3)

Combine **Timestamp-Based Backfill** with **Polling Fallback** for maximum reliability:

### Implementation Strategy

1. **Primary**: Timestamp-based backfill (Solution 1)
   - WebSocket connection includes `since` timestamp
   - Log Forwarder backfills missing logs before forwarding new ones
   - Handles most cases efficiently

2. **Fallback**: Client-side polling (Solution 3)
   - While WebSocket is connected, poll every 10 seconds
   - Compare to detect any missed logs
   - Backfill if gaps detected
   - Provides redundancy

3. **Reconnection**: Timestamp-based
   - Client tracks last received timestamp
   - On reconnection, includes `since` parameter
   - Server backfills automatically

### Implementation Plan

#### Phase 1: Backend Enhancements

1. **Add timestamp cursor to connection record**:
   ```go
   type WebSocketConnection struct {
       ConnectionID  string
       ExecutionID   string
       Functionality string
       ExpiresAt     int64
       ClientIP      string
       SinceTimestamp *int64  // NEW: timestamp cursor
   }
   ```

2. **Enhance Log Forwarder**:
   - Fetch missing logs between cursor and first new log
   - Forward backfilled logs first
   - Update cursor after forwarding

3. **Add CloudWatch Logs query method**:
   - `FetchLogsSinceTimestamp(ctx, executionID, sinceTimestamp)`

#### Phase 2: Client Enhancements

1. **Track last received timestamp**:
   - Store in `LogsService`
   - Include in WebSocket connection URL

2. **Add polling fallback**:
   - Poll every 10 seconds while WebSocket connected
   - Compare logs to detect gaps
   - Merge missing logs

3. **Reconnection handling**:
   - Include `since` timestamp on reconnection
   - Server automatically backfills

### Success Metrics

- ✅ No logs missed when connecting late
- ✅ No logs missed during temporary disconnections
- ✅ No duplicate logs displayed
- ✅ Consistent log ordering
- ✅ Works regardless of when client starts tailing

## Alternative: Simpler Quick Win (Timestamp Query Parameter)

For a faster implementation, start with a simpler approach:

1. **Client includes last timestamp in WebSocket URL**: `wss://...?execution_id=xxx&since=1234567890`
2. **Log Forwarder checks for gaps** before forwarding each batch
3. **Fetch and forward missing logs** if gap detected
4. **Client-side polling** (every 10s) as safety net

This provides most of the benefits with minimal complexity.
