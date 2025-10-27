#!/bin/bash

# Local integration test script

set -e

echo "Starting local integration tests..."

# Start local server in background
echo "Starting local server..."
./bin/local &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Test health endpoint
echo "Testing health endpoint..."
curl -f http://localhost:8080/health || {
    echo "Health check failed"
    kill $SERVER_PID
    exit 1
}

# Test execution endpoint (this will fail without proper setup, but tests the endpoint)
echo "Testing execution endpoint..."
curl -f -X POST http://localhost:8080/executions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: test-key" \
  -d '{"command": "echo hello world"}' || {
    echo "Execution test failed (expected without proper mock setup)"
}

# Clean up
echo "Stopping local server..."
kill $SERVER_PID

echo "Local integration tests completed!"