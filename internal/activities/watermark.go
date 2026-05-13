package activities

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"

	"github.com/alexandreroman/temporal-aws-autoscaling-demo/internal/manifest"
	"go.temporal.io/sdk/activity"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// WatermarkInput is the input of the ApplyWatermark activity.
type WatermarkInput struct {
	SessionID string
	ImageID   string
	SizeName  string
	Source    manifest.S3Ref
}

const watermarkText = "temporal demo"

// ApplyWatermark stamps a small bottom-right label on the source image and
// uploads the result to the session-scoped `watermarked/` prefix.
func (a *Activities) ApplyWatermark(ctx context.Context, in WatermarkInput) (manifest.S3Ref, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("watermark start", "imageId", in.ImageID, "size", in.SizeName)
	activity.RecordHeartbeat(ctx, "download")

	raw, err := a.download(ctx, in.Source)
	if err != nil {
		return manifest.S3Ref{}, fmt.Errorf("watermark: download: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return manifest.S3Ref{}, fmt.Errorf("watermark: decode: %w", err)
	}

	activity.RecordHeartbeat(ctx, "stamp")
	stamped := stampWatermark(src, watermarkText)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, stamped, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return manifest.S3Ref{}, fmt.Errorf("watermark: encode: %w", err)
	}

	key := watermarkedKey(in.SessionID, in.ImageID, in.SizeName)
	activity.RecordHeartbeat(ctx, "upload")
	if err := a.upload(ctx, key, buf.Bytes(), "image/jpeg"); err != nil {
		return manifest.S3Ref{}, fmt.Errorf("watermark: upload: %w", err)
	}

	logger.Info("watermark done", "imageId", in.ImageID, "size", in.SizeName)
	return manifest.S3Ref{Bucket: a.ImagesBucket, Key: key}, nil
}

// stampWatermark draws white text on a semi-transparent black pill at the
// bottom-right corner. basicfont.Face7x13 is fixed-width, no font file
// shipping required.
func stampWatermark(src image.Image, label string) image.Image {
	face := basicfont.Face7x13
	textWidth := font.MeasureString(face, label).Ceil()
	textHeight := face.Metrics().Ascent.Ceil() + face.Metrics().Descent.Ceil()

	padX, padY := 6, 4
	margin := 8
	boxW := textWidth + 2*padX
	boxH := textHeight + 2*padY

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	// Center the pill within the bottom-right corner; clamp to image size
	// for very small thumbnails (the small variant is only 150px wide).
	if boxW > bounds.Dx()-2*margin {
		margin = 2
	}
	x0 := bounds.Max.X - boxW - margin
	y0 := bounds.Max.Y - boxH - margin
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}

	pillRect := image.Rect(x0, y0, x0+boxW, y0+boxH)
	pillColor := color.NRGBA{R: 0, G: 0, B: 0, A: 160}
	draw.Draw(dst, pillRect, &image.Uniform{C: pillColor}, image.Point{}, draw.Over)

	drawer := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(x0 + padX),
			Y: fixed.I(y0 + padY + face.Metrics().Ascent.Ceil()),
		},
	}
	drawer.DrawString(label)
	return dst
}
