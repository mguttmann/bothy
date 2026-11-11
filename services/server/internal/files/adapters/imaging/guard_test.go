// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
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

// longEdge is an edge far above domain.MaxImageEdge; a PNG of one pixel by longEdge has fewer
// pixels than testMaxPixels, so only the edge rule can refuse it.
const longEdge = 900_000

// longPNG returns a gray PNG of w x h pixels, encoded with image/png: a real file of a few
// kilobytes whose scaling tables would grow with its longest edge.
func longPNG(tb testing.TB, w, h int) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		tb.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// tinyFramesGIF returns a GIF of frames frames of 1x1 pixel on a 1x1 screen with a two-colour
// global table. Every frame has a graphic control extension whose transparency index 255 lies
// outside that table, so image/gif keeps a palette of 256 entries per frame and writes a local
// colour table per frame when it encodes: 23 bytes of file per frame (ruling AUD-SF01/2 in
// docs/decisions.d/AUD-srv-files-01.md).
func tinyFramesGIF(tb testing.TB, frames int) []byte {
	tb.Helper()
	var b bytes.Buffer
	b.WriteString("GIF89a")
	b.Write([]byte{1, 0, 1, 0, gifColorTableFlag, 0, 0}) // 1x1 screen, global table of two entries
	b.Write([]byte{0, 0, 0, 255, 255, 255})
	for range frames {
		b.Write([]byte{gifExtension, gifGraphicControl, gifGraphicControlSize, 1, 0, 0, 255, 0})
		b.Write([]byte{gifImage, 0, 0, 0, 0, 1, 0, 1, 0, 0})
		b.Write([]byte{2, 2, 0x44, 0x01, 0}) // code size 2, one sub-block: clear, index 0, end
	}
	b.WriteByte(gifTrailer)
	return b.Bytes()
}

// TestPixelGuardDoesNotDecode renders images whose headers declare more pixels than the limit
// (SEC-T08 decompression bomb, 12 15 huge dimensions) or an edge above domain.MaxImageEdge with
// few pixels (12 7.2): RenderVariants returns ErrTooManyPixels, the decoder is never called and
// the call allocates less than 4 bytes per pixel of the limit. The JPEG and PNG originals are
// still cleaned, because the filters never decode. Not parallel: it measures the allocations of
// the process.
func TestPixelGuardDoesNotDecode(t *testing.T) {
	for _, tc := range []struct {
		name, mimeType string
		data           []byte
		plan           domain.OriginalPlan
		width, height  int
	}{
		{"png", mimePNG, hugePNG(t), domain.PlanStripPNG, hugePNGEdge, hugePNGEdge},
		{"jpeg", mimeJPEG, hugeJPEG(t), domain.PlanStripJPEG, hugeJPEGEdge, hugeJPEGEdge},
		{"png tall", mimePNG, longPNG(t, 1, longEdge), domain.PlanStripPNG, 1, longEdge},
		{"png wide", mimePNG, longPNG(t, longEdge, 1), domain.PlanStripPNG, longEdge, 1},
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
		t.Fatalf("decoded %d times behind the pixel guard", decodes)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown >= 4*testMaxPixels {
		t.Fatalf("allocated %d bytes, want below %d", grown, 4*testMaxPixels)
	}
}

// TestLongEdgeGetsNoThumbnails runs the real decoder and scaler over real PNGs around
// domain.MaxImageEdge (12 7.2, ruling AUD-SF01/1): the scaler keeps tables as long as the source
// edge, so one pixel above the limit on either edge is refused although the image has far fewer
// pixels than the limit, and an image at the limit is rendered.
func TestLongEdgeGetsNoThumbnails(t *testing.T) {
	spec := app.RenderSpec{MIMEType: mimePNG, Variants: testVariants, JPEGQuality: testJPEGQuality, MaxPixels: testMaxPixels}
	for _, tc := range []struct {
		name string
		w, h int
	}{{"tall", 1, domain.MaxImageEdge + 1}, {"wide", domain.MaxImageEdge + 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			vs, err := Codec{}.RenderVariants(t.Context(), openerOf(longPNG(t, tc.w, tc.h)), spec)
			if !errors.Is(err, app.ErrTooManyPixels) {
				t.Fatalf("%dx%d: got %d variants and %v, want ErrTooManyPixels", tc.w, tc.h, len(vs), err)
			}
		})
	}
	t.Run("at the limit", func(t *testing.T) {
		for i, v := range renderTest(t, mimePNG, longPNG(t, 1, domain.MaxImageEdge)) {
			if v.Width != 1 || v.Height != testVariants[i].LongestEdge {
				t.Errorf("variant %s is %dx%d, want 1x%d", v.Variant, v.Width, v.Height, testVariants[i].LongestEdge)
			}
			decodeVariant(t, v)
		}
	})
}

// tinyFrames is a number of 1x1 frames whose pixels are far below testMaxPixels and whose frame
// records are far above it.
const tinyFrames = 20_000

// TestGIFTinyFramesNotDecoded cleans a GIF of many 1x1 frames with the real image/gif decoder
// (12 7.2, ruling AUD-SF01/2): its pixels are below the limit, but the decoder keeps a palette
// per frame, so the guard counts gifFrameOverhead per frame and refuses it before DecodeAll. The
// call allocates less than 4 bytes per pixel of the limit. Not parallel: it measures the
// allocations of the process.
func TestGIFTinyFramesNotDecoded(t *testing.T) {
	data := tinyFramesGIF(t, tinyFrames)
	if info, err := scanGIF(bytes.NewReader(data)); err != nil || info.Frames != tinyFrames || info.Width != 1 || info.Height != 1 {
		t.Fatalf("scanGIF = %+v, %v", info, err)
	}
	ctx := t.Context()
	open := openerOf(data)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	c, err := Codec{}.CleanOriginal(ctx, domain.PlanReencodeGIF, open, int64(len(data)), testLimits)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, app.ErrTooManyPixels) {
		t.Fatalf("got a cleaned original of %d bytes from %d and %v, want ErrTooManyPixels", c.Size, len(data), err)
	}
	if grown := after.TotalAlloc - before.TotalAlloc; grown >= 4*testMaxPixels {
		t.Fatalf("allocated %d bytes, want below %d", grown, 4*testMaxPixels)
	}
}

// TestGIFFrameOverheadBoundary: the most 1x1 frames the limit admits with gifFrameOverhead per
// frame are re-encoded with every frame; one frame more is refused without a decode.
func TestGIFFrameOverheadBoundary(t *testing.T) {
	fit := testMaxPixels / (1 + gifFrameOverhead)
	calls := 0
	c := Codec{reencode: func(b []byte) ([]byte, error) {
		calls++
		return reencodeGIF(b)
	}}
	over := tinyFramesGIF(t, fit+1)
	if _, err := c.CleanOriginal(t.Context(), domain.PlanReencodeGIF, openerOf(over), int64(len(over)), testLimits); !errors.Is(err, app.ErrTooManyPixels) {
		t.Fatalf("%d frames: got %v, want ErrTooManyPixels", fit+1, err)
	}
	if calls != 0 {
		t.Fatalf("re-encoded %d times behind the pixel guard", calls)
	}
	at := tinyFramesGIF(t, fit)
	cleaned, err := c.CleanOriginal(t.Context(), domain.PlanReencodeGIF, openerOf(at), int64(len(at)), testLimits)
	if err != nil {
		t.Fatalf("%d frames: %v", fit, err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(checkBody(t, cleaned)))
	if err != nil || len(g.Image) != fit {
		t.Fatalf("re-encoded GIF: %v, %d frames, want %d", err, len(g.Image), fit)
	}
}
