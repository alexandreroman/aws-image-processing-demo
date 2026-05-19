package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	taskqueuepb "go.temporal.io/api/taskqueue/v1"
	"go.temporal.io/api/workflowservice/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

type fakeDescriber struct {
	// responses keyed by task queue type; missing key → empty Stats.
	responses map[enumspb.TaskQueueType]*taskqueuepb.TaskQueueStats
	err       error
}

func (f *fakeDescriber) DescribeTaskQueue(
	_ context.Context,
	req *workflowservice.DescribeTaskQueueRequest,
) (*workflowservice.DescribeTaskQueueResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &workflowservice.DescribeTaskQueueResponse{Stats: f.responses[req.TaskQueueType]}, nil
}

func TestAggregateBacklog_SumsAcrossQueueTypes(t *testing.T) {
	f := &fakeDescriber{responses: map[enumspb.TaskQueueType]*taskqueuepb.TaskQueueStats{
		enumspb.TASK_QUEUE_TYPE_WORKFLOW: {
			ApproximateBacklogCount: 7,
			ApproximateBacklogAge:   durationpb.New(2 * time.Second),
		},
		enumspb.TASK_QUEUE_TYPE_ACTIVITY: {
			ApproximateBacklogCount: 25,
			ApproximateBacklogAge:   durationpb.New(11 * time.Second),
		},
	}}

	b, err := aggregateBacklog(context.Background(), f, "ns", "tq")
	require.NoError(t, err)
	require.Equal(t, int64(32), b.count)
	require.Equal(t, 11*time.Second, b.age, "age aggregation is max, not sum")
}

func TestAggregateBacklog_HandlesNilStatsOnOneType(t *testing.T) {
	// Activity side has no stats yet — Server returns nil Stats. The
	// aggregation must not panic and must still report the workflow backlog.
	f := &fakeDescriber{responses: map[enumspb.TaskQueueType]*taskqueuepb.TaskQueueStats{
		enumspb.TASK_QUEUE_TYPE_WORKFLOW: {
			ApproximateBacklogCount: 4,
			ApproximateBacklogAge:   durationpb.New(500 * time.Millisecond),
		},
		// Activity intentionally omitted.
	}}

	b, err := aggregateBacklog(context.Background(), f, "ns", "tq")
	require.NoError(t, err)
	require.Equal(t, int64(4), b.count)
	require.Equal(t, 500*time.Millisecond, b.age)
}

func TestAggregateBacklog_ZeroBacklog(t *testing.T) {
	f := &fakeDescriber{responses: map[enumspb.TaskQueueType]*taskqueuepb.TaskQueueStats{
		enumspb.TASK_QUEUE_TYPE_WORKFLOW: {},
		enumspb.TASK_QUEUE_TYPE_ACTIVITY: {},
	}}

	b, err := aggregateBacklog(context.Background(), f, "ns", "tq")
	require.NoError(t, err)
	require.Zero(t, b.count)
	require.Zero(t, b.age)
}

func TestAggregateBacklog_PropagatesError(t *testing.T) {
	f := &fakeDescriber{err: errors.New("boom")}

	_, err := aggregateBacklog(context.Background(), f, "ns", "tq")
	require.Error(t, err)
}
