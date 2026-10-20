// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// palettedTestImage returns a small paletted image, so image/png writes PLTE and hIST is legal.
func palettedTestImage() *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, 12, 7), color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}})
	index := byte(0)
	for i := range img.Pix {
		img.Pix[i] = index
		index = (index + 1) % 3
	}
	return img
}

// fullPNGExtras is every kind of metadata of TestStripPNGRemovesMetadata.
func fullPNGExtras() pngExtras {
	return pngExtras{ICC: syntheticICC(testColourRGB, 900), Text: true, EXIF: true, Misc: true, Trailing: true}
}

// stripTestPNG runs stripPNG over in and fails the test on an error.
func stripTestPNG(tb testing.TB, in []byte) (pngInfo, []byte) {
	tb.Helper()
	var out bytes.Buffer
	info, err := stripPNG(bytes.NewReader(in), &out)
	if err != nil {
		tb.Fatalf("stripPNG: %v", err)
	}
	return info, out.Bytes()
}

// keptChunks returns the chunks of in that the allow list keeps, in input order.
func keptChunks(chunks []pngChunkT) []pngChunkT {
	var out []pngChunkT
	for _, c := range chunks {
		if domain.PNGChunkKept(c.Type) {
			out = append(out, c)
		}
	}
	return out
}

func TestStripPNGRemovesMetadata(t *testing.T) {
	for name, img := range map[string]image.Image{
		"paletted": palettedTestImage(),
		"rgba":     twoColour(10, 6, color.RGBA{G: 255, A: 255}, color.RGBA{B: 255, A: 128}),
	} {
		t.Run(name, func(t *testing.T) {
			x := fullPNGExtras()
			in := buildPNG(t, img, x)
			info, out := stripTestPNG(t, in)
			inChunks, trailing := parseTestPNG(t, out)
			if len(trailing) != 0 {
				t.Fatalf("bytes after IEND")
			}
			checkPNGChunks(t, pngChunks(t, in), inChunks)
			wantICCP, err := iccpChunkData(x.ICC)
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			if info.Width != b.Dx() || info.Height != b.Dy() || !bytes.Equal(info.ICCP, wantICCP) || info.Animated {
				t.Fatalf("info = %+v", info)
			}
			checkSamePNGPixels(t, in, out)
		})
	}
}

// checkPNGChunks fails unless out holds exactly the allow-list chunks of in, unchanged and in
// input order, and in held every inserted metadata chunk.
func checkPNGChunks(t *testing.T, in, out []pngChunkT) {
	t.Helper()
	types := map[string]bool{}
	for _, c := range in {
		types[c.Type] = true
	}
	for _, typ := range []string{"tEXt", "zTXt", "iTXt", "eXIf", "tIME", "sPLT", testPrivatePNG, "iCCP"} {
		if !types[typ] {
			t.Fatalf("input lacks %s", typ)
		}
	}
	want := keptChunks(in)
	if len(want) != len(out) {
		t.Fatalf("%d chunks, want %d", len(out), len(want))
	}
	for i := range want {
		if want[i].Type != out[i].Type || !bytes.Equal(want[i].Data, out[i].Data) {
			t.Fatalf("chunk %d: %s, want %s unchanged", i, out[i].Type, want[i].Type)
		}
	}
}

// checkSamePNGPixels fails unless image/png decodes a and b to equal pixels.
func checkSamePNGPixels(t *testing.T, a, b []byte) {
	t.Helper()
	ia, err := png.Decode(bytes.NewReader(a))
	if err != nil {
		t.Fatalf("decode input: %v", err)
	}
	ib, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	samePixels(t, ia, ib)
}

func TestStripPNGAnimated(t *testing.T) {
	in := buildPNG(t, palettedTestImage(), pngExtras{})
	ihdrEnd := len(pngSignature) + 12 + 13
	actl := pngChunk("acTL", []byte{0, 0, 0, 1, 0, 0, 0, 0})
	in = append(append(append([]byte{}, in[:ihdrEnd]...), actl...), in[ihdrEnd:]...)
	info, out := stripTestPNG(t, in)
	if !info.Animated || info.ColourType != 3 || info.ICCP != nil {
		t.Fatalf("info = %+v", info)
	}
	if !bytes.Contains(out, actl) {
		t.Fatalf("acTL dropped")
	}
}

// malformedPNGs are broken inputs of TestStripPNGMalformed and the fuzz seeds.
func malformedPNGs(tb testing.TB) map[string][]byte {
	tb.Helper()
	good := buildPNG(tb, palettedTestImage(), pngExtras{})
	sig := len(pngSignature)
	idat := bytes.Index(good, []byte("IDAT")) - 4
	crc := append([]byte{}, good...)
	crc[idat+8] ^= 0xFF
	text := pngChunk("tEXt", []byte("a\x00b"))
	huge := append([]byte{}, good...)
	binary.BigEndian.PutUint32(huge[idat:], 0x80000000)
	shortIHDR := append([]byte(pngSignature), pngChunk("IHDR", []byte{0, 0, 0, 1})...)
	return map[string][]byte{
		"bad signature":     append([]byte("\x89PNG\r\n\x1a\x0b"), good[sig:]...),
		"crc mismatch":      crc,
		"IHDR not first":    append(append(append([]byte{}, good[:sig]...), text...), good[sig:]...),
		"missing IEND":      good[:len(good)-12],
		"length beyond end": good[:idat+20],
		"length above 2^31": huge,
		"short IHDR":        append(shortIHDR, pngChunk("IEND", nil)...),
		"signature only":    good[:sig],
		"zero bytes":        {},
		"truncated header":  good[:sig+5],
	}
}

func TestStripPNGMalformed(t *testing.T) {
	for name, in := range malformedPNGs(t) {
		t.Run(name, func(t *testing.T) {
			_, err := stripPNG(bytes.NewReader(in), &bytes.Buffer{})
			if !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("error %v, want ErrImageMalformed", err)
			}
		})
	}
}

func TestStripPNGWriteAndReadErrorsUnwrapped(t *testing.T) {
	in := buildPNG(t, palettedTestImage(), pngExtras{})
	if _, err := stripPNG(bytes.NewReader(in), failingWriter{}); !errors.Is(err, errTestWrite) || errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("write error %v", err)
	}
	if _, err := stripPNG(&failingReader{data: in[:len(in)/2]}, &bytes.Buffer{}); !errors.Is(err, errTestRead) || errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("read error %v", err)
	}
}

// FuzzStripPNG checks the ADR 0047 property: for any input the filter fails, or its output holds
// only allow-list chunks with valid CRCs and nothing after IEND.
func FuzzStripPNG(f *testing.F) {
	f.Add(buildPNG(f, palettedTestImage(), fullPNGExtras()))
	f.Add(buildPNG(f, twoColour(4, 4, color.Black, color.White), pngExtras{Text: true, Trailing: true}))
	for _, b := range malformedPNGs(f) {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		var out bytes.Buffer
		if _, err := stripPNG(bytes.NewReader(in), &out); err != nil {
			return
		}
		chunks, trailing := parseTestPNG(t, out.Bytes())
		if len(trailing) != 0 {
			t.Fatalf("bytes after IEND")
		}
		if chunks[0].Type != "IHDR" {
			t.Fatalf("first chunk %s", chunks[0].Type)
		}
		for _, c := range chunks {
			if !domain.PNGChunkKept(c.Type) {
				t.Fatalf("chunk %q kept", c.Type)
			}
		}
	})
}
