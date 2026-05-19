package activities

import (
	"context"
	"fmt"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

// StartProcessImageInput is the input for the StartProcessImage activity.
// The per-image workflow ID is derived deterministically from PipelineID +
// Image.ImageID via manifest.ProcessImageWorkflowID, so it is not carried
// on the wire.
type StartProcessImageInput struct {
	PipelineID string                       `json:"pipelineId"`
	Image      manifest.LaunchPipelineImage `json:"image"`
}

// StartProcessImage schedules a ProcessImage workflow on the configured task
// queue and returns its workflow ID once Temporal has accepted the start.
//
// The target workflow is referenced by its registered name "ProcessImage"
// rather than by symbol on purpose: this keeps internal/activities free of
// an import on internal/workflows, which would otherwise create a cycle
// (workflows already imports activities). Temporal resolves the name from
// the worker registration at run time.
//
// This is the canonical "starter activity" pattern: it lets a workflow
// (LaunchPipelines) fan out child executions that are fully independent
// top-level workflows — no parent/child relationship, no
// PARENT_CLOSE_POLICY plumbing.
func (a *Activities) StartProcessImage(
	ctx context.Context, in StartProcessImageInput,
) (string, error) {
	workflowID := manifest.ProcessImageWorkflowID(in.PipelineID, in.Image.ImageID)

	logger := activity.GetLogger(ctx)
	logger.Info("StartProcessImage", "workflowId", workflowID, "pipelineId", in.PipelineID)

	// Inherit the parent's task queue so the fan-out lands on the same worker
	// runtime that scheduled it (e.g. ECS-launched bursts stay on ECS).
	taskQueue := activity.GetInfo(ctx).TaskQueue
	opts := client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	procIn := manifest.ProcessImageInput{
		PipelineID: in.PipelineID,
		ImageID:    in.Image.ImageID,
		Original:   in.Image.Original,
	}
	if _, err := a.Temporal.ExecuteWorkflow(ctx, opts, "ProcessImage", procIn); err != nil {
		return "", fmt.Errorf("start %s: %w", workflowID, err)
	}
	return workflowID, nil
}
