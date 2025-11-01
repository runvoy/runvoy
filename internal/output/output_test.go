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

func TestSpinner(_ *testing.T) {
	// This is a basic test - spinner behavior depends on terminal
	spinner := NewSpinner("Loading")
	spinner.Start()
	time.Sleep(100 * time.Millisecond)
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
