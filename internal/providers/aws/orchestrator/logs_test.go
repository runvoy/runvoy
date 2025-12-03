package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"
	"github.com/runvoy/runvoy/internal/constants"
	appErrors "github.com/runvoy/runvoy/internal/errors"
	awsConstants "github.com/runvoy/runvoy/internal/providers/aws/constants"
	"github.com/runvoy/runvoy/internal/testutil"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock CloudWatch Logs client for testing
type mockCloudWatchLogsClient struct {
	describeLogGroupsFunc func(
		ctx context.Context,
		params *cloudwatchlogs.DescribeLogGroupsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	describeLogStreamsFunc func(
		ctx context.Context,
		params *cloudwatchlogs.DescribeLogStreamsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	filterLogEventsFunc func(
		ctx context.Context,
		params *cloudwatchlogs.FilterLogEventsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

func (m *mockCloudWatchLogsClient) DescribeLogGroups(
	_ context.Context,
	params *cloudwatchlogs.DescribeLogGroupsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	if m.describeLogGroupsFunc != nil {
		return m.describeLogGroupsFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
}

func (m *mockCloudWatchLogsClient) DescribeLogStreams(
	_ context.Context,
	params *cloudwatchlogs.DescribeLogStreamsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	if m.describeLogStreamsFunc != nil {
		return m.describeLogStreamsFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.DescribeLogStreamsOutput{}, nil
}

func (m *mockCloudWatchLogsClient) FilterLogEvents(
	_ context.Context,
	params *cloudwatchlogs.FilterLogEventsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	if m.filterLogEventsFunc != nil {
		return m.filterLogEventsFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.FilterLogEventsOutput{}, nil
}

func TestVerifyLogStreamExists(t *testing.T) {
	ctx := context.Background()
	logGroup := "test-log-group"
	stream := "test-stream"
	executionID := "exec-123"

	t.Run("log stream exists", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{
							LogStreamName: aws.String(stream),
						},
					},
				}, nil
			},
		}

		err := verifyLogStreamExists(ctx, mock, logGroup, stream, executionID, testutil.SilentLogger())
		assert.NoError(t, err)
	})

	t.Run("log stream does not exist", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{},
				}, nil
			},
		}

		err := verifyLogStreamExists(ctx, mock, logGroup, stream, executionID, testutil.SilentLogger())
		require.Error(t, err)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "SERVICE_UNAVAILABLE", appErr.Code)
	})

	t.Run("describe log streams fails", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return nil, errors.New("AWS API error")
			},
		}

		err := verifyLogStreamExists(ctx, mock, logGroup, stream, executionID, testutil.SilentLogger())
		require.Error(t, err)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("multiple streams, correct one exists", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{
							LogStreamName: aws.String("other-stream-1"),
						},
						{
							LogStreamName: aws.String(stream),
						},
						{
							LogStreamName: aws.String("other-stream-2"),
						},
					},
				}, nil
			},
		}

		err := verifyLogStreamExists(ctx, mock, logGroup, stream, executionID, testutil.SilentLogger())
		assert.NoError(t, err)
	})
}

func TestGetAllLogEvents(t *testing.T) {
	ctx := context.Background()
	logGroup := "test-log-group"
	stream := "test-stream"

	t.Run("single page of events", func(t *testing.T) {
		expectedEvents := []cwlTypes.FilteredLogEvent{
			{
				EventId:       aws.String("event-id-1"),
				Timestamp:     aws.Int64(1000),
				Message:       aws.String("message 1"),
				LogStreamName: aws.String(stream),
			},
			{
				EventId:       aws.String("event-id-2"),
				Timestamp:     aws.Int64(2000),
				Message:       aws.String("message 2"),
				LogStreamName: aws.String(stream),
			},
		}

		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: expectedEvents,
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
		require.Len(t, events, 2)
		assert.Equal(t, "event-id-1", events[0].EventID)
		assert.Equal(t, int64(1000), events[0].Timestamp)
		assert.Equal(t, "message 1", events[0].Message)
		assert.Equal(t, "event-id-2", events[1].EventID)
		assert.Equal(t, int64(2000), events[1].Timestamp)
		assert.Equal(t, "message 2", events[1].Message)
	})

	t.Run("multiple pages of events", func(t *testing.T) {
		token1 := "next-token-1"
		token2 := "next-token-2"
		pageCount := 0

		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				pageCount++
				switch pageCount {
				case 1:
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								EventId:       aws.String("event-id-1"),
								Timestamp:     aws.Int64(1000),
								Message:       aws.String("message 1"),
								LogStreamName: aws.String(stream),
							},
						},
						NextToken: aws.String(token1),
					}, nil
				case 2:
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								EventId:       aws.String("event-id-2"),
								Timestamp:     aws.Int64(2000),
								Message:       aws.String("message 2"),
								LogStreamName: aws.String(stream),
							},
						},
						NextToken: aws.String(token2),
					}, nil
				default:
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								EventId:       aws.String("event-id-3"),
								Timestamp:     aws.Int64(3000),
								Message:       aws.String("message 3"),
								LogStreamName: aws.String(stream),
							},
						},
						NextToken: aws.String(token2), // Same as previous, should stop
					}, nil
				}
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
		require.Len(t, events, 3)
		assert.Equal(t, "event-id-1", events[0].EventID)
		assert.Equal(t, int64(1000), events[0].Timestamp)
		assert.Equal(t, "event-id-2", events[1].EventID)
		assert.Equal(t, int64(2000), events[1].Timestamp)
		assert.Equal(t, "event-id-3", events[2].EventID)
		assert.Equal(t, int64(3000), events[2].Timestamp)
	})

	t.Run("empty log stream", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []cwlTypes.FilteredLogEvent{},
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
		assert.Empty(t, events)
	})

	t.Run("handles ResourceNotFoundException", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return nil, &cwlTypes.ResourceNotFoundException{}
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
		assert.Empty(t, events)
	})

	t.Run("other API errors are returned", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return nil, errors.New("API error")
			},
		}

		_, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.Error(t, err)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("pagination stops when no next token", func(t *testing.T) {
		pageCount := 0
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				pageCount++
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []cwlTypes.FilteredLogEvent{
						{
							EventId:       aws.String(fmt.Sprintf("event-id-%d", pageCount)),
							Timestamp:     aws.Int64(int64(pageCount * 1000)),
							Message:       aws.String("message"),
							LogStreamName: aws.String(stream),
						},
					},
					NextToken: nil, // No next token, should stop
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, 1, pageCount)
	})

	t.Run("sets StartTime to 0 to fetch all logs from beginning", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				// Verify that StartTime is set to 0 (Unix epoch) to fetch all logs
				assert.NotNil(t, params.StartTime, "StartTime should be set")
				assert.Equal(t, int64(0), *params.StartTime, "StartTime should be 0 to fetch all logs from beginning")
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []cwlTypes.FilteredLogEvent{},
				}, nil
			},
		}

		_, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, 0, testutil.SilentLogger())
		require.NoError(t, err)
	})

	t.Run("respects custom startTime parameter", func(t *testing.T) {
		customStartTime := int64(1609459200000) // 2021-01-01 00:00:00 UTC in milliseconds
		mock := &mockCloudWatchLogsClient{
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				// Verify that StartTime is set to the custom value
				assert.NotNil(t, params.StartTime, "StartTime should be set")
				assert.Equal(t, customStartTime, *params.StartTime, "StartTime should match custom value")
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []cwlTypes.FilteredLogEvent{},
				}, nil
			},
		}

		_, err := getAllLogEvents(ctx, mock, logGroup, []string{stream}, customStartTime, testutil.SilentLogger())
		require.NoError(t, err)
	})
}

func TestBuildLogStreamName(t *testing.T) {
	// Test that BuildLogStreamName is used correctly in runner
	executionID := "test-exec-123"
	stream := awsConstants.BuildLogStreamName(executionID)
	assert.NotEmpty(t, stream)
	assert.Contains(t, stream, executionID)
}

func TestBuildSidecarLogStreamName(t *testing.T) {
	executionID := "test-exec-123"
	stream := buildSidecarLogStreamName(executionID)
	assert.Equal(t, "task/sidecar/test-exec-123", stream)
	assert.Contains(t, stream, executionID)
	assert.Contains(t, stream, awsConstants.SidecarContainerName)
}

// mergeAndSortLogs is a test helper that merges and sorts logs from two streams.
func mergeAndSortLogs(runnerEvents, sidecarEvents []api.LogEvent) []api.LogEvent {
	allEvents := make([]api.LogEvent, 0, len(runnerEvents)+len(sidecarEvents))
	allEvents = append(allEvents, runnerEvents...)
	allEvents = append(allEvents, sidecarEvents...)
	slices.SortFunc(allEvents, func(a, b api.LogEvent) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})
	return allEvents
}

func TestMergeAndSortLogs(t *testing.T) {
	t.Run("merges and sorts logs correctly", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{EventID: "runner-1", Timestamp: 2000, Message: "runner message 2"},
			{EventID: "runner-2", Timestamp: 4000, Message: "runner message 4"},
		}
		sidecarEvents := []api.LogEvent{
			{EventID: "sidecar-1", Timestamp: 1000, Message: "sidecar message 1"},
			{EventID: "sidecar-2", Timestamp: 3000, Message: "sidecar message 3"},
			{EventID: "sidecar-3", Timestamp: 5000, Message: "sidecar message 5"},
		}

		allEvents := make([]api.LogEvent, 0, len(runnerEvents)+len(sidecarEvents))
		allEvents = append(allEvents, runnerEvents...)
		allEvents = append(allEvents, sidecarEvents...)
		slices.SortFunc(allEvents, func(a, b api.LogEvent) int {
			if a.Timestamp < b.Timestamp {
				return -1
			}
			if a.Timestamp > b.Timestamp {
				return 1
			}
			return 0
		})
		result := allEvents
		require.Len(t, result, 5)
		assert.Equal(t, int64(1000), result[0].Timestamp)
		assert.Equal(t, "sidecar message 1", result[0].Message)
		assert.Equal(t, int64(2000), result[1].Timestamp)
		assert.Equal(t, "runner message 2", result[1].Message)
		assert.Equal(t, int64(3000), result[2].Timestamp)
		assert.Equal(t, "sidecar message 3", result[2].Message)
		assert.Equal(t, int64(4000), result[3].Timestamp)
		assert.Equal(t, "runner message 4", result[3].Message)
		assert.Equal(t, int64(5000), result[4].Timestamp)
		assert.Equal(t, "sidecar message 5", result[4].Message)
	})

	t.Run("handles empty runner logs", func(t *testing.T) {
		sidecarEvents := []api.LogEvent{
			{EventID: "sidecar-1", Timestamp: 1000, Message: "sidecar message"},
		}

		result := mergeAndSortLogs([]api.LogEvent{}, sidecarEvents)
		require.Len(t, result, 1)
		assert.Equal(t, sidecarEvents[0], result[0])
	})

	t.Run("handles empty sidecar logs", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{EventID: "runner-1", Timestamp: 1000, Message: "runner message"},
		}

		result := mergeAndSortLogs(runnerEvents, []api.LogEvent{})
		require.Len(t, result, 1)
		assert.Equal(t, runnerEvents[0], result[0])
	})

	t.Run("handles empty both logs", func(t *testing.T) {
		result := mergeAndSortLogs([]api.LogEvent{}, []api.LogEvent{})
		require.Empty(t, result)
	})

	t.Run("handles logs with equal timestamps", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{EventID: "runner-1", Timestamp: 1000, Message: "runner message"},
		}
		sidecarEvents := []api.LogEvent{
			{EventID: "sidecar-1", Timestamp: 1000, Message: "sidecar message"},
		}

		allEvents := make([]api.LogEvent, 0, len(runnerEvents)+len(sidecarEvents))
		allEvents = append(allEvents, runnerEvents...)
		allEvents = append(allEvents, sidecarEvents...)
		slices.SortFunc(allEvents, func(a, b api.LogEvent) int {
			if a.Timestamp < b.Timestamp {
				return -1
			}
			if a.Timestamp > b.Timestamp {
				return 1
			}
			return 0
		})
		result := allEvents
		require.Len(t, result, 2)
		// Both should be present, order may vary but both should exist
		messages := []string{result[0].Message, result[1].Message}
		assert.Contains(t, messages, "runner message")
		assert.Contains(t, messages, "sidecar message")
	})
}

func TestFetchLogsByExecutionID(t *testing.T) {
	ctx := context.Background()
	logGroup := "test-log-group"
	executionID := "exec-123"
	runnerStream := awsConstants.BuildLogStreamName(executionID)
	sidecarStream := buildSidecarLogStreamName(executionID)

	createLogManager := func(mock *mockCloudWatchLogsClient) *LogManagerImpl {
		return &LogManagerImpl{
			cwlClient: mock,
			cfg: &Config{
				LogGroup: logGroup,
			},
			logger: testutil.SilentLogger(),
		}
	}

	t.Run("successfully fetches and merges logs from both streams", func(t *testing.T) {
		callCount := 0
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				streamName := aws.ToString(params.LogStreamNamePrefix)
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{LogStreamName: aws.String(streamName)},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				callCount++
				// Check if both streams are requested in a single call
				hasRunner := false
				hasSidecar := false
				for _, streamName := range params.LogStreamNames {
					if streamName == runnerStream {
						hasRunner = true
					}
					if streamName == sidecarStream {
						hasSidecar = true
					}
				}

				// If both streams are requested, return events from both sorted by timestamp (AWS behavior)
				if hasRunner && hasSidecar {
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								EventId:       aws.String("sidecar-event-1"),
								Timestamp:     aws.Int64(1000),
								Message:       aws.String("sidecar log 1"),
								LogStreamName: aws.String(sidecarStream),
							},
							{
								EventId:       aws.String("runner-event-1"),
								Timestamp:     aws.Int64(2000),
								Message:       aws.String("runner log 1"),
								LogStreamName: aws.String(runnerStream),
							},
							{
								EventId:       aws.String("sidecar-event-2"),
								Timestamp:     aws.Int64(3000),
								Message:       aws.String("sidecar log 2"),
								LogStreamName: aws.String(sidecarStream),
							},
							{
								EventId:       aws.String("runner-event-2"),
								Timestamp:     aws.Int64(4000),
								Message:       aws.String("runner log 2"),
								LogStreamName: aws.String(runnerStream),
							},
						},
					}, nil
				}

				// Fallback for single stream (backward compatibility)
				if len(params.LogStreamNames) > 0 {
					streamName := params.LogStreamNames[0]
					switch streamName {
					case runnerStream:
						return &cloudwatchlogs.FilterLogEventsOutput{
							Events: []cwlTypes.FilteredLogEvent{
								{
									EventId:       aws.String("runner-event-1"),
									Timestamp:     aws.Int64(2000),
									Message:       aws.String("runner log 1"),
									LogStreamName: aws.String(runnerStream),
								},
								{
									EventId:       aws.String("runner-event-2"),
									Timestamp:     aws.Int64(4000),
									Message:       aws.String("runner log 2"),
									LogStreamName: aws.String(runnerStream),
								},
							},
						}, nil
					case sidecarStream:
						return &cloudwatchlogs.FilterLogEventsOutput{
							Events: []cwlTypes.FilteredLogEvent{
								{
									EventId:       aws.String("sidecar-event-1"),
									Timestamp:     aws.Int64(1000),
									Message:       aws.String("sidecar log 1"),
									LogStreamName: aws.String(sidecarStream),
								},
								{
									EventId:       aws.String("sidecar-event-2"),
									Timestamp:     aws.Int64(3000),
									Message:       aws.String("sidecar log 2"),
									LogStreamName: aws.String(sidecarStream),
								},
							},
						}, nil
					}
				}
				return &cloudwatchlogs.FilterLogEventsOutput{}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.NoError(t, err)
		require.Len(t, events, 4)
		// Verify logs are sorted by timestamp
		assert.Equal(t, int64(1000), events[0].Timestamp)
		assert.Equal(t, "sidecar log 1", events[0].Message)
		assert.Equal(t, int64(2000), events[1].Timestamp)
		assert.Equal(t, "runner log 1", events[1].Message)
		assert.Equal(t, int64(3000), events[2].Timestamp)
		assert.Equal(t, "sidecar log 2", events[2].Message)
		assert.Equal(t, int64(4000), events[3].Timestamp)
		assert.Equal(t, "runner log 2", events[3].Message)
		// Verify a single API call was made for both streams
		assert.Equal(t, 1, callCount)
	})

	t.Run("empty executionID returns error", func(t *testing.T) {
		manager := createLogManager(&mockCloudWatchLogsClient{})
		events, err := manager.FetchLogsByExecutionID(ctx, "")
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INVALID_REQUEST", appErr.Code)
	})

	for _, tc := range []struct {
		name        string
		emptyStream string
	}{
		{"runner stream does not exist", runnerStream},
		{"sidecar stream does not exist", sidecarStream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockCloudWatchLogsClient{
				describeLogStreamsFunc: func(
					_ context.Context,
					params *cloudwatchlogs.DescribeLogStreamsInput,
					_ ...func(*cloudwatchlogs.Options),
				) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
					streamName := aws.ToString(params.LogStreamNamePrefix)
					if streamName == tc.emptyStream {
						return &cloudwatchlogs.DescribeLogStreamsOutput{
							LogStreams: []cwlTypes.LogStream{},
						}, nil
					}
					return &cloudwatchlogs.DescribeLogStreamsOutput{
						LogStreams: []cwlTypes.LogStream{
							{LogStreamName: aws.String(streamName)},
						},
					}, nil
				},
			}

			manager := createLogManager(mock)
			events, err := manager.FetchLogsByExecutionID(ctx, executionID)
			require.Error(t, err)
			assert.Nil(t, events)
			var appErr *appErrors.AppError
			assert.ErrorAs(t, err, &appErr)
			assert.Equal(t, "SERVICE_UNAVAILABLE", appErr.Code)
		})
	}

	t.Run("error fetching runner logs", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{LogStreamName: aws.String(aws.ToString(params.LogStreamNamePrefix))},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				// Check if runner stream is in the request
				if slices.Contains(params.LogStreamNames, runnerStream) {
					return nil, errors.New("failed to fetch runner logs")
				}
				return &cloudwatchlogs.FilterLogEventsOutput{}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("error fetching sidecar logs", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{LogStreamName: aws.String(aws.ToString(params.LogStreamNamePrefix))},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				// Check if sidecar stream is in the request
				if slices.Contains(params.LogStreamNames, sidecarStream) {
					return nil, errors.New("failed to fetch sidecar logs")
				}
				// Return runner events if only runner stream is requested
				if slices.Contains(params.LogStreamNames, runnerStream) {
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								EventId:       aws.String("runner-event-1"),
								Timestamp:     aws.Int64(1000),
								Message:       aws.String("runner log"),
								LogStreamName: aws.String(runnerStream),
							},
						},
					}, nil
				}
				return &cloudwatchlogs.FilterLogEventsOutput{}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.ErrorAs(t, err, &appErr)
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("empty logs from both streams", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogStreamsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.DescribeLogStreamsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
				return &cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []cwlTypes.LogStream{
						{LogStreamName: aws.String(aws.ToString(params.LogStreamNamePrefix))},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return &cloudwatchlogs.FilterLogEventsOutput{
					Events: []cwlTypes.FilteredLogEvent{},
				}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.NoError(t, err)
		assert.Empty(t, events)
	})

	for _, tc := range []struct {
		name        string
		errorStream string
	}{
		{"error verifying runner stream", runnerStream},
		{"error verifying sidecar stream", sidecarStream},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockCloudWatchLogsClient{
				describeLogStreamsFunc: func(
					_ context.Context,
					params *cloudwatchlogs.DescribeLogStreamsInput,
					_ ...func(*cloudwatchlogs.Options),
				) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
					streamName := aws.ToString(params.LogStreamNamePrefix)
					if streamName == tc.errorStream {
						return nil, errors.New("AWS API error")
					}
					return &cloudwatchlogs.DescribeLogStreamsOutput{
						LogStreams: []cwlTypes.LogStream{
							{LogStreamName: aws.String(streamName)},
						},
					}, nil
				},
			}

			manager := createLogManager(mock)
			events, err := manager.FetchLogsByExecutionID(ctx, executionID)
			require.Error(t, err)
			assert.Nil(t, events)
			var appErr *appErrors.AppError
			assert.ErrorAs(t, err, &appErr)
			assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
		})
	}
}

func TestDiscoverLogGroups(t *testing.T) {
	ctx := context.Background()

	t.Run("single page of log groups", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{
						{LogGroupName: aws.String("/aws/lambda/runvoy-orchestrator")},
						{LogGroupName: aws.String("/aws/lambda/runvoy-processor")},
					},
				}, nil
			},
		}

		manager := &ObservabilityManagerImpl{cwlClient: mock, logger: testutil.SilentLogger()}
		groups, err := manager.discoverLogGroups(ctx, testutil.SilentLogger())
		require.NoError(t, err)
		require.Len(t, groups, 2)
		assert.Contains(t, groups, "/aws/lambda/runvoy-orchestrator")
		assert.Contains(t, groups, "/aws/lambda/runvoy-processor")
	})

	t.Run("multiple pages of log groups", func(t *testing.T) {
		pageCount := 0
		token1 := "next-token-1"
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				pageCount++
				switch pageCount {
				case 1:
					return &cloudwatchlogs.DescribeLogGroupsOutput{
						LogGroups: []cwlTypes.LogGroup{
							{LogGroupName: aws.String("/aws/lambda/runvoy-orchestrator")},
						},
						NextToken: aws.String(token1),
					}, nil
				default:
					return &cloudwatchlogs.DescribeLogGroupsOutput{
						LogGroups: []cwlTypes.LogGroup{
							{LogGroupName: aws.String("/aws/lambda/runvoy-processor")},
						},
					}, nil
				}
			},
		}

		manager := &ObservabilityManagerImpl{cwlClient: mock, logger: testutil.SilentLogger()}
		groups, err := manager.discoverLogGroups(ctx, testutil.SilentLogger())
		require.NoError(t, err)
		require.Len(t, groups, 2)
		assert.Equal(t, 2, pageCount)
	})

	t.Run("no log groups found", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{},
				}, nil
			},
		}

		manager := &ObservabilityManagerImpl{cwlClient: mock, logger: testutil.SilentLogger()}
		groups, err := manager.discoverLogGroups(ctx, testutil.SilentLogger())
		require.NoError(t, err)
		require.Empty(t, groups)
	})

	t.Run("describe log groups fails", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return nil, errors.New("AWS API error")
			},
		}

		manager := &ObservabilityManagerImpl{cwlClient: mock, logger: testutil.SilentLogger()}
		groups, err := manager.discoverLogGroups(ctx, testutil.SilentLogger())
		require.Error(t, err)
		assert.Nil(t, groups)
	})
}

func TestFetchBackendLogs(t *testing.T) {
	ctx := context.Background()
	requestID := "aws-request-id-12345"
	fixedNow := time.Date(2025, time.December, 1, 12, 0, 0, 0, time.UTC)
	startMillis := fixedNow.Add(-awsConstants.CloudWatchLogsObservabilityLookback).UnixMilli()
	endMillis := fixedNow.UnixMilli()

	testManager := func(mock *mockCloudWatchLogsClient) *ObservabilityManagerImpl {
		return &ObservabilityManagerImpl{
			cwlClient: mock,
			logger:    testutil.SilentLogger(),
			nowFn: func() time.Time {
				return fixedNow
			},
		}
	}

	t.Run("successful fetch returns sorted events from all groups", func(t *testing.T) {
		groupA := "/aws/lambda/runvoy-orchestrator"
		groupB := "/aws/lambda/runvoy-processor"
		expectedPattern := fmt.Sprintf("{ $.%s = %q }", constants.RequestIDLogField, requestID)
		paginated := false
		firstTimestamp := time.Date(2025, time.November, 21, 16, 40, 1, 123456789, time.UTC).UnixMilli()
		secondTimestamp := firstTimestamp + 1000
		thirdTimestamp := firstTimestamp + 2000
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{
						{LogGroupName: aws.String(groupA)},
						{LogGroupName: aws.String(groupB)},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				assert.Equal(t, expectedPattern, aws.ToString(params.FilterPattern))
				assert.Equal(t, startMillis, aws.ToInt64(params.StartTime))
				assert.Equal(t, endMillis, aws.ToInt64(params.EndTime))

				switch aws.ToString(params.LogGroupName) {
				case groupA:
					if params.NextToken == nil {
						paginated = true
						return &cloudwatchlogs.FilterLogEventsOutput{
							Events: []cwlTypes.FilteredLogEvent{
								{
									Message:   aws.String(`{"time":"2025-11-21T16:40:01.123456789Z","msg":"groupA page1"}`),
									Timestamp: aws.Int64(0), // overwritten by message timestamp
								},
							},
							NextToken: aws.String("page-2"),
						}, nil
					}
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								Message:   aws.String("groupA page2"),
								Timestamp: aws.Int64(secondTimestamp),
							},
						},
					}, nil
				case groupB:
					return &cloudwatchlogs.FilterLogEventsOutput{
						Events: []cwlTypes.FilteredLogEvent{
							{
								Message:   aws.String("groupB event"),
								Timestamp: aws.Int64(thirdTimestamp),
							},
						},
					}, nil
				default:
					return nil, fmt.Errorf("unexpected log group: %s", aws.ToString(params.LogGroupName))
				}
			},
		}

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.NoError(t, err)
		require.True(t, paginated, "expected pagination logic to run")
		require.Len(t, logs, 3)

		assert.Equal(t, firstTimestamp, logs[0].Timestamp)
		assert.Contains(t, logs[0].Message, "groupA page1")
		assert.Equal(t, secondTimestamp, logs[1].Timestamp)
		assert.Equal(t, "groupA page2", logs[1].Message)
		assert.Equal(t, thirdTimestamp, logs[2].Timestamp)
		assert.Equal(t, "groupB event", logs[2].Message)
	})

	t.Run("log group discovery fails", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return nil, errors.New("AWS API error")
			},
		}

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("no log groups found", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{},
				}, nil
			},
		}

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("filter log events fails", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			describeLogGroupsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.DescribeLogGroupsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{
						{LogGroupName: aws.String("/aws/lambda/runvoy-orchestrator")},
					},
				}, nil
			},
			filterLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.FilterLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.FilterLogEventsOutput, error) {
				return nil, errors.New("FilterLogEvents error")
			},
		}

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})
}
