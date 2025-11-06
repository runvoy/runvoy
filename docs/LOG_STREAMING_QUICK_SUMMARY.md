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

## Recommended Solution: Timestamp-Based Backfill + Polling Fallback

### Quick Win (Fastest to Implement)

1. **Client sends last timestamp to WebSocket**: Include `since` query parameter in WebSocket URL
2. **Log Forwarder backfills gaps**: Before forwarding new logs, check for and fetch missing logs from CloudWatch
3. **Client-side polling safety net**: Poll every 10 seconds to catch any missed logs

### Implementation Priority

#### High Priority (Immediate Impact)
- ✅ Add `since` timestamp parameter to WebSocket connection
- ✅ Enhance Log Forwarder to backfill missing logs
- ✅ Add `FetchLogsSinceTimestamp()` method to CloudWatch Logs client

#### Medium Priority (Enhanced Reliability)
- ✅ Client-side polling fallback
- ✅ Better deduplication (sequence numbers or better timestamp handling)
- ✅ Reconnection handling with timestamp tracking

#### Low Priority (Future Enhancements)
- ✅ Sequence number system for guaranteed ordering
- ✅ Periodic gap detection in Log Forwarder
- ✅ Metrics and monitoring for log streaming reliability

## Code Locations

### Files to Modify

1. **`internal/websocket/websocket_manager.go`**
   - Store `since` timestamp in connection record
   - Read from query parameter on connection

2. **`internal/websocket/log_forwarder.go`**
   - Check for gaps before forwarding
   - Fetch and forward missing logs
   - Update cursor after forwarding

3. **`internal/app/aws/logs.go`**
   - Add `FetchLogsSinceTimestamp(ctx, executionID, sinceTimestamp)` method

4. **`cmd/cli/cmd/logs.go`**
   - Track last received timestamp
   - Include in WebSocket URL
   - Add polling fallback

5. **`internal/api/types.go`**
   - Add optional `SinceTimestamp` field to `WebSocketConnection` (if storing in DB)

## Expected Outcomes

- ✅ **100% log coverage**: No logs missed regardless of connection timing
- ✅ **Consistent ordering**: Logs always displayed in correct chronological order
- ✅ **No duplicates**: Reliable deduplication mechanism
- ✅ **Reconnection resilient**: Automatic backfill on reconnection

## Testing Strategy

1. **Late connection test**: Start execution, wait 30 seconds, then connect → should receive all logs
2. **Disconnection test**: Connect, disconnect for 10 seconds, reconnect → should receive missed logs
3. **Duplicate test**: Verify no duplicate logs displayed
4. **High volume test**: Execution with many logs → verify completeness and ordering

## See Also

- Full analysis: `docs/LOG_STREAMING_RELIABILITY_ANALYSIS.md`
- Architecture: `docs/ARCHITECTURE.md` (WebSocket Architecture section)
