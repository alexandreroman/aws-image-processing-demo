package workflows_test

import (
	"fmt"
	"testing"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestLaunchPipelines_StartsChildWorkflows(t *testing.T) {
	const pipelineID = "deadbeef"
	imageIDs := []string{"img-1", "img-2", "img-3"}

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(workflows.ProcessImage)

	// Short-circuit the child: the launcher's contract is "started" not
	// "completed", but the test environment runs children to completion in
	// the same goroutine — so we mock the child workflow itself rather than
	// wire up every downstream activity.
	env.OnWorkflow(workflows.ProcessImage, mock.Anything, mock.Anything).
		Return(manifest.Manifest{}, nil)

	images := make([]manifest.LaunchPipelineImage, len(imageIDs))
	for i, id := range imageIDs {
		images[i] = manifest.LaunchPipelineImage{
			ImageID:  id,
			Original: manifest.S3Ref{Bucket: "test-bucket", Key: "uploads/" + id + ".jpg"},
		}
	}

	env.ExecuteWorkflow(workflows.LaunchPipelines, manifest.LaunchPipelinesInput{
		PipelineID: pipelineID,
		Images:     images,
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got manifest.LaunchPipelinesResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Len(t, got.WorkflowIDs, len(imageIDs))
	for i, id := range imageIDs {
		require.Equal(t, fmt.Sprintf("pipeline-%s-%s", pipelineID, id), got.WorkflowIDs[i])
	}
}
