// Package main provides a utility to update CHANGELOG.md with commits since the last release.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/runvoy/runvoy/internal/constants"
)

const changelogPath = "CHANGELOG.md"
const versionPath = "VERSION"
const githubRepo = "runvoy/runvoy"
const githubBaseURL = "https://github.com/" + githubRepo + "/commit/"
const categoryChanged = "Changed"

type commit struct {
	hash       string
	shortHash  string
	message    string
	commitType string
}

func main() {
	version, err := readVersion(versionPath)
	if err != nil {
		log.Fatalf("error reading version: %s", err)
	}

	// Get the last release version from CHANGELOG
	lastVersion, err := getLastReleaseVersion(changelogPath)
	if err != nil {
		log.Fatalf("error getting last release version: %s", err)
	}

	// Get commits since last release
	commits, err := getCommitsSinceTag(lastVersion)
	if err != nil {
		log.Fatalf("error getting commits: %s", err)
	}

	if len(commits) == 0 {
		log.Printf("no commits found since %s, skipping changelog update", lastVersion)
		return
	}

	// Categorize commits
	categorized := categorizeCommits(commits)

	// Generate new release section
	today := time.Now().Format("2006-01-02")
	newSection := generateReleaseSection(version, today, categorized)

	// Update CHANGELOG
	if err = updateChangelog(changelogPath, newSection); err != nil {
		log.Fatalf("error updating changelog: %s", err)
	}

	log.Printf("updated %s with release %s", changelogPath, version)
}

func readVersion(versionPath string) (string, error) {
	content, err := os.ReadFile(versionPath) //nolint:gosec // G304: VERSION path is a constant
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", versionPath, err)
	}
	return strings.TrimSpace(string(content)), nil
}

func getLastReleaseVersion(changelogPath string) (string, error) {
	content, err := os.ReadFile(changelogPath) //nolint:gosec // G304: CHANGELOG.md path is a constant
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", changelogPath, err)
	}

	// Find the first release version in the changelog
	// Pattern: ## [v0.3.0] - 2025-11-25
	pattern := regexp.MustCompile(`^## \[(v\d+\.\d+\.\d+)\]`)
	for line := range strings.SplitSeq(string(content), "\n") {
		matches := pattern.FindStringSubmatch(line)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not find last release version in %s", changelogPath)
}

func getCommitsSinceTag(tag string) ([]commit, error) {
	// Validate tag format to prevent command injection
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(tag) {
		return nil, fmt.Errorf("invalid tag format: %s", tag)
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.LongScriptContextTimeout)
	defer cancel()

	// Get commits since tag, format: %H|%s (full hash|subject)
	// tag is validated above, so it's safe to use in command
	//nolint:gosec // G204: tag validated above
	cmd := exec.CommandContext(ctx, "git", "log", tag+"..HEAD", "--pretty=format:%H|%s", "--no-merges")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	if len(output) == 0 {
		return []commit{}, nil
	}

	outputStr := strings.TrimSpace(string(output))
	commits := make([]commit, 0)

	for line := range strings.SplitSeq(outputStr, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}

		fullHash := parts[0]
		message := parts[1]

		// Get short hash
		shortHash, hashErr := getShortHash(fullHash)
		if hashErr != nil {
			log.Printf("warning: failed to get short hash for %s: %s", fullHash, hashErr)
			shortHash = fullHash[:8] // fallback to first 8 chars
		}

		// Extract commit type from message
		commitType := extractCommitType(message)

		commits = append(commits, commit{
			hash:       fullHash,
			shortHash:  shortHash,
			message:    message,
			commitType: commitType,
		})
	}

	return commits, nil
}

func getShortHash(fullHash string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ScriptContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", fullHash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get short hash: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func extractCommitType(message string) string {
	// Parse conventional commit format: type(scope): description
	// Examples: feat: add feature, fix: fix bug, perf: improve performance
	pattern := regexp.MustCompile(`^(\w+)(?:\([^)]+\))?:`)
	matches := pattern.FindStringSubmatch(message)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return "other"
}

func categorizeCommits(commits []commit) map[string][]commit {
	categorized := make(map[string][]commit)

	for _, c := range commits {
		var category string
		switch c.commitType {
		case "feat":
			category = "Added"
		case "fix":
			category = "Fixed"
		case "perf":
			category = categoryChanged // Performance improvements go under Changed
		case "refactor", "chore", "test", "docs", "style", "ci", "build", "tool", "utils":
			category = categoryChanged
		default:
			category = categoryChanged
		}

		categorized[category] = append(categorized[category], c)
	}

	return categorized
}

func generateReleaseSection(version, date string, categorized map[string][]commit) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("## [%s] - %s\n\n", version, date))

	// Order of sections according to Keep a Changelog
	sections := []string{"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security"}

	for _, section := range sections {
		commits, ok := categorized[section]
		if !ok || len(commits) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("### %s\n\n", section))

		// Sort commits by hash for consistent ordering
		sort.Slice(commits, func(i, j int) bool {
			return commits[i].hash < commits[j].hash
		})

		for _, c := range commits {
			url := githubBaseURL + c.hash
			b.WriteString(fmt.Sprintf("* [%s](%s) %s\n", c.shortHash, url, c.message))
		}

		b.WriteString("\n")
	}

	return b.String()
}

func updateChangelog(changelogPath, newSection string) error {
	content, err := os.ReadFile(changelogPath) //nolint:gosec // G304: CHANGELOG.md path is a constant
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", changelogPath, err)
	}

	contentStr := string(content)

	// Find the position after the header (after "and this project adheres to...")
	// Insert the new section right after the header and before the first release
	headerEndPattern := regexp.MustCompile(`(?m)^## \[`)
	matches := headerEndPattern.FindStringIndex(contentStr)
	if len(matches) == 0 {
		return fmt.Errorf("could not find release section start in %s", changelogPath)
	}

	// Insert new section at the position of the first release
	before := contentStr[:matches[0]]
	after := contentStr[matches[0]:]
	newContent := before + newSection + after

	if err = os.WriteFile(changelogPath, []byte(newContent), constants.ConfigFilePermissions); err != nil {
		return fmt.Errorf("failed to write %s: %w", changelogPath, err)
	}

	return nil
}
