// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

func TestOriginalPlanFor(t *testing.T) {
	t.Parallel()
	cases := map[string]domain.OriginalPlan{
		"image/jpeg":          domain.PlanStripJPEG,
		"image/png":           domain.PlanStripPNG,
		"image/apng":          domain.PlanStripPNG,
		"image/gif":           domain.PlanReencodeGIF,
		"image/webp":          domain.PlanConvertWebP,
		"image/svg+xml":       domain.PlanNone,
		"image/heic":          domain.PlanNone,
		"image/heif":          domain.PlanNone,
		"image/avif":          domain.PlanNone,
		"image/tiff":          domain.PlanNone,
		"application/pdf":     domain.PlanNone,
		"video/mp4":           domain.PlanNone,
		"":                    domain.PlanNone,
		"Image/JPEG":          domain.PlanNone,
		"image/jpeg; x=y":     domain.PlanNone,
		"application/x-image": domain.PlanNone,
	}
	for in, want := range cases {
		if got := domain.OriginalPlanFor(in); got != want {
			t.Errorf("OriginalPlanFor(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHasContainerMetadata(t *testing.T) {
	t.Parallel()
	for _, yes := range []string{"video/mp4", "video/quicktime", "video/x-m4v", "audio/mp4", "audio/x-m4a"} {
		if !domain.HasContainerMetadata(yes) {
			t.Errorf("%q has container metadata", yes)
		}
	}
	for _, no := range []string{"image/heic", "image/heif", "image/avif", "video/3gpp", "video/webm", "audio/mpeg", "image/jpeg", "application/octet-stream", ""} {
		if domain.HasContainerMetadata(no) {
			t.Errorf("%q has no container metadata", no)
		}
	}
}

func TestExceedsPixelLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		w, h, frames, limit int64
		want                bool
	}{
		{100, 100, 1, 10000, false},
		{100, 101, 1, 10000, true},
		{100, 50, 2, 10000, false},
		{100, 50, 3, 10000, true},
		{100, 100, 0, 10000, false},
		{math.MaxInt64, 2, 1, math.MaxInt64, true},
		{1 << 32, 1 << 32, 1, math.MaxInt64, true},
		{1 << 20, 1 << 20, 1 << 30, math.MaxInt64, true},
		{-1, 10, 1, 10000, true},
		{10, 10, -1, 10000, true},
		{1, 1, 1, 0, true},
	}
	for _, c := range cases {
		if got := domain.ExceedsPixelLimit(c.w, c.h, c.frames, c.limit); got != c.want {
			t.Errorf("ExceedsPixelLimit(%d, %d, %d, %d) = %v, want %v", c.w, c.h, c.frames, c.limit, got, c.want)
		}
	}
}

func TestVariantSize(t *testing.T) {
	t.Parallel()
	cases := []struct{ w, h, edge, ww, wh int }{
		{4000, 3000, 320, 320, 240},
		{3000, 4000, 320, 240, 320},
		{1000, 1000, 320, 320, 320},
		{100, 50, 320, 100, 50},
		{320, 200, 320, 320, 200},
		{3000, 1001, 1024, 1024, 342},
		{3000, 1, 320, 320, 1},
		{1, 3000, 320, 1, 320},
		{1000, 3, 320, 320, 1},
	}
	for _, c := range cases {
		if w, h := domain.VariantSize(c.w, c.h, c.edge); w != c.ww || h != c.wh {
			t.Errorf("VariantSize(%d, %d, %d) = %dx%d, want %dx%d", c.w, c.h, c.edge, w, h, c.ww, c.wh)
		}
	}
	if got := domain.RenderedVariants(); len(got) != 3 || got[0] != domain.VariantSmall || got[1] != domain.VariantMedium || got[2] != domain.VariantLarge {
		t.Errorf("RenderedVariants() = %v", got)
	}
}

func TestOrientationDisplaySize(t *testing.T) {
	t.Parallel()
	for o := domain.Orientation(1); o <= 8; o++ {
		w, h := o.DisplaySize(40, 30)
		swapped := o >= 5
		if swapped && (w != 30 || h != 40) || !swapped && (w != 40 || h != 30) {
			t.Errorf("orientation %d gives %dx%d", o, w, h)
		}
	}
	if w, h := domain.Orientation(0).DisplaySize(40, 30); w != 40 || h != 30 {
		t.Errorf("orientation 0 gives %dx%d", w, h)
	}
	if !domain.Orientation(8).Valid() || domain.Orientation(9).Valid() || domain.Orientation(0).Valid() {
		t.Error("Valid")
	}
}

func TestAllowLists(t *testing.T) {
	t.Parallel()
	kept := []byte{0xD8, 0xD9, 0xDB, 0xC4, 0xCC, 0xDD, 0xDC, 0xDE, 0xDF, 0xDA, 0xE0, 0xE1, 0xE2, 0xEE}
	for m := byte(0xC0); m <= 0xCF; m++ {
		kept = append(kept, m) // SOF0 to SOF15 with DHT, JPG and DAC among them
	}
	for m := byte(0xD0); m <= 0xD7; m++ {
		kept = append(kept, m)
	}
	keptSet := map[byte]bool{}
	for _, m := range kept {
		keptSet[m] = true
		if m != 0xC8 && !domain.JPEGMarkerKept(m) {
			t.Errorf("marker %#x is kept", m)
		}
	}
	for _, m := range []byte{0xC8, 0xE3, 0xE4, 0xED, 0xEF, 0xFE, 0xF0, 0xFD, 0x01, 0x02, 0xBF} {
		if domain.JPEGMarkerKept(m) {
			t.Errorf("marker %#x is dropped", m)
		}
	}
	for _, c := range []string{"IHDR", "PLTE", "IDAT", "IEND", "tRNS", "cHRM", "gAMA", "iCCP", "sBIT", "sRGB", "cICP", "mDCV", "cLLI", "bKGD", "pHYs", "acTL", "fcTL", "fdAT"} {
		if !domain.PNGChunkKept(c) {
			t.Errorf("chunk %s is kept", c)
		}
	}
	for _, c := range []string{"tEXt", "zTXt", "iTXt", "eXIf", "tIME", "hIST", "sPLT", "prVt", "vpAg", "caBX", "idat", ""} {
		if domain.PNGChunkKept(c) {
			t.Errorf("chunk %q is dropped", c)
		}
	}
}
