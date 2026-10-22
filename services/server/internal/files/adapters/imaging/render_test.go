// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// testChunkIDAT is the PNG image data chunk type.
const testChunkIDAT = "IDAT"

// testVariants are the variants of the tests: the edges 64 and 128 scale down, 1024 is above
// every test image and shows that nothing is scaled up.
var testVariants = []app.VariantSpec{
	{Variant: domain.VariantSmall, LongestEdge: 64},
	{Variant: domain.VariantMedium, LongestEdge: 128},
	{Variant: domain.VariantLarge, LongestEdge: 1024},
}

// renderTest runs RenderVariants over data of mimeType with testVariants and fails the test on an
// error or when a variant is missing.
func renderTest(tb testing.TB, mimeType string, data []byte) []app.RenderedVariant {
	tb.Helper()
	spec := app.RenderSpec{MIMEType: mimeType, Variants: testVariants, JPEGQuality: testJPEGQuality, MaxPixels: testMaxPixels}
	vs, err := Codec{}.RenderVariants(tb.Context(), openerOf(data), spec)
	if err != nil {
		tb.Fatalf("RenderVariants: %v", err)
	}
	if len(vs) != len(testVariants) {
		tb.Fatalf("%d variants, want %d", len(vs), len(testVariants))
	}
	for i, v := range vs {
		if v.Variant != testVariants[i].Variant {
			tb.Fatalf("variant %d is %s, want %s", i, v.Variant, testVariants[i].Variant)
		}
	}
	return vs
}

// decodeVariant decodes a rendered variant with the decoder of its media type and checks that
// the declared size is the decoded size.
func decodeVariant(tb testing.TB, v app.RenderedVariant) image.Image {
	tb.Helper()
	var img image.Image
	var err error
	switch v.MIMEType {
	case outJPEG:
		img, err = jpeg.Decode(bytes.NewReader(v.Data))
	case outPNG:
		img, err = png.Decode(bytes.NewReader(v.Data))
	default:
		tb.Fatalf("variant %s has the type %q", v.Variant, v.MIMEType)
	}
	if err != nil {
		tb.Fatalf("decode variant %s: %v", v.Variant, err)
	}
	if b := img.Bounds(); b.Dx() != v.Width || b.Dy() != v.Height {
		tb.Fatalf("variant %s declares %dx%d, holds %dx%d", v.Variant, v.Width, v.Height, b.Dx(), b.Dy())
	}
	return img
}

// TestThumbnailsOrientationApplied renders a 2:1 JPEG with orientation 6: every variant is
// portrait with the left colour on top, the largest has the display size of the original and no
// variant carries an APP1 (12 15, ADR 0047).
func TestThumbnailsOrientationApplied(t *testing.T) {
	data := buildJPEG(t, twoColour(400, 200, testRed, testBlue), jpegExtras{Orientation: 6})
	want := [][2]int{{32, 64}, {64, 128}, {200, 400}}
	for i, v := range renderTest(t, mimeJPEG, data) {
		if v.MIMEType != outJPEG || v.Width != want[i][0] || v.Height != want[i][1] {
			t.Fatalf("variant %s: %s %dx%d, want image/jpeg %dx%d", v.Variant, v.MIMEType, v.Width, v.Height, want[i][0], want[i][1])
		}
		checkRedOverBlue(t, decodeVariant(t, v))
		if slices.Contains(jpegMarkers(t, v.Data), markerAPP1) {
			t.Fatalf("variant %s has an APP1", v.Variant)
		}
	}
}

// TestThumbnailsKeepColourProfile checks that every variant carries the profile of the original:
// the APP2 profile of a JPEG, the iCCP chunk of a PNG with alpha byte for byte, and the
// decompressed iCCP profile in the JPEG variant of an opaque PNG (D-M3-06-6).
func TestThumbnailsKeepColourProfile(t *testing.T) {
	profile := syntheticICC(testColourRGB, 3000)

	t.Run("jpeg", func(t *testing.T) {
		data := buildJPEG(t, twoColour(300, 200, testRed, testBlue), jpegExtras{ICC: profile, ICCParts: 2})
		for _, v := range renderTest(t, mimeJPEG, data) {
			if got := jpegICC(t, v.Data); !bytes.Equal(got, profile) {
				t.Fatalf("variant %s: profile of %d bytes, want the original's", v.Variant, len(got))
			}
		}
	})

	t.Run("png with alpha", func(t *testing.T) {
		data := buildPNG(t, twoColour(300, 200, color.NRGBA{R: 255, A: 100}, testBlue), pngExtras{ICC: profile})
		want := pngICCPData(t, data)
		for _, v := range renderTest(t, mimePNG, data) {
			if v.MIMEType != outPNG {
				t.Fatalf("variant %s is %s, want image/png", v.Variant, v.MIMEType)
			}
			if got := pngICCPData(t, v.Data); !bytes.Equal(got, want) {
				t.Fatalf("variant %s: iCCP differs from the original's", v.Variant)
			}
		}
	})

	t.Run("png without alpha", func(t *testing.T) {
		data := buildPNG(t, twoColour(300, 200, testRed, testBlue), pngExtras{ICC: profile})
		for _, v := range renderTest(t, mimePNG, data) {
			if v.MIMEType != outJPEG {
				t.Fatalf("variant %s is %s, want image/jpeg", v.Variant, v.MIMEType)
			}
			if got := jpegICC(t, v.Data); !bytes.Equal(got, profile) {
				t.Fatalf("variant %s: profile of %d bytes, want the original's", v.Variant, len(got))
			}
		}
	})
}

// TestThumbnailsCarryNoMetadata renders sources with every extra of the builders: the JPEG
// variants have no APP0, APP1, APP13 or COM, the PNG variants no text, EXIF or time chunk.
func TestThumbnailsCarryNoMetadata(t *testing.T) {
	jpegData := buildJPEG(t, twoColour(300, 200, testRed, testBlue), fullJPEGExtras())
	for _, v := range renderTest(t, mimeJPEG, jpegData) {
		ms := jpegMarkers(t, v.Data)
		for _, m := range []byte{markerAPP0, markerAPP1, 0xED, 0xFE} {
			if slices.Contains(ms, m) {
				t.Fatalf("JPEG variant %s has the marker 0x%02X", v.Variant, m)
			}
		}
	}

	pngData := buildPNG(t, twoColour(300, 200, color.NRGBA{R: 255, A: 100}, testBlue), fullPNGExtras())
	for _, v := range renderTest(t, mimePNG, pngData) {
		types := pngChunkTypes(t, v.Data)
		for _, typ := range []string{"tEXt", "zTXt", "iTXt", "eXIf", "tIME", testPrivatePNG} {
			if slices.Contains(types, typ) {
				t.Fatalf("PNG variant %s has the chunk %s", v.Variant, typ)
			}
		}
	}
}

// TestThumbnailFormatByAlpha checks the format rule: PNG when the decoded image has alpha, JPEG
// otherwise (12 7.3, D-M3-06-6).
func TestThumbnailFormatByAlpha(t *testing.T) {
	transparent := image.NewPaletted(image.Rect(0, 0, 20, 10), color.Palette{color.Transparent, testRed})
	for i := range transparent.Pix {
		transparent.Pix[i] = uint8(i % 2)
	}
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, transparent, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	gray := image.NewGray(image.Rect(0, 0, 20, 10))
	for _, tc := range []struct {
		name, mimeType, want string
		data                 []byte
	}{
		{"png with alpha", mimePNG, outPNG, buildPNG(t, twoColour(20, 10, color.NRGBA{R: 255, A: 1}, testBlue), pngExtras{})},
		{"opaque png", mimePNG, outJPEG, buildPNG(t, twoColour(20, 10, testRed, testBlue), pngExtras{})},
		{"gray png", mimePNG, outJPEG, buildPNG(t, gray, pngExtras{})},
		{"jpeg", mimeJPEG, outJPEG, buildJPEG(t, twoColour(20, 10, testRed, testBlue), jpegExtras{})},
		{"gif with transparency", mimeGIF, outPNG, gifBuf.Bytes()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, v := range renderTest(t, tc.mimeType, tc.data) {
				if v.MIMEType != tc.want {
					t.Fatalf("variant %s is %s, want %s", v.Variant, v.MIMEType, tc.want)
				}
				decodeVariant(t, v)
			}
		})
	}
}

// TestThumbnailNoUpscale renders a 3x2 image: every variant keeps 3x2.
func TestThumbnailNoUpscale(t *testing.T) {
	data := buildPNG(t, twoColour(3, 2, testRed, testBlue), pngExtras{})
	for _, v := range renderTest(t, mimePNG, data) {
		if v.Width != 3 || v.Height != 2 {
			t.Fatalf("variant %s is %dx%d, want 3x2", v.Variant, v.Width, v.Height)
		}
		decodeVariant(t, v)
	}
}

// uniform returns a w x h RGBA image of colour c.
func uniform(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

// checkUniform fails the test unless the centre of every variant has the colour that match
// accepts.
func checkUniform(tb testing.TB, vs []app.RenderedVariant, match func(color.Color) bool) {
	tb.Helper()
	for _, v := range vs {
		img := decodeVariant(tb, v)
		b := img.Bounds()
		if c := img.At(b.Dx()/2, b.Dy()/2); !match(c) {
			tb.Fatalf("variant %s shows %v", v.Variant, c)
		}
	}
}

// TestThumbnailFirstFrame checks that an animation is rendered from its first frame (GIF) or its
// default image (APNG).
func TestThumbnailFirstFrame(t *testing.T) {
	t.Run("gif", func(t *testing.T) {
		pal := color.Palette{testRed, testBlue}
		frame := func(idx uint8) *image.Paletted {
			m := image.NewPaletted(image.Rect(0, 0, 40, 20), pal)
			for i := range m.Pix {
				m.Pix[i] = idx
			}
			return m
		}
		g := &gif.GIF{Image: []*image.Paletted{frame(0), frame(1), frame(1)}, Delay: []int{10, 10, 10}}
		var buf bytes.Buffer
		if err := gif.EncodeAll(&buf, g); err != nil {
			t.Fatalf("EncodeAll: %v", err)
		}
		checkUniform(t, renderTest(t, mimeGIF, buf.Bytes()), isRedish)
	})

	t.Run("apng", func(t *testing.T) {
		checkUniform(t, renderTest(t, mimeAPNG, buildAPNG(t)), isRedish)
	})
}

// buildAPNG returns an APNG of two 40x20 frames: the default image (red, its IDAT) and a second
// frame (blue, as fdAT), with acTL after IHDR and an fcTL before each frame (APNG 1.0).
func buildAPNG(tb testing.TB) []byte {
	tb.Helper()
	encode := func(c color.Color) []pngChunkT {
		var buf bytes.Buffer
		if err := png.Encode(&buf, uniform(40, 20, c)); err != nil {
			tb.Fatalf("encode png: %v", err)
		}
		return pngChunks(tb, buf.Bytes())
	}
	first, second := encode(testRed), encode(testBlue)
	be := binary.BigEndian
	fcTL := func(seq uint32) []byte {
		d := be.AppendUint32(nil, seq)
		d = be.AppendUint32(d, 40)
		d = be.AppendUint32(d, 20)
		d = be.AppendUint64(d, 0)        // x and y offset
		d = be.AppendUint32(d, 1<<16|10) // delay 1/10 s
		return append(d, 0, 0)           // dispose and blend op
	}
	out := []byte(pngSignature)
	out = append(out, pngChunk(chunkIHDR, first[0].Data)...)
	out = append(out, pngChunk(chunkACTL, be.AppendUint64(nil, 2<<32))...) // two frames, endless
	out = append(out, pngChunk("fcTL", fcTL(0))...)
	for _, c := range first {
		if c.Type == testChunkIDAT {
			out = append(out, pngChunk(c.Type, c.Data)...)
		}
	}
	out = append(out, pngChunk("fcTL", fcTL(1))...)
	var fdat []byte
	for _, c := range second {
		if c.Type == testChunkIDAT {
			fdat = append(fdat, c.Data...)
		}
	}
	out = append(out, pngChunk("fdAT", append(be.AppendUint32(nil, 2), fdat...))...)
	return append(out, pngChunk(chunkIEND, nil)...)
}

// TestThumbnailCMYKWithoutProfile renders a 4-component Adobe JPEG with a CMYK profile: the
// profile does not fit an RGB thumbnail, so no variant carries an APP2 (D-M3-06-6).
func TestThumbnailCMYKWithoutProfile(t *testing.T) {
	data := buildCMYKJPEG(t, syntheticICC("CMYK", 500))
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode source: %v", err)
	}
	if _, ok := img.(*image.CMYK); !ok {
		t.Fatalf("the source decodes as %T, want *image.CMYK", img)
	}
	if len(jpegICC(t, data)) == 0 {
		t.Fatalf("the source has no profile")
	}
	for _, v := range renderTest(t, mimeJPEG, data) {
		if v.MIMEType != outJPEG || slices.Contains(jpegMarkers(t, v.Data), markerAPP2) {
			t.Fatalf("variant %s: %s with markers % X", v.Variant, v.MIMEType, jpegMarkers(t, v.Data))
		}
		decodeVariant(t, v)
	}
}

// The layout of the hand-made CMYK JPEG: 16x16 pixels, four components without subsampling, one
// 8x8 block per component in each of the four MCUs.
const (
	cmykSize   = 16
	cmykBlocks = (cmykSize / 8) * (cmykSize / 8)
	// cmykDCCodeLen is the length of every DC Huffman code: 12 categories in codes of 4 bits.
	cmykDCCodeLen = 4
	cmykDCClasses = 12
)

// cmykSamples are the stored (Adobe inverted) sample values of the four components.
var cmykSamples = [4]int{200, 100, 50, 255}

// buildCMYKJPEG writes a baseline JPEG of four components by hand, because image/jpeg encodes
// every image as gray or YCbCr: an APP14 Adobe segment with transform 0 (CMYK), the APP2 parts of
// profile, a quantisation table of ones, and blocks of one colour, so each block is a DC value and
// an end-of-block code (ITU-T T.81 annex F).
func buildCMYKJPEG(tb testing.TB, profile []byte) []byte {
	tb.Helper()
	out := []byte{0xFF, markerSOI}
	out = append(out, testSegment(markerAPP14, []byte("Adobe\x00\x64\x00\x00\x00\x00\x00"))...)
	for _, s := range testICCSegments(profile, 1) {
		out = append(out, s...)
	}
	out = append(out, testSegment(0xDB, append([]byte{0}, bytes.Repeat([]byte{1}, 64)...))...)
	sof := []byte{8, 0, cmykSize, 0, cmykSize, 4}
	for id := byte(1); id <= 4; id++ {
		sof = append(sof, id, 0x11, 0)
	}
	out = append(out, testSegment(markerSOF0, sof)...)
	// DC table 0: twelve codes of four bits for the categories 0 to 11.
	dht := []byte{0x00}
	counts := make([]byte, 16)
	counts[cmykDCCodeLen-1] = cmykDCClasses
	dht = append(dht, counts...)
	for c := range byte(cmykDCClasses) {
		dht = append(dht, c)
	}
	// AC table 0: one code "0" of one bit for the end of block.
	dht = append(dht, 0x10, 1)
	dht = append(dht, make([]byte, 15)...)
	dht = append(dht, 0x00)
	out = append(out, testSegment(markerDHT, dht)...)
	sos := []byte{4}
	for id := byte(1); id <= 4; id++ {
		sos = append(sos, id, 0x00)
	}
	out = append(out, testSegment(markerSOS, append(sos, 0, 63, 0))...)
	out = append(out, cmykEntropy()...)
	return append(out, 0xFF, markerEOI)
}

// cmykEntropy codes the four MCUs: in the first, each component's DC difference is its DC value
// 8 * (sample - 128) (the DCT of a flat block); later blocks repeat it with difference 0.
func cmykEntropy() []byte {
	w := &jpegBitWriter{}
	for blk := range cmykBlocks {
		for _, s := range cmykSamples {
			diff := 0
			if blk == 0 {
				diff = 8 * (s - 128)
			}
			cat, bits := dcCategory(diff)
			w.write(uint32(cat), cmykDCCodeLen) //nolint:gosec // G115: a category is at most 11
			w.write(bits, uint(cat))
			w.write(0, 1) // end of block
		}
	}
	return w.flush()
}

// dcCategory returns the size category of a DC difference and its additional bits (T.81 F.1.2.1).
func dcCategory(diff int) (int, uint32) {
	mag := max(diff, -diff)
	cat := 0
	for mag>>cat != 0 {
		cat++
	}
	if diff < 0 {
		diff += 1<<cat - 1
	}
	return cat, uint32(diff) //nolint:gosec // G115: diff is between 0 and 2^11
}

// jpegBitWriter packs bits MSB first and stuffs a zero byte after every 0xFF (T.81 F.1.2.3).
type jpegBitWriter struct {
	out  []byte
	acc  uint32
	nAcc uint
}

// write appends the low n bits of v.
func (w *jpegBitWriter) write(v uint32, n uint) {
	for i := n; i > 0; i-- {
		w.acc = w.acc<<1 | (v>>(i-1))&1
		w.nAcc++
		if w.nAcc == 8 {
			w.emit()
		}
	}
}

// emit writes the full byte of the accumulator.
func (w *jpegBitWriter) emit() {
	b := byte(w.acc) //nolint:gosec // G115: the accumulator holds eight bits
	w.out = append(w.out, b)
	if b == 0xFF {
		w.out = append(w.out, 0)
	}
	w.acc, w.nAcc = 0, 0
}

// flush pads the last byte with ones and returns the entropy-coded data.
func (w *jpegBitWriter) flush() []byte {
	for w.nAcc != 0 {
		w.write(1, 1)
	}
	return w.out
}
