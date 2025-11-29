package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/runvoy/runvoy/internal/api"
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
	getLogEventsFunc func(
		ctx context.Context,
		params *cloudwatchlogs.GetLogEventsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.GetLogEventsOutput, error)
	startQueryFunc func(
		ctx context.Context,
		params *cloudwatchlogs.StartQueryInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.StartQueryOutput, error)
	getQueryResultsFunc func(
		ctx context.Context,
		params *cloudwatchlogs.GetQueryResultsInput,
		optFns ...func(*cloudwatchlogs.Options),
	) (*cloudwatchlogs.GetQueryResultsOutput, error)
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

func (m *mockCloudWatchLogsClient) GetLogEvents(
	_ context.Context,
	params *cloudwatchlogs.GetLogEventsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.GetLogEventsOutput, error) {
	if m.getLogEventsFunc != nil {
		return m.getLogEventsFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.GetLogEventsOutput{}, nil
}

func (m *mockCloudWatchLogsClient) StartQuery(
	_ context.Context,
	params *cloudwatchlogs.StartQueryInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.StartQueryOutput, error) {
	if m.startQueryFunc != nil {
		return m.startQueryFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.StartQueryOutput{}, nil
}

func (m *mockCloudWatchLogsClient) GetQueryResults(
	_ context.Context,
	params *cloudwatchlogs.GetQueryResultsInput,
	optFns ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	if m.getQueryResultsFunc != nil {
		return m.getQueryResultsFunc(context.Background(), params, optFns...)
	}
	return &cloudwatchlogs.GetQueryResultsOutput{}, nil
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
		assert.True(t, errors.As(err, &appErr))
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
		assert.True(t, errors.As(err, &appErr))
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
		expectedEvents := []cwlTypes.OutputLogEvent{
			{
				Timestamp: aws.Int64(1000),
				Message:   aws.String("message 1"),
			},
			{
				Timestamp: aws.Int64(2000),
				Message:   aws.String("message 2"),
			},
		}

		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: expectedEvents,
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.NoError(t, err)
		require.Len(t, events, 2)
		assert.Equal(t, int64(1000), events[0].Timestamp)
		assert.Equal(t, "message 1", events[0].Message)
		assert.Equal(t, int64(2000), events[1].Timestamp)
		assert.Equal(t, "message 2", events[1].Message)
	})

	t.Run("multiple pages of events", func(t *testing.T) {
		token1 := "next-token-1"
		token2 := "next-token-2"
		pageCount := 0

		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				pageCount++
				switch pageCount {
				case 1:
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{
								Timestamp: aws.Int64(1000),
								Message:   aws.String("message 1"),
							},
						},
						NextForwardToken: aws.String(token1),
					}, nil
				case 2:
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{
								Timestamp: aws.Int64(2000),
								Message:   aws.String("message 2"),
							},
						},
						NextForwardToken: aws.String(token2),
					}, nil
				default:
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{
								Timestamp: aws.Int64(3000),
								Message:   aws.String("message 3"),
							},
						},
						NextForwardToken: aws.String(token2), // Same as previous, should stop
					}, nil
				}
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.NoError(t, err)
		require.Len(t, events, 3)
		assert.Equal(t, int64(1000), events[0].Timestamp)
		assert.Equal(t, int64(2000), events[1].Timestamp)
		assert.Equal(t, int64(3000), events[2].Timestamp)
	})

	t.Run("empty log stream", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []cwlTypes.OutputLogEvent{},
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.NoError(t, err)
		assert.Len(t, events, 0)
	})

	t.Run("handles ResourceNotFoundException", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				return nil, &cwlTypes.ResourceNotFoundException{}
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.NoError(t, err)
		assert.Len(t, events, 0)
	})

	t.Run("other API errors are returned", func(t *testing.T) {
		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				return nil, errors.New("API error")
			},
		}

		_, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.Error(t, err)
		var appErr *appErrors.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
	})

	t.Run("pagination stops when no next token", func(t *testing.T) {
		pageCount := 0
		mock := &mockCloudWatchLogsClient{
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				pageCount++
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []cwlTypes.OutputLogEvent{
						{
							Timestamp: aws.Int64(int64(pageCount * 1000)),
							Message:   aws.String("message"),
						},
					},
					NextForwardToken: nil, // No next token, should stop
				}, nil
			},
		}

		events, err := getAllLogEvents(ctx, mock, logGroup, stream)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, 1, pageCount)
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

func TestMergeAndSortLogs(t *testing.T) {
	t.Run("merges and sorts logs correctly", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{Timestamp: 2000, Message: "runner message 2"},
			{Timestamp: 4000, Message: "runner message 4"},
		}
		sidecarEvents := []api.LogEvent{
			{Timestamp: 1000, Message: "sidecar message 1"},
			{Timestamp: 3000, Message: "sidecar message 3"},
			{Timestamp: 5000, Message: "sidecar message 5"},
		}

		result := mergeAndSortLogs(runnerEvents, sidecarEvents)
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
			{Timestamp: 1000, Message: "sidecar message"},
		}

		result := mergeAndSortLogs([]api.LogEvent{}, sidecarEvents)
		require.Len(t, result, 1)
		assert.Equal(t, sidecarEvents[0], result[0])
	})

	t.Run("handles empty sidecar logs", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{Timestamp: 1000, Message: "runner message"},
		}

		result := mergeAndSortLogs(runnerEvents, []api.LogEvent{})
		require.Len(t, result, 1)
		assert.Equal(t, runnerEvents[0], result[0])
	})

	t.Run("handles empty both logs", func(t *testing.T) {
		result := mergeAndSortLogs([]api.LogEvent{}, []api.LogEvent{})
		require.Len(t, result, 0)
	})

	t.Run("handles logs with equal timestamps", func(t *testing.T) {
		runnerEvents := []api.LogEvent{
			{Timestamp: 1000, Message: "runner message"},
		}
		sidecarEvents := []api.LogEvent{
			{Timestamp: 1000, Message: "sidecar message"},
		}

		result := mergeAndSortLogs(runnerEvents, sidecarEvents)
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
			getLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				callCount++
				streamName := aws.ToString(params.LogStreamName)
				switch streamName {
				case runnerStream:
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{Timestamp: aws.Int64(2000), Message: aws.String("runner log 1")},
							{Timestamp: aws.Int64(4000), Message: aws.String("runner log 2")},
						},
					}, nil
				case sidecarStream:
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{Timestamp: aws.Int64(1000), Message: aws.String("sidecar log 1")},
							{Timestamp: aws.Int64(3000), Message: aws.String("sidecar log 2")},
						},
					}, nil
				default:
					return &cloudwatchlogs.GetLogEventsOutput{}, nil
				}
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
		// Verify both streams were queried
		assert.GreaterOrEqual(t, callCount, 2)
	})

	t.Run("empty executionID returns error", func(t *testing.T) {
		manager := createLogManager(&mockCloudWatchLogsClient{})
		events, err := manager.FetchLogsByExecutionID(ctx, "")
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.True(t, errors.As(err, &appErr))
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
			assert.True(t, errors.As(err, &appErr))
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
			getLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				streamName := aws.ToString(params.LogStreamName)
				if streamName == runnerStream {
					return nil, errors.New("failed to fetch runner logs")
				}
				return &cloudwatchlogs.GetLogEventsOutput{}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.True(t, errors.As(err, &appErr))
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
			getLogEventsFunc: func(
				_ context.Context,
				params *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				streamName := aws.ToString(params.LogStreamName)
				if streamName == runnerStream {
					return &cloudwatchlogs.GetLogEventsOutput{
						Events: []cwlTypes.OutputLogEvent{
							{Timestamp: aws.Int64(1000), Message: aws.String("runner log")},
						},
					}, nil
				}
				if streamName == sidecarStream {
					return nil, errors.New("failed to fetch sidecar logs")
				}
				return &cloudwatchlogs.GetLogEventsOutput{}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.Error(t, err)
		assert.Nil(t, events)
		var appErr *appErrors.AppError
		assert.True(t, errors.As(err, &appErr))
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
			getLogEventsFunc: func(
				_ context.Context,
				_ *cloudwatchlogs.GetLogEventsInput,
				_ ...func(*cloudwatchlogs.Options),
			) (*cloudwatchlogs.GetLogEventsOutput, error) {
				return &cloudwatchlogs.GetLogEventsOutput{
					Events: []cwlTypes.OutputLogEvent{},
				}, nil
			},
		}

		manager := createLogManager(mock)
		events, err := manager.FetchLogsByExecutionID(ctx, executionID)
		require.NoError(t, err)
		assert.Len(t, events, 0)
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
			assert.True(t, errors.As(err, &appErr))
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
		require.Len(t, groups, 0)
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

func TestTransformBackendLogsResults(t *testing.T) {
	manager := &ObservabilityManagerImpl{}

	t.Run("parses JSON message timestamps", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{
			{
				{Field: aws.String("@timestamp"), Value: aws.String("2025-11-21T16:40:00Z")},
				{Field: aws.String("@message"), Value: aws.String(
					`{"time":"2025-11-21T16:40:01.123456789Z","level":"INFO","msg":"Request started"}`)},
			},
		}

		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 1)
		assert.Equal(t, `{"time":"2025-11-21T16:40:01.123456789Z","level":"INFO","msg":"Request started"}`, logs[0].Message)
		// Timestamp should come from message JSON, not CloudWatch @timestamp
		assert.Greater(t, logs[0].Timestamp, int64(0))
		// Verify it parsed the correct time (2025-11-21T16:40:01.123456789Z)
		expectedTime := time.Date(2025, 11, 21, 16, 40, 1, 123456789, time.UTC)
		assert.Equal(t, expectedTime.UnixMilli(), logs[0].Timestamp)
	})

	t.Run("falls back to CloudWatch timestamp for non-JSON messages", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{
			{
				{Field: aws.String("@timestamp"), Value: aws.String("2025-11-21T16:40:00.500Z")},
				{Field: aws.String("@message"), Value: aws.String("Plain text message")},
			},
		}

		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 1)
		assert.Equal(t, "Plain text message", logs[0].Message)
		// Timestamp should come from CloudWatch @timestamp
		expectedTime := time.Date(2025, 11, 21, 16, 40, 0, 500000000, time.UTC)
		assert.Equal(t, expectedTime.UnixMilli(), logs[0].Timestamp)
	})

	t.Run("handles empty results", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{}
		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 0)
	})

	t.Run("ignores unknown fields", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{
			{
				{Field: aws.String("@timestamp"), Value: aws.String("2025-11-21T16:40:00Z")},
				{Field: aws.String("@message"), Value: aws.String("Test message")},
				{Field: aws.String("custom_field"), Value: aws.String("ignored")},
			},
		}

		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 1)
		assert.Equal(t, "Test message", logs[0].Message)
	})

	t.Run("handles missing timestamp gracefully", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{
			{
				{Field: aws.String("@message"), Value: aws.String("Test message with no timestamp")},
			},
		}

		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 1)
		assert.Equal(t, "Test message with no timestamp", logs[0].Message)
		// Timestamp should be 0 when not available
		assert.Equal(t, int64(0), logs[0].Timestamp)
	})

	t.Run("handles JSON without time field", func(t *testing.T) {
		results := [][]cwlTypes.ResultField{
			{
				{Field: aws.String("@timestamp"), Value: aws.String("2025-11-21T16:40:00Z")},
				{Field: aws.String("@message"), Value: aws.String(`{"msg":"no time field"}`)},
			},
		}

		logs := manager.transformBackendLogsResults(results)
		require.Len(t, logs, 1)
		// Falls back to CloudWatch timestamp
		assert.Greater(t, logs[0].Timestamp, int64(0))
	})
}

type mockLogsClientOpts struct {
	describeErr       error
	noLogGroups       bool
	startQueryErr     error
	getResultsStatus  cwlTypes.QueryStatus
	getResultsErr     error
	getResultsResults [][]cwlTypes.ResultField
}

func createMockLogsClient(opts *mockLogsClientOpts) *mockCloudWatchLogsClient {
	return &mockCloudWatchLogsClient{
		describeLogGroupsFunc: func(
			_ context.Context,
			_ *cloudwatchlogs.DescribeLogGroupsInput,
			_ ...func(*cloudwatchlogs.Options),
		) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			if opts.describeErr != nil {
				return nil, opts.describeErr
			}
			if opts.noLogGroups {
				return &cloudwatchlogs.DescribeLogGroupsOutput{
					LogGroups: []cwlTypes.LogGroup{},
				}, nil
			}
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwlTypes.LogGroup{
					{LogGroupName: aws.String("/aws/lambda/runvoy-orchestrator")},
				},
			}, nil
		},
		startQueryFunc: func(
			_ context.Context,
			_ *cloudwatchlogs.StartQueryInput,
			_ ...func(*cloudwatchlogs.Options),
		) (*cloudwatchlogs.StartQueryOutput, error) {
			if opts.startQueryErr != nil {
				return nil, opts.startQueryErr
			}
			return &cloudwatchlogs.StartQueryOutput{
				QueryId: aws.String("query-123"),
			}, nil
		},
		getQueryResultsFunc: func(
			_ context.Context,
			_ *cloudwatchlogs.GetQueryResultsInput,
			_ ...func(*cloudwatchlogs.Options),
		) (*cloudwatchlogs.GetQueryResultsOutput, error) {
			if opts.getResultsErr != nil {
				return nil, opts.getResultsErr
			}
			return &cloudwatchlogs.GetQueryResultsOutput{
				Status:  opts.getResultsStatus,
				Results: opts.getResultsResults,
			}, nil
		},
	}
}

func TestFetchBackendLogs(t *testing.T) {
	ctx := context.Background()
	requestID := "aws-request-id-12345"

	// Use shorter delays for tests to speed up execution
	testManager := func(mock *mockCloudWatchLogsClient) *ObservabilityManagerImpl {
		return &ObservabilityManagerImpl{
			cwlClient:             mock,
			logger:                testutil.SilentLogger(),
			testQueryInitialDelay: 10 * time.Millisecond,
			testQueryPollInterval: 10 * time.Millisecond,
			testQueryMaxAttempts:  3,
		}
	}

	t.Run("successful query returns logs", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			getResultsStatus: cwlTypes.QueryStatusComplete,
			getResultsResults: [][]cwlTypes.ResultField{
				{
					{Field: aws.String("@timestamp"), Value: aws.String("2025-11-21T16:40:00Z")},
					{Field: aws.String("@message"), Value: aws.String("Log entry 1")},
				},
			},
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		assert.Equal(t, "Log entry 1", logs[0].Message)
	})

	t.Run("log group discovery fails", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			describeErr: errors.New("AWS API error"),
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("no log groups found", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			noLogGroups: true,
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("start query fails", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			startQueryErr: errors.New("failed to start query"),
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("query polling times out", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			getResultsStatus: cwlTypes.QueryStatusRunning,
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})

	t.Run("query fails with failed status", func(t *testing.T) {
		mock := createMockLogsClient(&mockLogsClientOpts{
			getResultsStatus: cwlTypes.QueryStatusFailed,
		})

		manager := testManager(mock)
		logs, err := manager.FetchBackendLogs(ctx, requestID)
		require.Error(t, err)
		assert.Nil(t, logs)
	})
}
