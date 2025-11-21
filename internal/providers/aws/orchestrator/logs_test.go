package orchestrator

import (
	"context"
	"errors"
	"testing"

	appErrors "runvoy/internal/errors"
	awsConstants "runvoy/internal/providers/aws/constants"
	"runvoy/internal/testutil"

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
