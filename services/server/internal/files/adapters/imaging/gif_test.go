// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io"
	"testing"
	"testing/iotest"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// gifTestNetscape is the identifier of the loop count application extension.
const gifTestNetscape = "NETSCAPE2.0"

// gifTestColors is the number of entries of gifTestPalette.
const gifTestColors = 4

// gifTestPalette is the global palette of the synthetic GIFs: four colours, so the global colour
// table has four entries (12 bytes).
var gifTestPalette = color.Palette{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{255, 0, 0, 255},
	color.RGBA{0, 255, 0, 255},
	color.RGBA{0, 0, 255, 255},
}

// gifTestFrame returns a 6x4 paletted frame whose pixels are a pattern derived from seed, so
// every frame differs.
func gifTestFrame(seed uint8) *image.Paletted {
	m := image.NewPaletted(image.Rect(0, 0, 6, 4), gifTestPalette)
	idx := seed % gifTestColors
	for i := range m.Pix {
		m.Pix[i] = idx
		idx = (idx + 1) % gifTestColors
	}
	return m
}

// gifTestAnimation returns an animated GIF of three frames with delays, disposal methods, loop
// count 3 and background index 2, encoded with image/gif.
func gifTestAnimation(t testing.TB) []byte {
	t.Helper()
	g := &gif.GIF{
		Image:           []*image.Paletted{gifTestFrame(0), gifTestFrame(1), gifTestFrame(2)},
		Delay:           []int{10, 20, 30},
		Disposal:        []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalPrevious},
		LoopCount:       3,
		BackgroundIndex: 2,
		Config:          image.Config{ColorModel: gifTestPalette, Width: 6, Height: 4},
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("EncodeAll: %v", err)
	}
	return buf.Bytes()
}

// gifTestSplice inserts extra blocks right after the global colour table of data (the header is
// 13 bytes, the table follows when the packed byte says so).
func gifTestSplice(t testing.TB, data, extra []byte) []byte {
	t.Helper()
	off := 13
	if data[10]&0x80 != 0 {
		off += 3 << (data[10]&0x07 + 1)
	}
	out := append([]byte{}, data[:off]...)
	out = append(out, extra...)
	return append(out, data[off:]...)
}

// gifTestExtras are a comment extension and an XMP application extension, both of which the
// re-encoding must drop.
var gifTestExtras = []byte{
	0x21, 0xFE, 12, 'S', 'a', 'm', ' ', 'w', 'a', 's', ' ', 'h', 'e', 'r', 'e', 0x00,
	0x21, 0xFF, 11, 'X', 'M', 'P', ' ', 'D', 'a', 't', 'a', 'X', 'M', 'P', 4, '<', 'x', '/', '>', 0x00,
}

// gifTestBlock is one extension seen by gifTestExtensions: its label and, for an application
// extension, its 11-byte identifier.
type gifTestBlock struct {
	label byte
	app   string
}

// gifTestExtensions walks the block structure of data independently of scanGIF and lists every
// extension, so the test does not trust the scanner it checks.
func gifTestExtensions(t *testing.T, data []byte) []gifTestBlock {
	t.Helper()
	r := bufio.NewReader(bytes.NewReader(data))
	head := make([]byte, 13)
	if _, err := io.ReadFull(r, head); err != nil {
		t.Fatalf("header: %v", err)
	}
	if head[10]&0x80 != 0 {
		gifTestDiscard(t, r, 3<<(head[10]&0x07+1))
	}
	var out []gifTestBlock
	for {
		b := gifTestByte(t, r)
		switch b {
		case 0x3B:
			return out
		case 0x2C:
			desc := make([]byte, 9)
			if _, err := io.ReadFull(r, desc); err != nil {
				t.Fatalf("descriptor: %v", err)
			}
			if desc[8]&0x80 != 0 {
				gifTestDiscard(t, r, 3<<(desc[8]&0x07+1))
			}
			gifTestByte(t, r) // LZW minimum code size
			gifTestSubBlocks(t, r)
		case 0x21:
			blk := gifTestBlock{label: gifTestByte(t, r)}
			if blk.label == 0xFF {
				first := gifTestSubBlocks(t, r)
				blk.app = string(first)
			} else {
				gifTestSubBlocks(t, r)
			}
			out = append(out, blk)
		default:
			t.Fatalf("unknown block 0x%02X", b)
		}
	}
}

// gifTestByte reads one byte or fails the test.
func gifTestByte(t *testing.T, r *bufio.Reader) byte {
	t.Helper()
	b, err := r.ReadByte()
	if err != nil {
		t.Fatalf("read byte: %v", err)
	}
	return b
}

// gifTestDiscard skips n bytes or fails the test.
func gifTestDiscard(t *testing.T, r *bufio.Reader, n int) {
	t.Helper()
	if _, err := r.Discard(n); err != nil {
		t.Fatalf("discard: %v", err)
	}
}

// gifTestSubBlocks skips a sub-block sequence and returns the data of its first sub-block.
func gifTestSubBlocks(t *testing.T, r *bufio.Reader) []byte {
	t.Helper()
	var first []byte
	for i := 0; ; i++ {
		n := int(gifTestByte(t, r))
		if n == 0 {
			return first
		}
		data := make([]byte, n)
		if _, err := io.ReadFull(r, data); err != nil {
			t.Fatalf("sub-block: %v", err)
		}
		if i == 0 {
			first = data
		}
	}
}

func TestGIFScanAndReencode(t *testing.T) {
	data := gifTestSplice(t, gifTestAnimation(t), gifTestExtras)

	info, err := scanGIF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("scanGIF: %v", err)
	}
	want := gifInfo{Width: 6, Height: 4, Frames: 3, LoopCount: 3, HasLoop: true}
	if info != want {
		t.Fatalf("scanGIF = %+v, want %+v", info, want)
	}

	out, err := reencodeGIF(data)
	if err != nil {
		t.Fatalf("reencodeGIF: %v", err)
	}
	for _, blk := range gifTestExtensions(t, out) {
		if blk.label == 0xFE {
			t.Errorf("re-encoded GIF still has a comment extension")
		}
		if blk.label == 0xFF && blk.app != gifTestNetscape {
			t.Errorf("re-encoded GIF still has application extension %q", blk.app)
		}
	}
	if !bytes.Contains(out, []byte(gifTestNetscape)) {
		t.Errorf("re-encoded GIF lost its loop count extension")
	}

	src, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeAll source: %v", err)
	}
	got, err := gif.DecodeAll(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("DecodeAll output: %v", err)
	}
	gifTestEqual(t, got, src)
}

// gifTestEqual compares everything reencodeGIF promises to keep.
func gifTestEqual(t *testing.T, got, want *gif.GIF) {
	t.Helper()
	if got.LoopCount != want.LoopCount || got.BackgroundIndex != want.BackgroundIndex {
		t.Errorf("loop %d background %d, want loop %d background %d",
			got.LoopCount, got.BackgroundIndex, want.LoopCount, want.BackgroundIndex)
	}
	if len(got.Image) != len(want.Image) {
		t.Fatalf("frames %d, want %d", len(got.Image), len(want.Image))
	}
	for i := range want.Image {
		g, w := got.Image[i], want.Image[i]
		if g.Rect != w.Rect || !bytes.Equal(g.Pix, w.Pix) {
			t.Errorf("frame %d: pixels or bounds differ", i)
		}
		if len(g.Palette) != len(w.Palette) {
			t.Errorf("frame %d: palette length %d, want %d", i, len(g.Palette), len(w.Palette))
		}
		for j := range w.Palette {
			if j < len(g.Palette) && g.Palette[j] != w.Palette[j] {
				t.Errorf("frame %d: palette entry %d differs", i, j)
			}
		}
		if got.Delay[i] != want.Delay[i] || got.Disposal[i] != want.Disposal[i] {
			t.Errorf("frame %d: delay %d disposal %d, want %d %d",
				i, got.Delay[i], got.Disposal[i], want.Delay[i], want.Disposal[i])
		}
	}
}

func TestGIFStill(t *testing.T) {
	var buf bytes.Buffer
	if err := gif.Encode(&buf, gifTestFrame(0), nil); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	data := buf.Bytes()

	info, err := scanGIF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("scanGIF: %v", err)
	}
	want := gifInfo{Width: 6, Height: 4, Frames: 1}
	if info != want {
		t.Fatalf("scanGIF = %+v, want %+v", info, want)
	}

	out, err := reencodeGIF(data)
	if err != nil {
		t.Fatalf("reencodeGIF: %v", err)
	}
	if bytes.Contains(out, []byte(gifTestNetscape)) {
		t.Errorf("a still GIF gained a loop count extension")
	}
	got, err := gif.DecodeAll(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if got.LoopCount != -1 {
		t.Errorf("LoopCount = %d, want -1 (no extension)", got.LoopCount)
	}
	src, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeAll source: %v", err)
	}
	gifTestEqual(t, got, src)
}

func TestScanGIFMalformed(t *testing.T) {
	anim := gifTestAnimation(t)
	unknown := gifTestSplice(t, anim, []byte{0x99})
	cases := map[string][]byte{
		"empty":          nil,
		"bad signature":  append([]byte("GIF88a"), anim[6:]...),
		"not a gif":      []byte("\x89PNG\r\n\x1a\n0000000000000"),
		"truncated head": anim[:10],
		"truncated body": anim[:len(anim)/2],
		"no trailer":     anim[:len(anim)-1],
		"unknown block":  unknown,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := scanGIF(bytes.NewReader(data)); !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("scanGIF error = %v, want ErrImageMalformed", err)
			}
		})
	}
}

func TestScanGIFReadError(t *testing.T) {
	boom := errors.New("store read failed")
	r := io.MultiReader(bytes.NewReader(gifTestAnimation(t)[:20]), iotest.ErrReader(boom))
	_, err := scanGIF(r)
	if !errors.Is(err, boom) || errors.Is(err, app.ErrImageMalformed) {
		t.Fatalf("scanGIF error = %v, want the read error and not ErrImageMalformed", err)
	}
}

func TestReencodeGIFUndecodable(t *testing.T) {
	anim := gifTestAnimation(t)
	for name, data := range map[string][]byte{
		"garbage":   []byte("not a gif at all"),
		"truncated": anim[:len(anim)/2],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := reencodeGIF(data); !errors.Is(err, app.ErrImageUndecodable) {
				t.Fatalf("reencodeGIF error = %v, want ErrImageUndecodable", err)
			}
		})
	}
}

func FuzzGIFScan(f *testing.F) {
	var still bytes.Buffer
	if err := gif.Encode(&still, gifTestFrame(0), nil); err != nil {
		f.Fatalf("Encode: %v", err)
	}
	f.Add(still.Bytes())
	f.Add(gifTestSplice(f, gifTestAnimation(f), gifTestExtras))
	f.Add([]byte("GIF89a"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		info, err := scanGIF(bytes.NewReader(data))
		if err != nil {
			return
		}
		if info.Width < 0 || info.Height < 0 || info.Frames < 0 || info.LoopCount < 0 {
			t.Fatalf("negative values in %+v", info)
		}
		// The pixel guard relies on the scan counting the frames DecodeAll decodes. Only small
		// screens are decoded here, so a fuzzed frame never allocates much.
		if info.Width*info.Height > gifFuzzMaxScreen {
			return
		}
		if g, err := gif.DecodeAll(bytes.NewReader(data)); err == nil && len(g.Image) != info.Frames {
			t.Fatalf("scan counts %d frames, DecodeAll decodes %d", info.Frames, len(g.Image))
		}
	})
}

// gifFuzzMaxScreen bounds the logical screen the fuzz target decodes.
const gifFuzzMaxScreen = 64 * 64

// gifHiddenFrames returns an animation of frames frames of w x h with a two-colour global palette
// and inject spliced in after the global colour table, the form of the review finding where the
// scan once stopped at an injected trailer while DecodeAll read on.
func gifHiddenFrames(tb testing.TB, w, h, frames int, inject []byte) []byte {
	tb.Helper()
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{LoopCount: -1, Config: image.Config{ColorModel: pal, Width: w, Height: h}}
	for range frames {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, w, h), pal))
		g.Delay = append(g.Delay, 0)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		tb.Fatal(err)
	}
	b := buf.Bytes()
	const afterGlobalTable = 13 + 6 // header, screen descriptor and a two-entry colour table
	out := append([]byte{}, b[:afterGlobalTable]...)
	out = append(out, inject...)
	return append(out, b[afterGlobalTable:]...)
}

// TestGIFScanMatchesDecoder: plain text and application extensions with unusual sizes cannot hide
// frames from the scan, so a GIF whose frames multiply above the limit is never decoded.
func TestGIFScanMatchesDecoder(t *testing.T) {
	t.Parallel()
	plainText := append([]byte{0x21, 0x01, 0x00, 0x3B}, make([]byte, 12)...)
	appExt := append([]byte{0x21, 0xFF, 0x00, 0x3B}, make([]byte, 59)...)
	appExt = append(appExt, 0x00)
	for name, inject := range map[string][]byte{"plain text": plainText, "application size 0": appExt} {
		data := gifHiddenFrames(t, 100, 100, 20, inject)
		info, err := scanGIF(bytes.NewReader(data))
		g, derr := gif.DecodeAll(bytes.NewReader(data))
		if err == nil && derr == nil && info.Frames != len(g.Image) {
			t.Errorf("%s: scan %d frames, DecodeAll %d", name, info.Frames, len(g.Image))
		}
		calls := 0
		c := Codec{reencode: func(b []byte) ([]byte, error) { calls++; return reencodeGIF(b) }}
		open := func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
		_, err = c.CleanOriginal(t.Context(), domain.PlanReencodeGIF, open, int64(len(data)), app.CleanLimits{MaxPixels: 100 * 100 * 2, JPEGQuality: 90})
		if calls != 0 || err == nil {
			t.Errorf("%s: CleanOriginal = %v after %d re-encodes, want a refusal without decode", name, err, calls)
		}
	}
}
