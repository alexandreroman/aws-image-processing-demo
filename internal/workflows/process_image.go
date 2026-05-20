// Package workflows hosts the Temporal workflows that drive the demo.
//
// Workflow code is subject to determinism: never use the time or log
// packages directly, never iterate Go maps with `range` — use
// `workflow.Now`, `workflow.GetLogger`, and the canonical slices in
// the manifest package instead.
package workflows

import (
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ManifestQueryName is the Temporal query handler name that returns the
// in-flight manifest accumulated so far by a ProcessImage execution.
const ManifestQueryName = "manifest"

// ProcessImage is the 8-activity image-processing workflow.
//
// Fan-out: 3 resize + 3 watermark activities run in parallel. Fan-in is
// done by iterating manifest.SizeNames (NEVER the map) so collection order
// is deterministic across replays.
func ProcessImage(ctx workflow.Context, in manifest.ProcessImageInput) (manifest.Manifest, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ProcessImage start", "pipelineId", in.PipelineID, "imageId", in.ImageID)

	cpuOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		HeartbeatTimeout:    10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	describeOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		HeartbeatTimeout:    20 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Second,
			BackoffCoefficient:     2.0,
			MaximumAttempts:        4,
			NonRetryableErrorTypes: []string{activities.ClaudeInvalidInputErrorType},
		},
	}
	storeOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    5,
		},
	}

	// Track the manifest as it grows so the API can surface in-flight state
	// via a query — the gallery then shows the resized image before
	// watermarking finishes, without waiting for the final DynamoDB write.
	state := manifest.Manifest{
		PipelineID:  in.PipelineID,
		ImageID:     in.ImageID,
		Original:    in.Original,
		Sizes:       map[string]manifest.Size{},
		Watermarked: map[string]manifest.S3Ref{},
	}
	if err := workflow.SetQueryHandler(ctx, ManifestQueryName, func() (manifest.Manifest, error) {
		return state, nil
	}); err != nil {
		return manifest.Manifest{}, err
	}

	// 1) Fan-out resize: one future per size, indexed positionally alongside
	//    manifest.SizeNames so we never iterate a map in workflow code.
	resizeFutures := make([]workflow.Future, len(manifest.SizeNames))
	resizeCtx := workflow.WithActivityOptions(ctx, cpuOpts)
	for i, sizeName := range manifest.SizeNames {
		resizeFutures[i] = workflow.ExecuteActivity(resizeCtx, (*activities.Activities).ResizeAndUpload, activities.ResizeInput{
			PipelineID: in.PipelineID,
			ImageID:    in.ImageID,
			SizeName:   sizeName,
			Original:   in.Original,
		})
	}

	// 2) Fan-in resize. Iterate the canonical slice for deterministic order.
	sizes := make(map[string]manifest.Size, len(manifest.SizeNames))
	for i, sizeName := range manifest.SizeNames {
		var sz manifest.Size
		if err := resizeFutures[i].Get(ctx, &sz); err != nil {
			return manifest.Manifest{}, err
		}
		sizes[sizeName] = sz
	}
	state.Sizes = sizes

	// 3) Describe on the medium size.
	describeCtx := workflow.WithActivityOptions(ctx, describeOpts)
	var description activities.DescribeResult
	mediumRef := sizes["medium"].S3Ref
	if err := workflow.ExecuteActivity(describeCtx, (*activities.Activities).GenerateDescription, mediumRef).
		Get(ctx, &description); err != nil {
		return manifest.Manifest{}, err
	}
	state.Description = description.Description
	state.Labels = description.Labels

	// 4) Fan-out watermark: one future per size, watermarking the resized
	//    output (not the original).
	watermarkFutures := make([]workflow.Future, len(manifest.SizeNames))
	watermarkCtx := workflow.WithActivityOptions(ctx, cpuOpts)
	for i, sizeName := range manifest.SizeNames {
		watermarkFutures[i] = workflow.ExecuteActivity(watermarkCtx, (*activities.Activities).ApplyWatermark, activities.WatermarkInput{
			PipelineID: in.PipelineID,
			ImageID:    in.ImageID,
			SizeName:   sizeName,
			Source:     sizes[sizeName].S3Ref,
		})
	}

	// 5) Fan-in watermark.
	watermarked := make(map[string]manifest.S3Ref, len(manifest.SizeNames))
	for i, sizeName := range manifest.SizeNames {
		var ref manifest.S3Ref
		if err := watermarkFutures[i].Get(ctx, &ref); err != nil {
			return manifest.Manifest{}, err
		}
		watermarked[sizeName] = ref
	}
	state.Watermarked = watermarked

	// 6) Persist.
	storeWFCtx := workflow.WithActivityOptions(ctx, storeOpts)
	if err := workflow.ExecuteActivity(storeWFCtx, (*activities.Activities).StoreManifest, state).
		Get(ctx, nil); err != nil {
		return manifest.Manifest{}, err
	}

	logger.Info("ProcessImage done", "imageId", in.ImageID)
	return state, nil
}
