// Package api exposes the HTTP handlers backing the demo's REST endpoints.
//
// All routes live under /api/* so CloudFront can dispatch by path
// (api → API Gateway, everything else → S3 frontend) without CORS
// gymnastics. /healthz at the root is the deliberate exception so
// container orchestrators can probe it directly.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// Runtime identifies one worker deployment + its Temporal task queue.
// "ecs" and "lambda" are the two runtimes the demo provisions; the backend
// is configured at startup with the queue name each one listens on.
//
// TaskQueue is intentionally not serialized: the browser only needs the
// runtime name to render a picker and echo it back in the start request.
type Runtime struct {
	Name      string `json:"name"`
	TaskQueue string `json:"-"`
}

// defaultTaskQueue is the queue used when no per-runtime queue is configured
// (local dev / compose). The deployed backend always populates Runtimes so it
// never falls back to this value.
const defaultTaskQueue = "image-processing"

// Dependencies holds the runtime collaborators of the API. The struct is
// the seam used both by main (production) and tests.
type Dependencies struct {
	Temporal     client.Client
	Dynamo       *dynamodb.Client
	ImagesBucket string
	ImagesTable  string
	// Runtimes lists available worker deployments in display order. The first
	// entry is the default when /api/workflows/start omits the runtime field.
	// When empty (local dev / compose), the handler falls back to
	// defaultTaskQueue and the response omits the runtime field so the
	// frontend can hide the selector.
	Runtimes  []Runtime
	Namespace string
	Logger    *slog.Logger
}

// Handler implements http.Handler. Build it once at startup; it is safe for
// concurrent use.
type Handler struct {
	deps Dependencies
	mux  *http.ServeMux
}

// New builds the API handler with all routes registered under /api/*.
func New(deps Dependencies) *Handler {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Namespace == "" {
		deps.Namespace = client.DefaultNamespace
	}
	h := &Handler{deps: deps, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /api/workflows/start", h.handleStart)
	h.mux.HandleFunc("GET /api/pipelines/{pipelineId}", h.handlePipeline)
	h.mux.HandleFunc("GET /api/pipelines/{pipelineId}/workers", h.handlePipelineWorkers)
	h.mux.HandleFunc("GET /api/runtimes", h.handleRuntimes)
	h.mux.HandleFunc("GET /healthz", h.handleHealth)
	h.mux.HandleFunc("GET /api/stats", h.handleStats)
	return h
}

// allowedOrigin returns the value of Access-Control-Allow-Origin to advertise.
// Production should set ALLOWED_ORIGIN=https://<your-cloudfront-domain> so the
// API does not advertise itself to arbitrary origins; local dev defaults to
// "*" so the Nuxt dev server on :3000 can talk to the backend on :8000.
func allowedOrigin() string {
	if v := os.Getenv("ALLOWED_ORIGIN"); v != "" {
		return v
	}
	return "*"
}

// ServeHTTP applies CORS for all requests, short-circuits preflights, then
// dispatches to the mux.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin())
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.mux.ServeHTTP(w, r)
}

// --- /healthz ---------------------------------------------------------------

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- /api/runtimes ----------------------------------------------------------

func (h *Handler) handleRuntimes(w http.ResponseWriter, _ *http.Request) {
	// Always emit a JSON array, never null, so the frontend can iterate
	// unconditionally even when no runtimes are configured.
	out := h.deps.Runtimes
	if out == nil {
		out = []Runtime{}
	}
	writeJSON(w, http.StatusOK, out)
}

// --- /api/workflows/start ---------------------------------------------------

// maxBurst caps the number of images one /api/workflows/start call may
// schedule. Keeps a single burst within reasonable Temporal Cloud / API
// Gateway boundaries.
const maxBurst = 50

type startRequest struct {
	Images  []manifest.S3Ref `json:"images"`
	Runtime string           `json:"runtime"`
}

type startResponse struct {
	PipelineID  string   `json:"pipelineId"`
	WorkflowIDs []string `json:"workflowIds"`
	Runtime     string   `json:"runtime,omitempty"`
}

// handleStart routes a burst to the task queue of the selected runtime and
// returns as soon as the launcher workflow has been enqueued on the Temporal
// frontend — it does NOT wait for the launcher to fan out the per-image
// children. This keeps the HTTP response (and the frontend redirect) snappy
// even on large bursts; the fan-out happens in the background and the
// pipeline page polls for state as the children appear.
//
// The response includes the per-image workflow IDs, which the handler
// derives locally via manifest.ProcessImageWorkflowID — the same helper the
// launcher uses — so the frontend can size its UI before the first poll
// returns without waiting on the launcher.
//
// When no runtimes are configured (local dev / compose), the handler falls
// back to defaultTaskQueue and omits the runtime field from the response so
// the frontend can hide the selector.
func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if len(req.Images) == 0 {
		writeError(w, http.StatusBadRequest, "images must not be empty")
		return
	}
	// Cap the burst at the same limit as presign so a caller cannot bypass
	// it by signing URLs elsewhere and posting a larger batch here.
	if len(req.Images) > maxBurst {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("images: at most %d allowed, got %d", maxBurst, len(req.Images)))
		return
	}
	// Pin each S3Ref to the prefixes the demo actually serves (curated
	// samples). Without this, a caller could queue workflows against
	// arbitrary objects in the images bucket.
	for i, img := range req.Images {
		if !strings.HasPrefix(img.Key, "samples/") {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("images[%d].key: must start with samples/", i))
			return
		}
	}

	var (
		taskQueue   = defaultTaskQueue
		runtimeName string
	)
	if len(h.deps.Runtimes) > 0 {
		runtime, ok := h.resolveRuntime(req.Runtime)
		if !ok {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("runtime %q is not configured; allowed values: %s",
					req.Runtime, strings.Join(h.runtimeNames(), ", ")))
			return
		}
		taskQueue = runtime.TaskQueue
		runtimeName = runtime.Name
	}

	pipelineID := shortID()

	images := make([]manifest.LaunchPipelineImage, len(req.Images))
	workflowIDs := make([]string, len(req.Images))
	for i, img := range req.Images {
		imageID := shortID()
		images[i] = manifest.LaunchPipelineImage{ImageID: imageID, Original: img}
		workflowIDs[i] = manifest.ProcessImageWorkflowID(pipelineID, imageID)
	}

	// Fire the launcher and return immediately — we don't wait for fan-out
	// to complete, so the frontend can redirect as soon as Temporal has
	// accepted the start.
	//
	// The launcher's ID is `image-pipeline-{id}` with no trailing
	// `-{imageId}` segment, so it stays out of any per-image listing
	// that filters on the `image-pipeline-{id}-` prefix.
	opts := client.StartWorkflowOptions{
		ID:                    fmt.Sprintf("image-pipeline-%s", pipelineID),
		TaskQueue:             taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	in := manifest.LaunchPipelinesInput{PipelineID: pipelineID, Images: images}
	if _, err := h.deps.Temporal.ExecuteWorkflow(r.Context(), opts, workflows.LaunchPipelines, in); err != nil {
		h.deps.Logger.Error("start launcher failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, startResponse{
		PipelineID:  pipelineID,
		WorkflowIDs: workflowIDs,
		Runtime:     runtimeName,
	})
}

// resolveRuntime picks the runtime for a start request. An empty name falls
// back to the first configured runtime; any other unknown name fails.
func (h *Handler) resolveRuntime(name string) (Runtime, bool) {
	if name == "" {
		return h.deps.Runtimes[0], true
	}
	for _, rt := range h.deps.Runtimes {
		if rt.Name == name {
			return rt, true
		}
	}
	return Runtime{}, false
}

func (h *Handler) runtimeNames() []string {
	names := make([]string, len(h.deps.Runtimes))
	for i, rt := range h.deps.Runtimes {
		names[i] = rt.Name
	}
	return names
}

// shortID returns the first 8 hex chars of a UUID v4. 32 bits is ample for a
// few dozen images per pipeline; WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
// catches the vanishingly rare collision.
func shortID() string {
	return uuid.NewString()[:8]
}

// --- /api/pipelines/{pipelineId} --------------------------------------------

type pipelineWorkflow struct {
	WorkflowID      string             `json:"workflowId"`
	ImageID         string             `json:"imageId,omitempty"`
	Status          string             `json:"status"`
	CurrentActivity string             `json:"currentActivity,omitempty"`
	StartedAt       *time.Time         `json:"startedAt,omitempty"`
	CompletedAt     *time.Time         `json:"completedAt,omitempty"`
	Manifest        *manifest.Manifest `json:"manifest,omitempty"`
}

type pipelineSummary struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type pipelineResponse struct {
	PipelineID  string             `json:"pipelineId"`
	CreatedAt   time.Time          `json:"createdAt,omitempty"`
	CompletedAt *time.Time         `json:"completedAt,omitempty"`
	DurationMs  *int64             `json:"durationMs,omitempty"`
	ImageCount  int                `json:"imageCount"`
	Summary     pipelineSummary    `json:"summary"`
	Workflows   []pipelineWorkflow `json:"workflows"`
}

func (h *Handler) handlePipeline(w http.ResponseWriter, r *http.Request) {
	pipelineID := r.PathValue("pipelineId")
	if pipelineID == "" {
		writeError(w, http.StatusBadRequest, "pipelineId is required")
		return
	}

	manifests, err := h.fetchManifests(r.Context(), pipelineID)
	if err != nil {
		h.deps.Logger.Error("fetch manifests failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to read pipeline: "+err.Error())
		return
	}

	workflowIDs, err := h.fetchPipelineWorkflowIDs(r.Context(), pipelineID)
	if err != nil {
		h.deps.Logger.Error("fetch launcher result failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to read pipeline: "+err.Error())
		return
	}

	executions, err := h.listWorkflows(r.Context(), workflowIDs)
	if err != nil {
		h.deps.Logger.Error("list workflows failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list workflows: "+err.Error())
		return
	}

	resp := pipelineResponse{
		PipelineID: pipelineID,
		ImageCount: len(workflowIDs),
		Workflows:  make([]pipelineWorkflow, 0, len(workflowIDs)),
	}

	// currentActivity and manifest-query lookups each make a Temporal RPC per
	// running workflow. Cap the per-poll fan-out (shared across both features)
	// so a 48-workflow burst does not stall the 1 s frontend poll loop.
	currentActivityLookups := 0
	manifestQueryLookups := 0
	seen := make(map[string]bool, len(executions))
	var latestClose time.Time
	for _, exec := range executions {
		wf := pipelineWorkflow{
			WorkflowID: exec.GetExecution().GetWorkflowId(),
			Status:     statusName(exec.GetStatus()),
		}
		seen[wf.WorkflowID] = true
		if t := exec.GetStartTime(); t != nil {
			started := t.AsTime()
			wf.StartedAt = &started
			if resp.CreatedAt.IsZero() || started.Before(resp.CreatedAt) {
				resp.CreatedAt = started
			}
		}
		if t := exec.GetCloseTime(); t != nil {
			closed := t.AsTime()
			wf.CompletedAt = &closed
			if closed.After(latestClose) {
				latestClose = closed
			}
		}

		// DynamoDB is authoritative once the final StoreManifest write lands,
		// so prefer it over the in-flight query result below.
		ddbManifest, hasDDBManifest := manifests[wf.WorkflowID]
		if hasDDBManifest {
			wf.ImageID = ddbManifest.ImageID
			wf.Manifest = ddbManifest
		}

		switch exec.GetStatus() {
		case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
			resp.Summary.Completed++
		case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED,
			enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED,
			enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT,
			enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
			resp.Summary.Failed++
		case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING,
			enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
			resp.Summary.Running++
			// Best effort: surface the currently scheduled activity so the
			// frontend can show "GenerateDescription…" mid-flight.
			if currentActivityLookups < maxLookupsPerPoll {
				currentActivityLookups++
				if act := h.currentActivity(r.Context(), wf.WorkflowID); act != "" {
					wf.CurrentActivity = act
				}
			}
			// Best effort: surface the in-flight manifest so the gallery can
			// show resized images before watermarking finishes. Skip when DDB
			// already has the final manifest to avoid wasting lookup budget.
			if !hasDDBManifest && manifestQueryLookups < maxLookupsPerPoll {
				manifestQueryLookups++
				if m := h.queryManifest(r.Context(), wf.WorkflowID); m != nil {
					wf.ImageID = m.ImageID
					wf.Manifest = m
				}
			}
		}

		resp.Workflows = append(resp.Workflows, wf)
	}

	// Synthesize entries for IDs the launcher promised but visibility hasn't
	// indexed yet. Without this the frontend would undercount the burst in
	// the first second or two after start, before visibility catches up.
	for _, id := range workflowIDs {
		if seen[id] {
			continue
		}
		resp.Workflows = append(resp.Workflows, pipelineWorkflow{
			WorkflowID: id,
			Status:     "RUNNING",
		})
		resp.Summary.Running++
	}
	resp.Summary.Total = len(workflowIDs)
	resp.CompletedAt, resp.DurationMs = pipelineTiming(
		resp.CreatedAt, latestClose, resp.Summary.Running, len(resp.Workflows), time.Now(),
	)

	writeJSON(w, http.StatusOK, resp)
}

// pipelineTiming returns the pipeline-level completedAt and duration.
// completedAt is set only when every child workflow has reached a terminal
// status (running == 0) and at least one workflow exists.
func pipelineTiming(
	createdAt, latestClose time.Time, running, total int, now time.Time,
) (*time.Time, *int64) {
	if createdAt.IsZero() {
		return nil, nil
	}
	if total > 0 && running == 0 && !latestClose.IsZero() {
		closed := latestClose
		d := closed.Sub(createdAt).Milliseconds()
		return &closed, &d
	}
	d := now.Sub(createdAt).Milliseconds()
	return nil, &d
}

// fetchPipelineWorkflowIDs returns the canonical list of per-image workflow
// IDs for a pipeline by querying the launcher workflow's GetWorkflowIDsQuery
// handler. We query rather than wait on run.Get() so the pipeline detail
// page stays responsive while the launcher is still fanning out — Temporal
// answers the query against both running and completed launcher executions,
// and the handler is registered before the activity fan-out begins so the
// list is always available. We deliberately rely on this rather than a
// visibility STARTS_WITH query so the pipeline page sees the full burst
// even if visibility hasn't caught up yet.
func (h *Handler) fetchPipelineWorkflowIDs(
	ctx context.Context, pipelineID string,
) ([]string, error) {
	launcherID := fmt.Sprintf("image-pipeline-%s", pipelineID)
	val, err := h.deps.Temporal.QueryWorkflow(ctx, launcherID, "", workflows.GetWorkflowIDsQuery)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := val.Get(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (h *Handler) listWorkflows(
	ctx context.Context, workflowIDs []string,
) ([]*workflowpb.WorkflowExecutionInfo, error) {
	if len(workflowIDs) == 0 {
		return nil, nil
	}
	// Drive the visibility query off the launcher's output: `WorkflowId IN
	// ("a","b",...)` returns exactly the executions the launcher promised,
	// so the response can never include stale neighbours and the caller can
	// still synthesize entries for IDs visibility hasn't indexed yet.
	quoted := make([]string, len(workflowIDs))
	for i, id := range workflowIDs {
		quoted[i] = strconv.Quote(id)
	}
	query := fmt.Sprintf("WorkflowId IN (%s)", strings.Join(quoted, ","))

	var out []*workflowpb.WorkflowExecutionInfo
	var pageToken []byte
	for {
		resp, err := h.deps.Temporal.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Namespace:     h.deps.Namespace,
			Query:         query,
			PageSize:      100,
			NextPageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, resp.Executions...)
		if len(resp.NextPageToken) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}
	return out, nil
}

// maxLookupsPerPoll caps the number of Temporal RPCs (DescribeWorkflowExecution
// or QueryWorkflow) each /api/pipelines/{id} poll fires per best-effort
// feature. Above this threshold the corresponding cosmetic field is left
// unset on the remaining running workflows.
const maxLookupsPerPoll = 10

// currentActivity returns the name of the first pending activity for the
// running workflow, or "" if none is reported. Errors are swallowed because
// this is a best-effort cosmetic field.
func (h *Handler) currentActivity(ctx context.Context, workflowID string) string {
	desc, err := h.deps.Temporal.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		return ""
	}
	for _, p := range desc.GetPendingActivities() {
		if t := p.GetActivityType(); t != nil {
			return t.GetName()
		}
	}
	return ""
}

// queryManifest returns the in-flight manifest exposed by the ProcessImage
// workflow's query handler, or nil if the query fails or decoding fails.
// Best-effort: errors are swallowed because the manifest is cosmetic until
// the final StoreManifest write lands in DynamoDB.
func (h *Handler) queryManifest(ctx context.Context, workflowID string) *manifest.Manifest {
	val, err := h.deps.Temporal.QueryWorkflow(ctx, workflowID, "", workflows.ManifestQueryName)
	if err != nil {
		return nil
	}
	var m manifest.Manifest
	if err := val.Get(&m); err != nil {
		return nil
	}
	return &m
}

// fetchManifests returns the persisted manifests for a pipeline, keyed by
// the workflowId attribute the StoreManifest activity records alongside
// each item.
func (h *Handler) fetchManifests(
	ctx context.Context, pipelineID string,
) (map[string]*manifest.Manifest, error) {
	out := make(map[string]*manifest.Manifest)
	var lastKey map[string]ddbtypes.AttributeValue
	for {
		resp, err := h.deps.Dynamo.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(h.deps.ImagesTable),
			KeyConditionExpression: aws.String("pipelineId = :pid"),
			ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
				":pid": &ddbtypes.AttributeValueMemberS{Value: pipelineID},
			},
			ExclusiveStartKey: lastKey,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range resp.Items {
			rawAttr, ok := item["manifest"].(*ddbtypes.AttributeValueMemberS)
			if !ok {
				continue
			}
			var m manifest.Manifest
			if err := json.Unmarshal([]byte(rawAttr.Value), &m); err != nil {
				h.deps.Logger.Warn("dropping malformed manifest", "pipelineId", pipelineID, "err", err)
				continue
			}
			wfAttr, ok := item["workflowId"].(*ddbtypes.AttributeValueMemberS)
			if !ok || wfAttr.Value == "" {
				continue
			}
			out[wfAttr.Value] = &m
		}
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		lastKey = resp.LastEvaluatedKey
	}
	return out, nil
}

// --- helpers ---------------------------------------------------------------

// statusName maps a Temporal status to the frontend's WorkflowStatus union.
// We can't use enumspb.WorkflowExecutionStatus.String() — it returns
// CamelCase ("Running") rather than the SCREAMING_SNAKE_CASE the frontend
// expects.
func statusName(s enumspb.WorkflowExecutionStatus) string {
	switch s {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "RUNNING"
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "COMPLETED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return "FAILED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return "CANCELED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return "TERMINATED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "CONTINUED_AS_NEW"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "TIMED_OUT"
	default:
		return "UNKNOWN"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// At this point headers are flushed; logging is the best we can do.
		slog.Error("write json failed", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
