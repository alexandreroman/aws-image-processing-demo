package workflows

import (
	"fmt"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// GetWorkflowIDsQuery is the Temporal query handler name that returns the
// canonical list of per-image workflow IDs the launcher is about to fan out.
// The backend queries this instead of waiting on the launcher's completion,
// so the pipeline detail page stays responsive while the launcher is still
// scheduling activities.
const GetWorkflowIDsQuery = "getWorkflowIDs"

// LaunchPipelines fans out one ProcessImage execution per input image and
// returns as soon as every execution has been *started* (not completed).
//
// Each ProcessImage execution is scheduled by the StartProcessImage starter
// activity, which calls client.ExecuteWorkflow. The resulting workflows are
// fully independent top-level executions with no parent/child relationship
// to this launcher — so this workflow can return as soon as the starts are
// acknowledged, keeping the synchronous backend call well within the API
// Gateway 29 s timeout.
func LaunchPipelines(ctx workflow.Context, in manifest.LaunchPipelinesInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("LaunchPipelines start",
		"pipelineId", in.PipelineID, "imageCount", len(in.Images))

	startOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, startOpts)

	workflowIDs := make([]string, len(in.Images))
	for i, img := range in.Images {
		workflowIDs[i] = manifest.ProcessImageWorkflowID(in.PipelineID, img.ImageID)
	}

	// Expose the ID list via a query so the backend can read it instantly
	// while this workflow is still fanning out.
	if err := workflow.SetQueryHandler(ctx, GetWorkflowIDsQuery, func() ([]string, error) {
		return workflowIDs, nil
	}); err != nil {
		return fmt.Errorf("set query handler: %w", err)
	}

	// Fan-out: schedule all starter activities in one pass so the underlying
	// ExecuteWorkflow calls happen in parallel.
	futures := make([]workflow.Future, len(in.Images))
	for i, img := range in.Images {
		actIn := activities.StartProcessImageInput{
			PipelineID: in.PipelineID,
			Image:      img,
		}
		futures[i] = workflow.ExecuteActivity(actCtx, (*activities.Activities).StartProcessImage, actIn)
	}

	for i, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			return fmt.Errorf("start %s: %w", workflowIDs[i], err)
		}
	}

	logger.Info("LaunchPipelines done",
		"pipelineId", in.PipelineID, "started", len(workflowIDs))
	return nil
}
