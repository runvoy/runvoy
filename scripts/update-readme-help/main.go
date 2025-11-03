// Package main provides a utility to update the README.md with the latest CLI help output.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"runvoy/internal/constants"
)

const startMarker = "<!-- CLI_HELP_START -->"
const endMarker = "<!-- CLI_HELP_END -->"
const readmePath = "README.md"

func main() {
	if len(os.Args) < constants.MinimumArgsUpdateReadmeHelp {
		log.Fatalf("usage: %s <cli-binary-path>", os.Args[0])
	}

	cliBinary := os.Args[1]
	if cliBinary == "" {
		log.Fatalf("error: cli binary path is required")
	}

	helpOutput, err := captureHelpOutput(cliBinary)
	if err != nil {
		log.Fatalf("error capturing help output: %s", err)
	}

	helpSection := generateHelpSection(helpOutput)

	if err = updateREADME(readmePath, helpSection); err != nil {
		log.Fatalf("error updating %s: %s", readmePath, err)
	}

	log.Printf("updated %s with latest CLI help output", readmePath)
}

func captureHelpOutput(cliBinary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.LongScriptContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliBinary, "--help")
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func generateHelpSection(helpOutput string) string {
	var b strings.Builder
	b.WriteString("### Available Commands\n\n")
	b.WriteString("To see all available commands and their descriptions:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("runvoy --help\n")
	b.WriteString("```\n\n")
	b.WriteString("```bash\n")
	b.WriteString(helpOutput)
	b.WriteString("\n```\n\n")
	b.WriteString("For more details about a specific command, use:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("runvoy [command] --help\n")
	b.WriteString("```\n\n")
	b.WriteString("For example, to see all user management commands:\n\n")
	b.WriteString("```bash\n")
	b.WriteString("runvoy users --help\n")
	b.WriteString("```\n")
	return b.String()
}

func updateREADME(readmePath, helpSection string) error {
	content, err := os.ReadFile(readmePath) //nolint:gosec // G304: README.md path is a constant
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", readmePath, err)
	}

	contentStr := string(content)

	if !strings.Contains(contentStr, startMarker) || !strings.Contains(contentStr, endMarker) {
		return fmt.Errorf("could not find %s and %s markers in %s",
			startMarker, endMarker, readmePath,
		)
	}

	// Replace content between markers using regex
	// Use (?s) flag to make . match newlines
	pattern := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(startMarker) + `.*?` + regexp.QuoteMeta(endMarker),
	)
	replacement := startMarker + "\n" + helpSection + "\n" + endMarker
	newContent := pattern.ReplaceAllString(contentStr, replacement)

	if err = os.WriteFile(readmePath, []byte(newContent), constants.ConfigFilePermissions); err != nil {
		return fmt.Errorf("failed to write %s: %w", readmePath, err)
	}

	return nil
}
