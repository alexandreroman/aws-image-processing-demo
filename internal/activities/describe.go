package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexandreroman/temporal-aws-autoscaling-demo/internal/anthropicclient"
	"github.com/alexandreroman/temporal-aws-autoscaling-demo/internal/manifest"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// DescribeResult is what GenerateDescription returns.
type DescribeResult struct {
	Description string
	Labels      []string
}

// claudeInvalidInputErrorType is the application error type used so the
// workflow can list it in NonRetryableErrorTypes.
const claudeInvalidInputErrorType = "ClaudeInvalidInput"

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
				err.Error(), claudeInvalidInputErrorType, err,
			)
		}
		return DescribeResult{}, fmt.Errorf("describe: anthropic: %w", err)
	}

	logger.Info("describe done", "labels", labels)
	return DescribeResult{Description: desc, Labels: labels}, nil
}
