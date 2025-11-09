## Logs Endpoint Refactor Plan

# TODOS:

 Todos
  [x] Review current codebase structure and identify key files for API handlers, DynamoDB models, event processor, and websocket manager
  [x] Update api.WebSocketConnection model to include replay cursor and lock fields
  [x] Update DynamoDB mappers/serialization for new WebSocketConnection fields
  [x] Simplify GET /api/v1/executions/{executionID}/logs handler: always require last_seen_timestamp, single response format
  [x] Implement response logic: completed executions return events, running executions return websocket URL only
  [x] Create SQS FIFO queue for CloudWatch logs with execution ID as message group ID
  [x] Update config to include LogsEventsQueueURL
  [ ] Implement Lambda function to write CloudWatch Logs events to SQS FIFO
  [ ] Update event processor to consume from SQS FIFO instead of direct Lambda invocation
  [ ] Implement replay lock mechanism in websocket connection record (DynamoDB)
  [ ] Implement $connect handler logic: acquire replay lock, fetch CloudWatch backlog, deliver to client
  [ ] Update event processor to check replay lock before sending events, buffer if locked
  [ ] Add TTL/watchdog to clear stale replay locks (prevent deadlocks)
  [ ] Write unit tests for handler logic, query param parsing, and DynamoDB changes
  [ ] Write integration tests for websocket connect flow with backlog replay
  [ ] Load test SQS FIFO + Lambda path to verify message group ordering and throughput
  [ ] Update ARCHITECTURE.md docs to reflect new flow
  [ ] Run 'just check' to ensure lint and tests pass
  [ ] Create PR and get review from team

### Overview
- Replace fragile mixed log fetch + websocket response with deterministic flow.
- Preserve zero-cost idle posture by preferring SQS FIFO over Kinesis.
- Guarantee lossless delivery so clients can replay historical logs and stream new ones without gaps.

### Goals
- Allow clients to request historical logs for completed executions via HTTP.
- Direct running executions to websocket-only streaming, delegating backlog replay to the websocket connect flow.
- Support client-provided `last_seen_timestamp` (CloudWatch native millisecond epoch) for resumable streaming.
- Ensure ordered, gap-free delivery of CloudWatch log events over websockets.

### Non-Goals
- Changing existing execution lifecycle semantics.
- Migrating away from CloudWatch Logs as the authoritative log store.
- Building long-term log retention or analytics beyond current scope.

### API Changes
- Extend `GET /api/v1/executions/{executionID}/logs` with optional `last_seen_timestamp` query parameter.
- For non-running executions: continue returning serialized log events.
- For running executions: respond with websocket URL only; no partial log payload included.
- Update `internal/api/types.go` so `events` becomes optional, and clarify response semantics in documentation.

### Data Model Updates
- Add `last_seen_log_timestamp` and replay coordination fields to `api.WebSocketConnection`.
- Persist the replay metadata in DynamoDB so both pending and active connection records understand backlog progress.

### Websocket & Event Flow
- On `$connect`, acquire a replay lock flag (stored in the connection record) to prevent concurrent event sends.
- Fetch backlog from CloudWatch Logs newer than the stored cursor and deliver to the connecting client.
- Clear the lock, advance the cursor, and resume live streaming from the event processor.
- Event processor checks the replay lock before sending new events; it buffers or requeues messages via SQS FIFO until replay completes.

### AWS Integrations
- Use CloudWatch Logs subscription → Lambda → SQS FIFO to ingest ordered log events per execution (`message group ID = executionID`).
- This keeps the idle cost at \$0 and relies on Lambda fan-out already in place.
- The Lambda processor (current event processor) pulls from SQS FIFO, ensuring ordered delivery per execution.

### Implementation Steps
1. **API & Handler Updates**
   - Parse `last_seen_timestamp` in `internal/server/handlers.go` and pass it into `Service.GetLogsByExecutionID`.
   - Split response logic for running vs. non-running executions and adjust unit tests accordingly.

2. **Persist Log Cursor**
   - Extend `api.WebSocketConnection` and DynamoDB mappers with the new replay/cursor fields.
   - Store metadata when creating pending connections and propagate to active connections on `$connect`.

3. **SQS FIFO Integration & Locking**
   - Add Lambda path that writes CloudWatch logs to SQS FIFO with per-execution message groups.
   - Update event processor and websocket manager to respect the replay lock, fetch backlog, then resume live sends.

4. **Docs & Validation**
   - Update `docs/ARCHITECTURE.md` after implementation to reflect the new flow.
   - Run `just check` before merging to ensure lint/tests are green.

### Risks & Mitigations
- **Replay deadlocks**: Use TTL or watchdog to clear stale locks.
- **SQS throughput limits**: Monitor executions for >300 msg/s bursts; fall back to batching or scale-out if needed.
- **Client backwards compatibility**: Communicate the new contract; update web/CLI clients to always connect via websocket for running executions.

### Testing Strategy
- Unit tests covering handler branching and DynamoDB serialization changes.
- Integration tests simulating websocket connect backlog replay and live event delivery order.
- Load-test SQS FIFO + Lambda path to ensure message group limits are respected.


