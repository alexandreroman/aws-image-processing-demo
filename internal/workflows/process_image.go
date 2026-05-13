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

// claudeInvalidInputErrorType mirrors the constant in the activities package.
// It is duplicated as a const here (not imported) because hard-coding the
// retry policy keeps the workflow self-contained and easy to audit.
const claudeInvalidInputErrorType = "ClaudeInvalidInput"

// ProcessImage is the 8-activity image-processing workflow.
//
// Fan-out: 3 resize + 3 watermark activities run in parallel. Fan-in is
// done by iterating manifest.SizeNames (NEVER the map) so collection order
// is deterministic across replays.
func ProcessImage(ctx workflow.Context, in manifest.ProcessImageInput) (manifest.Manifest, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ProcessImage start", "sessionId", in.SessionID, "imageId", in.ImageID)

	started := workflow.Now(ctx)

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
			NonRetryableErrorTypes: []string{claudeInvalidInputErrorType},
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

	// 1) Fan-out resize: one future per size.
	resizeFutures := make(map[string]workflow.Future, len(manifest.SizeNames))
	resizeCtx := workflow.WithActivityOptions(ctx, cpuOpts)
	for _, sizeName := range manifest.SizeNames {
		f := workflow.ExecuteActivity(resizeCtx, (*activities.Activities).ResizeAndUpload, activities.ResizeInput{
			SessionID: in.SessionID,
			ImageID:   in.ImageID,
			SizeName:  sizeName,
			Original:  in.Original,
		})
		resizeFutures[sizeName] = f
	}

	// 2) Fan-in resize. Iterate the canonical slice for deterministic order.
	sizes := make(map[string]manifest.Size, len(manifest.SizeNames))
	for _, sizeName := range manifest.SizeNames {
		var sz manifest.Size
		if err := resizeFutures[sizeName].Get(ctx, &sz); err != nil {
			return manifest.Manifest{}, err
		}
		sizes[sizeName] = sz
	}

	// 3) Describe on the medium size.
	describeCtx := workflow.WithActivityOptions(ctx, describeOpts)
	var description activities.DescribeResult
	mediumRef := sizes["medium"].S3Ref
	if err := workflow.ExecuteActivity(describeCtx, (*activities.Activities).GenerateDescription, mediumRef).
		Get(ctx, &description); err != nil {
		return manifest.Manifest{}, err
	}

	// 4) Fan-out watermark: one future per size, watermarking the resized
	//    output (not the original).
	watermarkFutures := make(map[string]workflow.Future, len(manifest.SizeNames))
	watermarkCtx := workflow.WithActivityOptions(ctx, cpuOpts)
	for _, sizeName := range manifest.SizeNames {
		f := workflow.ExecuteActivity(watermarkCtx, (*activities.Activities).ApplyWatermark, activities.WatermarkInput{
			SessionID: in.SessionID,
			ImageID:   in.ImageID,
			SizeName:  sizeName,
			Source:    sizes[sizeName].S3Ref,
		})
		watermarkFutures[sizeName] = f
	}

	// 5) Fan-in watermark.
	watermarked := make(map[string]manifest.S3Ref, len(manifest.SizeNames))
	for _, sizeName := range manifest.SizeNames {
		var ref manifest.S3Ref
		if err := watermarkFutures[sizeName].Get(ctx, &ref); err != nil {
			return manifest.Manifest{}, err
		}
		watermarked[sizeName] = ref
	}

	// 6) Persist.
	completed := workflow.Now(ctx)
	m := manifest.Manifest{
		SessionID:   in.SessionID,
		ImageID:     in.ImageID,
		Original:    in.Original,
		Sizes:       sizes,
		Description: description.Description,
		Labels:      description.Labels,
		Watermarked: watermarked,
		StartedAt:   started,
		CompletedAt: completed,
	}

	storeWFCtx := workflow.WithActivityOptions(ctx, storeOpts)
	if err := workflow.ExecuteActivity(storeWFCtx, (*activities.Activities).StoreManifest, m).
		Get(ctx, nil); err != nil {
		return manifest.Manifest{}, err
	}

	logger.Info("ProcessImage done", "imageId", in.ImageID)
	return m, nil
}
