// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"runtime"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The sizes the huge headers declare: far above testMaxPixels, and far more than the test
// could allocate if anything decoded them.
const (
	hugePNGEdge  = 100_000
	hugeJPEGEdge = 60_000
	// manyFrames 64x64 frames are 1.2 million pixels, above testMaxPixels.
	manyFrames    = 300
	manyFrameEdge = 64
)

// hugePNG returns a PNG whose IHDR declares hugePNGEdge x hugePNGEdge RGB pixels, with a tiny
// IDAT that holds almost nothing of them.
func hugePNG(tb testing.TB) []byte {
	tb.Helper()
	ihdr := binary.BigEndian.AppendUint32(nil, hugePNGEdge)
	ihdr = binary.BigEndian.AppendUint32(ihdr, hugePNGEdge)
	ihdr = append(ihdr, 8, 2, 0, 0, 0) // depth 8, colour type RGB, deflate, filter 0, no interlace
	idat := zlibTest(tb, make([]byte, 16))
	out := []byte(pngSignature)
	out = append(out, pngChunk(chunkIHDR, ihdr)...)
	out = append(out, pngChunk(testChunkIDAT, idat)...)
	return append(out, pngChunk(chunkIEND, nil)...)
}

// hugeJPEG returns a small encoded JPEG whose SOF0 is patched to declare hugeJPEGEdge x
// hugeJPEGEdge pixels.
func hugeJPEG(tb testing.TB) []byte {
	tb.Helper()
	out := encodeTestJPEG(tb, twoColour(16, 16, color.Black, color.White))
	off := 0
	for _, s := range jpegSegments(tb, out) {
		if s.Marker == markerSOF0 {
			// Marker (2), length (2), precision (1), then height and width.
			binary.BigEndian.PutUint16(out[off+5:], hugeJPEGEdge)
			binary.BigEndian.PutUint16(out[off+7:], hugeJPEGEdge)
			return out
		}
		off += len(s.Bytes)
	}
	tb.Fatalf("no SOF0")
	return nil
}

// manyFramesGIF returns a GIF of manyFrames frames of manyFrameEdge x manyFrameEdge pixels.
func manyFramesGIF(tb testing.TB) []byte {
	tb.Helper()
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{Config: image.Config{ColorModel: pal, Width: manyFrameEdge, Height: manyFrameEdge}}
	for range manyFrames {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, manyFrameEdge, manyFrameEdge), pal))
		g.Delay = append(g.Delay, 1)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		tb.Fatalf("EncodeAll: %v", err)
	}
	return buf.Bytes()
}

// TestPixelGuardDoesNotDecode renders images whose headers declare more pixels than the limit
// (SEC-T08 decompression bomb, 12 15 huge dimensions): RenderVariants returns ErrTooManyPixels,
// the decoder is never called and the call allocates less than 4 bytes per pixel of the limit.
// The JPEG and PNG originals are still cleaned, because the filters never decode. Not parallel:
// it measures the allocations of the process.
func TestPixelGuardDoesNotDecode(t *testing.T) {
	for _, tc := range []struct {
		name, mimeType string
		data           []byte
		plan           domain.OriginalPlan
		width, height  int
	}{
		{"png", mimePNG, hugePNG(t), domain.PlanStripPNG, hugePNGEdge, hugePNGEdge},
		{"jpeg", mimeJPEG, hugeJPEG(t), domain.PlanStripJPEG, hugeJPEGEdge, hugeJPEGEdge},
		{"gif", mimeGIF, manyFramesGIF(t), domain.PlanReencodeGIF, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkGuardRefuses(t, tc.mimeType, tc.data)
			if tc.plan == domain.PlanReencodeGIF {
				return
			}
			c := cleanTest(t, tc.plan, tc.data)
			if c.Width != tc.width || c.Height != tc.height {
				t.Fatalf("cleaned %dx%d, want %dx%d", c.Width, c.Height, tc.width, tc.height)
			}
			checkBody(t, c)
		})
	}
}

// checkGuardRefuses runs RenderVariants over data with a counting decoder and fails the test
// unless it returns ErrTooManyPixels without a decode and within the allocation bound.
func checkGuardRefuses(t *testing.T, mimeType string, data []byte) {
	t.Helper()
	decodes := 0
	c := Codec{decode: func(string, io.Reader) (image.Image, error) {
		decodes++
		return nil, errors.New("decoded")
	}}
	spec := app.RenderSpec{MIMEType: mimeType, Variants: testVariants, JPEGQuality: testJPEGQuality, MaxPixels: testMaxPixels}
	ctx := t.Context()
	open := openerOf(data)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := c.RenderVariants(ctx, open, spec)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, app.ErrTooManyPixels) {
		t.Fatalf("got %v, want ErrTooManyPixels", err)
	}
	if decodes != 0 {
		t.Fatalf("decoded %d times above the pixel limit", decodes)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown >= 4*testMaxPixels {
		t.Fatalf("allocated %d bytes, want below %d", grown, 4*testMaxPixels)
	}
}
