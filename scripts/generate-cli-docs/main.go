// Package main provides a utility to generate a single markdown file documenting all CLI commands.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"runvoy/cmd/cli/cmd"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	var outFile string
	flag.StringVar(&outFile, "out", "./docs/CLI.md", "output file for generated markdown")
	flag.Parse()

	if outFile == "" {
		log.Fatal("error: output file is required")
	}

	if err := generateCLIDocs(outFile); err != nil {
		log.Fatalf("error: %s", err)
	}
}

func generateCLIDocs(outFile string) error {
	outDir := filepath.Dir(outFile)
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	file, err := os.Create(filepath.Clean(outFile))
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("warning: error closing file: %v", closeErr)
		}
	}()

	root := cmd.RootCmd()
	root.DisableAutoGenTag = true

	// Write header
	if _, headerErr := fmt.Fprintln(file, "# Runvoy CLI Documentation"); headerErr != nil {
		return fmt.Errorf("writing header: %w", headerErr)
	}

	introText := "\nComprehensive guide to all Runvoy CLI commands, options, and usage examples."
	if _, introErr := fmt.Fprintln(file, introText); introErr != nil {
		return fmt.Errorf("writing introduction: %w", introErr)
	}

	// Generate table of contents
	if tocErr := generateTableOfContents(root, file); tocErr != nil {
		return fmt.Errorf("generating table of contents: %w", tocErr)
	}

	// Generate documentation for all commands (starting at level 2 for root)
	docErr := generateDocs(root, file, 1, "")
	if docErr != nil {
		return fmt.Errorf("generating documentation: %w", docErr)
	}

	absPath, err := filepath.Abs(outFile)
	if err != nil {
		absPath = outFile
	}

	log.Printf("✅ Successfully generated CLI documentation in %s", absPath)
	return nil
}

func generateTableOfContents(cobraCmd *cobra.Command, file *os.File) error {
	if _, err := fmt.Fprintln(file, "\n## Commands"); err != nil {
		return fmt.Errorf("writing table of contents header: %w", err)
	}

	if _, err := fmt.Fprintln(file); err != nil {
		return fmt.Errorf("writing blank line: %w", err)
	}

	// Collect all commands recursively
	allCommands := collectCommands(cobraCmd, "")

	// Write TOC
	for _, cmd := range allCommands {
		if cmd.name == "" { // Skip root
			continue
		}
		indent := strings.Repeat("  ", cmd.depth-1)
		// Generate anchor from command path: lowercase and replace spaces with hyphens
		anchor := strings.ToLower(cmd.path)
		anchor = strings.ReplaceAll(anchor, " ", "-")
		if _, err := fmt.Fprintf(file, "%s- [%s](#%s)\n", indent, cmd.path, anchor); err != nil {
			return fmt.Errorf("writing toc entry: %w", err)
		}
	}

	return nil
}

type cmdInfo struct {
	name  string
	path  string
	depth int
	cmd   *cobra.Command
}

func collectCommands(cobraCmd *cobra.Command, _ string) []cmdInfo {
	var results []cmdInfo

	if cobraCmd.IsAvailableCommand() && !cobraCmd.IsAdditionalHelpTopicCommand() {
		path := cobraCmd.CommandPath()
		depth := len(strings.Fields(path))
		results = append(results, cmdInfo{
			name:  cobraCmd.Name(),
			path:  path,
			depth: depth,
			cmd:   cobraCmd,
		})
	}

	// Process subcommands
	subcommands := cobraCmd.Commands()
	if len(subcommands) > 0 {
		sort.Slice(subcommands, func(i, j int) bool {
			return subcommands[i].Name() < subcommands[j].Name()
		})

		for _, subCmd := range subcommands {
			results = append(results, collectCommands(subCmd, "")...)
		}
	}

	return results
}

func generateDocs(cobraCmd *cobra.Command, file *os.File, _ int, _ string) error {
	if !cobraCmd.IsAvailableCommand() || cobraCmd.IsAdditionalHelpTopicCommand() {
		return nil
	}

	commandPath := cobraCmd.CommandPath()
	// All commands are at level 2 (##), options are at level 3 (###)
	commandLevel := 2
	headingPrefix := strings.Repeat("#", commandLevel)

	if err := writeDocHeader(file, headingPrefix, commandPath, cobraCmd); err != nil {
		return err
	}

	// Extract and write options from Cobra (at level 3)
	if err := writeOptionsSection(file, cobraCmd, 3); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(file); err != nil {
		return fmt.Errorf("writing newline: %w", err)
	}

	// Process subcommands recursively
	subcommands := cobraCmd.Commands()
	if len(subcommands) > 0 {
		sort.Slice(subcommands, func(i, j int) bool {
			return subcommands[i].Name() < subcommands[j].Name()
		})

		for _, subCmd := range subcommands {
			if err := generateDocs(subCmd, file, commandLevel, commandPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeDocHeader(file *os.File, headingPrefix, commandPath string, cobraCmd *cobra.Command) error {
	// Write heading (anchor is auto-generated from heading text)
	if _, err := fmt.Fprintf(file, "%s %s\n\n", headingPrefix, commandPath); err != nil {
		return fmt.Errorf("writing heading: %w", err)
	}

	if cobraCmd.Short != "" {
		if _, err := fmt.Fprintf(file, "%s\n\n", cobraCmd.Short); err != nil {
			return fmt.Errorf("writing short description: %w", err)
		}
	}

	if cobraCmd.Long != "" && cobraCmd.Long != cobraCmd.Short {
		if _, err := fmt.Fprintf(file, "%s\n\n", cobraCmd.Long); err != nil {
			return fmt.Errorf("writing long description: %w", err)
		}
	}

	if cobraCmd.Example != "" {
		if _, err := fmt.Fprintf(file, "**Examples:**\n\n```bash\n%s\n```\n\n", cobraCmd.Example); err != nil {
			return fmt.Errorf("writing examples: %w", err)
		}
	}

	return nil
}

func writeOptionsSection(file *os.File, cobraCmd *cobra.Command, level int) error {
	// Generate markdown using Cobra's built-in function
	var buf bytes.Buffer
	if err := doc.GenMarkdown(cobraCmd, &buf); err != nil {
		return fmt.Errorf("generating markdown for options: %w", err)
	}

	markdown := buf.String()

	// Extract only the Options section (both local and inherited flags)
	if !strings.Contains(markdown, "### Options") {
		return nil
	}

	// Find the start of Options section
	start := strings.Index(markdown, "### Options")
	if start < 0 {
		return nil
	}

	optionsSection := markdown[start:]

	// Find the end of the Options section by looking for the next heading
	end := findSectionEnd(optionsSection)
	if end > 0 {
		optionsSection = optionsSection[:end]
	}

	// Replace the Options heading level with the correct level
	headingPrefix := strings.Repeat("#", level)
	optionsSection = strings.Replace(optionsSection, "### Options", headingPrefix+" Options", 1)

	// Write the extracted Options section
	if _, err := fmt.Fprint(file, optionsSection); err != nil {
		return fmt.Errorf("writing options: %w", err)
	}

	return nil
}

func findSectionEnd(section string) int {
	// Look for the next heading (###, ##, or #)
	patterns := []string{"\\n### See Also", "\\n## ", "\\n### "}

	minIdx := -1
	for _, pattern := range patterns {
		// Simple pattern matching without regex
		searchStr := strings.TrimPrefix(pattern, "\\n")
		if idx := strings.Index(section[1:], "\n"+searchStr); idx >= 0 {
			idx++ // account for the search starting at position 1
			if minIdx < 0 || idx < minIdx {
				minIdx = idx
			}
		}
	}

	return minIdx
}
