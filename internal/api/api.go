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
	h.mux.HandleFunc("GET /api/sessions/{sessionId}", h.handleSession)
	h.mux.HandleFunc("GET /api/healthz", h.handleHealth)
	return h
}

// ServeHTTP applies CORS for all requests (the Nuxt dev server on :3000
// talks to the backend on :8000 in local mode), short-circuits preflights,
// then dispatches to the mux.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	Count int `json:"count"`
}

type presignedURL struct {
	URL string `json:"url"`
	Key string `json:"key"`
}

const (
	presignTTL    = 15 * time.Minute
	maxPresignCnt = 50
)

func (h *Handler) handlePresign(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.Count <= 0 || req.Count > maxPresignCnt {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("count must be between 1 and %d", maxPresignCnt))
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
	SessionID   string   `json:"sessionId"`
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

	sessionID := newSessionID()

	workflowIDs := make([]string, 0, len(req.Images))
	for i, img := range req.Images {
		wfID := fmt.Sprintf("%s-%d", sessionID, i)
		opts := client.StartWorkflowOptions{
			ID:                    wfID,
			TaskQueue:             h.deps.TaskQueue,
			WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		}
		in := manifest.ProcessImageInput{
			SessionID: sessionID,
			ImageID:   uuid.NewString(),
			Original:  img,
		}
		if _, err := h.deps.Temporal.ExecuteWorkflow(r.Context(), opts, workflows.ProcessImage, in); err != nil {
			h.deps.Logger.Error("start workflow failed", "workflowId", wfID, "err", err)
			writeError(w, http.StatusInternalServerError, "failed to start workflow: "+err.Error())
			return
		}
		workflowIDs = append(workflowIDs, wfID)
	}

	writeJSON(w, http.StatusOK, startResponse{SessionID: sessionID, WorkflowIDs: workflowIDs})
}

// newSessionID returns the first 8 hex chars of a UUID v4.
func newSessionID() string {
	return uuid.NewString()[:8]
}

// --- /api/sessions/{sessionId} ----------------------------------------------

type sessionWorkflow struct {
	WorkflowID      string             `json:"workflowId"`
	ImageID         string             `json:"imageId,omitempty"`
	Status          string             `json:"status"`
	CurrentActivity string             `json:"currentActivity,omitempty"`
	StartedAt       time.Time          `json:"startedAt,omitempty"`
	CompletedAt     *time.Time         `json:"completedAt,omitempty"`
	Manifest        *manifest.Manifest `json:"manifest,omitempty"`
}

type sessionSummary struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type sessionResponse struct {
	SessionID  string            `json:"sessionId"`
	CreatedAt  time.Time         `json:"createdAt,omitempty"`
	ImageCount int               `json:"imageCount"`
	Summary    sessionSummary    `json:"summary"`
	Workflows  []sessionWorkflow `json:"workflows"`
}

func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	manifests, err := h.fetchManifests(r.Context(), sessionID)
	if err != nil {
		h.deps.Logger.Error("fetch manifests failed", "sessionId", sessionID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to read session: "+err.Error())
		return
	}

	executions, err := h.listWorkflows(r.Context(), sessionID)
	if err != nil {
		h.deps.Logger.Error("list workflows failed", "sessionId", sessionID, "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list workflows: "+err.Error())
		return
	}

	resp := sessionResponse{
		SessionID:  sessionID,
		ImageCount: len(executions),
		Workflows:  make([]sessionWorkflow, 0, len(executions)),
	}

	for _, exec := range executions {
		wf := sessionWorkflow{
			WorkflowID: exec.GetExecution().GetWorkflowId(),
			Status:     statusName(exec.GetStatus()),
		}
		if t := exec.GetStartTime(); t != nil {
			started := t.AsTime()
			wf.StartedAt = started
			if resp.CreatedAt.IsZero() || started.Before(resp.CreatedAt) {
				resp.CreatedAt = started
			}
		}
		if t := exec.GetCloseTime(); t != nil {
			closed := t.AsTime()
			wf.CompletedAt = &closed
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
			if act := h.currentActivity(r.Context(), wf.WorkflowID); act != "" {
				wf.CurrentActivity = act
			}
		}

		if m, ok := manifests[wf.WorkflowID]; ok {
			wf.ImageID = m.ImageID
			wf.Manifest = m
		}

		resp.Workflows = append(resp.Workflows, wf)
	}
	resp.Summary.Total = len(executions)

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listWorkflows(
	ctx context.Context, sessionID string,
) ([]*workflowpb.WorkflowExecutionInfo, error) {
	// ListWorkflow (rather than ListOpen/ListClosed) so the result set
	// includes both running and terminal executions in one call.
	var out []*workflowpb.WorkflowExecutionInfo
	var pageToken []byte
	query := fmt.Sprintf(`WorkflowId STARTS_WITH "%s-"`, sessionID)
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

// fetchManifests returns the persisted manifests for a session, keyed by
// the workflowId attribute the StoreManifest activity records alongside
// each item.
func (h *Handler) fetchManifests(
	ctx context.Context, sessionID string,
) (map[string]*manifest.Manifest, error) {
	out := make(map[string]*manifest.Manifest)
	var lastKey map[string]ddbtypes.AttributeValue
	for {
		resp, err := h.deps.Dynamo.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(h.deps.ImagesTable),
			KeyConditionExpression: aws.String("sessionId = :sid"),
			ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
				":sid": &ddbtypes.AttributeValueMemberS{Value: sessionID},
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
				h.deps.Logger.Warn("dropping malformed manifest", "sessionId", sessionID, "err", err)
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
		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Compile-time sanity check that *Handler implements http.Handler.
var _ http.Handler = (*Handler)(nil)
