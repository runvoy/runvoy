package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	// Find repo root
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Change to repo root
	if err := os.Chdir(repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing to repo root: %v\n", err)
		os.Exit(1)
	}

	readmePath := filepath.Join(repoRoot, "README.md")
	cliBinary := filepath.Join(repoRoot, "bin", "runvoy")

	// Build CLI if needed
	if err := ensureCLIBuilt(repoRoot, cliBinary); err != nil {
		fmt.Fprintf(os.Stderr, "Error building CLI: %v\n", err)
		os.Exit(1)
	}

	// Capture help output
	helpOutput, err := captureHelpOutput(cliBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error capturing help output: %v\n", err)
		os.Exit(1)
	}

	// Generate help section
	helpSection := generateHelpSection(helpOutput)

	// Update README
	if err := updateREADME(readmePath, helpSection); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating README: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("? Updated README.md with latest CLI help output")
}

func findRepoRoot() (string, error) {
	// Start from current working directory and walk up to find go.mod
	current, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine current directory: %w", err)
	}

	// Walk up from current directory to find go.mod or README.md
	for {
		goModPath := filepath.Join(current, "go.mod")
		readmePath := filepath.Join(current, "README.md")

		// Check for go.mod first as the most reliable indicator
		if _, err := os.Stat(goModPath); err == nil {
			return current, nil
		}
		// Fall back to README.md check
		if _, err := os.Stat(readmePath); err == nil {
			// If we're in a scripts subdirectory, the parent is the repo root
			if filepath.Base(current) == "scripts" {
				return filepath.Dir(current), nil
			}
			// Otherwise, assume this directory with README.md is the root
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("could not find repo root (go.mod or README.md not found)")
}

func ensureCLIBuilt(repoRoot, cliBinary string) error {
	// Check if binary exists
	binaryInfo, err := os.Stat(cliBinary)
	if err == nil {
		// Check if any source files are newer
		needsRebuild, err := needsRebuild(repoRoot, binaryInfo.ModTime())
		if err != nil {
			return fmt.Errorf("error checking if rebuild needed: %w", err)
		}
		if !needsRebuild {
			return nil
		}
	}

	fmt.Println("Building CLI...")

	// Try just build-cli first, fall back to go build
	justCmd := exec.Command("just", "build-cli")
	justCmd.Dir = repoRoot
	justCmd.Stdout = os.Stderr
	justCmd.Stderr = os.Stderr
	if err := justCmd.Run(); err != nil {
		// Fall back to go build
		goCmd := exec.Command("go", "build", "-o", cliBinary, "./cmd/runvoy")
		goCmd.Dir = repoRoot
		goCmd.Stdout = os.Stderr
		goCmd.Stderr = os.Stderr
		if err := goCmd.Run(); err != nil {
			return fmt.Errorf("failed to build CLI: %w", err)
		}
	}

	return nil
}

func needsRebuild(repoRoot string, binaryTime time.Time) (bool, error) {
	runvoyDir := filepath.Join(repoRoot, "cmd", "runvoy")
	return hasNewerFiles(runvoyDir, binaryTime, ".go")
}

func hasNewerFiles(dir string, since time.Time, ext string) (bool, error) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ext {
			if info.ModTime().After(since) {
				return fmt.Errorf("found newer file") // Signal to stop
			}
		}
		return nil
	})

	// If we got an error indicating a newer file was found, return true
	if err != nil && err.Error() == "found newer file" {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

func captureHelpOutput(cliBinary string) (string, error) {
	cmd := exec.Command(cliBinary, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Help command might exit with non-zero, but we still want the output
		return strings.TrimSpace(string(output)), nil
	}
	return strings.TrimSpace(string(output)), nil
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
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("failed to read README: %w", err)
	}

	startMarker := "<!-- CLI_HELP_START -->"
	endMarker := "<!-- CLI_HELP_END -->"

	contentStr := string(content)

	if !strings.Contains(contentStr, startMarker) || !strings.Contains(contentStr, endMarker) {
		return fmt.Errorf("could not find <!-- CLI_HELP_START --> and <!-- CLI_HELP_END --> markers in README.md")
	}

	// Replace content between markers using regex
	pattern := regexp.MustCompile(
		regexp.QuoteMeta(startMarker) + `.*?` + regexp.QuoteMeta(endMarker),
	)
	replacement := startMarker + "\n" + helpSection + "\n" + endMarker

	newContent := pattern.ReplaceAllString(contentStr, replacement)

	if err := os.WriteFile(readmePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	return nil
}