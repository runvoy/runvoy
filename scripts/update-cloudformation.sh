#!/bin/bash
set -e

# Quick CloudFormation update helper for development
# This script updates CloudFormation stack without full destroy/init cycle

echo "🔨 Updating CloudFormation stack..."

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
STACK_NAME="${1:-mycli-backend}"

echo "📤 Updating CloudFormation stack: $STACK_NAME (region: $REGION)"

# Navigate to project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Get current stack parameters to preserve them
echo "→ Fetching current stack parameters..."
CURRENT_PARAMS=$(aws cloudformation describe-stacks \
    --stack-name "$STACK_NAME" \
    --region "$REGION" \
    --query 'Stacks[0].Parameters' \
    --output json)

if [ -z "$CURRENT_PARAMS" ] || [ "$CURRENT_PARAMS" = "null" ]; then
    echo "❌ Failed to get stack parameters"
    exit 1
fi

# Update the stack
echo "→ Updating stack with new template..."
aws cloudformation update-stack \
    --stack-name "$STACK_NAME" \
    --template-body "file://$PROJECT_ROOT/deploy/cloudformation-backend.yaml" \
    --capabilities CAPABILITY_NAMED_IAM \
    --parameters "$CURRENT_PARAMS" \
    --region "$REGION" \
    > /dev/null

if [ $? -ne 0 ]; then
    echo "⚠️  Update might have failed or no changes detected"
    echo "   Checking stack status..."
else
    # Wait for update to complete
    echo "⏳ Waiting for stack update to complete (this may take a few minutes)..."
    aws cloudformation wait stack-update-complete \
        --stack-name "$STACK_NAME" \
        --region "$REGION"
fi

echo "✅ Stack update complete!"
echo ""
echo "Usage tips:"
echo "  • Default stack name: mycli"
echo "  • Custom stack: $0 my-custom-stack"
echo "  • Check status: aws cloudformation describe-stacks --stack-name $STACK_NAME --region $REGION"
