package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestHandleStats_IssuesExpectedQueries(t *testing.T) {
	t.Parallel()

	temporal := &fakeTemporal{
		counts: map[string]int64{},
	}
	rec := &recordingTemporal{fakeTemporal: temporal}
	h := newStatsHandler(rec)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	want := []string{
		queryImagesProcessed,
		queryImagesInFlight,
		queryBurstsLaunched,
	}
	seen := rec.queries()
	for _, q := range want {
		if !containsString(seen, q) {
			t.Errorf("missing query %q in %v", q, seen)
		}
	}
}

// recordingTemporal embeds *fakeTemporal (not client.Client directly)
// so the fake's embedded client.Client still satisfies the
// Dependencies.Temporal interface type. The CountWorkflow override
// here shadows the fake's when invoked through the recorder. The
// stats handler fans the three queries out across goroutines, so
// accesses to the slice are guarded by a mutex.
type recordingTemporal struct {
	*fakeTemporal

	mu   sync.Mutex
	seen []string
}

func (r *recordingTemporal) CountWorkflow(
	ctx context.Context,
	req *workflowservice.CountWorkflowExecutionsRequest,
) (*workflowservice.CountWorkflowExecutionsResponse, error) {
	r.mu.Lock()
	r.seen = append(r.seen, req.Query)
	r.mu.Unlock()
	return r.fakeTemporal.CountWorkflow(ctx, req)
}

func (r *recordingTemporal) queries() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.seen))
	copy(out, r.seen)
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
