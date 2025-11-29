// Package main implements an AWS Lambda function that forwards CloudWatch Logs to Axiom.
// This is a standalone utility kept in the runvoy repository for convenience.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/runvoy/runvoy/internal/logger"
	awsOrchestrator "github.com/runvoy/runvoy/internal/providers/aws/orchestrator"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const (
	axiomAPIBaseURL = "https://api.axiom.co/v1"
	timeout         = 30 * time.Second
)

type config struct {
	token   string
	dataset string
	apiURL  string
	logger  *slog.Logger
}

func loadConfig() (*config, error) {
	token := os.Getenv("AXIOM_TOKEN")
	if token == "" {
		return nil, errors.New("AXIOM_TOKEN environment variable is required")
	}

	dataset := os.Getenv("AXIOM_DATASET")
	if dataset == "" {
		return nil, errors.New("AXIOM_DATASET environment variable is required")
	}

	apiURL := os.Getenv("AXIOM_API_URL")
	if apiURL == "" {
		apiURL = axiomAPIBaseURL
	}

	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	return &config{
		token:   token,
		dataset: dataset,
		apiURL:  apiURL,
		logger:  log,
	}, nil
}

type axiomEvent struct {
	Time    int64          `json:"_time"`
	Message string         `json:"message"`
	Log     map[string]any `json:"log,omitempty"`
}

// isLambdaMetadataLog checks if a log message is an AWS Lambda environment generated metadata log.
// These include START, REPORT, END, and XRAY TraceId messages.
func isLambdaMetadataLog(message string) bool {
	trimmed := strings.TrimSpace(message)
	return strings.HasPrefix(trimmed, "START RequestId:") ||
		strings.HasPrefix(trimmed, "REPORT RequestId:") ||
		strings.HasPrefix(trimmed, "END RequestId:") ||
		strings.HasPrefix(trimmed, "XRAY TraceId:")
}

func handleRequest(ctx context.Context, cfg *config, event events.CloudwatchLogsEvent) error {
	reqLogger := logger.DeriveRequestLogger(ctx, cfg.logger)

	if event.AWSLogs.Data == "" {
		reqLogger.Warn("received empty CloudWatch Logs data")
		return nil
	}

	data, err := event.AWSLogs.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse CloudWatch Logs data: %w", err)
	}

	reqLogger.Debug("processing CloudWatch logs event",
		"log_group", data.LogGroup,
		"log_stream", data.LogStream,
		"event_count", len(data.LogEvents),
	)

	axiomEvents := make([]axiomEvent, 0, len(data.LogEvents))
	filteredCount := 0
	for _, logEvent := range data.LogEvents {
		if isLambdaMetadataLog(logEvent.Message) {
			filteredCount++
			continue
		}

		axiomEvt := axiomEvent{
			Time:    logEvent.Timestamp,
			Message: logEvent.Message,
			Log: map[string]any{
				"log_group":  data.LogGroup,
				"log_stream": data.LogStream,
			},
		}
		axiomEvents = append(axiomEvents, axiomEvt)
	}

	if filteredCount > 0 {
		reqLogger.Debug("filtered Lambda metadata logs",
			"filtered_count", filteredCount,
		)
	}

	if sendErr := sendToAxiom(ctx, cfg, reqLogger, axiomEvents); sendErr != nil {
		return fmt.Errorf("failed to send logs to Axiom: %w", sendErr)
	}

	reqLogger.Debug("successfully forwarded logs to Axiom",
		"event_count", len(axiomEvents),
	)

	return nil
}

func sendToAxiom(ctx context.Context, cfg *config, reqLogger *slog.Logger, axiomEvents []axiomEvent) error {
	if len(axiomEvents) == 0 {
		return nil
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	for _, evt := range axiomEvents {
		if err := encoder.Encode(evt); err != nil {
			return fmt.Errorf("failed to encode event: %w", err)
		}
	}

	url := fmt.Sprintf("%s/datasets/%s/ingest", cfg.apiURL, cfg.dataset)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Content-Type", "application/x-ndjson")

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			reqLogger.Warn("failed to close response body", "error", closeErr)
		}
	}()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("axiom API returned error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

func main() {
	logger.RegisterContextExtractor(awsOrchestrator.NewLambdaContextExtractor())

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	handler := func(ctx context.Context, event events.CloudwatchLogsEvent) error {
		return handleRequest(ctx, cfg, event)
	}

	lambda.Start(handler)
}
