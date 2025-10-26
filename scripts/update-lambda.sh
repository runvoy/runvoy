#!/bin/bash
set -e

# Quick Lambda update helper for development
# This script builds and deploys Lambda code without full destroy/init cycle

echo "🔨 Building Lambda function..."

# Navigate to lambda directory
cd "$(dirname "$0")/../lambda/orchestrator"

# Build the Go binary for Lambda (ARM64)
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap .

# Create zip file
echo "📦 Creating deployment package..."
zip -q function.zip bootstrap
rm bootstrap

# Get stack name and region from mycli config
CONFIG_FILE="$HOME/.mycli/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "❌ Config file not found at $CONFIG_FILE"
    echo "   Run 'mycli init' first or manually specify stack name and region"
    exit 1
fi

# Extract region from config (simple parsing for YAML)
REGION=$(grep 'region:' "$CONFIG_FILE" | awk '{print $2}' | tr -d '"' | tr -d "'")
if [ -z "$REGION" ]; then
    echo "⚠️  Could not detect region from config, using default: us-east-2"
    REGION="us-east-2"
fi

# Allow override via arguments
STACK_NAME="${1:-mycli}"
FUNCTION_NAME="${STACK_NAME}-orchestrator"

echo "📤 Updating Lambda function: $FUNCTION_NAME (region: $REGION)"

# Update Lambda function code
aws lambda update-function-code \
    --function-name "$FUNCTION_NAME" \
    --zip-file fileb://function.zip \
    --region "$REGION" \
    --output json \
    > /dev/null

# Wait for update to complete
echo "⏳ Waiting for Lambda update to complete..."
aws lambda wait function-updated \
    --function-name "$FUNCTION_NAME" \
    --region "$REGION"

# Clean up
rm function.zip

echo "✅ Lambda function updated successfully!"
echo ""
echo "Usage tips:"
echo "  • Default stack name: mycli"
echo "  • Custom stack: $0 my-custom-stack"
echo "  • Test changes: mycli exec --repo=<repo> \"<command>\""
