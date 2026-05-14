package workflows

import (
	"fmt"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

// LaunchPipelines fans out one ProcessImage child workflow per input image
// and returns as soon as every child has been *started* (not completed).
//
// Children use ParentClosePolicy ABANDON so they outlive this launcher: the
// backend can synchronously wait for LaunchPipelines to return its list of
// workflow IDs without blocking on the actual image processing.
func LaunchPipelines(
	ctx workflow.Context, in manifest.LaunchPipelinesInput,
) (manifest.LaunchPipelinesResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("LaunchPipelines start",
		"pipelineId", in.PipelineID, "imageCount", len(in.Images))

	workflowIDs := make([]string, len(in.Images))
	for i, img := range in.Images {
		workflowIDs[i] = fmt.Sprintf("pipeline-%s-%s", in.PipelineID, img.ImageID)
	}

	// Fan-out: schedule every child in one pass so their starts happen in
	// parallel; we only await readiness in the second loop below.
	futures := make([]workflow.ChildWorkflowFuture, len(in.Images))
	for i, img := range in.Images {
		childOpts := workflow.ChildWorkflowOptions{
			WorkflowID:            workflowIDs[i],
			ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
			WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		}
		childCtx := workflow.WithChildOptions(ctx, childOpts)
		childIn := manifest.ProcessImageInput{
			PipelineID: in.PipelineID,
			ImageID:    img.ImageID,
			Original:   img.Original,
		}
		futures[i] = workflow.ExecuteChildWorkflow(childCtx, ProcessImage, childIn)
	}

	// Wait only for each child to be *started* (not completed). This keeps
	// the launcher's runtime in the sub-second range so the synchronous
	// backend call stays well within the API Gateway 29 s timeout.
	for i, f := range futures {
		if err := f.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
			return manifest.LaunchPipelinesResult{},
				fmt.Errorf("start child %s: %w", workflowIDs[i], err)
		}
	}

	logger.Info("LaunchPipelines done",
		"pipelineId", in.PipelineID, "started", len(workflowIDs))
	return manifest.LaunchPipelinesResult{WorkflowIDs: workflowIDs}, nil
}
