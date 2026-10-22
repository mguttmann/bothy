// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The test configuration values (plain values instead of the configuration keys).
const (
	testMaxPixels   = 1_000_000
	testJPEGQuality = 85
)

// testLimits are the clean limits of most tests.
var testLimits = app.CleanLimits{MaxPixels: testMaxPixels, JPEGQuality: testJPEGQuality}

// The colours of the synthetic images.
var (
	testRed  = color.RGBA{R: 255, A: 255}
	testBlue = color.RGBA{B: 255, A: 255}
)

// openerOf returns an Opener that reads b anew on every call.
func openerOf(b []byte) app.Opener {
	return func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b)), nil
	}
}

// failingOpener returns an Opener whose reader yields the first half of b and then fails with
// errTestRead, as a store that breaks in the middle of a read.
func failingOpener(b []byte) app.Opener {
	return func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(&failingReader{data: b[:len(b)/2]}), nil
	}
}

// readBody reads the cleaned bytes of c once through Body.
func readBody(tb testing.TB, c app.CleanedOriginal) []byte {
	tb.Helper()
	rc, err := c.Body(tb.Context())
	if err != nil {
		tb.Fatalf("Body: %v", err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		tb.Fatalf("read Body: %v", err)
	}
	return b
}

// checkBody reads Body twice and fails the test unless both reads are equal and Size and SHA256
// describe them; it returns the bytes.
func checkBody(tb testing.TB, c app.CleanedOriginal) []byte {
	tb.Helper()
	first := readBody(tb, c)
	if second := readBody(tb, c); !bytes.Equal(first, second) {
		tb.Fatalf("Body yields other bytes on the second read")
	}
	if c.Size != int64(len(first)) {
		tb.Fatalf("Size %d, Body yields %d bytes", c.Size, len(first))
	}
	if c.SHA256 != sha256.Sum256(first) {
		tb.Fatalf("SHA256 does not match the bytes of Body")
	}
	return first
}

// isRedish and isBlueish compare a decoded colour with a tolerance for JPEG and resampling loss.
func isRedish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 180 && g>>8 < 80 && b>>8 < 80
}

// isBlueish is the blue counterpart of isRedish.
func isBlueish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 < 80 && g>>8 < 80 && b>>8 > 180
}

// checkRedOverBlue fails the test unless img is portrait with red in its upper and blue in its
// lower quarter (a 2:1 red-left, blue-right image turned by orientation 6).
func checkRedOverBlue(tb testing.TB, img image.Image) {
	tb.Helper()
	b := img.Bounds()
	if b.Dy() <= b.Dx() {
		tb.Fatalf("size %dx%d is not portrait", b.Dx(), b.Dy())
	}
	top := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/4)
	bottom := img.At(b.Min.X+b.Dx()/2, b.Min.Y+3*b.Dy()/4)
	if !isRedish(top) || !isBlueish(bottom) {
		tb.Fatalf("top %v, bottom %v: want red over blue", top, bottom)
	}
}

// pngChunkTypes lists the chunk types of the PNG b.
func pngChunkTypes(tb testing.TB, b []byte) []string {
	tb.Helper()
	var types []string
	for _, c := range pngChunks(tb, b) {
		types = append(types, c.Type)
	}
	return types
}

// pngICCPData returns the data of the iCCP chunk of the PNG b, nil when none.
func pngICCPData(tb testing.TB, b []byte) []byte {
	tb.Helper()
	for _, c := range pngChunks(tb, b) {
		if c.Type == chunkICCP {
			return c.Data
		}
	}
	return nil
}

// jpegMarkers lists the markers of the JPEG b up to its EOI.
func jpegMarkers(tb testing.TB, b []byte) []byte {
	tb.Helper()
	var ms []byte
	for _, s := range jpegSegments(tb, b) {
		ms = append(ms, s.Marker)
	}
	return ms
}

// jpegICC returns the profile reassembled from the APP2 ICC_PROFILE segments of the JPEG b.
func jpegICC(tb testing.TB, b []byte) []byte {
	tb.Helper()
	info, err := scanJPEG(bytes.NewReader(b))
	if err != nil {
		tb.Fatalf("scanJPEG: %v", err)
	}
	return info.ICC
}

// TestConvertStillWebP converts still WebPs: PNG with alpha, JPEG without, the EXIF orientation
// applied to the pixels, the ICC profile kept and every other metadata dropped (design note 7,
// D-M3-06-5); an animated WebP is ErrImageAnimated, a WebP above the limit ErrTooManyPixels.
func TestConvertStillWebP(t *testing.T) {
	profile := syntheticICC(testColourRGB, 600)
	exif := orientationAPP1(6)[10:] // the TIFF block after marker, length and "Exif\0\0"
	opts := webpOptions{EXIF: exif, ICC: profile, XMP: true}

	t.Run("opaque gives JPEG", func(t *testing.T) {
		data := buildWebP(t, twoColour(8, 4, testRed, testBlue), opts)
		c := convertTestWebP(t, data)
		out := checkBody(t, c)
		if c.MIMEType != outJPEG || c.Width != 4 || c.Height != 8 {
			t.Fatalf("got %s %dx%d, want image/jpeg 4x8", c.MIMEType, c.Width, c.Height)
		}
		img, err := jpeg.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("image/jpeg cannot decode the output: %v", err)
		}
		checkRedOverBlue(t, img)
		if got := jpegICC(t, out); !bytes.Equal(got, profile) {
			t.Fatalf("profile not kept: %d bytes", len(got))
		}
		if slices.Contains(jpegMarkers(t, out), markerAPP1) {
			t.Fatalf("APP1 in the output")
		}
		checkNoWebPMetadata(t, out)
	})

	t.Run("alpha gives PNG", func(t *testing.T) {
		data := buildWebP(t, twoColour(8, 4, color.NRGBA{R: 255, A: 128}, testBlue), opts)
		c := convertTestWebP(t, data)
		out := checkBody(t, c)
		if c.MIMEType != outPNG || c.Width != 4 || c.Height != 8 {
			t.Fatalf("got %s %dx%d, want image/png 4x8", c.MIMEType, c.Width, c.Height)
		}
		img, err := png.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("image/png cannot decode the output: %v", err)
		}
		checkAlphaRedOverBlue(t, img)
		got, err := iccpProfile(pngICCPData(t, out), maxJPEGProfile)
		if err != nil || !bytes.Equal(got, profile) {
			t.Fatalf("profile not kept: %v, %d bytes", err, len(got))
		}
		if types := pngChunkTypes(t, out); slices.Contains(types, "eXIf") || slices.Contains(types, "iTXt") {
			t.Fatalf("metadata chunks in the output: %v", types)
		}
		checkNoWebPMetadata(t, out)
	})

	t.Run("animated", func(t *testing.T) {
		data := buildWebP(t, twoColour(8, 4, testRed, testBlue), webpOptions{Animated: true})
		_, err := Codec{}.CleanOriginal(t.Context(), domain.PlanConvertWebP, openerOf(data), int64(len(data)), testLimits)
		if !errors.Is(err, app.ErrImageAnimated) {
			t.Fatalf("got %v, want ErrImageAnimated", err)
		}
	})

	t.Run("above the pixel limit", func(t *testing.T) {
		data := buildWebP(t, twoColour(8, 4, testRed, testBlue), webpOptions{})
		lim := app.CleanLimits{MaxPixels: 31, JPEGQuality: testJPEGQuality}
		c := Codec{decode: func(string, io.Reader) (image.Image, error) {
			t.Fatalf("decoded above the pixel limit")
			return nil, nil
		}}
		_, err := c.CleanOriginal(t.Context(), domain.PlanConvertWebP, openerOf(data), int64(len(data)), lim)
		if !errors.Is(err, app.ErrTooManyPixels) {
			t.Fatalf("got %v, want ErrTooManyPixels", err)
		}
	})
}

// convertTestWebP runs CleanOriginal with PlanConvertWebP over data and fails the test on an
// error.
func convertTestWebP(tb testing.TB, data []byte) app.CleanedOriginal {
	tb.Helper()
	c, err := Codec{}.CleanOriginal(tb.Context(), domain.PlanConvertWebP, openerOf(data), int64(len(data)), testLimits)
	if err != nil {
		tb.Fatalf("CleanOriginal: %v", err)
	}
	return c
}

// checkAlphaRedOverBlue is checkRedOverBlue for the translucent red of the alpha WebP: the top
// keeps its alpha below 255, the bottom is opaque blue.
func checkAlphaRedOverBlue(tb testing.TB, img image.Image) {
	tb.Helper()
	b := img.Bounds()
	if b.Dy() <= b.Dx() {
		tb.Fatalf("size %dx%d is not portrait", b.Dx(), b.Dy())
	}
	top := color.NRGBAModel.Convert(img.At(b.Dx()/2, b.Dy()/4)).(color.NRGBA)
	bottom := img.At(b.Dx()/2, 3*b.Dy()/4)
	if top.R < 200 || top.B > 50 || top.A == 0xFF || !isBlueish(bottom) {
		tb.Fatalf("top %v, bottom %v: want translucent red over blue", top, bottom)
	}
}

// checkNoWebPMetadata fails the test when the EXIF block or the XMP text of the source WebP
// survives anywhere in out.
func checkNoWebPMetadata(tb testing.TB, out []byte) {
	tb.Helper()
	for _, s := range []string{"Exif", "xmpmeta", "Robin"} {
		if bytes.Contains(out, []byte(s)) {
			tb.Fatalf("output contains %q", s)
		}
	}
}

// TestCleanOriginalJPEGAndPNG checks the filter plans: Size and SHA256 describe what Body yields
// on every read, the size is the display size, a truncated JPEG is ErrImageMalformed and a read
// error of the store is returned as it is (design note 3c).
func TestCleanOriginalJPEGAndPNG(t *testing.T) {
	src := twoColour(40, 20, testRed, testBlue)
	jpegData := buildJPEG(t, src, fullJPEGExtras())
	pngData := buildPNG(t, src, fullPNGExtras())

	t.Run("jpeg", func(t *testing.T) {
		c := cleanTest(t, domain.PlanStripJPEG, jpegData)
		out := checkBody(t, c)
		if _, want := stripTestJPEG(t, jpegData); !bytes.Equal(out, want) {
			t.Fatalf("Body differs from the filter output")
		}
		if c.MIMEType != outJPEG || c.Width != 20 || c.Height != 40 {
			t.Fatalf("got %s %dx%d, want image/jpeg 20x40 (orientation 6 swaps)", c.MIMEType, c.Width, c.Height)
		}
	})

	t.Run("png", func(t *testing.T) {
		c := cleanTest(t, domain.PlanStripPNG, pngData)
		out := checkBody(t, c)
		if _, want := stripTestPNG(t, pngData); !bytes.Equal(out, want) {
			t.Fatalf("Body differs from the filter output")
		}
		if c.MIMEType != outPNG || c.Width != 40 || c.Height != 20 {
			t.Fatalf("got %s %dx%d, want image/png 40x20", c.MIMEType, c.Width, c.Height)
		}
	})

	t.Run("truncated jpeg", func(t *testing.T) {
		in := jpegData[:len(jpegData)/2]
		_, err := Codec{}.CleanOriginal(t.Context(), domain.PlanStripJPEG, openerOf(in), int64(len(in)), testLimits)
		if !errors.Is(err, app.ErrImageMalformed) {
			t.Fatalf("got %v, want ErrImageMalformed", err)
		}
	})

	for _, tc := range []struct {
		name string
		plan domain.OriginalPlan
		data []byte
	}{{"jpeg read error", domain.PlanStripJPEG, jpegData}, {"png read error", domain.PlanStripPNG, pngData}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Codec{}.CleanOriginal(t.Context(), tc.plan, failingOpener(tc.data), int64(len(tc.data)), testLimits)
			if !errors.Is(err, errTestRead) || errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("got %v, want the read error and not ErrImageMalformed", err)
			}
		})
	}
}

// cleanTest runs CleanOriginal with plan over data and fails the test on an error.
func cleanTest(tb testing.TB, plan domain.OriginalPlan, data []byte) app.CleanedOriginal {
	tb.Helper()
	c, err := Codec{}.CleanOriginal(tb.Context(), plan, openerOf(data), int64(len(data)), testLimits)
	if err != nil {
		tb.Fatalf("CleanOriginal: %v", err)
	}
	return c
}

// TestGIFCleanOriginal re-encodes a GIF without its comment (design note 7) and checks that a GIF
// whose frames multiply above the pixel limit is never re-encoded.
func TestGIFCleanOriginal(t *testing.T) {
	data := gifTestSplice(t, gifTestAnimation(t), gifTestExtras)
	calls := 0
	c := Codec{reencode: func(b []byte) ([]byte, error) {
		calls++
		return reencodeGIF(b)
	}}

	// Three 6x4 frames are 72 pixels: one below is above the limit.
	lim := app.CleanLimits{MaxPixels: 71, JPEGQuality: testJPEGQuality}
	if _, err := c.CleanOriginal(t.Context(), domain.PlanReencodeGIF, openerOf(data), int64(len(data)), lim); !errors.Is(err, app.ErrTooManyPixels) {
		t.Fatalf("got %v, want ErrTooManyPixels", err)
	}
	if calls != 0 {
		t.Fatalf("re-encoded %d times above the pixel limit", calls)
	}

	lim.MaxPixels = 72
	cleaned, err := c.CleanOriginal(t.Context(), domain.PlanReencodeGIF, openerOf(data), int64(len(data)), lim)
	if err != nil {
		t.Fatalf("CleanOriginal: %v", err)
	}
	out := checkBody(t, cleaned)
	if cleaned.MIMEType != mimeGIF || cleaned.Width != 6 || cleaned.Height != 4 || calls != 1 {
		t.Fatalf("got %s %dx%d after %d calls", cleaned.MIMEType, cleaned.Width, cleaned.Height, calls)
	}
	for _, blk := range gifTestExtensions(t, out) {
		if blk.label == 0xFE || blk.app == "XMP DataXMP" {
			t.Fatalf("extension 0x%02X %q kept", blk.label, blk.app)
		}
	}
	if bytes.Contains(out, []byte("Sam was here")) {
		t.Fatalf("comment text kept")
	}
}
