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

// buildShellCommand is deprecated in the new architecture
// Use buildDirectCommand instead
// Keeping this for backward compatibility if needed
func buildShellCommand(cfg *Config, repo, branch, userCommand string) string {
	// For now, just execute the command directly
	// Git cloning is no longer part of the core architecture
	return buildDirectCommand(userCommand)
}
