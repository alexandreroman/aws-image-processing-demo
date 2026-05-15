package api

import (
	"context"
	"net/http"

	"go.temporal.io/api/workflowservice/v1"
)

// statsWindowDays is the Temporal Cloud default retention. Surfacing
// it lets the frontend caption counter values honestly.
const statsWindowDays = 30

// Visibility queries powering the counters. Kept as package vars so
// tests can compare against the exact strings the handler issues.
var (
	queryImagesProcessed = `WorkflowType = "ProcessImage" AND ExecutionStatus = "Completed"`
	queryBurstsLaunched  = `WorkflowType = "LaunchPipelines" AND ExecutionStatus = "Completed"`
	queryImagesFailed    = `WorkflowType = "ProcessImage" AND ExecutionStatus = "Failed"`
)

// StatsResponse is the JSON payload of GET /api/stats. Counts are
// pulled live from Temporal Visibility; a -1 value means the count
// for that field could not be fetched (logged but not fatal).
type StatsResponse struct {
	ImagesProcessed int64 `json:"imagesProcessed"`
	ImagesFailed    int64 `json:"imagesFailed"`
	BurstsLaunched  int64 `json:"burstsLaunched"`
	WindowDays      int   `json:"windowDays"`
}

// statResult carries one count's value-or-error from a goroutine back
// to the handler. We use a per-result struct rather than errgroup so a
// single Visibility failure cannot poison the other count: each field
// is decided independently via collectStat.
type statResult struct {
	value int64
	err   error
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	run := func(query string, out chan<- statResult) {
		n, err := h.countWorkflows(ctx, query)
		out <- statResult{value: n, err: err}
	}

	processedCh := make(chan statResult, 1)
	failedCh := make(chan statResult, 1)
	burstsCh := make(chan statResult, 1)
	go run(queryImagesProcessed, processedCh)
	go run(queryImagesFailed, failedCh)
	go run(queryBurstsLaunched, burstsCh)

	resp := StatsResponse{WindowDays: statsWindowDays}
	resp.ImagesProcessed = h.collectStat(<-processedCh, queryImagesProcessed)
	resp.ImagesFailed = h.collectStat(<-failedCh, queryImagesFailed)
	resp.BurstsLaunched = h.collectStat(<-burstsCh, queryBurstsLaunched)

	// Browsers respect max-age; CloudFront and Cloudflare honor s-maxage and
	// serve stale during refresh / origin failure (stale-while-revalidate /
	// stale-if-error) so a Lambda hiccup never breaks the landing page.
	w.Header().Set(
		"Cache-Control",
		"public, max-age=5, s-maxage=15, stale-while-revalidate=30, stale-if-error=300",
	)
	writeJSON(w, http.StatusOK, resp)
}

// collectStat returns the count value, or the -1 sentinel after logging
// a warning when the underlying CountWorkflow call failed.
func (h *Handler) collectStat(r statResult, query string) int64 {
	if r.err != nil {
		h.deps.Logger.Warn("stats count failed", "query", query, "err", r.err)
		return -1
	}
	return r.value
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
