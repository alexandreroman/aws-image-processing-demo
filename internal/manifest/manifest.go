// Package manifest holds the shared types passed between the workflow,
// activities, API handlers, and the manifest stored in DynamoDB.
package manifest

// S3Ref points to an object in S3.
type S3Ref struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// Size describes one resized version of an image.
type Size struct {
	S3Ref  S3Ref `json:"s3Ref"`
	Width  int   `json:"width"`
	Height int   `json:"height"`
	Bytes  int64 `json:"bytes"`
}

// Manifest is the final record persisted to DynamoDB once a ProcessImage
// workflow completes.
type Manifest struct {
	PipelineID  string           `json:"pipelineId"`
	ImageID     string           `json:"imageId"`
	Original    S3Ref            `json:"original"`
	Sizes       map[string]Size  `json:"sizes"`
	Description string           `json:"description,omitempty"`
	Labels      []string         `json:"labels,omitempty"`
	Watermarked map[string]S3Ref `json:"watermarked,omitempty"`
}

// ProcessImageInput is the input of the ProcessImage workflow.
type ProcessImageInput struct {
	PipelineID string `json:"pipelineId"`
	ImageID    string `json:"imageId"`
	Original   S3Ref  `json:"original"`
}

// SizeNames is the canonical, ordered list of size keys. Workflow code MUST
// iterate this slice (Go map iteration is non-deterministic and would break
// replay).
var SizeNames = []string{"small", "medium", "large"}

// SizeWidths maps each size name to its target width in pixels. Only the
// values are read by name — never iterate the map directly in workflow code.
var SizeWidths = map[string]int{
	"small":  150,
	"medium": 480,
	"large":  1080,
}
