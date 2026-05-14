package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.temporal.io/sdk/activity"
)

// StoreManifest persists the final manifest as a single DynamoDB item.
// Schema matches what compose.yaml `init` provisions:
//
//	sessionId (HASH) + imageId (RANGE), plus description, manifest (JSON),
//	workflowId, createdAt.
//
// workflowId is stored so the API can map manifests back to executions
// when listing a session (workflow IDs are not present in the manifest
// otherwise).
func (a *Activities) StoreManifest(ctx context.Context, m manifest.Manifest) error {
	logger := activity.GetLogger(ctx)
	info := activity.GetInfo(ctx)
	wfID := info.WorkflowExecution.ID
	logger.Info("store manifest", "sessionId", m.SessionID, "imageId", m.ImageID, "workflowId", wfID)

	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("store: marshal: %w", err)
	}

	createdAt := time.Now().UTC()

	_, err = a.Dynamo.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(a.ImagesTable),
		Item: map[string]ddbtypes.AttributeValue{
			"sessionId":   &ddbtypes.AttributeValueMemberS{Value: m.SessionID},
			"imageId":     &ddbtypes.AttributeValueMemberS{Value: m.ImageID},
			"description": &ddbtypes.AttributeValueMemberS{Value: m.Description},
			"manifest":    &ddbtypes.AttributeValueMemberS{Value: string(raw)},
			"workflowId":  &ddbtypes.AttributeValueMemberS{Value: wfID},
			"createdAt":   &ddbtypes.AttributeValueMemberS{Value: createdAt.UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("store: putitem: %w", err)
	}
	return nil
}
