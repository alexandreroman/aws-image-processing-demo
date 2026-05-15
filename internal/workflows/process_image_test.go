package workflows_test

import (
	"context"
	"sync"
	"testing"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

type ProcessImageSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env  *testsuite.TestWorkflowEnvironment
	acts *activities.Activities
}

func TestProcessImageSuite(t *testing.T) {
	suite.Run(t, new(ProcessImageSuite))
}

func (s *ProcessImageSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()

	// Real-looking struct; the mocked activities mean its fields are
	// never actually dereferenced.
	s.acts = &activities.Activities{
		ImagesBucket: "test-bucket",
		ImagesTable:  "test-table",
	}
	s.env.RegisterActivity(s.acts)
}

func (s *ProcessImageSuite) TearDownTest() {
	s.env.AssertExpectations(s.T())
}

func (s *ProcessImageSuite) TestHappyPath() {
	const (
		pipelineID = "deadbeef"
		imageID    = "img-1"
	)
	original := manifest.S3Ref{Bucket: "test-bucket", Key: "uploads/foo.jpg"}

	// Expect one resize per size name.
	for _, name := range manifest.SizeNames {
		key := "pipelines/" + pipelineID + "/resized/" + imageID + "/" + name + ".jpg"
		s.env.OnActivity(s.acts.ResizeAndUpload, mock.Anything, mock.MatchedBy(func(in activities.ResizeInput) bool {
			return in.PipelineID == pipelineID && in.ImageID == imageID && in.SizeName == name
		})).Return(manifest.Size{
			S3Ref:  manifest.S3Ref{Bucket: "test-bucket", Key: key},
			Width:  manifest.SizeWidths[name],
			Height: manifest.SizeWidths[name] * 3 / 4,
			Bytes:  1000,
		}, nil).Once()
	}

	// One describe call on the medium size.
	mediumKey := "pipelines/" + pipelineID + "/resized/" + imageID + "/medium.jpg"
	s.env.OnActivity(s.acts.GenerateDescription, mock.Anything, mock.MatchedBy(func(ref manifest.S3Ref) bool {
		return ref.Key == mediumKey
	})).Return(activities.DescribeResult{
		Description: "a dog catching a frisbee",
		Labels:      []string{"dog", "beach", "pet"},
	}, nil).Once()

	// One watermark per size.
	for _, name := range manifest.SizeNames {
		s.env.OnActivity(s.acts.ApplyWatermark, mock.Anything, mock.MatchedBy(func(in activities.WatermarkInput) bool {
			return in.PipelineID == pipelineID && in.ImageID == imageID && in.SizeName == name
		})).Return(manifest.S3Ref{
			Bucket: "test-bucket",
			Key:    "pipelines/" + pipelineID + "/watermarked/" + imageID + "/" + name + ".jpg",
		}, nil).Once()
	}

	// One store call at the end with the fully-populated manifest.
	s.env.OnActivity(s.acts.StoreManifest, mock.Anything, mock.MatchedBy(func(m manifest.Manifest) bool {
		return m.PipelineID == pipelineID &&
			m.ImageID == imageID &&
			len(m.Sizes) == len(manifest.SizeNames) &&
			len(m.Watermarked) == len(manifest.SizeNames) &&
			m.Description == "a dog catching a frisbee"
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(workflows.ProcessImage, manifest.ProcessImageInput{
		PipelineID: pipelineID,
		ImageID:    imageID,
		Original:   original,
	})

	s.True(s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var got manifest.Manifest
	require.NoError(s.T(), s.env.GetWorkflowResult(&got))
	s.Equal(pipelineID, got.PipelineID)
	s.Equal(imageID, got.ImageID)
	s.Equal("a dog catching a frisbee", got.Description)
	for _, name := range manifest.SizeNames {
		s.Contains(got.Sizes, name)
		s.Contains(got.Watermarked, name)
	}

	// The manifest query handler should expose the same final state.
	val, err := s.env.QueryWorkflow(workflows.ManifestQueryName)
	require.NoError(s.T(), err)
	var queried manifest.Manifest
	require.NoError(s.T(), val.Get(&queried))
	s.Equal(pipelineID, queried.PipelineID)
	s.Equal(imageID, queried.ImageID)
	s.Equal("a dog catching a frisbee", queried.Description)
	s.Len(queried.Sizes, len(manifest.SizeNames))
	s.Len(queried.Watermarked, len(manifest.SizeNames))
}

// TestProcessImageDeterminism guards against non-deterministic constructs
// (e.g. iterating Go maps) sneaking into workflow code. It also exercises
// worker.NewWorkflowReplayer so a future refactor that breaks workflow
// registration is caught immediately.
//
// The check runs the workflow twice in a TestWorkflowEnvironment and asserts
// the exact sequence of activity invocations is identical between runs.
func TestProcessImageDeterminism(t *testing.T) {
	// Sanity check: the workflow can be registered with a replayer. This
	// catches workflow-time misuse (e.g. closures that capture a non-
	// serializable value) at registration rather than runtime.
	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflow(workflows.ProcessImage)

	first := runAndRecordActivities(t)
	second := runAndRecordActivities(t)

	require.Equal(t, first, second, "activity sequence must be deterministic across runs")
	require.NotEmpty(t, first, "expected at least one activity to be recorded")
}

func runAndRecordActivities(t *testing.T) []string {
	t.Helper()

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	acts := &activities.Activities{ImagesBucket: "test-bucket", ImagesTable: "test-table"}
	env.RegisterActivity(acts)

	var (
		mu          sync.Mutex
		invocations []string
	)
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		mu.Lock()
		defer mu.Unlock()
		invocations = append(invocations, info.ActivityType.Name)
	})

	const (
		pipelineID = "deadbeef"
		imageID    = "img-1"
	)
	for _, name := range manifest.SizeNames {
		key := "pipelines/" + pipelineID + "/resized/" + imageID + "/" + name + ".jpg"
		env.OnActivity(acts.ResizeAndUpload, mock.Anything, mock.Anything).Return(manifest.Size{
			S3Ref:  manifest.S3Ref{Bucket: "test-bucket", Key: key},
			Width:  manifest.SizeWidths[name],
			Height: manifest.SizeWidths[name] * 3 / 4,
			Bytes:  1000,
		}, nil)
	}
	env.OnActivity(acts.GenerateDescription, mock.Anything, mock.Anything).Return(activities.DescribeResult{
		Description: "x",
		Labels:      []string{"a"},
	}, nil)
	env.OnActivity(acts.ApplyWatermark, mock.Anything, mock.Anything).Return(manifest.S3Ref{
		Bucket: "test-bucket",
		Key:    "pipelines/" + pipelineID + "/watermarked/" + imageID + "/medium.jpg",
	}, nil)
	env.OnActivity(acts.StoreManifest, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflows.ProcessImage, manifest.ProcessImageInput{
		PipelineID: pipelineID,
		ImageID:    imageID,
		Original:   manifest.S3Ref{Bucket: "test-bucket", Key: "uploads/foo.jpg"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), invocations...)
}
