// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"encoding/binary"
	"image"
	"image/color"
	"slices"
	"testing"
)

// webpOptions selects what buildWebP writes.
type webpOptions struct {
	VP8X     bool   // wrap in VP8X (implied by any option below)
	Animated bool   // set the VP8X animation flag and add an ANIM chunk and one ANMF frame chunk
	ANIMOnly bool   // add an ANIM chunk without setting the flag
	EXIF     []byte // EXIF chunk payload (TIFF block)
	ICC      []byte // ICCP chunk payload
	XMP      bool   // an "XMP " chunk with synthetic text
}

// webpBitWriter packs values LSB first, the bit order of the VP8L bitstream. It sets single
// bits so no integer conversion can overflow.
type webpBitWriter struct {
	buf []byte
	n   uint // bits written so far
}

// write appends the low bits of v.
func (w *webpBitWriter) write(v uint32, bits uint) {
	for i := range bits {
		if w.n%8 == 0 {
			w.buf = append(w.buf, 0)
		}
		if v>>i&1 == 1 {
			w.buf[len(w.buf)-1] |= 1 << (w.n % 8)
		}
		w.n++
	}
}

// webpTestU32 converts a test size or index to uint32.
func webpTestU32(n int) uint32 {
	return uint32(n) //nolint:gosec // G115: test images and chunks are far below 4 GiB.
}

// webpSimpleCode is a VP8L "simple" prefix code of one or two 8-bit symbols, sorted ascending
// because the canonical code gives the smaller symbol the code 0.
type webpSimpleCode []uint8

// header writes the code definition: simple, num_symbols-1, is_first_8bits = 1, the symbols.
func (c webpSimpleCode) header(w *webpBitWriter) {
	w.write(1, 1)
	w.write(webpTestU32(len(c)-1), 1)
	w.write(1, 1)
	for _, s := range c {
		w.write(uint32(s), 8)
	}
}

// symbol writes the code of v: nothing for a one-symbol code, one bit otherwise.
func (c webpSimpleCode) symbol(w *webpBitWriter, v uint8) {
	if len(c) == 2 {
		w.write(webpTestU32(slices.Index(c, v)), 1)
	}
}

// webpChannelCode collects the distinct values of one channel and fails the test when there are
// more than two.
func webpChannelCode(tb testing.TB, values []uint8) webpSimpleCode {
	tb.Helper()
	var c webpSimpleCode
	for _, v := range values {
		if !slices.Contains(c, v) {
			c = append(c, v)
		}
	}
	if len(c) > 2 {
		tb.Fatalf("buildWebP: a channel has %d distinct values, the encoder takes at most 2", len(c))
	}
	slices.Sort(c)
	return c
}

// webpVP8L encodes img as a VP8L bitstream (no transforms, no colour cache, simple codes) and
// reports whether any alpha is below 255.
func webpVP8L(tb testing.TB, img image.Image) ([]byte, bool) {
	tb.Helper()
	b := img.Bounds()
	var ch [4][]uint8 // green, red, blue, alpha in coding order
	alpha := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			ch[0] = append(ch[0], c.G)
			ch[1] = append(ch[1], c.R)
			ch[2] = append(ch[2], c.B)
			ch[3] = append(ch[3], c.A)
			alpha = alpha || c.A < 0xFF
		}
	}
	var codes [4]webpSimpleCode
	for i := range ch {
		codes[i] = webpChannelCode(tb, ch[i])
	}
	w := &webpBitWriter{buf: []byte{0x2F}}
	w.write(webpTestU32(b.Dx()-1), 14)
	w.write(webpTestU32(b.Dy()-1), 14)
	if alpha {
		w.write(1, 1)
	} else {
		w.write(0, 1)
	}
	w.write(0, 3) // version
	w.write(0, 1) // transform_present
	w.write(0, 1) // color_cache
	w.write(0, 1) // meta_prefix
	for _, c := range codes {
		c.header(w)
	}
	webpSimpleCode{0}.header(w) // distance
	for i := range ch[0] {
		for k := range codes {
			codes[k].symbol(w, ch[k][i])
		}
	}
	return w.buf, alpha
}

// webpTestChunk returns a RIFF chunk with its padding byte.
func webpTestChunk(fourcc string, payload []byte) []byte {
	out := append([]byte(fourcc), binary.LittleEndian.AppendUint32(nil, webpTestU32(len(payload)))...)
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// webpTestRIFF wraps chunks into the RIFF WEBP container.
func webpTestRIFF(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, webpTestU32(len(body)))...)
	return append(out, body...)
}

// webpTestUint24 appends v as 24-bit little endian.
func webpTestUint24(b []byte, v int) []byte {
	return append(b, binary.LittleEndian.AppendUint32(nil, webpTestU32(v))[:3]...)
}

// buildWebP encodes img as a lossless VP8L bitstream and wraps it as requested. The encoder is
// minimal: no transforms, no colour cache, no backward references; every one of the four
// channels is coded with a simple prefix code of one or two symbols, so each channel of img may
// take at most two distinct values (the test fails otherwise). For Animated the VP8L chunk sits
// inside the ANMF frame, as in a real animation; for ANIMOnly it stays at the top level.
func buildWebP(tb testing.TB, img image.Image, o webpOptions) []byte {
	tb.Helper()
	bits, alpha := webpVP8L(tb, img)
	vp8l := webpTestChunk("VP8L", bits)
	if !o.VP8X && !o.Animated && !o.ANIMOnly && o.EXIF == nil && o.ICC == nil && !o.XMP {
		return webpTestRIFF(vp8l)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var flags byte
	for _, f := range []struct {
		on  bool
		bit byte
	}{{o.ICC != nil, 0x20}, {alpha, 0x10}, {o.EXIF != nil, 0x08}, {o.XMP, 0x04}, {o.Animated, 0x02}} {
		if f.on {
			flags |= f.bit
		}
	}
	vp8x := webpTestUint24(webpTestUint24([]byte{flags, 0, 0, 0}, w-1), h-1)
	chunks := [][]byte{webpTestChunk("VP8X", vp8x)}
	if o.ICC != nil {
		chunks = append(chunks, webpTestChunk("ICCP", o.ICC))
	}
	if o.Animated || o.ANIMOnly {
		// Background colour (4 bytes) and loop count 0.
		chunks = append(chunks, webpTestChunk("ANIM", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0, 0}))
	}
	if o.Animated {
		// Frame X/2, Y/2, width-1, height-1, duration 100 ms, flags, then the frame bitstream.
		frame := webpTestUint24(webpTestUint24(webpTestUint24(webpTestUint24(nil, 0), 0), w-1), h-1)
		frame = append(webpTestUint24(frame, 100), 0)
		chunks = append(chunks, webpTestChunk("ANMF", append(frame, vp8l...)))
	} else {
		chunks = append(chunks, vp8l)
	}
	if o.EXIF != nil {
		chunks = append(chunks, webpTestChunk("EXIF", o.EXIF))
	}
	if o.XMP {
		chunks = append(chunks, webpTestChunk("XMP ", []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/">Robin</x:xmpmeta>`)))
	}
	return webpTestRIFF(chunks...)
}
