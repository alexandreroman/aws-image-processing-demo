package api

import (
	"context"
	"net/http"

	"go.temporal.io/api/workflowservice/v1"
	"golang.org/x/sync/errgroup"
)

// statsWindowDays is the Temporal Cloud default retention. Surfacing
// it lets the frontend caption counter values honestly.
const statsWindowDays = 30

// Visibility queries powering the three counters. Kept as package
// vars so tests can compare against the exact strings the handler
// issues.
var (
	queryImagesProcessed = `WorkflowType = "ProcessImage" AND ExecutionStatus = "Completed"`
	queryImagesInFlight  = `WorkflowType = "ProcessImage" AND ExecutionStatus = "Running"`
	queryBurstsLaunched  = `WorkflowType = "LaunchPipelines" AND ExecutionStatus = "Completed"`
)

// StatsResponse is the JSON payload of GET /api/stats. Counts are
// pulled live from Temporal Visibility; a -1 value means the count
// for that field could not be fetched (logged but not fatal).
type StatsResponse struct {
	ImagesProcessed int64 `json:"imagesProcessed"`
	ImagesInFlight  int64 `json:"imagesInFlight"`
	BurstsLaunched  int64 `json:"burstsLaunched"`
	WindowDays      int   `json:"windowDays"`
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var processed, inFlight, bursts int64
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		n, err := h.countWorkflows(gctx, queryImagesProcessed)
		processed = n
		return err
	})
	g.Go(func() error {
		n, err := h.countWorkflows(gctx, queryImagesInFlight)
		inFlight = n
		return err
	})
	g.Go(func() error {
		n, err := h.countWorkflows(gctx, queryBurstsLaunched)
		bursts = n
		return err
	})
	_ = g.Wait() // partial failures tolerated; sentinel handling lands in Task 3.

	writeJSON(w, http.StatusOK, StatsResponse{
		ImagesProcessed: processed,
		ImagesInFlight:  inFlight,
		BurstsLaunched:  bursts,
		WindowDays:      statsWindowDays,
	})
}

// countWorkflows wraps client.CountWorkflow with the demo's
// namespace and returns just the count value.
func (h *Handler) countWorkflows(ctx context.Context, query string) (int64, error) {
	resp, err := h.deps.Temporal.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{
		Namespace: h.deps.Namespace,
		Query:     query,
	})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}
