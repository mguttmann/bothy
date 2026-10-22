// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"testing"

	"golang.org/x/image/webp"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// webpTestImage returns a w x h image with two colours in a checker pattern; with alpha the
// second colour is half transparent.
func webpTestImage(w, h int, alpha bool) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	a := color.NRGBA{R: 200, G: 10, B: 30, A: 255}
	b := color.NRGBA{R: 5, G: 220, B: 90, A: 255}
	if alpha {
		b.A = 128
	}
	for y := range h {
		for x := range w {
			if (x+y)%2 == 0 {
				m.SetNRGBA(x, y, a)
			} else {
				m.SetNRGBA(x, y, b)
			}
		}
	}
	return m
}

// webpTestICC returns a synthetic ICC profile: a 128-byte header with its size, the "acsp"
// signature and no tags.
func webpTestICC() []byte {
	p := make([]byte, 132)
	binary.BigEndian.PutUint32(p, webpTestU32(len(p)))
	copy(p[12:], "mntrRGB XYZ ")
	copy(p[36:], "acsp")
	return p
}

// webpTestEXIF returns a synthetic little-endian TIFF block with one IFD of one entry (an
// orientation of 1) and no next IFD; odd length so the chunk needs padding.
func webpTestEXIF() []byte {
	b := []byte("II*\x00\x08\x00\x00\x00")
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = append(b, 0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00)
	b = binary.LittleEndian.AppendUint32(b, 0)
	return append(b, 0xEE)
}

// webpTestSameNRGBA fails when got does not hold exactly the pixels of want.
func webpTestSameNRGBA(t *testing.T, got image.Image, want *image.NRGBA) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := color.NRGBAModel.Convert(got.At(x, y)).(color.NRGBA)
			if w := want.NRGBAAt(x, y); g != w {
				t.Fatalf("pixel %d,%d = %v, want %v", x, y, g, w)
			}
		}
	}
}

func TestBuildWebPDecodes(t *testing.T) {
	cases := map[string]struct {
		img *image.NRGBA
		o   webpOptions
	}{
		"opaque":            {webpTestImage(7, 5, false), webpOptions{}},
		"alpha":             {webpTestImage(3, 9, true), webpOptions{}},
		"vp8x bare":         {webpTestImage(4, 4, false), webpOptions{VP8X: true}},
		"vp8x icc exif xmp": {webpTestImage(5, 3, true), webpOptions{ICC: webpTestICC(), EXIF: webpTestEXIF(), XMP: true}},
		"one colour":        {image.NewNRGBA(image.Rect(0, 0, 2, 2)), webpOptions{}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			data := buildWebP(t, c.img, c.o)
			cfg, err := webp.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("DecodeConfig: %v", err)
			}
			if cfg.Width != c.img.Rect.Dx() || cfg.Height != c.img.Rect.Dy() {
				t.Fatalf("DecodeConfig size %dx%d, want %v", cfg.Width, cfg.Height, c.img.Rect.Size())
			}
			m, err := webp.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			webpTestSameNRGBA(t, m, c.img)
		})
	}
}

func TestAnimatedWebPDetected(t *testing.T) {
	img := webpTestImage(4, 4, false)
	for name, o := range map[string]webpOptions{
		"animation flag":          {Animated: true},
		"ANIM chunk without flag": {ANIMOnly: true},
	} {
		t.Run(name, func(t *testing.T) {
			info, err := scanWebP(buildWebP(t, img, o))
			if err != nil {
				t.Fatalf("scanWebP: %v", err)
			}
			if !info.Animated || info.Width != 4 || info.Height != 4 {
				t.Fatalf("scanWebP = %+v, want animated 4x4", info)
			}
		})
	}
	info, err := scanWebP(buildWebP(t, img, webpOptions{VP8X: true}))
	if err != nil || info.Animated {
		t.Fatalf("still VP8X: %+v, %v; want not animated", info, err)
	}
}

func TestScanWebP(t *testing.T) {
	t.Run("still VP8L", func(t *testing.T) {
		info, err := scanWebP(buildWebP(t, webpTestImage(7, 5, true), webpOptions{}))
		if err != nil {
			t.Fatalf("scanWebP: %v", err)
		}
		if info.Width != 7 || info.Height != 5 || info.Animated || info.EXIF != nil || info.ICC != nil {
			t.Fatalf("scanWebP = %+v, want 7x5 still without metadata", info)
		}
	})
	t.Run("VP8X with ICCP and EXIF", func(t *testing.T) {
		icc, exif := webpTestICC(), webpTestEXIF()
		data := buildWebP(t, webpTestImage(5, 3, false), webpOptions{ICC: icc, EXIF: exif, XMP: true})
		info, err := scanWebP(data)
		if err != nil {
			t.Fatalf("scanWebP: %v", err)
		}
		if info.Width != 5 || info.Height != 3 || info.Animated {
			t.Fatalf("scanWebP = %+v, want 5x3 still", info)
		}
		if !bytes.Equal(info.ICC, icc) || !bytes.Equal(info.EXIF, exif) {
			t.Fatalf("payloads changed: ICC %x EXIF %x", info.ICC, info.EXIF)
		}
	})
	t.Run("EXIF with Exif prefix", func(t *testing.T) {
		exif := webpTestEXIF()
		data := buildWebP(t, webpTestImage(2, 2, false), webpOptions{EXIF: append([]byte("Exif\x00\x00"), exif...)})
		info, err := scanWebP(data)
		if err != nil {
			t.Fatalf("scanWebP: %v", err)
		}
		if !bytes.Equal(info.EXIF, exif) {
			t.Fatalf("EXIF = %x, want the TIFF block without the prefix", info.EXIF)
		}
	})
	t.Run("VP8 key frame header", func(t *testing.T) {
		info, err := scanWebP(webpTestRIFF(webpTestChunk("VP8 ", webpTestVP8Header(321, 123))))
		if err != nil {
			t.Fatalf("scanWebP: %v", err)
		}
		if info.Width != 321 || info.Height != 123 {
			t.Fatalf("scanWebP = %+v, want 321x123", info)
		}
	})
}

// webpTestVP8Header returns the first ten bytes of a VP8 key frame (frame tag, start code,
// 14-bit width and height with zero scaling bits); the scanner reads nothing more.
func webpTestVP8Header(w, h int) []byte {
	b := []byte{0x00, 0x00, 0x00, 0x9D, 0x01, 0x2A}
	// Width in the low and height in the high 16 bits give the two little-endian fields in order.
	return binary.LittleEndian.AppendUint32(b, webpTestU32(w|h<<16))
}

func TestWebPConfig(t *testing.T) {
	w, h, err := webpConfig(buildWebP(t, webpTestImage(6, 2, false), webpOptions{EXIF: webpTestEXIF()}))
	if err != nil || w != 6 || h != 2 {
		t.Fatalf("webpConfig = %d, %d, %v; want 6, 2, nil", w, h, err)
	}
	if _, _, err := webpConfig([]byte("RIFF")); !errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("webpConfig error = %v, want ErrImageMalformed", err)
	}
}

func TestScanWebPMalformed(t *testing.T) {
	good := buildWebP(t, webpTestImage(3, 3, false), webpOptions{})
	vp8x := webpTestChunk("VP8X", make([]byte, 10))
	cases := map[string][]byte{
		"empty":             nil,
		"short":             []byte("RIFF\x04\x00\x00\x00WEB"),
		"not RIFF":          append([]byte("RIFX"), good[4:]...),
		"not WEBP":          append(append([]byte{}, good[:8]...), append([]byte("WAVE"), good[12:]...)...),
		"RIFF size too big": append(append([]byte("RIFF"), 0xFF, 0xFF, 0x00, 0x00), good[8:]...),
		"truncated":         good[:len(good)-3],
		"chunk too long":    webpTestRIFF([]byte("VP8L\xFF\x00\x00\x00\x2F")),
		"chunk header cut":  webpTestRIFF([]byte("VP8")),
		"no image chunk":    webpTestRIFF(vp8x, webpTestChunk("EXIF", webpTestEXIF())),
		"bad VP8L":          webpTestRIFF(webpTestChunk("VP8L", []byte{0x2E, 0, 0, 0, 0})),
		"short VP8L":        webpTestRIFF(webpTestChunk("VP8L", []byte{0x2F, 0})),
		"VP8L version":      webpTestRIFF(webpTestChunk("VP8L", []byte{0x2F, 0, 0, 0, 0x20})),
		"short VP8X":        webpTestRIFF(webpTestChunk("VP8X", make([]byte, 4)), webpTestChunk("VP8L", good[20:])),
		"VP8 not key frame": webpTestRIFF(webpTestChunk("VP8 ", append([]byte{0x01}, webpTestVP8Header(4, 4)[1:]...))),
		"VP8 start code":    webpTestRIFF(webpTestChunk("VP8 ", append([]byte{0, 0, 0, 0, 0, 0}, 4, 0, 4, 0))),
		"short VP8":         webpTestRIFF(webpTestChunk("VP8 ", []byte{0, 0, 0})),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := scanWebP(data); !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("scanWebP error = %v, want ErrImageMalformed", err)
			}
		})
	}
}

func FuzzWebPScan(f *testing.F) {
	img := webpTestImage(3, 2, true)
	f.Add(buildWebP(f, img, webpOptions{}))
	f.Add(buildWebP(f, img, webpOptions{ICC: webpTestICC(), EXIF: webpTestEXIF(), XMP: true}))
	f.Add(buildWebP(f, img, webpOptions{Animated: true}))
	f.Add(webpTestRIFF(webpTestChunk("VP8 ", webpTestVP8Header(9, 9))))
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBP"))
	f.Fuzz(func(t *testing.T, data []byte) {
		info, err := scanWebP(data)
		if err != nil {
			return
		}
		if info.Width < 0 || info.Height < 0 {
			t.Fatalf("negative size in %+v", info)
		}
	})
}

// TestWebPVP8XAfterImageRefused (red team finding of M3-06): a VP8X chunk after the bitstream
// cannot shrink the size the pixel guard checks; the decoder of x/image never reads it and decodes
// the bitstream at its own size.
func TestWebPVP8XAfterImageRefused(t *testing.T) {
	t.Parallel()
	vp8l, _ := webpVP8L(t, webpTestImage(40, 40, false))
	canvas := webpTestUint24(webpTestUint24([]byte{0, 0, 0, 0}, 0), 0) // 1x1
	for name, data := range map[string][]byte{
		"vp8l then vp8x": webpTestRIFF(webpTestChunk("VP8L", vp8l), webpTestChunk("VP8X", canvas)),
		"vp8 then vp8x":  webpTestRIFF(webpTestChunk("VP8 ", webpTestVP8Header(40, 40)), webpTestChunk("VP8X", canvas)),
	} {
		info, err := scanWebP(data)
		if err == nil && info.Width*info.Height < 40*40 {
			t.Errorf("%s: scan reports %dx%d", name, info.Width, info.Height)
		}
		open := func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		_, err = Codec{}.CleanOriginal(t.Context(), domain.PlanConvertWebP, open, int64(len(data)), app.CleanLimits{MaxPixels: 100, JPEGQuality: 90})
		if err == nil {
			t.Errorf("%s: converted above the pixel limit", name)
		}
	}
}
