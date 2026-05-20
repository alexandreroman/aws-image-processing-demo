// Package activities holds the side-effecting work executed by the
// ProcessImage workflow: image manipulation, S3 I/O, LLM calls, and the
// final DynamoDB write.
//
// All activities are methods on *Activities so the worker can register
// them as a single struct and Temporal can introspect their method names.
package activities

import (
	"errors"
	"fmt"
	"os"

	"github.com/alexandreroman/aws-image-processing-demo/internal/anthropicclient"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/client"
)

// ClaudeInvalidInputErrorType is the application error type returned by
// GenerateDescription when the model rejects or cannot parse the image.
// Exported so the workflow can reference the same literal in its
// NonRetryableErrorTypes list without risk of drift.
const ClaudeInvalidInputErrorType = "ClaudeInvalidInput"

// Activities bundles the external clients and configuration used by every
// activity method. It is constructed once at worker startup.
type Activities struct {
	S3        *s3.Client
	Dynamo    *dynamodb.Client
	Anthropic *anthropicclient.Client
	// Temporal is used by the StartProcessImage starter activity to schedule
	// independent top-level ProcessImage workflows.
	Temporal client.Client

	ImagesBucket string
	ImagesTable  string
}

// New builds an Activities struct, resolving IMAGES_BUCKET and IMAGES_TABLE
// from the environment.
func New(
	s3c *s3.Client,
	ddb *dynamodb.Client,
	ac *anthropicclient.Client,
	tc client.Client,
) (*Activities, error) {
	bucket := os.Getenv("IMAGES_BUCKET")
	table := os.Getenv("IMAGES_TABLE")
	if bucket == "" {
		return nil, errors.New("activities: IMAGES_BUCKET is required")
	}
	if table == "" {
		return nil, errors.New("activities: IMAGES_TABLE is required")
	}
	return &Activities{
		S3:           s3c,
		Dynamo:       ddb,
		Anthropic:    ac,
		Temporal:     tc,
		ImagesBucket: bucket,
		ImagesTable:  table,
	}, nil
}

// resizedKey is the canonical S3 key for a resized variant.
func resizedKey(pipelineID, imageID, size string) string {
	return fmt.Sprintf("pipelines/%s/resized/%s/%s.jpg", pipelineID, imageID, size)
}

// watermarkedKey is the canonical S3 key for a watermarked variant.
func watermarkedKey(pipelineID, imageID, size string) string {
	return fmt.Sprintf("pipelines/%s/watermarked/%s/%s.jpg", pipelineID, imageID, size)
}
