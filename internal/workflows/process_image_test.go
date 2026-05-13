package workflows_test

import (
	"testing"

	"github.com/alexandreroman/aws-image-processing-demo/internal/activities"
	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/alexandreroman/aws-image-processing-demo/internal/workflows"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
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
		sessionID = "deadbeef"
		imageID   = "img-1"
	)
	original := manifest.S3Ref{Bucket: "test-bucket", Key: "uploads/foo.jpg"}

	// Expect one resize per size name.
	for _, name := range manifest.SizeNames {
		size := name
		key := "sessions/" + sessionID + "/resized/" + imageID + "/" + size + ".jpg"
		s.env.OnActivity(s.acts.ResizeAndUpload, mock.Anything, mock.MatchedBy(func(in activities.ResizeInput) bool {
			return in.SessionID == sessionID && in.ImageID == imageID && in.SizeName == size
		})).Return(manifest.Size{
			S3Ref:  manifest.S3Ref{Bucket: "test-bucket", Key: key},
			Width:  manifest.SizeWidths[size],
			Height: manifest.SizeWidths[size],
			Bytes:  1000,
		}, nil).Once()
	}

	// One describe call on the medium size.
	mediumKey := "sessions/" + sessionID + "/resized/" + imageID + "/medium.jpg"
	s.env.OnActivity(s.acts.GenerateDescription, mock.Anything, mock.MatchedBy(func(ref manifest.S3Ref) bool {
		return ref.Key == mediumKey
	})).Return(activities.DescribeResult{
		Description: "a dog catching a frisbee",
		Labels:      []string{"dog", "beach", "pet"},
	}, nil).Once()

	// One watermark per size.
	for _, name := range manifest.SizeNames {
		size := name
		s.env.OnActivity(s.acts.ApplyWatermark, mock.Anything, mock.MatchedBy(func(in activities.WatermarkInput) bool {
			return in.SessionID == sessionID && in.ImageID == imageID && in.SizeName == size
		})).Return(manifest.S3Ref{
			Bucket: "test-bucket",
			Key:    "sessions/" + sessionID + "/watermarked/" + imageID + "/" + size + ".jpg",
		}, nil).Once()
	}

	// One store call.
	s.env.OnActivity(s.acts.StoreManifest, mock.Anything, mock.MatchedBy(func(m manifest.Manifest) bool {
		return m.SessionID == sessionID &&
			m.ImageID == imageID &&
			len(m.Sizes) == len(manifest.SizeNames) &&
			len(m.Watermarked) == len(manifest.SizeNames) &&
			m.Description == "a dog catching a frisbee"
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(workflows.ProcessImage, manifest.ProcessImageInput{
		SessionID: sessionID,
		ImageID:   imageID,
		Original:  original,
	})

	s.True(s.env.IsWorkflowCompleted())
	require.NoError(s.T(), s.env.GetWorkflowError())

	var got manifest.Manifest
	require.NoError(s.T(), s.env.GetWorkflowResult(&got))
	s.Equal(sessionID, got.SessionID)
	s.Equal(imageID, got.ImageID)
	s.Equal("a dog catching a frisbee", got.Description)
	for _, name := range manifest.SizeNames {
		s.Contains(got.Sizes, name)
		s.Contains(got.Watermarked, name)
	}
}
