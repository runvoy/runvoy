#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[mycli]${NC} $1"
}

error() {
    echo -e "${RED}[mycli ERROR]${NC} $1" >&2
}

warn() {
    echo -e "${YELLOW}[mycli WARN]${NC} $1"
}

# Validate required environment variables
if [ -z "$REPO_URL" ]; then
    error "REPO_URL environment variable is required"
    exit 1
fi

if [ -z "$USER_COMMAND" ]; then
    error "USER_COMMAND environment variable is required"
    exit 1
fi

# Set defaults
REPO_BRANCH=${REPO_BRANCH:-main}
WORKSPACE_DIR="/workspace"
EXECUTION_ID=${EXECUTION_ID:-"unknown"}

log "=========================================="
log "mycli Remote Execution"
log "=========================================="
log "Execution ID: $EXECUTION_ID"
log "Repository:   $REPO_URL"
log "Branch:       $REPO_BRANCH"
log "Command:      $USER_COMMAND"
log "=========================================="

# Setup Git credentials if provided
if [ -n "$GITHUB_TOKEN" ]; then
    log "Configuring GitHub authentication..."
    # Configure git to use token for HTTPS
    git config --global credential.helper store
    echo "https://$GITHUB_TOKEN:x-oauth-basic@github.com" > ~/.git-credentials
    chmod 600 ~/.git-credentials
elif [ -n "$GITLAB_TOKEN" ]; then
    log "Configuring GitLab authentication..."
    git config --global credential.helper store
    echo "https://oauth2:$GITLAB_TOKEN@gitlab.com" > ~/.git-credentials
    chmod 600 ~/.git-credentials
elif [ -n "$SSH_PRIVATE_KEY" ]; then
    log "Configuring SSH authentication..."
    mkdir -p ~/.ssh
    echo "$SSH_PRIVATE_KEY" | base64 -d > ~/.ssh/id_rsa
    chmod 600 ~/.ssh/id_rsa
    # Add common Git hosts to known_hosts
    ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null
    ssh-keyscan gitlab.com >> ~/.ssh/known_hosts 2>/dev/null
    ssh-keyscan bitbucket.org >> ~/.ssh/known_hosts 2>/dev/null
else
    warn "No Git credentials provided - only public repositories will be accessible"
fi

# Create workspace directory
mkdir -p "$WORKSPACE_DIR"
cd "$WORKSPACE_DIR"

# Clone the repository
log "Cloning repository..."
if ! git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" code 2>&1; then
    error "Failed to clone repository"
    error "Please check:"
    error "  - Repository URL is correct"
    error "  - Branch '$REPO_BRANCH' exists"
    error "  - Git credentials are properly configured (for private repos)"
    exit 1
fi

cd code

log "Repository cloned successfully"
log "Working directory: $(pwd)"
log ""
log "=========================================="
log "Executing command..."
log "=========================================="
log ""

# Execute the user's command
# Use eval to properly handle shell expansion, quotes, etc.
if eval "$USER_COMMAND"; then
    EXIT_CODE=0
    log ""
    log "=========================================="
    log "Command completed successfully"
    log "=========================================="
else
    EXIT_CODE=$?
    error ""
    error "=========================================="
    error "Command failed with exit code: $EXIT_CODE"
    error "=========================================="
fi

# Cleanup credentials
rm -f ~/.git-credentials
rm -f ~/.ssh/id_rsa

exit $EXIT_CODE
