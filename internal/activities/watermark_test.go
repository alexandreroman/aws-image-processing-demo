package activities

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

func TestTemporalLogoDecodes(t *testing.T) {
	if temporalLogo == nil {
		t.Fatal("temporalLogo is nil; embedded asset failed to decode")
	}
	b := temporalLogo.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		t.Errorf("temporalLogo has non-positive dimensions: %dx%d", b.Dx(), b.Dy())
	}
}

func TestStampWatermarkSizes(t *testing.T) {
	sizes := []struct {
		w, h      int
		wantStamp bool
	}{
		{16, 16, false}, // under the 32 px floor: no room for a readable mark
		{150, 100, true},
		{320, 240, true},
		{1024, 768, true},
		{4096, 3000, true},
	}

	for _, s := range sizes {
		t.Run(fmt.Sprintf("%dx%d", s.w, s.h), func(t *testing.T) {
			src := flatGreyImage(s.w, s.h)

			out := stampWatermark(src)
			if out == nil {
				t.Fatal("stampWatermark returned nil")
			}
			if got, want := out.Bounds(), src.Bounds(); got != want {
				t.Fatalf("bounds = %v, want %v", got, want)
			}

			if !s.wantStamp {
				if n := countDifferingPixels(out, src, src.Bounds()); n != 0 {
					t.Errorf("image left unmodified: %d pixels changed, want 0", n)
				}
				return
			}

			plate := plateRect(s.w, s.h)
			if n := countDifferingPixels(out, src, plate); n == 0 {
				t.Errorf("no pixel changed inside the watermark plate %v", plate)
			}
		})
	}
}

// flatGreyImage builds a uniformly grey image, so any pixel that differs
// afterwards was drawn by the watermark.
func flatGreyImage(w, h int) *image.NRGBA {
	grey := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = grey.R, grey.G, grey.B, grey.A
	}
	return img
}

// plateRect mirrors the plate geometry of stampWatermark, narrowed to a few
// columns around the horizontal center so it stays within the plate whatever
// the logo aspect ratio is.
func plateRect(w, h int) image.Rectangle {
	const (
		minLogoH = 12
		maxLogoH = 80
		padY     = 6
		marginY  = 20
	)
	logoH := min(max(min(w, h)*11/100, minLogoH), maxLogoH)
	plateH := logoH + 2*padY
	bottom := h - marginY
	return image.Rect(w/2-2, bottom-plateH, w/2+2, bottom)
}

// countDifferingPixels counts the pixels of r whose color differs between got
// and want. Colors are compared on their RGBA components, since the two
// images need not share a pixel format.
func countDifferingPixels(got, want image.Image, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				n++
			}
		}
	}
	return n
}
