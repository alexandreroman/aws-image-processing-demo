package api

import (
	"context"
	"net/http"
	"sync"

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

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fan the three independent counts out in parallel; one Visibility hiccup
	// must not poison the other counters, so failures land as a -1 sentinel.
	queries := [3]string{queryImagesProcessed, queryImagesFailed, queryBurstsLaunched}
	var counts [3]int64
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func(i int, q string) {
			defer wg.Done()
			n, err := h.countWorkflows(ctx, q)
			if err != nil {
				h.deps.Logger.Warn("stats count failed", "query", q, "err", err)
				counts[i] = -1
				return
			}
			counts[i] = n
		}(i, q)
	}
	wg.Wait()

	// Browsers respect max-age; CloudFront and Cloudflare honor s-maxage and
	// serve stale during refresh / origin failure (stale-while-revalidate /
	// stale-if-error) so a Lambda hiccup never breaks the landing page.
	w.Header().Set(
		"Cache-Control",
		"public, max-age=5, s-maxage=15, stale-while-revalidate=30, stale-if-error=300",
	)
	writeJSON(w, http.StatusOK, StatsResponse{
		ImagesProcessed: counts[0],
		ImagesFailed:    counts[1],
		BurstsLaunched:  counts[2],
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
