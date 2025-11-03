package cmd

import "runvoy/internal/output"

// OutputInterface defines the interface for output operations to enable dependency injection and testing
type OutputInterface interface {
	Infof(format string, a ...interface{})
	Errorf(format string, a ...interface{})
	Successf(format string, a ...interface{})
	Warningf(format string, a ...interface{})
	Table(headers []string, rows [][]string)
	Blank()
	Bold(text string) string
	Cyan(text string) string
	KeyValue(key, value string)
	Prompt(prompt string) string
}

// outputWrapper wraps the global output package functions to implement OutputInterface
type outputWrapper struct{}

// NewOutputWrapper creates a new output wrapper that implements OutputInterface
func NewOutputWrapper() OutputInterface {
	return &outputWrapper{}
}

func (o *outputWrapper) Infof(format string, a ...interface{}) {
	output.Infof(format, a...)
}

func (o *outputWrapper) Errorf(format string, a ...interface{}) {
	output.Errorf(format, a...)
}

func (o *outputWrapper) Successf(format string, a ...interface{}) {
	output.Successf(format, a...)
}

func (o *outputWrapper) Warningf(format string, a ...interface{}) {
	output.Warningf(format, a...)
}

func (o *outputWrapper) Table(headers []string, rows [][]string) {
	output.Table(headers, rows)
}

func (o *outputWrapper) Blank() {
	output.Blank()
}

func (o *outputWrapper) Bold(text string) string {
	return output.Bold(text)
}

func (o *outputWrapper) Cyan(text string) string {
	return output.Cyan(text)
}

func (o *outputWrapper) KeyValue(key, value string) {
	output.KeyValue(key, value)
}

func (o *outputWrapper) Prompt(prompt string) string {
	return output.Prompt(prompt)
}
