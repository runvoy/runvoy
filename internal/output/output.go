package output

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	// Colors and styles
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
	gray   = color.New(color.FgHiBlack)
	bold   = color.New(color.Bold)
	blue   = color.New(color.FgBlue)

	// Output writers (can be overridden for testing)
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr

	// Disable colors if not TTY or NO_COLOR is set
	noColor = os.Getenv("NO_COLOR") != "" || !isTerminal(os.Stdout)
)

func init() {
	if noColor {
		color.NoColor = true
	}
}

// Success prints a success message with a checkmark
// Example: ✓ Stack created successfully
func Success(format string, a ...interface{}) {
	fmt.Fprintf(Stdout, green.Sprint("✓")+" "+format+"\n", a...)
}

// Info prints an informational message with an arrow
// Example: → Creating CloudFormation stack...
func Info(format string, a ...interface{}) {
	fmt.Fprintf(Stdout, cyan.Sprint("→")+" "+format+"\n", a...)
}

// Warning prints a warning message with a warning symbol
// Example: ⚠ Lock already held by alice@acme.com
func Warning(format string, a ...interface{}) {
	fmt.Fprintf(Stdout, yellow.Sprint("⚠")+" "+format+"\n", a...)
}

// Error prints an error message with an X symbol
// Example: ✗ Failed to create stack: permission denied
func Error(format string, a ...interface{}) {
	fmt.Fprintf(Stdout, red.Sprint("✗")+" "+format+"\n", a...)
}

// Fatal prints an error message and exits with code 1
func Fatal(format string, a ...interface{}) {
	Error(format, a...)
	os.Exit(1)
}

// Step prints a step in a multi-step process
// Example: [1/3] Waiting for stack creation
func Step(step int, total int, message string) {
	gray.Fprintf(Stdout, "[%d/%d] ", step, total)
	fmt.Fprintln(Stdout, message)
}

// StepSuccess prints a successful step completion
// Example: [1/3] ✓ Stack created
func StepSuccess(step int, total int, message string) {
	gray.Fprintf(Stdout, "[%d/%d] ", step, total)
	fmt.Fprintf(Stdout, "%s %s\n", green.Sprint("✓"), message)
}

// StepError prints a failed step
// Example: [2/3] ✗ Failed to generate API key
func StepError(step int, total int, message string) {
	gray.Fprintf(Stdout, "[%d/%d] ", step, total)
	fmt.Fprintf(Stdout, "%s %s\n", red.Sprint("✗"), message)
}

// Header prints a section header with a separator line
// Example:
// 🚀 Initializing runvoy infrastructure
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
func Header(text string) {
	fmt.Fprintln(Stdout)
	fmt.Fprintln(Stdout, bold.Sprint(text))
	fmt.Fprintln(Stdout, gray.Sprint(strings.Repeat("━", 50)))
}

// Subheader prints a smaller section header
// Example:
// Configuration Details
// ────────────────────
func Subheader(text string) {
	fmt.Fprintln(Stdout)
	fmt.Fprintln(Stdout, cyan.Sprint(text))
	fmt.Fprintln(Stdout, gray.Sprint(strings.Repeat("─", len(text))))
}

// KeyValue prints a key-value pair with indentation
// Example:   Stack name: runvoy
func KeyValue(key, value string) {
	fmt.Fprintf(Stdout, "  %s: %s\n", gray.Sprint(key), value)
}

// KeyValueBold prints a key-value pair with bold value
// Example:   API Key: sk_live_abc123...
func KeyValueBold(key, value string) {
	fmt.Fprintf(Stdout, "  %s: %s\n", gray.Sprint(key), bold.Sprint(value))
}

// Blank prints a blank line
func Blank() {
	fmt.Fprintln(Stdout)
}

// Println prints a plain line without any formatting
func Println(a ...interface{}) {
	fmt.Fprintln(Stdout, a...)
}

// Printf prints a formatted plain line
func Printf(format string, a ...interface{}) {
	fmt.Fprintf(Stdout, format, a...)
}

// Bold prints text in bold
func Bold(text string) string {
	return bold.Sprint(text)
}

// Cyan prints text in cyan
func Cyan(text string) string {
	return cyan.Sprint(text)
}

// Gray prints text in gray
func Gray(text string) string {
	return gray.Sprint(text)
}

// Green prints text in green
func Green(text string) string {
	return green.Sprint(text)
}

// Red prints text in red
func Red(text string) string {
	return red.Sprint(text)
}

// Yellow prints text in yellow
func Yellow(text string) string {
	return yellow.Sprint(text)
}

// Box prints text in a rounded box
// Example:
// ╭─────────────────────────────╮
// │  Configuration saved!       │
// ╰─────────────────────────────╯
func Box(text string) {
	lines := strings.Split(text, "\n")
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	// Top border
	fmt.Fprintln(Stdout, gray.Sprint("╭─"+strings.Repeat("─", maxLen+2)+"─╮"))

	// Content
	for _, line := range lines {
		padding := strings.Repeat(" ", maxLen-len(line))
		fmt.Fprintf(Stdout, "%s  %s%s  %s\n",
			gray.Sprint("│"),
			line,
			padding,
			gray.Sprint("│"))
	}

	// Bottom border
	fmt.Fprintln(Stdout, gray.Sprint("╰─"+strings.Repeat("─", maxLen+2)+"─╯"))
}

// Table prints a simple table with headers
// Example:
// Execution ID    Status      Duration
// ────────────    ──────      ────────
// exec_abc123     completed   10m 35s
// exec_def456     running     2m 15s
func Table(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, h := range headers {
		fmt.Fprintf(Stdout, "%-*s  ", widths[i], bold.Sprint(h))
	}
	fmt.Fprintln(Stdout)

	// Print separator
	for i := range headers {
		fmt.Fprintf(Stdout, "%s  ", gray.Sprint(strings.Repeat("─", widths[i])))
	}
	fmt.Fprintln(Stdout)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Fprintf(Stdout, "%-*s  ", widths[i], cell)
			}
		}
		fmt.Fprintln(Stdout)
	}
}

// List prints a bulleted list
// Example:
//   - Item one
//   - Item two
//   - Item three
func List(items []string) {
	for _, item := range items {
		fmt.Fprintf(Stdout, "  %s %s\n", cyan.Sprint("•"), item)
	}
}

// NumberedList prints a numbered list
// Example:
//  1. First step
//  2. Second step
//  3. Third step
func NumberedList(items []string) {
	for i, item := range items {
		fmt.Fprintf(Stdout, "  %s %s\n", gray.Sprintf("%d.", i+1), item)
	}
}

// Spinner represents a simple text spinner for long operations
type Spinner struct {
	message string
	frames  []string
	frame   int
	done    chan bool
	running bool
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:    make(chan bool),
	}
}

// Start starts the spinner animation
func (s *Spinner) Start() {
	if noColor || !isTerminal(os.Stdout) {
		// If not a TTY, just print the message once
		Info(s.message)
		return
	}

	s.running = true
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				frame := s.frames[s.frame%len(s.frames)]
				fmt.Fprintf(Stdout, "\r%s %s", cyan.Sprint(frame), s.message)
				s.frame++
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	if !s.running {
		return
	}
	s.done <- true
	fmt.Fprint(Stdout, "\r"+strings.Repeat(" ", len(s.message)+10)+"\r")
}

// Success stops the spinner and prints a success message
func (s *Spinner) Success(message string) {
	s.Stop()
	Success(message)
}

// Error stops the spinner and prints an error message
func (s *Spinner) Error(message string) {
	s.Stop()
	Error(message)
}

// ProgressBar represents a simple text progress bar
type ProgressBar struct {
	total   int
	current int
	width   int
	message string
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, message string) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		width:   40,
		message: message,
	}
}

// Update updates the progress bar to the given value
func (p *ProgressBar) Update(current int) {
	if noColor || !isTerminal(os.Stdout) {
		// Simple percentage output for non-TTY
		if current%10 == 0 || current == p.total {
			fmt.Fprintf(Stdout, "\r%s... %d%%", p.message, (current*100)/p.total)
		}
		if current == p.total {
			fmt.Fprintln(Stdout)
		}
		return
	}

	p.current = current
	percent := float64(current) / float64(p.total)
	filled := int(percent * float64(p.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	fmt.Fprintf(Stdout, "\r%s %s %3.0f%%",
		p.message,
		cyan.Sprint(bar),
		percent*100)

	if current == p.total {
		fmt.Fprintln(Stdout)
	}
}

// Increment increments the progress bar by 1
func (p *ProgressBar) Increment() {
	p.Update(p.current + 1)
}

// Complete marks the progress bar as complete
func (p *ProgressBar) Complete() {
	p.Update(p.total)
}

// Confirm prompts the user for yes/no confirmation
// Returns true if user confirms (y/Y), false otherwise
func Confirm(prompt string) bool {
	fmt.Fprintf(Stdout, "%s [y/N]: ", yellow.Sprint("?")+" "+prompt)

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// Prompt prompts the user for input
func Prompt(prompt string) string {
	fmt.Fprintf(Stdout, "%s: ", cyan.Sprint("?")+" "+prompt)

	var response string
	fmt.Scanln(&response)

	return strings.TrimSpace(response)
}

// PromptRequired prompts the user for input and requires a non-empty response
func PromptRequired(prompt string) string {
	for {
		response := Prompt(prompt)
		if response != "" {
			return response
		}
		Warning("This field is required")
	}
}

// PromptSecret prompts for sensitive input (like passwords)
// Note: This is a simple implementation. For production, consider using
// golang.org/x/term for proper terminal handling
func PromptSecret(prompt string) string {
	fmt.Fprintf(Stdout, "%s: ", cyan.Sprint("?")+" "+prompt)

	var response string
	fmt.Scanln(&response)

	return strings.TrimSpace(response)
}

// StatusBadge prints a colored status badge
func StatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "completed", "success", "succeeded":
		return green.Sprint("● " + status)
	case "running", "in_progress", "starting":
		return yellow.Sprint("● " + status)
	case "failed", "error":
		return red.Sprint("● " + status)
	case "pending", "queued":
		return gray.Sprint("● " + status)
	default:
		return cyan.Sprint("● " + status)
	}
}

// Duration formats a duration in a human-readable way
func Duration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// Bytes formats bytes in a human-readable way
func Bytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// isTerminal checks if the writer is a terminal
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fileInfo, _ := f.Stat()
		return (fileInfo.Mode() & os.ModeCharDevice) != 0
	}
	return false
}
