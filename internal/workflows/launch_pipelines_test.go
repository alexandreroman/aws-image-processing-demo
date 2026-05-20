package workflows_test

import (
	"context"
	"errors"
	"testing"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestLaunchPipelines_StartsProcessImageWorkflows(t *testing.T) {
	const pipelineID = "deadbeef"
	imageIDs := []string{"img-1", "img-2", "img-3"}

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	// No Temporal client wired in: the mock intercepts the activity before
	// it would dereference a.Temporal.
	acts := &activities.Activities{
		ImagesBucket: "test-bucket",
		ImagesTable:  "test-table",
	}
	env.RegisterActivity(acts)

	started := make([]string, 0, len(imageIDs))
	env.OnActivity(acts.StartProcessImage, mock.Anything,
		mock.MatchedBy(func(in activities.StartProcessImageInput) bool { return true }),
	).Return(func(_ context.Context, in activities.StartProcessImageInput) (string, error) {
		id := manifest.ProcessImageWorkflowID(in.PipelineID, in.Image.ImageID)
		started = append(started, id)
		return id, nil
	})

	images := make([]manifest.LaunchPipelineImage, len(imageIDs))
	for i, id := range imageIDs {
		images[i] = manifest.LaunchPipelineImage{
			ImageID:  id,
			Original: manifest.S3Ref{Key: "samples/" + id + ".jpg"},
		}
	}

	env.ExecuteWorkflow(workflows.LaunchPipelines, manifest.LaunchPipelinesInput{
		PipelineID: pipelineID,
		Images:     images,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	want := make([]string, len(imageIDs))
	for i, id := range imageIDs {
		want[i] = manifest.ProcessImageWorkflowID(pipelineID, id)
	}
	require.ElementsMatch(t, want, started)
}

func TestLaunchPipelines_PropagatesActivityError(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	acts := &activities.Activities{
		ImagesBucket: "test-bucket",
		ImagesTable:  "test-table",
	}
	env.RegisterActivity(acts)

	env.OnActivity(acts.StartProcessImage, mock.Anything, mock.Anything).
		Return("", errors.New("boom"))

	env.ExecuteWorkflow(workflows.LaunchPipelines, manifest.LaunchPipelinesInput{
		PipelineID: "deadbeef",
		Images: []manifest.LaunchPipelineImage{
			{ImageID: "img-1", Original: manifest.S3Ref{Key: "samples/img-1.jpg"}},
		},
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
}
