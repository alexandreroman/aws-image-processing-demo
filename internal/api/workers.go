package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	enumspb "go.temporal.io/api/enums/v1"
)

// workersResponse is the JSON payload of
// GET /api/pipelines/{pipelineId}/workers. The count is the number of
// distinct worker Identity values observed across ActivityTaskStarted
// events of the pipeline's launcher and its child workflows — i.e. how
// many physical workers actually picked up work for this burst (~1 for
// ECS, N for Lambda).
type workersResponse struct {
	WorkerCount int `json:"workerCount"`
}

// maxConcurrentHistoryFetches caps fan-out so a 50-image burst does not
// fire 51 simultaneous gRPC calls into Temporal Cloud per poll.
const maxConcurrentHistoryFetches = 8

func (h *Handler) handlePipelineWorkers(w http.ResponseWriter, r *http.Request) {
	pipelineID := r.PathValue("pipelineId")
	if pipelineID == "" {
		writeError(w, http.StatusBadRequest, "pipelineId is required")
		return
	}

	childIDs, err := h.fetchPipelineWorkflowIDs(r.Context(), pipelineID)
	if err != nil {
		h.deps.Logger.Error("fetch launcher result failed",
			"pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError,
			"failed to read pipeline: "+err.Error())
		return
	}

	launcherID := fmt.Sprintf("image-pipeline-%s", pipelineID)
	allIDs := make([]string, 0, len(childIDs)+1)
	allIDs = append(allIDs, launcherID)
	allIDs = append(allIDs, childIDs...)

	identities, err := h.collectWorkerIdentities(r.Context(), allIDs)
	if err != nil {
		h.deps.Logger.Error("collect worker identities failed",
			"pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError,
			"failed to read pipeline workers: "+err.Error())
		return
	}

	// Short cache: the value only drifts as new activities start, so a 2 s
	// browser cache is harmless and shields the backend from accidental
	// poll storms.
	w.Header().Set(
		"Cache-Control",
		"public, max-age=2, s-maxage=5, stale-while-revalidate=30",
	)
	writeJSON(w, http.StatusOK, workersResponse{WorkerCount: len(identities)})
}

// collectWorkerIdentities fans out per-workflow history scans, capped by
// a small semaphore so we never flood Temporal Cloud with simultaneous
// gRPC calls. Per-workflow errors are best-effort: they are logged and
// skipped so a single bad history can't return 0 for the whole pipeline.
func (h *Handler) collectWorkerIdentities(
	ctx context.Context, workflowIDs []string,
) (map[string]struct{}, error) {
	identities := make(map[string]struct{})
	if len(workflowIDs) == 0 {
		return identities, nil
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, maxConcurrentHistoryFetches)
	)
	for _, id := range workflowIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(workflowID string) {
			defer wg.Done()
			defer func() { <-sem }()

			seen, err := h.workflowActivityIdentities(ctx, workflowID)
			if err != nil {
				h.deps.Logger.Warn("read workflow history failed",
					"workflowId", workflowID, "err", err)
				return
			}
			if len(seen) == 0 {
				return
			}
			mu.Lock()
			for identity := range seen {
				identities[identity] = struct{}{}
			}
			mu.Unlock()
		}(id)
	}
	wg.Wait()

	return identities, nil
}

// workflowActivityIdentities walks one workflow's history and returns
// the set of Identity values seen on ActivityTaskStarted events. The
// iterator is bounded (isLongPoll=false) so it returns as soon as the
// currently visible history is drained, even for running workflows.
func (h *Handler) workflowActivityIdentities(
	ctx context.Context, workflowID string,
) (map[string]struct{}, error) {
	iter := h.deps.Temporal.GetWorkflowHistory(
		ctx, workflowID, "", false,
		enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT,
	)
	out := make(map[string]struct{})
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return nil, err
		}
		if event.GetEventType() != enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED {
			continue
		}
		attrs := event.GetActivityTaskStartedEventAttributes()
		if attrs == nil {
			continue
		}
		if id := attrs.GetIdentity(); id != "" {
			out[id] = struct{}{}
		}
	}
	return out, nil
}
