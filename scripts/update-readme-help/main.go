// Package main provides a utility to update the README.md with the latest CLI help output and version.
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

const cliHelpStartMarker = "<!-- CLI_HELP_START -->"
const cliHelpEndMarker = "<!-- CLI_HELP_END -->"
const versionExamplesStartMarker = "<!-- VERSION_EXAMPLES_START -->"
const versionExamplesEndMarker = "<!-- VERSION_EXAMPLES_END -->"
const readmePath = "README.md"
const versionPath = "VERSION"

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

	version, err := readVersion(versionPath)
	if err != nil {
		log.Fatalf("error reading version: %s", err)
	}

	versionExamplesSection := generateVersionExamplesSection(version)

	if err = updateREADME(readmePath, helpSection, versionExamplesSection); err != nil {
		log.Fatalf("error updating %s: %s", readmePath, err)
	}

	log.Printf("updated %s with latest CLI help output and version", readmePath)
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
	b.WriteString("```text\n")
	b.WriteString(helpOutput)
	b.WriteString("\n```\n")
	return b.String()
}

func generateVersionExamplesSection(version string) string {
	var b strings.Builder
	b.WriteString("For Linux:\n\n")
	b.WriteString("```bash\n")
	linuxURL := fmt.Sprintf(
		"curl -L -o runvoy-cli-linux-arm64.tar.gz "+
			"https://github.com/runvoy/runvoy/releases/download/%s/runvoy_linux_amd64.tar.gz\n",
		version,
	)
	b.WriteString(linuxURL)
	b.WriteString("tar -xzf runvoy_linux_amd64.tar.gz\n")
	b.WriteString("sudo mv runvoy_linux_amd64/runvoy /usr/local/bin/runvoy\n")
	b.WriteString("```\n\n")
	b.WriteString("For macOS:\n\n")
	b.WriteString("```bash\n")
	macosURL := fmt.Sprintf(
		"curl -L -o runvoy_linux_amd64.tar.gz "+
			"https://github.com/runvoy/runvoy/releases/download/%s/runvoy_darwin_arm64.tar.gz\n",
		version,
	)
	b.WriteString(macosURL)
	b.WriteString("tar -xzf runvoy_darwin_arm64.tar.gz\n")
	b.WriteString("xattr -dr com.apple.quarantine runvoy_darwin_arm64/runvoy\n")
	b.WriteString("codesign -s - --deep --force runvoy_darwin_arm64/runvoy\n")
	b.WriteString("sudo mv runvoy_darwin_arm64/runvoy /usr/local/bin/runvoy\n")
	b.WriteString("```\n")
	return b.String()
}

func readVersion(versionPath string) (string, error) {
	content, err := os.ReadFile(versionPath) //nolint:gosec // G304: VERSION path is a constant
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", versionPath, err)
	}
	return strings.TrimSpace(string(content)), nil
}

func updateREADME(readmePath, helpSection, versionExamplesSection string) error {
	content, err := os.ReadFile(readmePath) //nolint:gosec // G304: README.md path is a constant
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", readmePath, err)
	}

	contentStr := string(content)

	// Update CLI help section
	if !strings.Contains(contentStr, cliHelpStartMarker) || !strings.Contains(contentStr, cliHelpEndMarker) {
		return fmt.Errorf("could not find %s and %s markers in %s",
			cliHelpStartMarker, cliHelpEndMarker, readmePath,
		)
	}

	// Replace content between CLI help markers using regex
	// Use (?s) flag to make . match newlines
	cliHelpPattern := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(cliHelpStartMarker) + `.*?` + regexp.QuoteMeta(cliHelpEndMarker),
	)
	cliHelpReplacement := cliHelpStartMarker + "\n" + helpSection + "\n" + cliHelpEndMarker
	contentStr = cliHelpPattern.ReplaceAllString(contentStr, cliHelpReplacement)

	// Update version examples section
	if !strings.Contains(contentStr, versionExamplesStartMarker) ||
		!strings.Contains(contentStr, versionExamplesEndMarker) {
		return fmt.Errorf("could not find %s and %s markers in %s",
			versionExamplesStartMarker, versionExamplesEndMarker, readmePath,
		)
	}

	// Replace version examples between markers using regex
	// Use (?s) flag to make . match newlines
	versionExamplesPattern := regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(versionExamplesStartMarker) + `.*?` + regexp.QuoteMeta(versionExamplesEndMarker),
	)
	versionExamplesReplacement := versionExamplesStartMarker + "\n" +
		versionExamplesSection + "\n" + versionExamplesEndMarker
	contentStr = versionExamplesPattern.ReplaceAllString(contentStr, versionExamplesReplacement)

	if err = os.WriteFile(readmePath, []byte(contentStr), constants.ConfigFilePermissions); err != nil {
		return fmt.Errorf("failed to write %s: %w", readmePath, err)
	}

	return nil
}
