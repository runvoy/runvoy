package main

import (
    "fmt"
    "strings"
)

// shellEscape escapes a string for safe use in a shell command
// This prevents injection attacks when embedding variables in shell scripts
func shellEscape(s string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	// This allows us to safely wrap the string in single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// buildDirectCommand constructs a simple shell script that:
// 1. Executes the user's command directly (without git cloning)
//
// This is used when --skip-git flag is enabled
func buildDirectCommand(userCommand string) string {
	escapedCommand := shellEscape(userCommand)

    script := `set -e
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution (No Git)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "→ Mode: Direct command execution (git cloning skipped)"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
`

	// Execute user command
	script += fmt.Sprintf("eval %s\n", escapedCommand)

	// Capture exit code
    script += `
EXIT_CODE=$?
echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✓ Command completed successfully"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✗ Command failed with exit code: $EXIT_CODE"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
exit $EXIT_CODE
`

	return script
}

// buildShellCommand constructs a shell script that:
// 1. Installs git if not present (for generic images)
// 2. Configures git credentials
// 3. Clones the repository
// 4. Executes the user's command
//
// NOTE: This is a pragmatic bash solution for MVP.
// Future: Consider more robust solutions (Go binary, Python script, etc.)
func buildShellCommand(cfg *Config, repo, branch, userCommand string) string {
    script := `set -e
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "mycli Remote Execution"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
`

	// Install git if not present (for minimal images like alpine, ubuntu)
    script += `
if ! command -v git &> /dev/null; then
  echo "→ Installing git..."
  if command -v apk &> /dev/null; then
    apk add --no-cache git openssh-client
  elif command -v apt-get &> /dev/null; then
    apt-get update && apt-get install -y git openssh-client
  elif command -v yum &> /dev/null; then
    yum install -y git openssh-clients
  else
    echo "ERROR: Cannot install git - unsupported package manager"
    exit 1
  fi
fi
`

	// Setup git credentials (with proper shell escaping for security)
	if cfg.GitHubToken != "" {
		escapedToken := shellEscape(cfg.GitHubToken)
        script += fmt.Sprintf(`
echo "→ Configuring GitHub authentication..."
git config --global credential.helper store
echo "https://%s:x-oauth-basic@github.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
`, escapedToken)
	} else if cfg.GitLabToken != "" {
		escapedToken := shellEscape(cfg.GitLabToken)
        script += fmt.Sprintf(`
echo "→ Configuring GitLab authentication..."
git config --global credential.helper store
echo "https://oauth2:%s@gitlab.com" > ~/.git-credentials
chmod 600 ~/.git-credentials
`, escapedToken)
	} else if cfg.SSHPrivateKey != "" {
		escapedKey := shellEscape(cfg.SSHPrivateKey)
        script += fmt.Sprintf(`
echo "→ Configuring SSH authentication..."
mkdir -p ~/.ssh
echo %s | base64 -d > ~/.ssh/id_rsa
chmod 600 ~/.ssh/id_rsa
ssh-keyscan github.com >> ~/.ssh/known_hosts 2>/dev/null
ssh-keyscan gitlab.com >> ~/.ssh/known_hosts 2>/dev/null
ssh-keyscan bitbucket.org >> ~/.ssh/known_hosts 2>/dev/null
`, escapedKey)
	}

	// Clone repository (with proper shell escaping for security)
	escapedRepo := shellEscape(repo)
	escapedBranch := shellEscape(branch)
	escapedCommand := shellEscape(userCommand)

    script += fmt.Sprintf(`
echo "→ Repository: %s"
echo "→ Branch: %s"
echo "→ Cloning repository..."
git clone --depth 1 --branch %s %s /workspace/repo || {
  echo "ERROR: Failed to clone repository"
  echo "Please verify:"
  echo "  - Repository URL is correct"
  echo "  - Branch exists"
  echo "  - Git credentials are configured (for private repos)"
  exit 1
}
cd /workspace/repo
echo "✓ Repository cloned"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Executing command..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
`, escapedRepo, escapedBranch, escapedBranch, escapedRepo)

	// Execute user command (already escaped above, now we need to use eval)
	// Using eval to properly handle the escaped command
	script += fmt.Sprintf("eval %s\n", escapedCommand)

	// Capture exit code and cleanup
    script += `
EXIT_CODE=$?
echo ""
if [ $EXIT_CODE -eq 0 ]; then
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✓ Command completed successfully"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "✗ Command failed with exit code: $EXIT_CODE"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi
rm -f ~/.git-credentials ~/.ssh/id_rsa
exit $EXIT_CODE
`

	return script
}
