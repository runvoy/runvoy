# Proposal: Event Processor for ECS Task Completions

## Current State

Currently, the runvoy platform:
1. **Orchestrator Lambda** starts ECS Fargate tasks and creates execution records in DynamoDB with initial data:
   - `execution_id`, `started_at`, `user_email`, `command`, `lock_name`
   - `status = "RUNNING"`
   - `request_id`, `compute_platform`
2. **No completion tracking**: Once the ECS task completes, there's no mechanism to detect completion and update the execution record

## Problem Statement

We need to:
1. Detect when ECS tasks complete (successfully or with errors)
2. Enrich DynamoDB execution records with:
   - **Final status** (SUCCEEDED, FAILED, STOPPED)
   - **Exit code** (from container exit)
   - **Completion time** (`completed_at`)
   - **Duration** (calculated from `started_at` and `completed_at`)
   - **Computed cost** (based on Fargate pricing)
   - Any other relevant metadata

## Proposed Solution

### Architecture: EventBridge + Dedicated Event Processor Lambda

**Recommended Approach:**
- Use **Amazon EventBridge** to capture ECS Task State Change events natively (no polling)
- Create a **dedicated Lambda function** (`event-processor`) separate from the orchestrator
- This Lambda processes async events and updates DynamoDB
- **Designed for extensibility**: Can handle multiple event types (ECS completions, future events)

**Why a dedicated event processor Lambda?**
- ✅ **Single Responsibility**: Orchestrator handles sync API requests; event processor handles async events
- ✅ **Independent Scaling**: Each Lambda scales based on its workload
- ✅ **Easier Testing**: Separate concerns are easier to test and debug
- ✅ **Cleaner Code**: No mixing of synchronous API requests with async event processing
- ✅ **Different Configurations**: Different timeout, memory, and retry settings for each function
- ✅ **Extensible**: Easy to add new event types without changing the orchestrator

### Event Flow Diagram

```text
┌─────────────────────────────────────────────────────────────────┐
│                         AWS Account                              │
│                                                                  │
│  ┌──────────────┐                                               │
│  │ API Request  │                                               │
│  └──────┬───────┘                                               │
│         │                                                        │
│  ┌──────▼───────────┐                                          │
│  │ Lambda           │                                           │
│  │ (Orchestrator)   │         ┌──────────────┐                │
│  │                  │────────►│ DynamoDB     │                │
│  │ - Start ECS task │         │ - Create     │                │
│  │ - Create exec    │         │   execution  │                │
│  │   record         │         │   (RUNNING)  │                │
│  └──────┬───────────┘         └──────────────┘                │
│         │                                                        │
│         │ RunTask                                               │
│         ▼                                                        │
│  ┌──────────────────┐                                          │
│  │ ECS Fargate Task │                                           │
│  │ (Running...)     │                                           │
│  └──────┬───────────┘                                          │
│         │                                                        │
│         │ Task State Change                                     │
│         │ (STOPPED)                                             │
│         ▼                                                        │
│  ┌──────────────────┐                                          │
│  │ EventBridge      │                                           │
│  │ Rule             │                                           │
│  │ - Filter: STOPPED│                                           │
│  │   tasks for our  │                                           │
│  │   cluster        │                                           │
│  └──────┬───────────┘                                          │
│         │                                                        │
│         │ Event                                                 │
│         ▼                                                        │
│  ┌──────────────────┐                                          │
│  │ Lambda           │                                           │
│  │ (Event           │         ┌──────────────┐                │
│  │  Processor)      │────────►│ DynamoDB     │                │
│  │                  │         │ - Update     │                │
│  │ - Route event    │         │   execution  │                │
│  │ - Calculate cost │         │   (COMPLETE) │                │
│  │ - Update record  │         └──────────────┘                │
│  └──────────────────┘                                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. EventBridge Rule

**Event Pattern:**
```json
{
  "source": ["aws.ecs"],
  "detail-type": ["ECS Task State Change"],
  "detail": {
    "clusterArn": ["arn:aws:ecs:REGION:ACCOUNT:cluster/runvoy-cluster"],
    "lastStatus": ["STOPPED"]
  }
}
```

**What this captures:**
- All ECS tasks in the runvoy cluster
- Only when they reach `STOPPED` status (completed, failed, or terminated)
- Provides full task details including exit codes, stop reason, timestamps

### 2. Event Processor Lambda

**Responsibilities:**
1. **Route events** by type (currently only ECS task completion, extensible for future events)
2. **For ECS Task Completions:**
   - Extract execution ID from task ARN (last segment)
   - Retrieve task details from event payload
   - Determine final status (SUCCEEDED, FAILED, STOPPED)
   - Calculate duration (stopped_at - started_at)
   - Calculate cost based on Fargate pricing
   - Update DynamoDB execution record
3. **Future event types** can be added with minimal changes

**Input (EventBridge Event):**
```json
{
  "version": "0",
  "id": "event-id",
  "detail-type": "ECS Task State Change",
  "source": "aws.ecs",
  "account": "123456789012",
  "time": "2024-01-01T12:00:00Z",
  "region": "us-east-1",
  "detail": {
    "clusterArn": "arn:aws:ecs:us-east-1:123456789012:cluster/runvoy-cluster",
    "taskArn": "arn:aws:ecs:us-east-1:123456789012:task/runvoy-cluster/abc123def456",
    "lastStatus": "STOPPED",
    "desiredStatus": "STOPPED",
    "containers": [
      {
        "containerArn": "arn:...",
        "name": "executor",
        "exitCode": 0,
        "reason": "Essential container in task exited"
      }
    ],
    "startedAt": "2024-01-01T11:50:00Z",
    "stoppedAt": "2024-01-01T12:00:00Z",
    "stoppedReason": "Essential container in task exited",
    "stopCode": "EssentialContainerExited",
    "cpu": "256",
    "memory": "512"
  }
}
```

**Processing Logic:**
```go
// Pseudo-code for event processor
func HandleEvent(ctx context.Context, event interface{}) error {
    // Route by event type
    switch evt := event.(type) {
    case events.CloudWatchEvent:
        return handleCloudWatchEvent(ctx, evt)
    default:
        return fmt.Errorf("unknown event type: %T", event)
    }
}

func handleCloudWatchEvent(ctx context.Context, event events.CloudWatchEvent) error {
    // Route by detail-type
    switch event.DetailType {
    case "ECS Task State Change":
        return handleECSTaskCompletion(ctx, event)
    default:
        log.Info("ignoring event type", "detailType", event.DetailType)
        return nil
    }
}

func handleECSTaskCompletion(ctx context.Context, event events.CloudWatchEvent) error {
    // 1. Extract execution ID from task ARN
    // Task ARN format: arn:aws:ecs:region:account:task/cluster/EXECUTION_ID
    // Execution ID is the last segment (same logic as orchestrator)
    executionID := extractExecutionIDFromTaskArn(event.Detail.TaskArn)
    
    // 2. Get execution from DynamoDB (need started_at for composite key)
    execution := getExecution(executionID)
    if execution == nil {
        log.Warn("execution not found", "executionID", executionID, "taskArn", event.Detail.TaskArn)
        return nil // Don't fail for orphaned tasks
    }
    
    // 3. Determine final status
    status := determineStatus(event.Detail.Containers[0].ExitCode, event.Detail.StopCode)
    
    // 4. Calculate duration
    startedAt := parseTime(event.Detail.StartedAt)
    stoppedAt := parseTime(event.Detail.StoppedAt)
    durationSecs := int(stoppedAt.Sub(startedAt).Seconds())
    
    // 5. Calculate cost (Fargate pricing)
    cost := calculateFargateCost(event.Detail.Cpu, event.Detail.Memory, durationSecs)
    
    // 6. Update execution record
    execution.Status = status
    execution.ExitCode = event.Detail.Containers[0].ExitCode
    execution.CompletedAt = &stoppedAt
    execution.DurationSeconds = durationSecs
    execution.CostUSD = cost
    
    return updateExecution(execution)
}

// Helper function to extract execution ID from task ARN
func extractExecutionIDFromTaskArn(taskArn string) string {
    // Example: arn:aws:ecs:us-east-1:123456789012:task/runvoy-cluster/abc123def456
    parts := strings.Split(taskArn, "/")
    return parts[len(parts)-1] // Returns: abc123def456
}
```

### 3. Cost Calculation

**Fargate Pricing (as of 2024):**
- **vCPU**: $0.04048 per vCPU hour
- **Memory**: $0.004445 per GB hour
- **Architecture**: ARM64 (per CloudFormation template)

**Calculation:**
```go
func calculateFargateCost(cpu string, memory string, durationSecs int) float64 {
    // Parse CPU (e.g., "256" = 0.25 vCPU)
    vCPU := parseFloat(cpu) / 1024.0
    
    // Parse memory (e.g., "512" = 0.5 GB)
    memoryGB := parseFloat(memory) / 1024.0
    
    // Calculate hours
    hours := float64(durationSecs) / 3600.0
    
    // Fargate pricing (ARM64)
    cpuCost := vCPU * 0.04048 * hours
    memoryCost := memoryGB * 0.004445 * hours
    
    return cpuCost + memoryCost
}
```

**Example:**
- Task: 0.25 vCPU, 0.5 GB memory, 10 minutes (600 seconds)
- Hours: 0.167
- CPU cost: 0.25 × $0.04048 × 0.167 = $0.00169
- Memory cost: 0.5 × $0.004445 × 0.167 = $0.00037
- **Total: $0.00206** (approximately $0.002)

### 4. Status Determination Logic

```go
func determineStatus(exitCode int, stopCode string) string {
    // Check stop reason first
    switch stopCode {
    case "UserInitiated":
        return "STOPPED" // Manual termination
    case "EssentialContainerExited":
        // Check exit code
        if exitCode == 0 {
            return "SUCCEEDED"
        }
        return "FAILED"
    case "TaskFailedToStart":
        return "FAILED"
    default:
        // Fallback to exit code
        if exitCode == 0 {
            return "SUCCEEDED"
        }
        return "FAILED"
    }
}
```

## Database Schema Changes

### Current Execution Record

Already has all necessary fields (from `/workspace/internal/database/dynamodb/executions.go`):
```go
type executionItem struct {
    ExecutionID     string     `dynamodbav:"execution_id"`
    StartedAt       time.Time  `dynamodbav:"started_at"`
    UserEmail       string     `dynamodbav:"user_email"`
    Command         string     `dynamodbav:"command"`
    LockName        string     `dynamodbav:"lock_name,omitempty"`
    Status          string     `dynamodbav:"status"`
    CompletedAt     *time.Time `dynamodbav:"completed_at,omitempty"`
    ExitCode        int        `dynamodbav:"exit_code,omitempty"`
    DurationSecs    int        `dynamodbav:"duration_seconds,omitempty"`
    LogStreamName   string     `dynamodbav:"log_stream_name,omitempty"`
    CostUSD         float64    `dynamodbav:"cost_usd,omitempty"`
    RequestID       string     `dynamodbav:"request_id,omitempty"`
    ComputePlatform string     `dynamodbav:"compute_platform,omitempty"`
}
```

**No schema changes needed!** The execution ID can be extracted directly from the task ARN in the EventBridge event (it's the last segment of the ARN), so we don't need to store the task ARN separately.

## CloudFormation Changes

### 1. Add EventBridge Rule

```yaml
# EventBridge Rule for ECS Task State Changes
TaskCompletionEventRule:
  Type: AWS::Events::Rule
  Properties:
    Name: !Sub '${ProjectName}-task-completion'
    Description: 'Captures ECS task completion events for runvoy'
    State: ENABLED
    EventPattern:
      source:
        - aws.ecs
      detail-type:
        - ECS Task State Change
      detail:
        clusterArn:
          - !GetAtt ECSCluster.Arn
        lastStatus:
          - STOPPED
    Targets:
      - Arn: !GetAtt EventProcessorFunction.Arn
        Id: EventProcessorTarget
```

### 2. Add Event Processor CloudWatch Log Group

```yaml
# CloudWatch Log Group for Event Processor Lambda
EventProcessorLogGroup:
  Type: AWS::Logs::LogGroup
  Properties:
    LogGroupName: !Sub '/aws/lambda/${ProjectName}-event-processor'
    RetentionInDays: 7
    Tags:
      - Key: Name
        Value: !Sub '${ProjectName}-event-processor-logs'
      - Key: Application
        Value: !Ref ProjectName
      - Key: ManagedBy
        Value: 'cloudformation'
```

### 3. Add Event Processor Lambda

```yaml
# Lambda Function for processing async events (ECS completions, etc.)
EventProcessorFunction:
  Type: AWS::Lambda::Function
  DependsOn: EventProcessorLogGroup
  Properties:
    FunctionName: !Sub '${ProjectName}-event-processor'
    Runtime: provided.al2023
    Role: !GetAtt EventProcessorRole.Arn
    Handler: bootstrap-event-processor
    Code:
      S3Bucket: !Ref LambdaCodeBucket
      S3Key: event-processor.zip
    Timeout: 30
    Architectures:
      - arm64
    Environment:
      Variables:
        RUNVOY_EXECUTIONS_TABLE: !Ref ExecutionsTable
        RUNVOY_ECS_CLUSTER: !Ref ECSCluster
```

### 4. Add IAM Role for Event Processor

```yaml
EventProcessorRole:
  Type: AWS::IAM::Role
  Properties:
    RoleName: !Sub '${ProjectName}-event-processor-role'
    AssumeRolePolicyDocument:
      Version: '2012-10-17'
      Statement:
        - Effect: Allow
          Principal:
            Service: lambda.amazonaws.com
          Action: 'sts:AssumeRole'
    ManagedPolicyArns:
      - arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
    Policies:
      - PolicyName: EventProcessorPolicy
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
            - Effect: Allow
              Action:
                - 'dynamodb:GetItem'
                - 'dynamodb:UpdateItem'
                - 'dynamodb:Query'
              Resource:
                - !GetAtt ExecutionsTable.Arn
                - !Sub '${ExecutionsTable.Arn}/index/*'
            - Effect: Allow
              Action:
                - 'ecs:DescribeTasks'
              Resource: '*'
```

### 5. Add EventBridge Permission

```yaml
EventProcessorEventPermission:
  Type: AWS::Lambda::Permission
  Properties:
    FunctionName: !Ref EventProcessorFunction
    Action: lambda:InvokeFunction
    Principal: events.amazonaws.com
    SourceArn: !GetAtt TaskCompletionEventRule.Arn
```

## Code Structure

### New Files to Create

```text
cmd/backend/aws/
  ├── main.go                    # Existing orchestrator
  └── event-processor/
      └── main.go                # New event processor

internal/
  ├── events/
  │   ├── processor.go          # Main event routing logic
  │   ├── ecs_completion.go     # ECS task completion handler
  │   ├── cost.go               # Cost calculation
  │   └── types.go              # Event types
  └── database/dynamodb/
      └── executions.go         # Update with new methods if needed
```

### Orchestrator Changes

**No changes needed!** The execution ID is already derived from the task ARN (last segment), and the EventBridge event includes the full task ARN. The event processor can extract the execution ID from the event's task ARN using the same logic as the orchestrator.

### Event Processor Architecture

The event processor is designed as a **generic async event handler** that can be extended for multiple event types:

```go
// Main handler entry point
func HandleEvent(ctx context.Context, event events.CloudWatchEvent) error {
    switch event.DetailType {
    case "ECS Task State Change":
        return handleECSTaskCompletion(ctx, event)
    // Future event types can be added here:
    // case "Another Event Type":
    //     return handleAnotherEventType(ctx, event)
    default:
        log.Info("ignoring unhandled event type", "detailType", event.DetailType)
        return nil
    }
}
```

This makes it easy to add new event types without creating new Lambda functions.

## Benefits

### 1. **Event-Driven (No Polling)**
- ✅ No continuous polling of ECS API
- ✅ Near real-time updates (typically < 1 second after task stops)
- ✅ Cost-efficient (only pay for Lambda invocations on events)

### 2. **Separation of Concerns**
- ✅ Orchestrator focuses on sync API requests
- ✅ Event processor focuses on async event handling
- ✅ Each Lambda optimized for its specific task

### 3. **Extensible Architecture**
- ✅ Single event processor can handle multiple event types
- ✅ Easy to add new event handlers without new infrastructure
- ✅ Follows the same pattern as orchestrator (dedicated to its domain)

### 4. **Reliable**
- ✅ EventBridge guarantees at-least-once delivery
- ✅ Lambda automatic retries on failures
- ✅ DLQ for failed events (can be added)

### 5. **Scalable**
- ✅ Handles any number of concurrent task completions
- ✅ No bottlenecks or rate limits
- ✅ Lambda concurrency handles burst traffic

### 6. **Auditable**
- ✅ Full execution history in DynamoDB
- ✅ Cost tracking per execution
- ✅ Complete lifecycle visibility

## Testing Strategy

### Unit Tests
- Cost calculation accuracy
- Status determination logic
- Event parsing

### Integration Tests
1. Start test ECS task
2. Wait for completion
3. Verify DynamoDB updated correctly
4. Verify cost calculation

### Error Scenarios
- Handle missing execution records (orphaned tasks)
- Handle malformed events
- Handle unknown event types (log and skip)
- Handle DynamoDB update failures (with retries)

## Rollout Plan

### Phase 1: Implementation
1. Create event processor Lambda code
2. Add EventBridge rule to CloudFormation
3. Add event processor Lambda to CloudFormation
4. Deploy and configure EventBridge → Lambda integration

### Phase 2: Testing
1. Deploy to dev/staging environment
2. Run end-to-end tests
3. Verify cost calculations against actual AWS bills

### Phase 3: Production
1. Deploy to production
2. Monitor CloudWatch logs for both Lambdas
3. Set up alarms for event processor failures

## Monitoring & Alerts

### CloudWatch Metrics
- **Event Processor Invocations**: Should match event volume
- **Event Processor Errors**: Alert if > 1% error rate
- **DynamoDB UpdateItem Latency**: Monitor update performance
- **Execution Records in RUNNING State**: Alert if any > 1 hour old

### CloudWatch Alarms
```yaml
EventProcessorErrorAlarm:
  Type: AWS::CloudWatch::Alarm
  Properties:
    AlarmName: !Sub '${ProjectName}-event-processor-errors'
    MetricName: Errors
    Namespace: AWS/Lambda
    Statistic: Sum
    Period: 300
    EvaluationPeriods: 1
    Threshold: 5
    ComparisonOperator: GreaterThanThreshold
    Dimensions:
      - Name: FunctionName
        Value: !Ref EventProcessorFunction
```

## Cost Impact

### Additional AWS Costs

**EventBridge:**
- First 1M events/month: Free
- After: $1.00 per million events
- **Expected**: ~$0 (well within free tier for most workloads)

**Lambda (Event Processor):**
- Invocations: $0.20 per 1M requests
- Duration: $0.0000133 per GB-second (ARM64)
- **Example**: 1000 tasks/month, 100ms each = ~$0.01/month

**DynamoDB:**
- UpdateItem: Already using PAY_PER_REQUEST
- Minimal additional cost (already accounted for)

**Total Additional Cost**: < $0.10/month for typical workloads

## Alternative Approaches (Considered & Rejected)

### 1. Polling from Orchestrator
- ❌ Requires keeping Lambda warm or separate polling loop
- ❌ Higher latency for updates
- ❌ More complex state management
- ❌ Higher costs (continuous Lambda execution)

### 2. Same Lambda for Both
- ❌ Mixing sync API requests with async events
- ❌ Different timeout requirements
- ❌ More complex routing logic
- ❌ Harder to test and maintain

### 3. CloudWatch Logs Subscription
- ❌ Less reliable (logs might be delayed)
- ❌ No structured task metadata
- ❌ Requires log parsing
- ❌ Exit codes not guaranteed in logs

## Conclusion

**Recommendation: Implement EventBridge + Dedicated Event Processor Lambda**

This approach provides:
- ✅ Native AWS integration (no polling)
- ✅ Clean separation of concerns (sync API vs async events)
- ✅ Extensible architecture (single Lambda for multiple event types)
- ✅ Reliable, scalable, and cost-effective
- ✅ Easy to test and maintain
- ✅ Production-ready with minimal changes
- ✅ Future-proof for additional event types

The implementation is straightforward, follows AWS best practices, and aligns with the existing architecture's design principles.

## Next Steps

1. Review and approve this proposal
2. Create implementation plan with tasks
3. Implement event processor Lambda
4. Update CloudFormation template
5. Add comprehensive tests
6. Deploy to staging
7. Monitor and validate
8. Deploy to production

Would you like me to proceed with the implementation?
