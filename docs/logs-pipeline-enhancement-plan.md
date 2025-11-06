# Logs Pipeline Enhancement Plan: DynamoDB Cache & Live Updates

## Executive Summary

Enhance the Runvoy logs pipeline to be more robust by introducing a DynamoDB caching layer that acts as a buffer between CloudWatch Logs (source of truth) and clients. This enables:

1. **Improved Reliability**: Reduce direct dependency on CloudWatch Logs API rate limits
2. **Better Ordering**: Persist sorted logs in DynamoDB for consistent ordering across requests
3. **Live Updates**: Leverage DynamoDB Streams to push updates to connected WebSocket clients in real-time
4. **Reduced Latency**: Cache frequently accessed log segments for faster retrieval
5. **Audit Trail**: No need to maintain complete log history with metadata in DynamoDB, we'll use TTL to automatically delete logs from dynamodb after configurable number of days (e.g. 7). We also need to make sure the process of creating the log cache is idempotent and consistent

---

## Current State Analysis

### Existing Architecture

- **Source of Truth**: CloudWatch Logs (retention: 7 days)
- **Metadata Store**: DynamoDB (`runvoy-executions` table)
- **Live Streaming**: AWS API Gateway WebSocket + Lambda (CloudWatch Logs Subscription Filter triggered)
- **Caching**: None (direct CloudWatch Logs queries)
- **Deduplication**: Client-side (timestamp-based)

### Key Files

- Logs fetching: `internal/app/aws/logs.go`
- WebSocket manager: `internal/websocket/websocket_manager.go`
- Log forwarder: `internal/websocket/log_forwarder.go` (deprecated, would be nice to extend the event processor to handle the log processing too instead of using a dedicated lambda)
- Connection repo: `internal/database/dynamodb/connections.go`
- Infrastructure: `deployments/cloudformation-backend.yaml`

### Pain Point

1. No consistent way to get the same result across multiple requests

---

## Proposed Architecture

### Overview

```
CloudWatch Logs (Source of Truth)
    ↓
    ├→ Event Processor Lambda
    │   └→ DynamoDB Logs Cache Table (new)
    │       ├→ Streams
    │       │   └→ WebSocket manager Lambda -> WebSocket Clients
    │       └→ REST API -> Clients
    │           └→ Clients
    │
    └→ (No fallback, we'll always use the cache, we create it if it doesn't exist when the request is made)
```

### Key Components

#### 1. DynamoDB Logs Cache Table (`runvoy-logs`)

**Purpose**: Persistent cache of log events with efficient ordering and filtering

**Schema**:

```
Partition Key: execution_id (String)
Sort Key: timestamp#log_index (String) - Format: "{timestamp}#{sequence}"

Attributes:
- execution_id (String) - HASH
- timestamp#log_index (String) - RANGE (e.g., "1699200123456#0")
- timestamp (Number) - For filtering by time range
- message (String) - Log message content
- line_number (Number) - Line number in execution (for ordering)
- ingested_at (Number) - When log was cached
- ttl (Number) - Auto-expiration (e.g., 7 days)

Global Secondary Indexes (GSI):
1. execution_id-line_number-index
   - PK: execution_id
   - SK: line_number
   - Use case: Get logs by line number range (paginated retrieval)
```

**Billing**: On-demand (pay-per-request) to handle variable load

#### 2. Logs Ingestion Pipeline

**New Component**: Logs Ingestion Lambda (`cmd/backend/aws/logs/ingester/main.go`)

**Trigger**: CloudWatch Logs Subscription Filter

**Responsibilities**:

1. Receive CloudWatch Logs events
2. Parse and enrich log events with metadata
3. Write to DynamoDB Logs Cache Table (idempotent writes). We can assume that the cloudwatch logs subscription filter is at-least-once delivery, so we don't need to worry about duplicate events.
4. Use built-in DynamoDB auto-incrementing line_number for ordering

**Flow**:

```
CloudWatch Logs Event (via Subscription Filter)
    ↓
Parse log event (execution_id, timestamp, message)
    ↓
Write to DynamoDB Logs Cache Table with:
  - PK: execution_id
  - SK: timestamp#log_index
  - line_number: auto-incrementing counter
  - ttl: 7 days from now
    ↓
Triggers the websocket manager lambda to send the new log events to the clients
    ↓
Success
```

#### 3. DynamoDB Streams

This needs more thinking, we should be able to tap directly into the dynamodb stream and have them trigger our websocket manager lambda to send the new log events to the clients. Evaluate if we can use the dynamodb stream to trigger the websocket manager lambda directly, or if it's better to trigger in the event processor lambda.

#### 4. Enhanced REST API Endpoint

**Endpoint**: `GET /api/v1/executions/{executionID}/logs`

**Enhancement**: Add query parameters for pagination and filtering

**Existing Behavior** (to be replaced):

1. Query DynamoDB for execution metadata
2. Fetch all logs from CloudWatch
3. Return complete log list

**New Behavior**:

1. Query DynamoDB Logs Cache Table for execution metadata
2. **Try DynamoDB cache first**:
   - Query `runvoy-logs` table
   - If logs exist for this execution, use them
   - Apply pagination (limit 1000 events per request)
3. **Fallback to CloudWatch**:
   - If cache is empty or incomplete, fetch from CloudWatch
   - Write fetched logs to cache (batch insert)
   - Return cached logs
4. Return with pagination metadata and websocket url (if the execution is still running)

**Query Parameters** (new):
```

GET /api/v1/executions/{executionID}/logs?
  limit=1000
  after_line=0
  before_line=10000
```

**Response Format**:

```json
{
  "execution_id": "task-abc123",
  "status": "RUNNING",
  "events": [
    {
      "timestamp": 1699200123456,
      "message": "Starting execution...",
      "line_number": 0
    }
  ],
  "pagination": {
    "total_lines": 1234,
    "returned_count": 1000,
    "has_more": true,
    "next_after_line": 1000
  },
  "websocket_url": "wss://...",
  "cached": true
}
```

#### 5. Execution Completion Handler

**Enhancement**: When execution completes, ensure all logs are cached

**Trigger**: Execution status changes to COMPLETED/FAILED

**Responsibilities**:

1. Check how many logs are in cache
2. If less than expected (from CloudWatch), fetch remaining logs
3. Batch insert missing logs to cache
4. Set execution as "logs_cached=true"

**Purpose**: Guarantee all logs are persisted for future retrieval

---

## Implementation Plan

### Phase 1: Foundation (Week 1)

#### 1.1 Create DynamoDB Logs Cache Table

- **File**: `deployments/cloudformation-backend.yaml`
- Add new table resource: `RunvoyExecutionLogsTable`
- Configure schema (execution_id, timestamp#log_index, GSIs)
- Set billing mode to on-demand
- Set TTL to 7 days
- Update config to reference table name: `RUNVOY_EXECUTION_LOGS_TABLE`
- **Estimate**: 2-3 hours

#### 1.2 Extend Event Processor Lambda

- **Files**:
  - `internal/app/aws/logs.go` (update)
  - `internal/websocket/log_ingester.go` (new)
  - `internal/database/dynamodb/logs.go` (new)
  - `internal/database/repository.go` (update)
- Implement log event parsing and enrichment
- Implement line number calculation (query last line, increment)
- Implement idempotent writes (check for duplicates)
- Implement re-creation and validation of the log cache table, possibly on demand. If the process is sound we should be able at any point in time to recreate the log cache table and have it be consistent with the cloudwatch logs.
- Implement batch write to DynamoDB
- Add comprehensive logging and error handling
- **Estimate**: 4-5 hours

#### 1.3 Create Logs Repository

- **File**: `internal/database/dynamodb/logs.go`
- Methods:
  - `CreateLogEvent(ctx, executionID, logEvent) error`
  - `GetLogsByExecutionID(ctx, executionID, limit, afterLine) ([]LogEvent, error)`
  - `GetLogsByTimeRange(ctx, executionID, after, before) ([]LogEvent, error)`
  - `DeleteLogsByExecutionID(ctx, executionID) error`
  - `GetLastLineNumber(ctx, executionID) (int, error)`
- **Estimate**: 3-4 hours

#### 1.4 Update Infrastructure

- **Files**:
  - `deployments/cloudformation-backend.yaml`
  - `internal/config/config.go`
- Add Logs Table to CloudFormation
- Add Logs Ingestion Lambda function
- Update environment variables
- **Estimate**: 2-3 hours

**Phase 1 Total**: ~11-15 hours

### Phase 2: Live Updates (Week 2)

#### 2.1 Extend Event Processor Lambda
- **Files**:
  - `cmd/backend/aws/logs/live_updater/main.go` (new)
  - `internal/app/aws/logs.go` (update)
- Implement websocket manager lambda trigger
- **Estimate**: 4-5 hours

#### 2.2 Configure DynamoDB Streams? (needs more thinking)

#### 2.3 remove Log Forwarder lambda

- **File**: `cmd/backend/aws/websocket/log_forwarder/main.go`

**Phase 2 Total**: ~4-5 hours

### Phase 3: REST API Enhancement (Week 3)

#### 3.1 Update REST API Endpoint

- **Files**:
  - `internal/server/handlers.go`
  - `internal/app/main.go`
- Update `GetLogsByExecutionID` to:
  1. Try cache first (DynamoDB)
- Add pagination support
- Add filter parameters
- Update response schema
- **Estimate**: 3-4 hours

#### 3.4 Update CLI and Webapp

- **Files**:
  - `cmd/cli/cmd/logs.go`
  - `cmd/webapp/src/lib/websocket.js`
- Remove client-side deduplication (now server-side)
- Update to handle pagination metadata
- Optimize for new response format
- Update message sending (e.g. use new lines to batch send messages to the websocket)

**Phase 3 Total**: ~6-8 hours

### Phase 4: Robustness & Testing (Week 4)

#### 4.1 Error Handling & Retries

- **Files**: All Lambda functions

- Implement error handling and retries
- **Estimate**: 3-4 hours

#### 4.2 Comprehensive Testing

- **Files**: `internal/websocket/*_test.go`, `internal/app/*_test.go`
- Unit tests for log ingestion
- Unit tests for cache population
- Integration tests for end-to-end flow
- Load tests for concurrent access
- **Estimate**: 6-8 hours


#### 4.4 Documentation & Rollout Plan

- Update architecture documentation

**Phase 4 Total**: ~6-8 hours

---

## Technical Details

### Idempotency Implementation

**Challenge**: CloudWatch Logs Subscription Filter delivers events at least once (may duplicate)

**Solution**: Auto incrementing line number

### Line Number Calculation (using DynamoDB auto-incrementing line_number)

### Pagination Strategy

**Query Pattern**: Use sort key range queries for efficient pagination

```go
// Get next 1000 logs after line 500
queryInput := &dynamodb.QueryInput{
    TableName: "runvoy-logs",
    IndexName: aws.String("execution_id-line_number-index"),
    KeyConditionExpression: aws.String(
        "execution_id = :exec AND line_number > :after",
    ),
    ExpressionAttributeValues: map[string]types.AttributeValue{
        ":exec": &types.AttributeValueMemberS{Value: executionID},
        ":after": &types.AttributeValueMemberN{Value: "500"},
    },
    Limit: aws.Int32(1000),
}
result, _ := client.Query(ctx, queryInput)

// For next page: after_line = last_line_in_result
```

---

## Data Flow Diagrams

### Initial Setup (First Request)

```
Client requests logs for execution (cache empty)
    ↓
REST API: GET /api/v1/executions/{id}/logs
    ↓
Query DynamoDB cache (runvoy-logs)
    ├→ No results (execution not cached yet) -> create cache
    │
    └→ Return logs to client (from cache)
```

### Subsequent Requests (Cached)

```
Client requests logs for same execution (cache populated)
    ↓
REST API: GET /api/v1/executions/{id}/logs
    ↓
Query DynamoDB cache (runvoy-logs)
    ├→ Immediate hit: return all events from cache
    │   (or paginated results if limit specified)
    │
    └→ Response includes: cached=true, source="dynamodb_cache"
    ↓
Return cached logs to client (fast!)
```

### Live Updates (WebSocket)

```
1. Execution starts
2. Client gets logs via REST (see above)
3. Client extracts WebSocketURL from response
4. Client initiates WebSocket: wss://.../production?execution_id={id},last_line={last_line}

5. ECS Task writes new log line to CloudWatch
    ↓
6. CloudWatch Logs Subscription Filter triggers Event Processor Lambda
    ↓
7. Event Processor Lambda:
    ├→ Parse event
    ├→ Calculate line number
    └→ Write to DynamoDB Logs Cache Table with line number
    └→ Trigger WebSocket manager lambda to send the new log events to the clients
    ↓
10. WebSocket clients receive new log event (websocket manager takes care of the deduplication filling up missing lines from last_line)
    ├→ Update UI
    └→ No deduplication needed (guaranteed by server)
```

### Execution Completion (Cache Finalization)

```
ECS Task completes (success or error)
    ↓
Orchestrator updates execution status to COMPLETED in DynamoDB
    ↓
Execution Completion Handler triggered
    ├→ Query CloudWatch for total log count
    ├→ Query DynamoDB cache for cached log count
    │
    ├─ If cache count < CloudWatch count:
    │   ├→ Fetch missing logs from CloudWatch
    │   ├→ Batch insert to DynamoDB cache
    │   └→ Update execution: logs_cached=true
    │
    └─ Else (all logs already cached):
        └→ Update execution: logs_cached=true
    ↓
WebSocket clients receive disconnect notification
    (existing behavior, no change)
```

---

## Configuration Changes

### CloudFormation Updates

```yaml
# New Table
RunvoyLogsTable:
  Type: AWS::DynamoDB::Table
  Properties:
    TableName: !Sub "${StackNamePrefix}-execution-logs"
    BillingMode: PAY_PER_REQUEST
    AttributeDefinitions:
      - AttributeName: execution_id
        AttributeType: S
      - AttributeName: timestamp_log_index
        AttributeType: S
      - AttributeName: timestamp
        AttributeType: N
      - AttributeName: line_number
        AttributeType: N
    KeySchema:
      - AttributeName: execution_id
        KeyType: HASH
      - AttributeName: timestamp_log_index
        KeyType: RANGE
    GlobalSecondaryIndexes:
      - IndexName: execution_id-line_number-index
        KeySchema:
          - AttributeName: execution_id
            KeyType: HASH
          - AttributeName: line_number
            KeyType: RANGE
        Projection:
          ProjectionType: ALL
      - IndexName: execution_id-ingested_at-index
        KeySchema:
          - AttributeName: execution_id
            KeyType: HASH
          - AttributeName: ingested_at
            KeyType: RANGE
        Projection:
          ProjectionType: ALL
    StreamSpecification:
      StreamViewType: NEW_AND_OLD_IMAGES
    TimeToLiveSpecification:
      Enabled: true
      AttributeName: ttl

# Event Source Mapping
LogsIngesterSubscription:
  Type: AWS::Lambda::Permission
  Properties:
    FunctionName: !Ref EventProcessorFunction
    Action: lambda:InvokeFunction
    Principal: logs.amazonaws.com
    SourceArn: !Sub "arn:aws:logs:${AWS::Region}:${AWS::AccountId}:log-group:/aws/ecs/runvoy/runner:*"

EventProcessorEventSourceMapping:
  Type: AWS::Lambda::EventSourceMapping
  Properties:
    EventSourceArn: !GetAtt RunvoyLogsTable.StreamArn
    FunctionName: !Ref EventProcessorFunction
    Enabled: true
    BatchSize: 50
    MaximumBatchingWindowInSeconds: 2
    ParallelizationFactor: 5
    StartingPosition: LATEST
    FunctionResponseTypes:
      - ReportBatchItemFailures
```

### Environment Variables

```bash
# In deployments/
RUNVOY_EXECUTION_LOGS_TABLE=runvoy-execution-logs
RUNVOY_EXECUTION_LOGS_TABLE_TTL_DAYS=7
```

---

## Testing Strategy

### Unit Tests

1. **Event Processor Lambda**
   - Test event parsing
   - Test line number calculation (concurrent writes)
   - Test error handling (DynamoDB errors, malformed events)

3. **Logs Repository**
   - Test CRUD operations
   - Test pagination

4. **REST API Handler**
   - Test cache hit scenario
   - Test pagination parameters
   - Test error responses
   - Test websocket url generation

### Integration Tests

1. End-to-end log flow:
   - Create execution
   - Write logs to CloudWatch
   - Verify logs appear in cache
   - Retrieve via REST API
   - Verify WebSocket delivery

2. Failover scenarios:
   - Disable cache table
   - Verify fallback to CloudWatch
   - Verify performance degradation is acceptable

3. Concurrent access:
   - Multiple clients requesting same execution logs
   - Verify cache hit rate improves over time
   - Verify no duplicate logs in response

### Load Tests

1. Single execution with many logs (100K+ lines)
2. Multiple executions with concurrent requests

---

## Implementation Checklist

- [ ] Phase 1: Foundation
  - [ ] 1.1 Create DynamoDB table
  - [ ] 1.2 Create Event Processor Lambda
  - [ ] 1.3 Create Logs Repository
  - [ ] 1.4 Update Infrastructure & Config

- [ ] Phase 3: REST API Enhancement
  - [ ] 3.1 Update REST API Endpoint
  - [ ] 3.4 Update CLI and Webapp

---

## Appendix: Code Structure

```
runvoy/
├── internal/
│   ├── app/
│   │   └── aws/logs.go (update)
│   ├── database/
│   │   ├── repository.go (update)
│   │   └── dynamodb/
│   │       └── logs.go (update)
│   ├── websocket/
│   │   └── websocket_manager.go (update)
│   └── server/
│       └── handlers.go (update)
├── cmd/backend/aws/
│   └── logs/ (new directory)
│       ├── ingester/main.go
│       └── live_updater/main.go
└── deployments/
    └── cloudformation-backend.yaml (update)
```

---

## Next Steps

1. **Review this plan** with the team
2. **Identify blockers** or additional requirements
3. **Start Phase 1** foundation work
4. **Iterate based on feedback** and testing results
