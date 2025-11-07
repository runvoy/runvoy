# Local Development Server

This is the local development server for runvoy. It runs both the orchestrator and async event processor services in a single process for convenient testing and development.

## Overview

The local server runs two services:

1. **Orchestrator** (port 8080) - Main API for orchestrating command execution
   - REST API for managing executions, users, images
   - WebSocket support for real-time execution status

2. **Async Event Processor** (port 8081) - Handles async Lambda events
   - CloudWatch Events (e.g., ECS task state changes)
   - CloudWatch Logs Events (log streaming)
   - WebSocket Events (real-time connections)

## Building

```bash
just build-local
```

## Running

```bash
just run-local
```

Both services will start automatically:
- Orchestrator on port 8080
- Async processor on port 8081

Press `Ctrl+C` to gracefully shut down both services together.

## API Endpoints

### Orchestrator (Port 8080)

```bash
# Health check
curl http://localhost:8080/api/v1/health

# List executions (requires API key)
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8080/api/v1/executions

# Run a command (requires API key)
curl -X POST http://localhost:8080/api/v1/run \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "imageUri": "public.ecr.aws/docker/library/ubuntu:22.04",
    "command": "echo hello"
  }'
```

### Async Event Processor (Port 8081)

```bash
# Health check
curl http://localhost:8081/health

# Process a Lambda event
curl -X POST http://localhost:8081/process \
  -H "Content-Type: application/json" \
  -d @examples/ecs-task-completion.json
```

## Example Events

### CloudWatch Event (ECS Task Completion)

See `examples/ecs-task-completion.json` for a complete example. Test with:

```bash
curl -X POST http://localhost:8081/process -d @examples/ecs-task-completion.json
```

### WebSocket Event

See `examples/websocket-event.json` for a complete example.

## Configuration

Both services read configuration from environment variables:

### Orchestrator Configuration
- `RUNVOY_DEV_SERVER_PORT` - Server port (default: 8080)
- `RUNVOY_REQUEST_TIMEOUT` - Request timeout duration
- `RUNVOY_LOG_LEVEL` - Logging level

### Event Processor Configuration
- `RUNVOY_EVENT_PROCESSOR_PORT` - Server port (default: 8081)
- `RUNVOY_EXECUTIONS_TABLE` - DynamoDB table for executions
- `RUNVOY_WEBSOCKET_CONNECTIONS_TABLE` - DynamoDB table for WebSocket connections
- `RUNVOY_WEBSOCKET_API_ENDPOINT` - API Gateway Management API endpoint
- `RUNVOY_LOG_LEVEL` - Logging level

For local development, ensure these are set in your `.env` file.

## Development Workflow

1. **Start the local server**
   ```bash
   just run-local
   ```

2. **In another terminal, start an execution**
   ```bash
   curl -X POST http://localhost:8080/api/v1/run \
     -H "X-API-Key: YOUR_API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"imageUri": "ubuntu:22.04", "command": "sleep 2"}'
   ```

3. **Simulate ECS task completion** (async event)
   ```bash
   # Get the execution ID from the previous response, then:
   curl -X POST http://localhost:8081/process \
     -d @examples/ecs-task-completion.json
   ```

4. **Check execution status**
   ```bash
   curl -H "X-API-Key: YOUR_API_KEY" \
     http://localhost:8080/api/v1/executions/{executionID}/status
   ```

## Testing with the Provided Scripts

Run the automated test script:

```bash
./examples/test.sh
```

This will test both the orchestrator and async processor endpoints.

## Debugging

Both services output structured logs for debugging. You can set the log level via the `RUNVOY_LOG_LEVEL` environment variable:

```bash
RUNVOY_LOG_LEVEL=debug just run-local
```

Look for log output from:
- `starting orchestrator server` - Orchestrator startup info
- `starting async processor server` - Async processor startup info
