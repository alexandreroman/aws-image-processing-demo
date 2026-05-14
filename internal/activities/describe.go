package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// DescribeResult is what GenerateDescription returns.
type DescribeResult struct {
	Description string
	Labels      []string
}

// GenerateDescription downloads the medium-size image and asks Claude for a
// caption and a few labels. A malformed image becomes a non-retryable
// application error so the workflow does not waste retries on it.
func (a *Activities) GenerateDescription(ctx context.Context, ref manifest.S3Ref) (DescribeResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("describe start", "key", ref.Key)
	activity.RecordHeartbeat(ctx, "download")

	raw, err := a.download(ctx, ref)
	if err != nil {
		return DescribeResult{}, fmt.Errorf("describe: download: %w", err)
	}

	activity.RecordHeartbeat(ctx, "anthropic")
	desc, labels, err := a.Anthropic.Describe(ctx, raw, "image/jpeg")
	if err != nil {
		if errors.Is(err, anthropicclient.ErrClaudeInvalidInput) {
			return DescribeResult{}, temporal.NewNonRetryableApplicationError(
				err.Error(), ClaudeInvalidInputErrorType, err,
			)
		}
		return DescribeResult{}, fmt.Errorf("describe: anthropic: %w", err)
	}

	logger.Info("describe done", "labels", labels)
	return DescribeResult{Description: desc, Labels: labels}, nil
}
