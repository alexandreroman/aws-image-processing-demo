// Package awsclient builds an AWS SDK v2 config that transparently targets
// LocalStack when AWS_ENDPOINT_URL is set, and real AWS otherwise.
//
// The same Go code runs against both, which is the design constraint that
// keeps local dev and production behavior in sync.
package awsclient

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const defaultRegion = "eu-west-1"

// Load builds an AWS config. If AWS_REGION is empty, defaults to eu-west-1.
// AWS credentials come from the default SDK chain (env, profile, IAM role).
func Load(ctx context.Context) (aws.Config, error) {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = defaultRegion
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if ep := os.Getenv("AWS_ENDPOINT_URL"); ep != "" {
		opts = append(opts, config.WithBaseEndpoint(ep))
	}
	return config.LoadDefaultConfig(ctx, opts...)
}

// NewS3 builds an S3 client. When AWS_ENDPOINT_URL is set (LocalStack),
// path-style addressing is forced — virtual-hosted style does not work with
// localhost-rooted endpoints.
func NewS3(cfg aws.Config) *s3.Client {
	usingLocalStack := os.Getenv("AWS_ENDPOINT_URL") != ""
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if usingLocalStack {
			o.UsePathStyle = true
		}
	})
}

// NewDynamoDB builds a DynamoDB client from the shared config.
func NewDynamoDB(cfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg)
}
