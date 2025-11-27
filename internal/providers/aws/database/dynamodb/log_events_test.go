package dynamodb

import (
	"context"
	"strconv"
	"testing"
	"time"

	"runvoy/internal/api"
	"runvoy/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogEventRepository_DeleteLogEventsSetsTTL(t *testing.T) {
	ctx := context.Background()
	client := NewMockDynamoDBClient()
	repo := NewLogEventRepository(client, "log-events", testutil.SilentLogger())

	executionID := "exec-1"
	logEvents := []api.LogEvent{
		{EventID: "evt-1", Timestamp: 1_700_000_000, Message: "first"},
		{EventID: "evt-2", Timestamp: 1_700_000_005, Message: "second"},
	}

	require.NoError(t, repo.SaveLogEvents(ctx, executionID, logEvents))

	items := client.collectTableItems("log-events")
	require.Len(t, items, len(logEvents))

	for _, item := range items {
		_, hasTTL := item[logEventTTLAttribute]
		assert.False(t, hasTTL)
	}

	before := time.Now()
	require.NoError(t, repo.DeleteLogEvents(ctx, executionID))
	after := time.Now()

	items = client.collectTableItems("log-events")
	require.Len(t, items, len(logEvents))

	minTTL := before.Add(9 * time.Minute).Unix()
	maxTTL := after.Add(11 * time.Minute).Unix()

	for _, item := range items {
		ttlAttr, ok := item[logEventTTLAttribute]
		require.True(t, ok)

		ttlVal := getStringValue(ttlAttr)
		ttlInt, err := strconv.ParseInt(ttlVal, 10, 64)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, ttlInt, minTTL)
		assert.LessOrEqual(t, ttlInt, maxTTL)
	}
}

func TestLogEventRepository_ListLogEvents(t *testing.T) {
	ctx := context.Background()
	client := NewMockDynamoDBClient()
	repo := NewLogEventRepository(client, "log-events", testutil.SilentLogger())

	executionID := "exec-2"
	logEvents := []api.LogEvent{
		{EventID: "evt-1", Timestamp: 10, Message: "first"},
		{EventID: "evt-2", Timestamp: 20, Message: "second"},
		{EventID: "evt-3", Timestamp: 30, Message: "third"},
	}

	require.NoError(t, repo.SaveLogEvents(ctx, executionID, logEvents))

	fetched, err := repo.ListLogEvents(ctx, executionID)
	require.NoError(t, err)

	assert.Len(t, fetched, len(logEvents))
	for i, event := range fetched {
		assert.Equal(t, logEvents[i].EventID, event.EventID)
		assert.Equal(t, logEvents[i].Timestamp, event.Timestamp)
		assert.Equal(t, logEvents[i].Message, event.Message)
	}
}
