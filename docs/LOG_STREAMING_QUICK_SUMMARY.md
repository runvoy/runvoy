# Log Streaming Reliability - Quick Summary

## Problem Statement

The current log streaming implementation has reliability issues where logs can be missed when:
- Clients connect to the WebSocket after some logs have already been generated
- Temporary disconnections occur
- There are delays in CloudWatch Logs subscription filter processing

## Root Causes

1. **No gap detection**: No mechanism to detect or fill missing logs between initial fetch and WebSocket connection
2. **Race conditions**: Time gap between `GET /logs` call and WebSocket connection establishment
3. **Fragile deduplication**: Timestamp-based deduplication can fail with duplicate timestamps
4. **No backfill on reconnection**: Disconnected clients lose logs generated during disconnection

## Recommended Solution: DynamoDB-Based Indexed Logs

**Designed for CLI streaming and piping scenarios** where logs must be output in strict chronological order without buffering.

### Architecture

1. **DynamoDB temporary storage**: Logs stored with sequential indexes (1, 2, 3, ...)
2. **Lazy population**: First `/logs` request triggers CloudWatch fetch and DynamoDB storage
3. **Index-based streaming**: WebSocket uses `since_index` parameter instead of timestamp
4. **Streaming-friendly**: Client requests logs starting from last seen index

### Key Components

1. **DynamoDB Table**: `{project-name}-execution-logs`
   - Partition Key: `execution_id`
   - Sort Key: `log_index`
   - TTL: `expires_at` (7 days after execution completion)

2. **Indexed Log Events**: Each log gets sequential index per execution
3. **Log Repository**: New interface for storing/querying indexed logs
4. **Enhanced Log Forwarder**: Stores logs in DynamoDB, queries by index range

### Implementation Priority

#### High Priority (Core Functionality)
- ✅ Create DynamoDB table for execution logs
- ✅ Add `LogRepository` interface and DynamoDB implementation
- ✅ Add `IndexedLogEvent` type with `Index` field
- ✅ Enhance `/logs` endpoint to store logs in DynamoDB on first request
- ✅ Update Log Forwarder to store logs with indexes

#### Medium Priority (Streaming Support)
- ✅ Add `since_index` parameter to WebSocket connection
- ✅ Enhance Log Forwarder to query DynamoDB for logs after `since_index`
- ✅ Update CLI to track and use `last_index`
- ✅ Add `LastIndex` field to `LogsResponse`

#### Low Priority (Enhancements)
- ✅ TTL management for log cleanup
- ✅ Connection metadata storage for `since_index`
- ✅ Metrics and monitoring for log streaming reliability

## Code Locations

### Files to Create

1. **`internal/database/repository.go`**
   - Add `LogRepository` interface

2. **`internal/database/dynamodb/logs.go`** (new file)
   - Implement `LogRepository` using DynamoDB
   - Methods: `StoreLogs`, `GetLogsSinceIndex`, `GetMaxIndex`, `SetExpiration`

### Files to Modify

1. **`internal/api/types.go`**
   - Add `IndexedLogEvent` type (extends `LogEvent` with `Index` field)
   - Add `LastIndex` field to `LogsResponse`

2. **`internal/app/main.go`**
   - Enhance `GetLogsByExecutionID` to check/store logs in DynamoDB
   - Return indexed logs with `last_index`

3. **`internal/websocket/log_forwarder.go`**
   - Store incoming logs in DynamoDB with sequential indexes
   - Query DynamoDB for logs after connection's `since_index`
   - Forward logs in index order

4. **`internal/websocket/websocket_manager.go`**
   - Read `since_index` from query parameter
   - Pass to Log Forwarder

5. **`cmd/cli/cmd/logs.go`**
   - Track `last_index` from initial response
   - Include `since_index` in WebSocket URL
   - Update `last_index` on each received log

6. **`deployments/cloudformation-backend.yaml`**
   - Add DynamoDB table: `{project-name}-execution-logs`
   - Add IAM permissions for logs table access
   - Configure TTL on `expires_at` attribute

## Expected Outcomes

- ✅ **Streaming-friendly**: Logs output in strict order without buffering
- ✅ **CLI piping works**: `runvoy logs <id> | grep error` functions correctly
- ✅ **100% log coverage**: No logs missed regardless of connection timing
- ✅ **No gaps**: All logs delivered in sequence via index-based queries
- ✅ **No duplicates**: Index-based deduplication (guaranteed unique per index)
- ✅ **Reconnection resilient**: Automatic catch-up using last index
- ✅ **Cost-effective**: TTL auto-cleans old logs after 7 days

## Testing Strategy

1. **Late connection test**: Start execution, wait 30 seconds, then connect → should receive all logs
2. **Disconnection test**: Connect, disconnect for 10 seconds, reconnect → should receive missed logs
3. **Duplicate test**: Verify no duplicate logs displayed
4. **High volume test**: Execution with many logs → verify completeness and ordering

## See Also

- Full analysis: `docs/LOG_STREAMING_RELIABILITY_ANALYSIS.md`
- Architecture: `docs/ARCHITECTURE.md` (WebSocket Architecture section)
