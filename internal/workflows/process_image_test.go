package workflows_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
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
	original := manifest.S3Ref{Key: "samples/foo.jpg"}

	// Expect one resize per size name.
	for _, name := range manifest.SizeNames {
		key := "pipelines/" + pipelineID + "/resized/" + imageID + "/" + name + ".jpg"
		s.env.OnActivity(s.acts.ResizeAndUpload, mock.Anything, mock.MatchedBy(func(in activities.ResizeInput) bool {
			return in.PipelineID == pipelineID && in.ImageID == imageID && in.SizeName == name
		})).Return(manifest.Size{
			S3Ref:  manifest.S3Ref{Key: key},
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
			Key: "pipelines/" + pipelineID + "/watermarked/" + imageID + "/" + name + ".jpg",
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
	s.Require().NoError(s.env.GetWorkflowError())

	var got manifest.Manifest
	s.Require().NoError(s.env.GetWorkflowResult(&got))
	s.Equal(pipelineID, got.PipelineID)
	s.Equal(imageID, got.ImageID)
	s.Equal("a dog catching a frisbee", got.Description)
	for _, name := range manifest.SizeNames {
		s.Contains(got.Sizes, name)
		s.Contains(got.Watermarked, name)
	}

	// The manifest query handler should expose the same final state.
	val, err := s.env.QueryWorkflow(workflows.ManifestQueryName)
	s.Require().NoError(err)
	var queried manifest.Manifest
	s.Require().NoError(val.Get(&queried))
	s.Equal(pipelineID, queried.PipelineID)
	s.Equal(imageID, queried.ImageID)
	s.Equal("a dog catching a frisbee", queried.Description)
	s.Len(queried.Sizes, len(manifest.SizeNames))
	s.Len(queried.Watermarked, len(manifest.SizeNames))
}

// TestProcessImageDeterminism guards against non-deterministic constructs
// (e.g. iterating a Go map) sneaking into workflow code. Every recorded entry
// carries the size the activity was scheduled for, so the expected sequence
// pins the fan-out to manifest.SizeNames order — ranging manifest.SizeWidths
// instead reorders it.
//
// The workflow is executed repeatedly because Go randomizes iteration over a
// small map only weakly: three keys still come out in insertion order about
// three times out of four, so a single execution would let the regression
// through more often than not.
func TestProcessImageDeterminism(t *testing.T) {
	const runs = 10

	want := make([]string, 0, 2*len(manifest.SizeNames)+2)
	for _, name := range manifest.SizeNames {
		want = append(want, "ResizeAndUpload:"+name)
	}
	want = append(want, "GenerateDescription")
	for _, name := range manifest.SizeNames {
		want = append(want, "ApplyWatermark:"+name)
	}
	want = append(want, "StoreManifest")

	for range runs {
		require.Equal(t, want, runAndRecordActivities(t),
			"activities must be scheduled in manifest.SizeNames order, identically on every run")
	}
}

func runAndRecordActivities(t *testing.T) []string {
	t.Helper()

	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	acts := &activities.Activities{ImagesBucket: "test-bucket", ImagesTable: "test-table"}
	env.RegisterActivity(acts)

	rec := &scheduleRecorder{}
	env.SetWorkerOptions(worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{rec},
	})

	const (
		pipelineID = "deadbeef"
		imageID    = "img-1"
	)
	for _, name := range manifest.SizeNames {
		key := "pipelines/" + pipelineID + "/resized/" + imageID + "/" + name + ".jpg"
		env.OnActivity(acts.ResizeAndUpload, mock.Anything, mock.MatchedBy(func(in activities.ResizeInput) bool {
			return in.SizeName == name
		})).Return(manifest.Size{
			S3Ref:  manifest.S3Ref{Key: key},
			Width:  manifest.SizeWidths[name],
			Height: manifest.SizeWidths[name] * 3 / 4,
			Bytes:  1000,
		}, nil).Once()
	}
	env.OnActivity(acts.GenerateDescription, mock.Anything, mock.Anything).Return(activities.DescribeResult{
		Description: "x",
		Labels:      []string{"a"},
	}, nil)
	env.OnActivity(acts.ApplyWatermark, mock.Anything, mock.Anything).Return(manifest.S3Ref{
		Key: "pipelines/" + pipelineID + "/watermarked/" + imageID + "/medium.jpg",
	}, nil)
	env.OnActivity(acts.StoreManifest, mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflows.ProcessImage, manifest.ProcessImageInput{
		PipelineID: pipelineID,
		ImageID:    imageID,
		Original:   manifest.S3Ref{Key: "samples/foo.jpg"},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	return rec.scheduled()
}

// scheduleRecorder records, in order, every activity a workflow schedules.
//
// It hooks the outbound interceptor rather than SetOnActivityStartedListener
// because interception happens on the workflow goroutine: the recorded order
// is the order in which the workflow emitted its commands. Activities, by
// contrast, start on their own goroutines and in arbitrary order, which is
// precisely what determinism does not depend on.
type scheduleRecorder struct {
	interceptor.WorkerInterceptorBase

	mu      sync.Mutex
	entries []string
}

func (r *scheduleRecorder) InterceptWorkflow(
	_ workflow.Context, next interceptor.WorkflowInboundInterceptor,
) interceptor.WorkflowInboundInterceptor {
	return &recordingInbound{
		WorkflowInboundInterceptorBase: interceptor.WorkflowInboundInterceptorBase{Next: next},
		recorder:                       r,
	}
}

// scheduled returns a copy of what has been recorded so far.
func (r *scheduleRecorder) scheduled() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.entries)
}

// record appends "<activity type>:<size name>", the size being empty for the
// activities that do not take one (describe, store).
func (r *scheduleRecorder) record(activityType string, args []any) {
	entry := activityType
	if len(args) > 0 {
		switch in := args[0].(type) {
		case activities.ResizeInput:
			entry += ":" + in.SizeName
		case activities.WatermarkInput:
			entry += ":" + in.SizeName
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

type recordingInbound struct {
	interceptor.WorkflowInboundInterceptorBase
	recorder *scheduleRecorder
}

func (i *recordingInbound) Init(outbound interceptor.WorkflowOutboundInterceptor) error {
	return i.Next.Init(&recordingOutbound{
		WorkflowOutboundInterceptorBase: interceptor.WorkflowOutboundInterceptorBase{Next: outbound},
		recorder:                        i.recorder,
	})
}

type recordingOutbound struct {
	interceptor.WorkflowOutboundInterceptorBase
	recorder *scheduleRecorder
}

func (o *recordingOutbound) ExecuteActivity(
	ctx workflow.Context, activityType string, args ...any,
) workflow.Future {
	o.recorder.record(activityType, args)
	return o.Next.ExecuteActivity(ctx, activityType, args...)
}
