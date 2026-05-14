package activities

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"

	"github.com/alexandreroman/aws-image-processing-demo/internal/manifest"
	"go.temporal.io/sdk/activity"
	xdraw "golang.org/x/image/draw"
)

// WatermarkInput is the input of the ApplyWatermark activity.
type WatermarkInput struct {
	SessionID string
	ImageID   string
	SizeName  string
	Source    manifest.S3Ref
}

//go:embed assets/temporal-logo.png
var temporalLogoPNG []byte

// temporalLogo is the decoded source logo, cached at package init.
var temporalLogo = mustDecodeLogo()

func mustDecodeLogo() image.Image {
	img, err := png.Decode(bytes.NewReader(temporalLogoPNG))
	if err != nil {
		panic(fmt.Errorf("watermark: decode embedded logo: %w", err))
	}
	return img
}

// ApplyWatermark composites the Temporal logo on a rounded translucent plate at
// the bottom edge, centered horizontally, and uploads the result to the
// session's `watermarked/` prefix.
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
	stamped := stampWatermark(src)

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

// stampWatermark draws the Temporal logo on a rounded translucent plate at the
// bottom edge of src, centered horizontally.
func stampWatermark(src image.Image) image.Image {
	const (
		minLogoH = 12
		maxLogoH = 80
		padX     = 8
		padY     = 6
	)
	marginX, marginY := 8, 20

	bounds := src.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()

	logoBounds := temporalLogo.Bounds()
	logoOrigW, logoOrigH := logoBounds.Dx(), logoBounds.Dy()

	// Target logo height: ~11% of the shorter image side, clamped.
	logoH := min(imgW, imgH) * 11 / 100
	logoH = clamp(logoH, minLogoH, maxLogoH)
	logoW := logoH * logoOrigW / logoOrigH

	plateW := logoW + 2*padX
	plateH := logoH + 2*padY

	// Tiny image: shrink margin, then shrink the logo proportionally rather
	// than overflow the canvas.
	if plateW > imgW-2*marginX || plateH > imgH-2*marginY {
		marginX, marginY = 2, 2
	}
	if maxW, maxH := imgW-2*marginX, imgH-2*marginY; plateW > maxW || plateH > maxH {
		scaleW := float64(maxW-2*padX) / float64(logoW)
		scaleH := float64(maxH-2*padY) / float64(logoH)
		scale := min(scaleW, scaleH)
		if scale < 0 {
			scale = 0
		}
		logoW = int(float64(logoW) * scale)
		logoH = int(float64(logoH) * scale)
		plateW = logoW + 2*padX
		plateH = logoH + 2*padY
	}

	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	if logoW <= 0 || logoH <= 0 {
		return dst
	}

	radius := max(4, logoH/4)
	if m := min(plateW, plateH) / 2; radius > m {
		radius = m
	}

	plateX0 := bounds.Min.X + (imgW-plateW)/2
	plateY0 := bounds.Max.Y - plateH - marginY
	plateRect := image.Rect(plateX0, plateY0, plateX0+plateW, plateY0+plateH)

	mask := roundedRectMask(plateW, plateH, radius)
	plateColor := &image.Uniform{C: color.NRGBA{R: 0, G: 0, B: 0, A: 160}}
	draw.DrawMask(dst, plateRect, plateColor, image.Point{}, mask, image.Point{}, draw.Over)

	logo := image.NewNRGBA(image.Rect(0, 0, logoW, logoH))
	xdraw.CatmullRom.Scale(logo, logo.Bounds(), temporalLogo, logoBounds, xdraw.Over, nil)

	logoRect := image.Rect(plateX0+padX, plateY0+padY, plateX0+padX+logoW, plateY0+padY+logoH)
	draw.Draw(dst, logoRect, logo, image.Point{}, draw.Over)

	return dst
}

// roundedRectMask returns an alpha mask of size w×h with rounded corners of
// radius r. Corners are aliased; the plate is small enough that this is fine.
func roundedRectMask(w, h, r int) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	if r <= 0 {
		for i := range mask.Pix {
			mask.Pix[i] = 255
		}
		return mask
	}
	r2 := r * r
	for y := 0; y < h; y++ {
		// Distance to the nearest horizontal edge of the inner rect.
		var dy int
		switch {
		case y < r:
			dy = r - y
		case y >= h-r:
			dy = y - (h - r - 1)
		}
		for x := 0; x < w; x++ {
			var dx int
			switch {
			case x < r:
				dx = r - x
			case x >= w-r:
				dx = x - (w - r - 1)
			}
			var a uint8
			switch {
			case dx == 0 || dy == 0:
				a = 255
			case dx*dx+dy*dy <= r2:
				a = 255
			}
			mask.Pix[y*mask.Stride+x] = a
		}
	}
	return mask
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
