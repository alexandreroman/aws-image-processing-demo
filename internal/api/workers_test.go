package api

import (
	"context"
	"errors"
	"testing"

	historypb "go.temporal.io/api/history/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// historyTemporal is a fakeTemporal extension that serves canned
// history iterators per workflow ID. Workflows in `errs` return a
// failing iterator on the first Next() call.
type historyTemporal struct {
	client.Client
	histories map[string][]*historypb.HistoryEvent
	errs      map[string]error
}

func (f *historyTemporal) GetWorkflowHistory(
	_ context.Context, workflowID, _ string, _ bool,
	_ enumspb.HistoryEventFilterType,
) client.HistoryEventIterator {
	return &fakeHistoryIterator{
		events: f.histories[workflowID],
		err:    f.errs[workflowID],
	}
}

type fakeHistoryIterator struct {
	events []*historypb.HistoryEvent
	err    error
	idx    int
}

func (it *fakeHistoryIterator) HasNext() bool {
	if it.err != nil {
		return true
	}
	return it.idx < len(it.events)
}

func (it *fakeHistoryIterator) Next() (*historypb.HistoryEvent, error) {
	if it.err != nil {
		err := it.err
		it.err = nil
		return nil, err
	}
	e := it.events[it.idx]
	it.idx++
	return e, nil
}

func activityStartedEvent(identity string) *historypb.HistoryEvent {
	return &historypb.HistoryEvent{
		EventType: enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED,
		Attributes: &historypb.HistoryEvent_ActivityTaskStartedEventAttributes{
			ActivityTaskStartedEventAttributes: &historypb.ActivityTaskStartedEventAttributes{
				Identity: identity,
			},
		},
	}
}

func TestCollectWorkerIdentities_CountsDistinct(t *testing.T) {
	t.Parallel()

	temporal := &historyTemporal{histories: map[string][]*historypb.HistoryEvent{
		"wf-a": {
			activityStartedEvent("worker-1"),
			activityStartedEvent("worker-2"),
			activityStartedEvent("worker-1"), // dup within a workflow
		},
		"wf-b": {
			activityStartedEvent("worker-2"), // dup across workflows
			activityStartedEvent("worker-3"),
		},
	}}
	h := New(Dependencies{Temporal: temporal})

	got, err := h.collectWorkerIdentities(t.Context(), []string{"wf-a", "wf-b"})
	if err != nil {
		t.Fatalf("collectWorkerIdentities: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("count: got %d, want 3 (identities=%v)", len(got), got)
	}
}

func TestCollectWorkerIdentities_PerWorkflowErrorIsSwallowed(t *testing.T) {
	t.Parallel()

	temporal := &historyTemporal{
		histories: map[string][]*historypb.HistoryEvent{
			"wf-ok": {
				activityStartedEvent("worker-1"),
				activityStartedEvent("worker-2"),
			},
		},
		errs: map[string]error{"wf-bad": errors.New("history unreachable")},
	}
	h := New(Dependencies{Temporal: temporal})

	got, err := h.collectWorkerIdentities(t.Context(), []string{"wf-ok", "wf-bad"})
	if err != nil {
		t.Fatalf("collectWorkerIdentities: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("count: got %d, want 2 (identities=%v)", len(got), got)
	}
}
