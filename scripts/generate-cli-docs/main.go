// Package main provides a utility to generate a single markdown file documenting all CLI commands.
package main

import (
	"bytes"
	"errors"
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

	if _, headerErr := fmt.Fprintln(file, "# Runvoy CLI Documentation"); headerErr != nil {
		return fmt.Errorf("writing header: %w", headerErr)
	}

	introText := "\nThis document contains all available CLI commands, their descriptions, "
	introText += "flags, and examples."
	if _, introErr := fmt.Fprintln(file, introText); introErr != nil {
		return fmt.Errorf("writing introduction: %w", introErr)
	}

	docErr := generateDocs(root, file, 2)
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

func generateDocs(cobraCmd *cobra.Command, file *os.File, level int) error {
	if !cobraCmd.IsAvailableCommand() || cobraCmd.IsAdditionalHelpTopicCommand() {
		return nil
	}

	commandPath := cobraCmd.CommandPath()
	headingPrefix := strings.Repeat("#", level)

	if err := writeDocHeader(file, headingPrefix, commandPath, cobraCmd); err != nil {
		return err
	}

	// Generate markdown using Cobra's built-in function
	var buf bytes.Buffer
	if err := doc.GenMarkdown(cobraCmd, &buf); err != nil {
		return fmt.Errorf("generating markdown for %s: %w", commandPath, err)
	}

	markdown := buf.String()

	// Extract and write the Options section from the generated markdown
	if err := writeOptionsSection(file, markdown); err != nil {
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
			if err := generateDocs(subCmd, file, level+1); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeDocHeader(file *os.File, headingPrefix, commandPath string, cobraCmd *cobra.Command) error {
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

func writeOptionsSection(file *os.File, markdown string) error {
	if !strings.Contains(markdown, "### Options") {
		return nil
	}

	start := strings.Index(markdown, "### Options")
	if start < 0 {
		return errors.New("invalid markdown index")
	}

	optionsSection := markdown[start:]

	// Find the end of the Options section
	end := findOptionsSectionEnd(optionsSection)
	if end > 0 {
		optionsSection = optionsSection[:end]
	}

	if optionsSection != "" {
		if _, err := fmt.Fprint(file, optionsSection); err != nil {
			return fmt.Errorf("writing options: %w", err)
		}
		if _, err := fmt.Fprintln(file); err != nil {
			return fmt.Errorf("writing newline after options: %w", err)
		}
	}

	return nil
}

func findOptionsSectionEnd(optionsSection string) int {
	// Find the end of the Options section
	end := strings.Index(optionsSection, "\n\n\n### ")
	if end > 0 {
		return end
	}

	end = strings.Index(optionsSection, "\n\n## ")
	if end > 0 {
		return end
	}

	// If no clear end marker, look for the next heading at same or higher level
	end = strings.Index(optionsSection, "\n\n### See Also")
	if end > 0 {
		return end
	}

	return 0
}
