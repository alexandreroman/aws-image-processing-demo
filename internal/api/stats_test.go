package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// fakeTemporal lets tests stub Client.CountWorkflow without
// implementing the full client.Client interface. The embedded nil
// interface satisfies the type at compile time; any other method
// called on it panics, which is exactly what we want for tests that
// only exercise the count path.
type fakeTemporal struct {
	client.Client
	counts map[string]int64
	errs   map[string]error
}

func (f *fakeTemporal) CountWorkflow(
	_ context.Context,
	req *workflowservice.CountWorkflowExecutionsRequest,
) (*workflowservice.CountWorkflowExecutionsResponse, error) {
	if err, ok := f.errs[req.Query]; ok && err != nil {
		return nil, err
	}
	return &workflowservice.CountWorkflowExecutionsResponse{
		Count: f.counts[req.Query],
	}, nil
}

func newStatsHandler(temporal client.Client) *Handler {
	return New(Dependencies{Temporal: temporal})
}

func TestHandleStats_HappyPath(t *testing.T) {
	t.Parallel()

	temporal := &fakeTemporal{counts: map[string]int64{
		`WorkflowType = "ProcessImage" AND ExecutionStatus = "Completed"`:    1234,
		`WorkflowType = "ProcessImage" AND ExecutionStatus = "Running"`:      7,
		`WorkflowType = "LaunchPipelines" AND ExecutionStatus = "Completed"`: 42,
	}}
	h := newStatsHandler(temporal)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	var got StatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	want := StatsResponse{
		ImagesProcessed: 1234,
		ImagesInFlight:  7,
		BurstsLaunched:  42,
		WindowDays:      30,
	}
	if got != want {
		t.Fatalf("response: got %+v, want %+v", got, want)
	}
}
