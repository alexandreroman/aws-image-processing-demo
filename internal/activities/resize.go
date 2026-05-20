package activities

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"  // decoder registration
	"image/jpeg"
	_ "image/png" // decoder registration
	"io"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/disintegration/imaging"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// ResizeInput is the input of the ResizeAndUpload activity.
type ResizeInput struct {
	PipelineID string
	ImageID    string
	SizeName   string
	Original   manifest.S3Ref
}

// jpegQuality is intentionally on the lower side: this is a demo, and the
// derived artifacts only need to look good as gallery thumbnails.
const jpegQuality = 85

// maxImageBytes caps the size of objects pulled from S3. With a 1 GiB worker
// task and Go's image decoder allocating several times the raw size,
// anything past ~25 MiB risks OOM. Enforced via Content-Length on GetObject.
const maxImageBytes = 25 * 1024 * 1024

// ResizeAndUpload downloads the original image, scales it to the target
// width (keeping aspect), re-encodes as JPEG, and uploads it to a
// pipeline-scoped S3 key.
func (a *Activities) ResizeAndUpload(ctx context.Context, in ResizeInput) (manifest.Size, error) {
	width, ok := manifest.SizeWidths[in.SizeName]
	if !ok {
		return manifest.Size{}, fmt.Errorf("resize: unknown size %q", in.SizeName)
	}

	logger := activity.GetLogger(ctx)
	logger.Info("resize start", "imageId", in.ImageID, "size", in.SizeName, "width", width)
	activity.RecordHeartbeat(ctx, "download")

	raw, err := a.download(ctx, in.Original)
	if err != nil {
		return manifest.Size{}, fmt.Errorf("resize: download: %w", err)
	}

	activity.RecordHeartbeat(ctx, "decode")
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return manifest.Size{}, fmt.Errorf("resize: decode: %w", err)
	}

	activity.RecordHeartbeat(ctx, "resize")
	// Resize width while preserving aspect ratio (height=0 => auto).
	dst := imaging.Resize(src, width, 0, imaging.Lanczos)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return manifest.Size{}, fmt.Errorf("resize: encode: %w", err)
	}

	key := resizedKey(in.PipelineID, in.ImageID, in.SizeName)
	activity.RecordHeartbeat(ctx, "upload")
	if err := a.upload(ctx, key, buf.Bytes(), "image/jpeg"); err != nil {
		return manifest.Size{}, fmt.Errorf("resize: upload: %w", err)
	}

	logger.Info("resize done", "imageId", in.ImageID, "size", in.SizeName, "bytes", buf.Len())
	return manifest.Size{
		S3Ref:  manifest.S3Ref{Key: key},
		Width:  dst.Bounds().Dx(),
		Height: dst.Bounds().Dy(),
		Bytes:  int64(buf.Len()),
	}, nil
}

func (a *Activities) download(ctx context.Context, ref manifest.S3Ref) ([]byte, error) {
	out, err := a.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.ImagesBucket),
		Key:    aws.String(ref.Key),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	if out.ContentLength != nil && *out.ContentLength > maxImageBytes {
		return nil, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("image too large: %d bytes (max %d)", *out.ContentLength, maxImageBytes),
			"ImageTooLarge", nil,
		)
	}

	return io.ReadAll(out.Body)
}

func (a *Activities) upload(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := a.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.ImagesBucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	return err
}
