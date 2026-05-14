package activities

import (
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
	sizes := []struct{ w, h int }{
		{150, 100},
		{320, 240},
		{1024, 768},
		{4096, 3000},
	}
	grey := color.NRGBA{R: 128, G: 128, B: 128, A: 255}
	for _, s := range sizes {
		t.Run("", func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, s.w, s.h))
			for i := 0; i < len(src.Pix); i += 4 {
				src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = grey.R, grey.G, grey.B, grey.A
			}
			out := stampWatermark(src)
			if out == nil {
				t.Fatalf("stampWatermark(%dx%d) returned nil", s.w, s.h)
			}
			if got, want := out.Bounds(), src.Bounds(); got != want {
				t.Errorf("stampWatermark(%dx%d): bounds = %v, want %v", s.w, s.h, got, want)
			}
		})
	}
}
