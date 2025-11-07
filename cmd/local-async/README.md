# Local Async Event Processor

This is a local development server for testing the async event processor without deploying to AWS Lambda.

## Overview

The async event processor handles various Lambda events:
- **CloudWatch Events** (e.g., ECS task state changes)
- **CloudWatch Logs Events** (log streaming)
- **WebSocket Events** (real-time connections)

## Building

```bash
just build-local-async
```

## Running

```bash
just run-local-async
```

The server will start on the port specified by `RUNVOY_EVENT_PROCESSOR_PORT` (default: 8081).

## API Endpoints

### Health Check
```bash
curl http://localhost:8081/health
```

### Process Event
Send a raw Lambda event to the processor:
```bash
curl -X POST http://localhost:8081/process \
  -H "Content-Type: application/json" \
  -d @event.json
```

## Example Events

### CloudWatch Event (ECS Task Completion)

Create a file `ecs-task-completion.json`:

```json
{
  "version": "0",
  "id": "12345678-1234-1234-1234-123456789012",
  "detail-type": "ECS Task State Change",
  "source": "aws.ecs",
  "account": "123456789012",
  "time": "2024-11-07T17:27:00Z",
  "region": "us-east-1",
  "resources": [],
  "detail": {
    "taskArn": "arn:aws:ecs:us-east-1:123456789012:task/cluster/test-exec-123",
    "lastStatus": "STOPPED",
    "containers": [
      {
        "name": "runner",
        "exitCode": 0
      }
    ],
    "startedAt": "2024-11-07T17:26:00Z",
    "stoppedAt": "2024-11-07T17:27:00Z",
    "stopCode": "EssentialContainerExited"
  }
}
```

Test it:
```bash
curl -X POST http://localhost:8081/process -d @ecs-task-completion.json
```

### CloudWatch Logs Event

To test CloudWatch logs streaming, you'll need a compressed and base64-encoded log event. The processor expects logs in the standard CloudWatch Logs Lambda format.

### WebSocket Event

WebSocket events follow the API Gateway WebSocket format. The processor will route these appropriately based on the route key.

## Configuration

The async processor uses the same configuration as the Lambda version. Key environment variables:

- `RUNVOY_EVENT_PROCESSOR_PORT` - Server port (default: 8081)
- `RUNVOY_EXECUTIONS_TABLE` - DynamoDB table for executions
- `RUNVOY_WEBSOCKET_CONNECTIONS_TABLE` - DynamoDB table for WebSocket connections
- `RUNVOY_WEBSOCKET_API_ENDPOINT` - API Gateway Management API endpoint
- `RUNVOY_LOG_LEVEL` - Logging level

For local development, ensure these are set in your `.env` file.

## Development Tips

1. **Run both servers simultaneously** for full local testing:
   - Terminal 1: `just run-local` (orchestrator server on port 8080)
   - Terminal 2: `just run-local-async` (async processor on port 8081)

2. **Monitor logs** to understand event processing:
   - Both servers output structured logs for debugging

3. **Test the full flow**:
   - Start an execution via the orchestrator (`/api/v1/run`)
   - Manually send events to the async processor to simulate Lambda invocations
   - Verify that executions are updated correctly
