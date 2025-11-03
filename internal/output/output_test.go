package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSuccess(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Successf("operation completed")

	output := buf.String()
	if !strings.Contains(output, "operation completed") {
		t.Errorf("expected output to contain 'operation completed', got %q", output)
	}
}

func TestError(t *testing.T) {
	// Save original stdout (Errorf writes to Stdout, not Stderr)
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Errorf("something went wrong")

	output := buf.String()
	if !strings.Contains(output, "something went wrong") {
		t.Errorf("expected output to contain 'something went wrong', got %q", output)
	}
}

func TestKeyValue(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	KeyValue("key", "value")

	output := buf.String()
	if !strings.Contains(output, "key") || !strings.Contains(output, "value") {
		t.Errorf("expected output to contain 'key' and 'value', got %q", output)
	}
}

func TestStep(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Step(1, 3, "first step")

	output := buf.String()
	if !strings.Contains(output, "[1/3]") || !strings.Contains(output, "first step") {
		t.Errorf("expected output to contain '[1/3]' and 'first step', got %q", output)
	}
}

func TestTable(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
	}

	Table(headers, rows)

	output := buf.String()
	if !strings.Contains(output, "Name") ||
		!strings.Contains(output, "Value") ||
		!strings.Contains(output, "key1") ||
		!strings.Contains(output, "value1") {
		t.Errorf("table output missing expected content: %q", output)
	}
}

func TestList(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	items := []string{"item 1", "item 2", "item 3"}
	List(items)

	output := buf.String()
	for _, item := range items {
		if !strings.Contains(output, item) {
			t.Errorf("expected output to contain %q, got %q", item, output)
		}
	}
}

func TestNumberedList(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	items := []string{"first", "second", "third"}
	NumberedList(items)

	output := buf.String()
	if !strings.Contains(output, "1.") ||
		!strings.Contains(output, "2.") ||
		!strings.Contains(output, "3.") {
		t.Errorf("expected numbered list output, got %q", output)
	}
}

func TestStatusBadge(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"completed", "completed"},
		{"running", "running"},
		{"failed", "failed"},
		{"pending", "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := StatusBadge(tt.status)
			if !strings.Contains(result, tt.want) {
				t.Errorf("StatusBadge(%q) should contain %q, got %q", tt.status, tt.want, result)
			}
		})
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Duration(tt.duration)
			if got != tt.want {
				t.Errorf("Duration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1234567, "1.2 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := Bytes(tt.bytes)
			if got != tt.want {
				t.Errorf("Bytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	pb := NewProgressBar(10, "Testing")
	pb.Update(5)
	pb.Complete()

	output := buf.String()
	if !strings.Contains(output, "Testing") {
		t.Errorf("expected output to contain 'Testing', got %q", output)
	}
}

func TestProgressBarIncrement(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	pb := NewProgressBar(5, "Incrementing")

	// Test Increment method by incrementing a few times and completing
	pb.Increment()
	pb.Increment()
	pb.Complete() // This will trigger output

	// Verify output was produced
	output := buf.String()
	if !strings.Contains(output, "Incrementing") {
		t.Errorf("expected output to contain 'Incrementing', got %q", output)
	}
}

func TestProgressBarUpdateEdgeCases(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	pb := NewProgressBar(100, "Edge cases")

	// Test updates that trigger percentage output
	pb.Update(0)
	pb.Update(10)
	pb.Update(20)
	pb.Update(100)

	output := buf.String()
	if !strings.Contains(output, "Edge cases") {
		t.Errorf("expected output to contain 'Edge cases', got %q", output)
	}
}

func TestSpinner(_ *testing.T) {
	// This is a basic test - spinner behavior depends on terminal
	spinner := NewSpinner("Loading")
	spinner.Start()
	time.Sleep(100 * time.Millisecond)
	spinner.Stop()
	// If we get here without panic, test passes
}

func TestSpinnerSuccess(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	spinner := NewSpinner("Processing")
	spinner.Start()
	time.Sleep(50 * time.Millisecond)
	spinner.Success("Done!")

	output := buf.String()
	if !strings.Contains(output, "Done!") {
		t.Errorf("expected output to contain 'Done!', got %q", output)
	}
}

func TestSpinnerError(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	spinner := NewSpinner("Processing")
	spinner.Start()
	time.Sleep(50 * time.Millisecond)
	spinner.Error("Failed!")

	output := buf.String()
	if !strings.Contains(output, "Failed!") {
		t.Errorf("expected output to contain 'Failed!', got %q", output)
	}
}

func TestSpinnerStopWithoutStart(_ *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	spinner := NewSpinner("Test")
	// Stop without start should not panic
	spinner.Stop()
	// If we get here without panic, test passes
}

func TestBox(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Box("Test message")

	output := buf.String()
	if !strings.Contains(output, "Test message") {
		t.Errorf("expected output to contain 'Test message', got %q", output)
	}
	// Check for box borders
	if !strings.Contains(output, "╭") || !strings.Contains(output, "╰") {
		t.Errorf("expected output to contain box borders, got %q", output)
	}
}

func TestHeader(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Header("Test Header")

	output := buf.String()
	if !strings.Contains(output, "Test Header") {
		t.Errorf("expected output to contain 'Test Header', got %q", output)
	}
	// Check for separator
	if !strings.Contains(output, "━") {
		t.Errorf("expected output to contain separator, got %q", output)
	}
}

func TestSubheader(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Subheader("Test Subheader")

	output := buf.String()
	if !strings.Contains(output, "Test Subheader") {
		t.Errorf("expected output to contain 'Test Subheader', got %q", output)
	}
	// Check for separator
	if !strings.Contains(output, "─") {
		t.Errorf("expected output to contain separator, got %q", output)
	}
}

func TestColorFormatters(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"Bold", Bold},
		{"Cyan", Cyan},
		{"Gray", Gray},
		{"Green", Green},
		{"Red", Red},
		{"Yellow", Yellow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("test")
			// Just ensure it doesn't panic and returns something
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
		})
	}
}

func TestInfof(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Infof("test info message %s", "arg")

	output := buf.String()
	if !strings.Contains(output, "test info message arg") {
		t.Errorf("expected output to contain 'test info message arg', got %q", output)
	}
}

func TestWarningf(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Warningf("test warning %d", 123)

	output := buf.String()
	if !strings.Contains(output, "test warning 123") {
		t.Errorf("expected output to contain 'test warning 123', got %q", output)
	}
}

func TestStepSuccess(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	StepSuccess(2, 5, "completed step")

	output := buf.String()
	if !strings.Contains(output, "[2/5]") || !strings.Contains(output, "completed step") {
		t.Errorf("expected output to contain '[2/5]' and 'completed step', got %q", output)
	}
}

func TestStepError(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	StepError(3, 5, "failed step")

	output := buf.String()
	if !strings.Contains(output, "[3/5]") || !strings.Contains(output, "failed step") {
		t.Errorf("expected output to contain '[3/5]' and 'failed step', got %q", output)
	}
}

func TestKeyValueBold(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	KeyValueBold("API Key", "sk_test_123")

	output := buf.String()
	if !strings.Contains(output, "API Key") || !strings.Contains(output, "sk_test_123") {
		t.Errorf("expected output to contain 'API Key' and 'sk_test_123', got %q", output)
	}
}

func TestBlank(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Blank()

	output := buf.String()
	if output != "\n" {
		t.Errorf("expected output to be a newline, got %q", output)
	}
}

func TestPrintln(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Println("test", "message")

	output := buf.String()
	if !strings.Contains(output, "test") || !strings.Contains(output, "message") {
		t.Errorf("expected output to contain 'test' and 'message', got %q", output)
	}
}

func TestPrintf(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Printf("formatted: %s %d", "test", 42)

	output := buf.String()
	if output != "formatted: test 42" {
		t.Errorf("expected output to be 'formatted: test 42', got %q", output)
	}
}

func TestTableEmptyHeaders(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	// Test with empty headers - should return early
	Table([]string{}, [][]string{{"a", "b"}})

	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for empty headers, got %q", output)
	}
}

func TestTableWithMismatchedColumns(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	headers := []string{"Col1", "Col2", "Col3"}
	rows := [][]string{
		{"a", "b", "c", "d"}, // Extra column
		{"e", "f"},           // Missing column
	}

	Table(headers, rows)

	output := buf.String()
	if !strings.Contains(output, "Col1") {
		t.Errorf("expected output to contain headers, got %q", output)
	}
}

func TestStatusBadgeAllVariants(t *testing.T) {
	tests := []struct {
		status string
	}{
		{"completed"},
		{"success"},
		{"succeeded"},
		{"running"},
		{"in_progress"},
		{"starting"},
		{"failed"},
		{"error"},
		{"pending"},
		{"queued"},
		{"unknown_status"}, // Default case
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := StatusBadge(tt.status)
			if result == "" {
				t.Errorf("StatusBadge(%q) returned empty string", tt.status)
			}
		})
	}
}

func TestBoxMultiline(t *testing.T) {
	// Save original stdout
	oldStdout := Stdout
	defer func() { Stdout = oldStdout }()

	// Capture output
	buf := &bytes.Buffer{}
	Stdout = buf

	Box("Line 1\nLine 2\nLine 3")

	output := buf.String()
	if !strings.Contains(output, "Line 1") ||
		!strings.Contains(output, "Line 2") ||
		!strings.Contains(output, "Line 3") {
		t.Errorf("expected output to contain all lines, got %q", output)
	}
	// Check for box borders
	if !strings.Contains(output, "╭") || !strings.Contains(output, "╰") {
		t.Errorf("expected output to contain box borders, got %q", output)
	}
}

// Benchmark tests
func BenchmarkSuccess(b *testing.B) {
	oldStdout := Stdout
	Stdout = &bytes.Buffer{}
	defer func() { Stdout = oldStdout }()

	for i := 0; i < b.N; i++ {
		Successf("benchmark test")
	}
}

func BenchmarkTable(b *testing.B) {
	oldStdout := Stdout
	Stdout = &bytes.Buffer{}
	defer func() { Stdout = oldStdout }()

	headers := []string{"Col1", "Col2", "Col3"}
	rows := [][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
		{"g", "h", "i"},
	}

	for i := 0; i < b.N; i++ {
		Table(headers, rows)
	}
}

func BenchmarkStatusBadge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		StatusBadge("completed")
	}
}

func BenchmarkDuration(b *testing.B) {
	d := 125 * time.Second
	for i := 0; i < b.N; i++ {
		Duration(d)
	}
}

func BenchmarkBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Bytes(1234567)
	}
}
