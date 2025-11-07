#!/bin/bash
# Test script for the local async event processor

set -e

# Configuration
PROCESSOR_URL="${PROCESSOR_URL:-http://localhost:8081}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Testing local async event processor at $PROCESSOR_URL"
echo ""

# Health check
echo "1. Testing health check endpoint..."
curl -s "$PROCESSOR_URL/health" | jq . || echo "Failed to connect"
echo ""

# Test ECS task completion event
if [ -f "$SCRIPT_DIR/ecs-task-completion.json" ]; then
    echo "2. Testing ECS task completion event..."
    curl -s -X POST "$PROCESSOR_URL/process" \
        -H "Content-Type: application/json" \
        -d @"$SCRIPT_DIR/ecs-task-completion.json" | jq .
    echo ""
fi

# Test WebSocket event
if [ -f "$SCRIPT_DIR/websocket-event.json" ]; then
    echo "3. Testing WebSocket connection event..."
    curl -s -X POST "$PROCESSOR_URL/process" \
        -H "Content-Type: application/json" \
        -d @"$SCRIPT_DIR/websocket-event.json" | jq .
    echo ""
fi

echo "Tests complete!"
