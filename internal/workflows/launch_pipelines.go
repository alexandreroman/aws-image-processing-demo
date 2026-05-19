package workflows

import (
	"fmt"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// LaunchPipelines fans out one ProcessImage execution per input image and
// returns as soon as every execution has been *started* (not completed).
//
// Each ProcessImage execution is scheduled by the StartProcessImage starter
// activity, which calls client.ExecuteWorkflow. The resulting workflows are
// fully independent top-level executions with no parent/child relationship
// to this launcher — so this workflow can return its list of IDs as soon as
// the starts are acknowledged, keeping the synchronous backend call well
// within the API Gateway 29 s timeout.
func LaunchPipelines(
	ctx workflow.Context, in manifest.LaunchPipelinesInput,
) (manifest.LaunchPipelinesResult, error) {
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
		workflowIDs[i] = fmt.Sprintf("image-pipeline-%s-%s", in.PipelineID, img.ImageID)
	}

	// Fan-out: schedule all starter activities in one pass so the underlying
	// ExecuteWorkflow calls happen in parallel.
	futures := make([]workflow.Future, len(in.Images))
	for i, img := range in.Images {
		actIn := activities.StartProcessImageInput{
			WorkflowID: workflowIDs[i],
			PipelineID: in.PipelineID,
			ImageID:    img.ImageID,
			Original:   img.Original,
		}
		futures[i] = workflow.ExecuteActivity(actCtx, (*activities.Activities).StartProcessImage, actIn)
	}

	for i, f := range futures {
		if err := f.Get(ctx, nil); err != nil {
			return manifest.LaunchPipelinesResult{},
				fmt.Errorf("start %s: %w", workflowIDs[i], err)
		}
	}

	logger.Info("LaunchPipelines done",
		"pipelineId", in.PipelineID, "started", len(workflowIDs))
	return manifest.LaunchPipelinesResult{WorkflowIDs: workflowIDs}, nil
}
