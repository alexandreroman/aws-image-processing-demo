// Package api exposes the HTTP handlers backing the demo's REST endpoints.
//
// All routes live under /api/* so a single CloudFront distribution can
// dispatch by path (api → API Gateway, everything else → S3 frontend) with
// no CORS gymnastics in production.
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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// Dependencies holds the runtime collaborators of the API. The struct is
// the seam used both by main (production) and tests.
type Dependencies struct {
	Temporal     client.Client
	Presigner    *s3.PresignClient
	Dynamo       *dynamodb.Client
	ImagesBucket string
	ImagesTable  string
	TaskQueue    string
	Namespace    string
	Logger       *slog.Logger
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
	if deps.TaskQueue == "" {
		deps.TaskQueue = "image-processing"
	}
	if deps.Namespace == "" {
		deps.Namespace = client.DefaultNamespace
	}

	h := &Handler{deps: deps, mux: http.NewServeMux()}
	h.mux.HandleFunc("POST /api/uploads/presign", h.handlePresign)
	h.mux.HandleFunc("POST /api/workflows/start", h.handleStart)
	h.mux.HandleFunc("GET /api/pipelines/{pipelineId}", h.handlePipeline)
	h.mux.HandleFunc("GET /api/healthz", h.handleHealth)
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

// --- /api/healthz -----------------------------------------------------------

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- /api/uploads/presign ---------------------------------------------------

type presignRequest struct {
	Count int   `json:"count"`
	Size  int64 `json:"size,omitempty"`
}

type presignedURL struct {
	URL string `json:"url"`
	Key string `json:"key"`
}

const (
	presignTTL    = 15 * time.Minute
	maxPresignCnt = 50
	// maxPresignSize mirrors maxImageBytes in internal/activities/resize.go.
	// S3 PUT presigned URLs cannot fully enforce object size at the SDK
	// level — a client can ignore Content-Length — so the worker also
	// enforces the cap as defense in depth when it downloads the object.
	maxPresignSize = 25 * 1024 * 1024
)

func (h *Handler) handlePresign(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.Count <= 0 || req.Count > maxPresignCnt {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("count must be between 1 and %d, got %d", maxPresignCnt, req.Count))
		return
	}
	if req.Size > maxPresignSize {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("size must be <= %d bytes, got %d", maxPresignSize, req.Size))
		return
	}

	out := make([]presignedURL, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		key := "uploads/" + uuid.NewString() + ".jpg"
		req, err := h.deps.Presigner.PresignPutObject(r.Context(), &s3.PutObjectInput{
			Bucket:      aws.String(h.deps.ImagesBucket),
			Key:         aws.String(key),
			ContentType: aws.String("image/jpeg"),
		}, s3.WithPresignExpires(presignTTL))
		if err != nil {
			h.deps.Logger.Error("presign failed", "err", err)
			writeError(w, http.StatusInternalServerError, "presign failed")
			return
		}
		out = append(out, presignedURL{URL: req.URL, Key: key})
	}
	writeJSON(w, http.StatusOK, out)
}

// --- /api/workflows/start ---------------------------------------------------

type startRequest struct {
	Images []manifest.S3Ref `json:"images"`
}

type startResponse struct {
	PipelineID  string   `json:"pipelineId"`
	WorkflowIDs []string `json:"workflowIds"`
}

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

	pipelineID := newPipelineID()

	imageIDs := make([]string, len(req.Images))
	images := make([]manifest.LaunchPipelineImage, len(req.Images))
	for i, img := range req.Images {
		imageIDs[i] = newImageID()
		images[i] = manifest.LaunchPipelineImage{ImageID: imageIDs[i], Original: img}
	}

	// One call instead of N: the LaunchPipelines workflow fans out the
	// per-image ProcessImage children and returns as soon as they are
	// started, so the backend stays well inside the API Gateway 29 s budget
	// regardless of burst size.
	//
	// The "launch-" ID prefix keeps this workflow out of the pipeline
	// listing (which filters on `pipeline-{id}-`).
	opts := client.StartWorkflowOptions{
		ID:                    fmt.Sprintf("launch-%s", pipelineID),
		TaskQueue:             h.deps.TaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	in := manifest.LaunchPipelinesInput{PipelineID: pipelineID, Images: images}
	run, err := h.deps.Temporal.ExecuteWorkflow(r.Context(), opts, workflows.LaunchPipelines, in)
	if err != nil {
		h.deps.Logger.Error("start launcher failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		return
	}

	var result manifest.LaunchPipelinesResult
	if err := run.Get(r.Context(), &result); err != nil {
		h.deps.Logger.Error("launcher failed", "pipelineId", pipelineID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, startResponse{PipelineID: pipelineID, WorkflowIDs: result.WorkflowIDs})
}

// shortID returns the first 8 hex chars of a UUID v4.
//
// Why 8 chars: a burst is at most a few dozen images per pipeline, so the
// 32-bit space leaves collision probability well under one in a million,
// and short IDs make URLs, logs, and the Temporal UI dramatically more
// readable. WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE surfaces the
// vanishingly rare collision as a start-workflow error.
func shortID() string {
	return uuid.NewString()[:8]
}

func newPipelineID() string { return shortID() }
func newImageID() string    { return shortID() }

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
	PipelineID string             `json:"pipelineId"`
	CreatedAt  time.Time          `json:"createdAt,omitempty"`
	ImageCount int                `json:"imageCount"`
	Summary    pipelineSummary    `json:"summary"`
	Workflows  []pipelineWorkflow `json:"workflows"`
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
	// running workflow. Cap the per-poll fan-out so a 48-workflow burst does
	// not stall the 1 s frontend poll loop. The two caps are tracked
	// independently so each feature stays tunable on its own.
	currentActivityLookups := 0
	manifestQueryLookups := 0
	seen := make(map[string]bool, len(executions))
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
			if currentActivityLookups < maxCurrentActivityLookups {
				currentActivityLookups++
				if act := h.currentActivity(r.Context(), wf.WorkflowID); act != "" {
					wf.CurrentActivity = act
				}
			}
			// Best effort: surface the in-flight manifest so the gallery can
			// show resized images before watermarking finishes. Skip when DDB
			// already has the final manifest to avoid wasting lookup budget.
			if !hasDDBManifest && manifestQueryLookups < maxManifestQueryLookups {
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

	writeJSON(w, http.StatusOK, resp)
}

// fetchPipelineWorkflowIDs reads the LaunchPipelines workflow output to get
// the canonical list of pipeline workflow IDs. The launcher completes as
// soon as all children are started, so this returns quickly even on the
// 1 s frontend poll cadence. We deliberately rely on the launcher's output
// rather than a visibility STARTS_WITH query so the pipeline page sees the
// full burst even if visibility hasn't caught up yet.
func (h *Handler) fetchPipelineWorkflowIDs(
	ctx context.Context, pipelineID string,
) ([]string, error) {
	launcherID := fmt.Sprintf("launch-%s", pipelineID)
	run := h.deps.Temporal.GetWorkflow(ctx, launcherID, "")
	var result manifest.LaunchPipelinesResult
	if err := run.Get(ctx, &result); err != nil {
		return nil, err
	}
	return result.WorkflowIDs, nil
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

// maxCurrentActivityLookups caps the number of DescribeWorkflowExecution
// calls each /api/pipelines/{id} poll fires. Above this threshold the
// currentActivity field is left empty on the remaining running workflows.
const maxCurrentActivityLookups = 10

// maxManifestQueryLookups caps the number of QueryWorkflow calls fired per
// /api/pipelines/{id} poll to fetch in-flight manifests. Kept separate from
// maxCurrentActivityLookups so the two features stay independently tunable.
const maxManifestQueryLookups = 10

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

// Compile-time sanity check that *Handler implements http.Handler.
var _ http.Handler = (*Handler)(nil)
