// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The synthetic metadata the builders insert. The GPS values point nowhere (0 degrees, 0 minutes,
// 0 seconds), the names are the test people of the project.
const (
	testXMPText     = `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:Description exif:GPSLatitude="0,0.0N" dc:creator="Alex"/></x:xmpmeta>`
	testCommentText = "comment written by Sam for example.com"
	testIPTCText    = "Robin"
	testPrivatePNG  = "prVt"
	// testColourRGB is the ICC colour space signature of RGB profiles.
	testColourRGB = "RGB "
)

// jpegExtras selects the metadata buildJPEG inserts.
type jpegExtras struct {
	Orientation   domain.Orientation // 0: no EXIF orientation tag
	EXIF          bool               // APP1 Exif with GPS IFD (synthetic coordinates) and an embedded EXIF thumbnail (IFD1 with JPEGInterchangeFormat)
	JFIFThumbnail bool               // APP0 JFIF with a 2x2 embedded RGB thumbnail
	JFXX          bool
	XMP           bool
	IPTC          bool
	COM           bool
	MPF           bool
	ICC           []byte // embedded as APP2 ICC_PROFILE
	ICCParts      int    // split ICC into this many parts (0 or 1: one part)
	Adobe         bool   // APP14 Adobe
	Appended      bool   // a second complete JPEG after EOI
}

// buildJPEG encodes img with image/jpeg (quality 95) and inserts the extras after SOI (APP0
// first, then APP1 Exif, APP1 XMP, APP13, COM, APP2 MPF, APP2 ICC parts, APP14), and appends a
// second JPEG if asked.
func buildJPEG(tb testing.TB, img image.Image, x jpegExtras) []byte {
	tb.Helper()
	plain := encodeTestJPEG(tb, img)
	out := append([]byte{}, plain[:2]...)
	for _, seg := range jpegExtraSegments(tb, x) {
		out = append(out, seg...)
	}
	out = append(out, plain[2:]...)
	if x.Appended {
		out = append(out, encodeTestJPEG(tb, twoColour(8, 8, color.White, color.Black))...)
	}
	return out
}

// encodeTestJPEG encodes img with image/jpeg at quality 95.
func encodeTestJPEG(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		tb.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// jpegExtraSegments returns the complete segments of x in the order buildJPEG inserts them.
func jpegExtraSegments(tb testing.TB, x jpegExtras) [][]byte {
	tb.Helper()
	var segs [][]byte
	if x.JFIFThumbnail {
		segs = append(segs, testSegment(0xE0, jfifWithThumbnail()))
	}
	if x.JFXX {
		segs = append(segs, testSegment(0xE0, jfxxPayload()))
	}
	if x.EXIF || x.Orientation != 0 {
		segs = append(segs, testSegment(0xE1, append([]byte(exifIdentifier), testTIFF(tb, x)...)))
	}
	if x.XMP {
		segs = append(segs, testSegment(0xE1, append([]byte("http://ns.adobe.com/xap/1.0/\x00"), testXMPText...)))
	}
	if x.IPTC {
		segs = append(segs, testSegment(0xED, append([]byte("Photoshop 3.0\x008BIM\x04\x04\x00\x00\x00\x00\x00\x09\x1c\x02\x50\x00\x05"), testIPTCText...)))
	}
	if x.COM {
		segs = append(segs, testSegment(0xFE, []byte(testCommentText)))
	}
	if x.MPF {
		segs = append(segs, testSegment(0xE2, []byte("MPF\x00MM\x00\x2a\x00\x00\x00\x08\x00\x00")))
	}
	segs = append(segs, testICCSegments(x.ICC, x.ICCParts)...)
	if x.Adobe {
		// Version 100, flags 0, transform 1 (YCbCr), as image/jpeg writes the components.
		segs = append(segs, testSegment(0xEE, []byte("Adobe\x00\x64\x00\x00\x00\x00\x01")))
	}
	return segs
}

// testSegment returns one complete marker segment with its big-endian length.
func testSegment(marker byte, payload []byte) []byte {
	seg := []byte{0xFF, marker}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2)) //nolint:gosec // G115: test payloads are far below 64 KiB
	return append(seg, payload...)
}

// jfifWithThumbnail is a JFIF 1.02 APP0 payload with a 2x2 RGB thumbnail.
func jfifWithThumbnail() []byte {
	p := []byte("JFIF\x00\x01\x02\x01\x00\x48\x00\x48\x02\x02")
	return append(p, bytes.Repeat([]byte{0x10, 0x80, 0xF0}, 4)...)
}

// jfxxPayload is a JFXX APP0 payload with a 2x2 RGB thumbnail (extension code 0x13).
func jfxxPayload() []byte {
	p := []byte("JFXX\x00\x13\x02\x02")
	return append(p, bytes.Repeat([]byte{0xF0, 0x10, 0x80}, 4)...)
}

// testICCSegments splits profile into parts APP2 ICC_PROFILE segments of nearly equal size.
func testICCSegments(profile []byte, parts int) [][]byte {
	if len(profile) == 0 {
		return nil
	}
	parts = max(parts, 1)
	size := (len(profile) + parts - 1) / parts
	var segs [][]byte
	var seq byte
	for i := range parts {
		seq++
		part := profile[min(i*size, len(profile)):min((i+1)*size, len(profile))]
		payload := append([]byte(iccIdentifier), seq, 0)
		segs = append(segs, testSegment(0xE2, append(payload, part...)))
	}
	for _, s := range segs {
		s[4+len(iccIdentifier)+1] = seq
	}
	return segs
}

// tiffEntry is one IFD entry of the test TIFF builder with its value bytes (inline when at most
// four bytes, else stored after the IFDs).
type tiffEntry struct {
	tag, typ uint16
	count    uint32
	value    []byte
}

// testTIFF returns a little-endian TIFF block for x: IFD0 with the orientation (if any) and, with
// x.EXIF, a GPS IFD pointer, a GPS IFD with synthetic coordinates and IFD1 with an embedded JPEG
// thumbnail.
func testTIFF(tb testing.TB, x jpegExtras) []byte {
	tb.Helper()
	var ifd0 []tiffEntry
	if x.Orientation != 0 {
		ifd0 = append(ifd0, tiffEntry{tag: 0x0112, typ: 3, count: 1, value: []byte{byte(x.Orientation), 0}})
	}
	if !x.EXIF {
		return layoutTIFF(ifd0, nil, nil)
	}
	gps := []tiffEntry{
		{tag: 0x0001, typ: 2, count: 2, value: []byte("N\x00")},
		{tag: 0x0002, typ: 5, count: 3, value: make([]byte, 24)},
	}
	thumb := encodeTestJPEG(tb, twoColour(8, 8, color.Black, color.White))
	return layoutTIFF(ifd0, gps, thumb)
}

// layoutTIFF lays out IFD0 (plus a GPS pointer when gps is set), the GPS IFD, IFD1 with the
// thumbnail pointers and the out-of-line values, all little-endian.
func layoutTIFF(ifd0, gps []tiffEntry, thumb []byte) []byte {
	le := binary.LittleEndian
	ifdSize := func(n int) int { return 2 + 12*n + 4 }
	n0 := len(ifd0)
	if gps != nil {
		n0++
	}
	gpsOff := 8 + ifdSize(n0)
	ifd1Off := gpsOff + ifdSize(len(gps))
	dataOff := ifd1Off + ifdSize(2)
	if gps == nil {
		dataOff = gpsOff
	}
	var data []byte
	place := func(e tiffEntry) []byte {
		out := le.AppendUint16(nil, e.tag)
		out = le.AppendUint16(out, e.typ)
		out = le.AppendUint32(out, e.count)
		if len(e.value) <= 4 {
			return append(out, append(e.value, make([]byte, 4-len(e.value))...)...)
		}
		out = le.AppendUint32(out, uint32(dataOff+len(data))) //nolint:gosec // G115: test offsets are tiny
		data = append(data, e.value...)
		return out
	}
	b := []byte("II\x2a\x00\x08\x00\x00\x00")
	b = le.AppendUint16(b, uint16(n0)) //nolint:gosec // G115: a handful of entries
	for _, e := range ifd0 {
		b = append(b, place(e)...)
	}
	if gps == nil {
		b = le.AppendUint32(b, 0)
		return append(b, data...)
	}
	b = append(b, place(tiffEntry{tag: 0x8825, typ: 4, count: 1, value: le.AppendUint32(nil, uint32(gpsOff))})...) //nolint:gosec // G115: tiny offset
	b = le.AppendUint32(b, uint32(ifd1Off))                                                                        //nolint:gosec // G115: tiny offset
	b = le.AppendUint16(b, uint16(len(gps)))                                                                       //nolint:gosec // G115: tiny count
	for _, e := range gps {
		b = append(b, place(e)...)
	}
	b = le.AppendUint32(b, 0)
	// Every out-of-line value is placed now; the thumbnail follows them.
	thumbOff := dataOff + len(data)
	b = le.AppendUint16(b, 2)
	b = append(b, place(tiffEntry{tag: 0x0201, typ: 4, count: 1, value: le.AppendUint32(nil, uint32(thumbOff))})...)   //nolint:gosec // G115: tiny offset
	b = append(b, place(tiffEntry{tag: 0x0202, typ: 4, count: 1, value: le.AppendUint32(nil, uint32(len(thumb)))})...) //nolint:gosec // G115: tiny thumbnail
	b = le.AppendUint32(b, 0)
	b = append(b, data...)
	return append(b, thumb...)
}

// pngExtras selects the chunks buildPNG inserts.
type pngExtras struct {
	ICC      []byte // iCCP after IHDR
	Text     bool   // tEXt, zTXt, iTXt (with synthetic XMP text containing "GPSLatitude")
	EXIF     bool   // eXIf with GPS
	Misc     bool   // tIME, hIST (only with a palette image; otherwise skip), sPLT, a private chunk "prVt" that is safe to copy
	Trailing bool   // bytes after IEND
}

// buildPNG encodes img with image/png and inserts the extras: iCCP right after IHDR, the other
// chunks right before the first IDAT (after PLTE), trailing bytes after IEND.
func buildPNG(tb testing.TB, img image.Image, x pngExtras) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("encode png: %v", err)
	}
	out := []byte(pngSignature)
	inserted := false
	for _, c := range pngChunks(tb, buf.Bytes()) {
		if c.Type == "IDAT" && !inserted {
			out = append(out, pngExtraChunks(tb, x, hasChunk(tb, buf.Bytes(), "PLTE"))...)
			inserted = true
		}
		out = append(out, pngChunk(c.Type, c.Data)...)
		if c.Type == "IHDR" && x.ICC != nil {
			data, err := iccpChunkData(x.ICC)
			if err != nil {
				tb.Fatalf("iccp: %v", err)
			}
			out = append(out, pngChunk("iCCP", data)...)
		}
	}
	if x.Trailing {
		out = append(out, "trailing bytes from example.com"...)
	}
	return out
}

// hasChunk reports whether the PNG b has a chunk of type typ.
func hasChunk(tb testing.TB, b []byte, typ string) bool {
	tb.Helper()
	for _, c := range pngChunks(tb, b) {
		if c.Type == typ {
			return true
		}
	}
	return false
}

// pngExtraChunks returns the complete chunks of x that go before the first IDAT.
func pngExtraChunks(tb testing.TB, x pngExtras, palette bool) []byte {
	tb.Helper()
	var out []byte
	if x.Text {
		out = append(out, pngChunk("tEXt", []byte("Author\x00Alex"))...)
		out = append(out, pngChunk("zTXt", append([]byte("Comment\x00\x00"), zlibTest(tb, []byte(testCommentText))...))...)
		out = append(out, pngChunk("iTXt", append([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), testXMPText...))...)
	}
	if x.EXIF {
		out = append(out, pngChunk("eXIf", testTIFF(tb, jpegExtras{Orientation: 6, EXIF: true}))...)
	}
	if x.Misc {
		out = append(out, pngChunk("tIME", []byte{0x07, 0xEA, 10, 20, 7, 0, 0})...)
		if palette {
			out = append(out, pngChunk("hIST", make([]byte, 2*256))...)
		}
		out = append(out, pngChunk("sPLT", []byte("Sam\x00\x08\x01\x02\x03\x04\x00\x01"))...)
		out = append(out, pngChunk(testPrivatePNG, []byte("private data"))...)
	}
	return out
}

// zlibTest returns the zlib stream of b (through the production helper's compressor).
func zlibTest(tb testing.TB, b []byte) []byte {
	tb.Helper()
	data, err := iccpChunkData(b)
	if err != nil {
		tb.Fatalf("zlib: %v", err)
	}
	return data[len(iccpProfileName)+2:]
}

// syntheticICC returns a profile of size bytes (at least 132) with a valid 128-byte header: size
// field, signature "acsp" at 36, colour space at 16 (for example "RGB ", "GRAY", "CMYK"), a tag
// count 0, and deterministic filler bytes.
func syntheticICC(colourSpace string, size int) []byte {
	size = max(size, 132)
	p := make([]byte, size)
	filler := byte(3)
	for i := range p {
		p[i] = filler
		filler += 7
	}
	binary.BigEndian.PutUint32(p[0:], uint32(size)) //nolint:gosec // G115: test sizes are far below 4 GiB
	copy(p[4:8], "none")
	binary.BigEndian.PutUint32(p[8:], 0x04300000)
	copy(p[12:16], "mntr")
	copy(p[16:20], colourSpace)
	copy(p[20:24], "XYZ ")
	copy(p[36:40], "acsp")
	binary.BigEndian.PutUint32(p[128:], 0)
	return p
}

// jpegSegment is one marker segment of a JPEG as parsed by jpegSegments (Marker, the whole
// segment bytes including 0xFF and marker, for SOS including the entropy data up to the next
// non-RST marker).
type jpegSegment struct {
	Marker byte
	Bytes  []byte
}

// jpegSegments is an independent simple test parser; it stops at the first EOI.
func jpegSegments(tb testing.TB, b []byte) []jpegSegment {
	tb.Helper()
	segs, _ := parseTestJPEG(tb, b)
	return segs
}

// jpegTrailing returns the bytes after the first EOI.
func jpegTrailing(tb testing.TB, b []byte) []byte {
	tb.Helper()
	_, trailing := parseTestJPEG(tb, b)
	return trailing
}

// parseTestJPEG splits b into segments up to the first EOI and returns the bytes after it. It
// fails the test on any structure it does not understand.
func parseTestJPEG(tb testing.TB, b []byte) ([]jpegSegment, []byte) {
	tb.Helper()
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
		tb.Fatalf("no SOI")
	}
	segs := []jpegSegment{{Marker: 0xD8, Bytes: b[:2]}}
	i := 2
	for {
		start := i
		if i >= len(b) || b[i] != 0xFF {
			tb.Fatalf("no marker at %d", i)
		}
		for i < len(b) && b[i] == 0xFF {
			i++
		}
		if i >= len(b) {
			tb.Fatalf("truncated marker")
		}
		m := b[i]
		i++
		if m == 0xD9 {
			return append(segs, jpegSegment{Marker: m, Bytes: b[start:i]}), b[i:]
		}
		i = testSegmentEnd(tb, b, m, i)
		segs = append(segs, jpegSegment{Marker: m, Bytes: b[start:i]})
	}
}

// testSegmentEnd returns the end of the segment of marker m whose length field starts at i.
func testSegmentEnd(tb testing.TB, b []byte, m byte, i int) int {
	tb.Helper()
	if (m >= 0xD0 && m <= 0xD7) || m == 0x01 {
		return i
	}
	if i+2 > len(b) {
		tb.Fatalf("truncated length")
	}
	end := i + int(binary.BigEndian.Uint16(b[i:]))
	if end > len(b) {
		tb.Fatalf("segment beyond the end")
	}
	if m != 0xDA {
		return end
	}
	for end+1 < len(b) {
		if b[end] == 0xFF && b[end+1] != 0x00 && b[end+1] != 0xFF && (b[end+1] < 0xD0 || b[end+1] > 0xD7) {
			return end
		}
		end++
	}
	tb.Fatalf("truncated scan")
	return end
}

// pngChunkT is one chunk of a PNG as parsed by pngChunks.
type pngChunkT struct {
	Type string
	Data []byte
}

// pngChunks is an independent simple PNG parser up to and including IEND; it checks every CRC.
func pngChunks(tb testing.TB, b []byte) []pngChunkT {
	tb.Helper()
	chunks, _ := parseTestPNG(tb, b)
	return chunks
}

// parseTestPNG splits b into chunks up to IEND and returns the bytes after it.
func parseTestPNG(tb testing.TB, b []byte) ([]pngChunkT, []byte) {
	tb.Helper()
	if !bytes.HasPrefix(b, []byte(pngSignature)) {
		tb.Fatalf("no PNG signature")
	}
	var chunks []pngChunkT
	i := len(pngSignature)
	for {
		if i+12 > len(b) {
			tb.Fatalf("truncated chunk at %d", i)
		}
		n := int(binary.BigEndian.Uint32(b[i:]))
		if n < 0 || i+12+n > len(b) {
			tb.Fatalf("chunk beyond the end")
		}
		c := pngChunkT{Type: string(b[i+4 : i+8]), Data: b[i+8 : i+8+n]}
		if !bytes.Equal(pngChunk(c.Type, c.Data), b[i:i+12+n]) {
			tb.Fatalf("CRC of %s", c.Type)
		}
		chunks = append(chunks, c)
		i += 12 + n
		if c.Type == "IEND" {
			return chunks, b[i:]
		}
	}
}

// twoColour returns a w x h RGBA image whose left half is a and right half b.
func twoColour(w, h int, a, b color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, image.Rect(0, 0, w/2, h), image.NewUniform(a), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(w/2, 0, w, h), image.NewUniform(b), image.Point{}, draw.Src)
	return img
}

// samePixels fails the test when a and b differ in bounds or any pixel.
func samePixels(tb testing.TB, a, b image.Image) {
	tb.Helper()
	if a.Bounds() != b.Bounds() {
		tb.Fatalf("bounds %v != %v", a.Bounds(), b.Bounds())
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				tb.Fatalf("pixel %d,%d: %v != %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
}
