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
)

// Activities bundles the external clients and configuration used by every
// activity method. It is constructed once at worker startup.
type Activities struct {
	S3        *s3.Client
	Presigner *s3.PresignClient
	Dynamo    *dynamodb.Client
	Anthropic *anthropicclient.Client

	ImagesBucket string
	ImagesTable  string
}

// Config carries optional overrides. Empty values fall back to env vars.
type Config struct {
	ImagesBucket string
	ImagesTable  string
}

// New builds an Activities struct, resolving IMAGES_BUCKET and IMAGES_TABLE
// from the environment when Config leaves them empty.
func New(
	s3c *s3.Client,
	presigner *s3.PresignClient,
	ddb *dynamodb.Client,
	ac *anthropicclient.Client,
	cfg Config,
) (*Activities, error) {
	bucket := cfg.ImagesBucket
	if bucket == "" {
		bucket = os.Getenv("IMAGES_BUCKET")
	}
	table := cfg.ImagesTable
	if table == "" {
		table = os.Getenv("IMAGES_TABLE")
	}
	if bucket == "" {
		return nil, errors.New("activities: IMAGES_BUCKET is required")
	}
	if table == "" {
		return nil, errors.New("activities: IMAGES_TABLE is required")
	}
	return &Activities{
		S3:           s3c,
		Presigner:    presigner,
		Dynamo:       ddb,
		Anthropic:    ac,
		ImagesBucket: bucket,
		ImagesTable:  table,
	}, nil
}

// resizedKey is the canonical S3 key for a resized variant.
func resizedKey(sessionID, imageID, size string) string {
	return fmt.Sprintf("sessions/%s/resized/%s/%s.jpg", sessionID, imageID, size)
}

// watermarkedKey is the canonical S3 key for a watermarked variant.
func watermarkedKey(sessionID, imageID, size string) string {
	return fmt.Sprintf("sessions/%s/watermarked/%s/%s.jpg", sessionID, imageID, size)
}
