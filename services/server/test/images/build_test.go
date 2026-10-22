// SPDX-License-Identifier: Apache-2.0

package images_test

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"slices"
	"testing"
)

// The synthetic metadata of the test images. Every value is a canary the tests look for in the
// stored objects, the event payloads and the logs; the GPS coordinates are 0/1 (they point
// nowhere) and the map datum carries the canary text.
const (
	canaryGPS     = "CANARY-GPS-DATUM-EXAMPLE"
	canaryXMP     = "canary-xmp-text-by-Alex"
	canaryComment = "canary comment written by Sam for example.com"
	canaryIPTC    = "canary-iptc-Robin"
	// canaryNamePrefix starts the original name of every upload of the package, so the log test
	// finds any name that leaked.
	canaryNamePrefix = "canary-name-"
)

// Format constants of the specifications the builders follow.
const (
	// exifIdentifier starts an APP1 EXIF payload (CIPA DC-008 4.7.2).
	exifIdentifier = "Exif\x00\x00"
	// iccIdentifier starts an APP2 ICC payload (ICC.1 B.4), followed by sequence and count.
	iccIdentifier = "ICC_PROFILE\x00"
	// xmpIdentifier starts an APP1 XMP payload (XMP part 3, 1.1.3).
	xmpIdentifier = "http://ns.adobe.com/xap/1.0/\x00"
	// pngSignature starts every PNG file (PNG 5.2).
	pngSignature = "\x89PNG\r\n\x1a\n"
	// jpegQuality is the quality the test JPEGs are encoded with.
	jpegQuality = 95
	// The JPEG markers the tests name (ITU-T T.81 table B.1).
	markerSOI   = 0xD8
	markerEOI   = 0xD9
	markerSOS   = 0xDA
	markerSOF0  = 0xC0
	markerAPP0  = 0xE0
	markerAPP1  = 0xE1
	markerAPP2  = 0xE2
	markerAPP14 = 0xEE
	markerCOM   = 0xFE
	// tagOrientation is the TIFF tag of the EXIF orientation (TIFF 6.0 section 8).
	tagOrientation = 0x0112
	// chunkIDAT and chunkICCP are PNG chunk types (PNG 11.2.4 and 11.3.3.3).
	chunkIDAT = "IDAT"
	chunkICCP = "iCCP"
	chunkIHDR = "IHDR"
)

// The colours of the two-colour test images: left half first, right half second.
var (
	colourFirst  = color.RGBA{R: 0xE0, G: 0x10, B: 0x10, A: 0xFF}
	colourSecond = color.RGBA{R: 0x10, G: 0x10, B: 0xE0, A: 0xFF}
)

// u16 and u32 convert the lengths and offsets of test structures, which are a few kilobytes at
// most.
func u16(n int) uint16 { return uint16(n) } //nolint:gosec // G115: test segments are far below 64 KiB.
func u32(n int) uint32 { return uint32(n) } //nolint:gosec // G115: test structures are far below 4 GiB.

// cat joins byte slices.
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// twoColour returns a w x h RGBA image whose left half is a and right half b.
func twoColour(w, h int, a, b color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, image.Rect(0, 0, w/2, h), image.NewUniform(a), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(w/2, 0, w, h), image.NewUniform(b), image.Point{}, draw.Src)
	return img
}

// encodeJPEG encodes img with image/jpeg at jpegQuality.
func encodeJPEG(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		tb.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// encodePNG encodes img with image/png.
func encodePNG(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		tb.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// segment returns one complete JPEG marker segment with its big-endian length.
func segment(marker byte, payload []byte) []byte {
	return cat([]byte{0xFF, marker}, binary.BigEndian.AppendUint16(nil, u16(len(payload)+2)), payload)
}

// syntheticICC returns a profile of size bytes with a valid 128-byte ICC header (size, class
// "mntr", colour space, PCS "XYZ ", signature "acsp") and zero tags; the rest is filler.
func syntheticICC(colourSpace string, size int) []byte {
	p := make([]byte, size)
	filler := byte(5)
	for i := range p {
		p[i] = filler
		filler += 11
	}
	binary.BigEndian.PutUint32(p[0:], u32(size))
	copy(p[4:8], "none")
	binary.BigEndian.PutUint32(p[8:], 0x04300000)
	copy(p[12:16], "mntr")
	copy(p[16:20], colourSpace)
	copy(p[20:24], "XYZ ")
	copy(p[36:40], "acsp")
	binary.BigEndian.PutUint32(p[128:], 0)
	return p
}

// iccSegments splits profile into two APP2 ICC_PROFILE segments (sequence 1 and 2 of 2).
func iccSegments(profile []byte) [][]byte {
	half := len(profile) / 2
	return [][]byte{
		segment(markerAPP2, cat([]byte(iccIdentifier), []byte{1, 2}, profile[:half])),
		segment(markerAPP2, cat([]byte(iccIdentifier), []byte{2, 2}, profile[half:])),
	}
}

// tiffEntry is one little-endian IFD entry whose value field holds the value or its offset.
func tiffEntry(tag, typ uint16, count uint32, value []byte) []byte {
	le := binary.LittleEndian
	field := append(append([]byte{}, value...), make([]byte, 4-len(value))...)
	return cat(le.AppendUint16(nil, tag), le.AppendUint16(nil, typ), le.AppendUint32(nil, count), field)
}

// tiffIFD returns an IFD of entries followed by the offset of the next IFD.
func tiffIFD(next uint32, entries ...[]byte) []byte {
	le := binary.LittleEndian
	return cat(le.AppendUint16(nil, u16(len(entries))), cat(entries...), le.AppendUint32(nil, next))
}

// ifdLen is the size of an IFD with n entries (TIFF 6.0 section 2).
func ifdLen(n int) int { return 2 + 12*n + 4 }

// exifTIFF returns a little-endian TIFF block: IFD0 with the orientation and a GPS pointer, a
// GPS IFD with latitude reference, latitude 0/1 0/1 0/1 and the canary map datum, and IFD1 with
// an embedded JPEG thumbnail.
func exifTIFF(orientation uint16, thumb []byte) []byte {
	le := binary.LittleEndian
	datum := []byte(canaryGPS + "\x00")
	lat := make([]byte, 0, 24)
	for range 3 {
		lat = le.AppendUint32(le.AppendUint32(lat, 0), 1)
	}
	gpsOff := 8 + ifdLen(2)
	ifd1Off := gpsOff + ifdLen(3)
	latOff := ifd1Off + ifdLen(2)
	datumOff := latOff + len(lat)
	thumbOff := datumOff + len(datum)
	return cat([]byte("II\x2a\x00"), le.AppendUint32(nil, 8),
		tiffIFD(u32(ifd1Off),
			tiffEntry(tagOrientation, 3, 1, le.AppendUint16(nil, orientation)),
			tiffEntry(0x8825, 4, 1, le.AppendUint32(nil, u32(gpsOff)))),
		tiffIFD(0,
			tiffEntry(0x0001, 2, 2, []byte("N\x00")),
			tiffEntry(0x0002, 5, 3, le.AppendUint32(nil, u32(latOff))),
			tiffEntry(0x0012, 2, u32(len(datum)), le.AppendUint32(nil, u32(datumOff)))),
		tiffIFD(0,
			tiffEntry(0x0201, 4, 1, le.AppendUint32(nil, u32(thumbOff))),
			tiffEntry(0x0202, 4, 1, le.AppendUint32(nil, u32(len(thumb))))),
		lat, datum, thumb)
}

// jfifWithThumbnail is a JFIF 1.02 APP0 payload with a 2x2 RGB thumbnail.
func jfifWithThumbnail() []byte {
	return cat([]byte("JFIF\x00\x01\x02\x01\x00\x48\x00\x48\x02\x02"), bytes.Repeat([]byte{0x10, 0x80, 0xF0}, 4))
}

// metadataJPEG is the JPEG of TestStripJPEGRemovesMetadata: a 64x32 two-colour image with APP0
// JFIF and its thumbnail, APP1 EXIF with orientation 6, a GPS IFD and an EXIF thumbnail, APP1
// XMP, APP13 IPTC, COM, APP2 MPF, a two-part APP2 ICC profile, APP14 Adobe and a second JPEG
// after EOI. It returns the file and the profile.
func metadataJPEG(tb testing.TB) ([]byte, []byte) {
	tb.Helper()
	profile := syntheticICC("RGB ", 600)
	plain := encodeJPEG(tb, twoColour(64, 32, colourFirst, colourSecond))
	thumb := encodeJPEG(tb, twoColour(8, 8, color.Black, color.White))
	segs := [][]byte{
		segment(markerAPP0, jfifWithThumbnail()),
		segment(markerAPP1, cat([]byte(exifIdentifier), exifTIFF(6, thumb))),
		segment(markerAPP1, cat([]byte(xmpIdentifier), []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/">`+canaryXMP+`</x:xmpmeta>`))),
		segment(0xED, cat([]byte("Photoshop 3.0\x008BIM\x04\x04\x00\x00\x00\x00\x00\x18\x1c\x02\x50\x00\x11"), []byte(canaryIPTC))),
		segment(markerCOM, []byte(canaryComment)),
		segment(markerAPP2, []byte("MPF\x00MM\x00\x2a\x00\x00\x00\x08\x00\x00")),
	}
	segs = append(segs, iccSegments(profile)...)
	// Version 100, flags 0, transform 1 (YCbCr), as image/jpeg writes the components.
	segs = append(segs, segment(markerAPP14, []byte("Adobe\x00\x64\x00\x00\x00\x00\x01")))
	out := cat(plain[:2], cat(segs...), plain[2:], encodeJPEG(tb, twoColour(8, 8, color.White, color.Black)))
	return out, profile
}

// plainJPEG is a 32x16 two-colour JPEG without metadata.
func plainJPEG(tb testing.TB) []byte {
	tb.Helper()
	return encodeJPEG(tb, twoColour(32, 16, colourFirst, colourSecond))
}

// truncatedJPEG cuts a valid JPEG in the middle of its scan data.
func truncatedJPEG(tb testing.TB) []byte {
	tb.Helper()
	b := encodeJPEG(tb, twoColour(64, 64, colourFirst, colourSecond))
	sos := bytes.Index(b, []byte{0xFF, markerSOS})
	if sos < 0 {
		tb.Fatal("no SOS in the test JPEG")
	}
	return b[:sos+(len(b)-sos)/2]
}

// hugeJPEG is a tiny JPEG whose frame header declares 65535 x 65535 pixels.
func hugeJPEG(tb testing.TB) []byte {
	tb.Helper()
	b := encodeJPEG(tb, twoColour(16, 16, colourFirst, colourSecond))
	sof := bytes.Index(b, []byte{0xFF, markerSOF0})
	if sof < 0 {
		tb.Fatal("no SOF0 in the test JPEG")
	}
	// SOF0: marker (2), length (2), precision (1), height (2), width (2).
	binary.BigEndian.PutUint16(b[sof+5:], 0xFFFF)
	binary.BigEndian.PutUint16(b[sof+7:], 0xFFFF)
	return b
}

// pngChunk returns one PNG chunk with its CRC.
func pngChunk(typ string, data []byte) []byte {
	c := cat(binary.BigEndian.AppendUint32(nil, u32(len(data))), []byte(typ), data)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

// zlibBytes compresses b with zlib.
func zlibBytes(tb testing.TB, b []byte) []byte {
	tb.Helper()
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(b); err != nil {
		tb.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

// pngPart is one chunk of a parsed PNG.
type pngPart struct {
	Type string
	Data []byte
}

// parsePNG splits b into chunks up to IEND and returns the bytes after it; it fails the test on
// a broken structure or CRC.
func parsePNG(tb testing.TB, b []byte) ([]pngPart, []byte) {
	tb.Helper()
	if !bytes.HasPrefix(b, []byte(pngSignature)) {
		tb.Fatal("no PNG signature")
	}
	var out []pngPart
	for i := len(pngSignature); ; {
		if i+12 > len(b) {
			tb.Fatalf("truncated PNG chunk at %d", i)
		}
		n := int(binary.BigEndian.Uint32(b[i:]))
		if n < 0 || i+12+n > len(b) {
			tb.Fatal("PNG chunk beyond the end")
		}
		c := pngPart{Type: string(b[i+4 : i+8]), Data: b[i+8 : i+8+n]}
		if !bytes.Equal(pngChunk(c.Type, c.Data), b[i:i+12+n]) {
			tb.Fatalf("CRC of %s", c.Type)
		}
		out = append(out, c)
		i += 12 + n
		if c.Type == "IEND" {
			return out, b[i:]
		}
	}
}

// joinPNG writes the signature and chunks.
func joinPNG(chunks []pngPart) []byte {
	out := []byte(pngSignature)
	for _, c := range chunks {
		out = append(out, pngChunk(c.Type, c.Data)...)
	}
	return out
}

// metadataPNG is an opaque 48x24 two-colour PNG with an iCCP profile after IHDR, tEXt, zTXt,
// iTXt with XMP, eXIf with GPS, tIME, sPLT and a private chunk before IDAT, and trailing bytes
// after IEND. It returns the file and the profile.
func metadataPNG(tb testing.TB) ([]byte, []byte) {
	tb.Helper()
	profile := syntheticICC("RGB ", 400)
	plain, _ := parsePNG(tb, encodePNG(tb, twoColour(48, 24, colourFirst, colourSecond)))
	var chunks []pngPart
	for _, c := range plain {
		if c.Type == chunkIDAT && !slices.ContainsFunc(chunks, func(p pngPart) bool { return p.Type == "tEXt" }) {
			chunks = append(chunks,
				pngPart{"tEXt", []byte("Author\x00Alex")},
				pngPart{"zTXt", cat([]byte("Comment\x00\x00"), zlibBytes(tb, []byte(canaryComment)))},
				pngPart{"iTXt", cat([]byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"), []byte(canaryXMP))},
				pngPart{"eXIf", exifTIFF(6, nil)},
				pngPart{"tIME", []byte{0x07, 0xEA, 10, 20, 7, 0, 0}},
				pngPart{"sPLT", []byte("Sam\x00\x08\x01\x02\x03\x04\x00\x01")},
				pngPart{"prVt", []byte("private data of Robin")})
		}
		chunks = append(chunks, c)
		if c.Type == chunkIHDR {
			chunks = append(chunks, pngPart{chunkICCP, cat([]byte("ICC Profile\x00\x00"), zlibBytes(tb, profile))})
		}
	}
	return cat(joinPNG(chunks), []byte("trailing bytes from example.com")), profile
}

// hugePNG is a tiny PNG whose IHDR declares 100000 x 100000 pixels (CRC recomputed).
func hugePNG(tb testing.TB) []byte {
	tb.Helper()
	chunks, _ := parsePNG(tb, encodePNG(tb, image.NewGray(image.Rect(0, 0, 4, 4))))
	ihdr := append([]byte{}, chunks[0].Data...)
	binary.BigEndian.PutUint32(ihdr[0:], 100000)
	binary.BigEndian.PutUint32(ihdr[4:], 100000)
	chunks[0].Data = ihdr
	return joinPNG(chunks)
}

// bombPNG is a real decompression bomb: a valid 4000 x 4000 grey PNG of zeros whose file is a
// few kilobytes and whose pixels would take 16 MB.
func bombPNG(tb testing.TB) []byte {
	tb.Helper()
	return encodePNG(tb, image.NewGray(image.Rect(0, 0, 4000, 4000)))
}

// gifPalette is the palette of the test GIFs.
var gifPalette = color.Palette{color.Black, color.White, colourFirst, colourSecond}

// gifFrame returns a w x h paletted frame filled with index i.
func gifFrame(w, h int, i uint8) *image.Paletted {
	f := image.NewPaletted(image.Rect(0, 0, w, h), gifPalette)
	for p := range f.Pix {
		f.Pix[p] = i + uint8(p%2)
	}
	return f
}

// metadataGIF is an animated two-frame 16x16 GIF with loop count 0, a comment extension and an
// XMP application extension spliced in before the first frame.
func metadataGIF(tb testing.TB) []byte {
	tb.Helper()
	g := &gif.GIF{
		Image:    []*image.Paletted{gifFrame(16, 16, 0), gifFrame(16, 16, 2)},
		Delay:    []int{10, 20},
		Disposal: []byte{gif.DisposalNone, gif.DisposalBackground},
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		tb.Fatalf("encode gif: %v", err)
	}
	b := buf.Bytes()
	at := gifBlocksStart(b)
	comment := cat([]byte{0x21, 0xFE, byte(len(canaryComment))}, []byte(canaryComment), []byte{0})
	xmp := cat([]byte{0x21, 0xFF, 0x0B}, []byte("XMP DataXMP"), []byte{byte(len(canaryXMP))}, []byte(canaryXMP), []byte{0})
	return cat(b[:at], comment, xmp, b[at:])
}

// hugeGIF is a tiny GIF whose logical screen declares 65535 x 65535 pixels.
func hugeGIF(tb testing.TB) []byte {
	tb.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, gifFrame(4, 4, 0), nil); err != nil {
		tb.Fatalf("encode gif: %v", err)
	}
	b := buf.Bytes()
	binary.LittleEndian.PutUint16(b[6:], 0xFFFF)
	binary.LittleEndian.PutUint16(b[8:], 0xFFFF)
	return b
}

// gifBlocksStart returns the offset after the header, the logical screen descriptor and the
// global colour table (GIF89a 18 and 19).
func gifBlocksStart(b []byte) int {
	at := 13
	if b[10]&0x80 != 0 {
		at += 3 << (int(b[10]&0x07) + 1)
	}
	return at
}

// gifExtension is one extension block of a GIF: its label and, for application extensions, the
// identifier.
type gifExtension struct {
	Label byte
	App   string
}

// gifExtensions lists the extension blocks of b by a block scan without LZW decoding.
func gifExtensions(tb testing.TB, b []byte) []gifExtension {
	tb.Helper()
	var out []gifExtension
	i := gifBlocksStart(b)
	for i < len(b) {
		switch b[i] {
		case 0x3B:
			return out
		case 0x21:
			e := gifExtension{Label: b[i+1]}
			if e.Label == 0xFF && i+14 <= len(b) {
				e.App = string(b[i+3 : i+14])
			}
			out = append(out, e)
			i = skipSubBlocks(tb, b, i+2)
		case 0x2C:
			i = skipImage(tb, b, i)
		default:
			tb.Fatalf("unknown GIF block 0x%02x at %d", b[i], i)
		}
	}
	tb.Fatal("GIF without trailer")
	return nil
}

// skipSubBlocks returns the offset after the data sub-blocks starting at i.
func skipSubBlocks(tb testing.TB, b []byte, i int) int {
	tb.Helper()
	for i < len(b) && b[i] != 0 {
		i += int(b[i]) + 1
	}
	if i >= len(b) {
		tb.Fatal("truncated GIF sub-blocks")
	}
	return i + 1
}

// skipImage returns the offset after the image descriptor at i, its local colour table and its
// image data.
func skipImage(tb testing.TB, b []byte, i int) int {
	tb.Helper()
	packed := b[i+9]
	i += 10
	if packed&0x80 != 0 {
		i += 3 << (int(packed&0x07) + 1)
	}
	return skipSubBlocks(tb, b, i+1)
}

// webpBits packs values LSB first, the bit order of the VP8L bitstream.
type webpBits struct {
	buf []byte
	n   uint
}

// write appends the low bits of v.
func (w *webpBits) write(v uint32, bits uint) {
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

// simpleCode writes a VP8L "simple" prefix code of the sorted symbols (one or two 8-bit values)
// and returns them.
func simpleCode(w *webpBits, values []uint8) []uint8 {
	var syms []uint8
	for _, v := range values {
		if !slices.Contains(syms, v) {
			syms = append(syms, v)
		}
	}
	slices.Sort(syms)
	w.write(1, 1)
	w.write(u32(len(syms)-1), 1)
	w.write(1, 1)
	for _, s := range syms {
		w.write(uint32(s), 8)
	}
	return syms
}

// vp8l encodes img (every channel at most two distinct values) as a VP8L bitstream without
// transforms, colour cache or backward references (WebP lossless bitstream specification).
func vp8l(tb testing.TB, img image.Image) []byte {
	tb.Helper()
	b := img.Bounds()
	var ch [4][]uint8 // green, red, blue, alpha in coding order
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			ch[0], ch[1], ch[2], ch[3] = append(ch[0], c.G), append(ch[1], c.R), append(ch[2], c.B), append(ch[3], c.A)
		}
	}
	w := &webpBits{buf: []byte{0x2F}}
	w.write(u32(b.Dx()-1), 14)
	w.write(u32(b.Dy()-1), 14)
	w.write(0, 1) // alpha_is_used: the test images are opaque
	w.write(0, 3) // version
	w.write(0, 1) // transform_present
	w.write(0, 1) // color_cache
	w.write(0, 1) // meta_prefix
	var codes [4][]uint8
	for i := range ch {
		codes[i] = simpleCode(w, ch[i])
		if len(codes[i]) > 2 {
			tb.Fatal("vp8l: a channel has more than two values")
		}
	}
	simpleCode(w, []uint8{0}) // distance
	for p := range ch[0] {
		for k := range codes {
			if len(codes[k]) == 2 {
				w.write(u32(slices.Index(codes[k], ch[k][p])), 1)
			}
		}
	}
	return w.buf
}

// riffChunk returns a RIFF chunk with its padding byte.
func riffChunk(fourcc string, payload []byte) []byte {
	out := cat([]byte(fourcc), binary.LittleEndian.AppendUint32(nil, u32(len(payload))), payload)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// riffWebP wraps chunks into the RIFF WEBP container.
func riffWebP(chunks ...[]byte) []byte {
	body := cat([]byte("WEBP"), cat(chunks...))
	return cat([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, u32(len(body))), body)
}

// uint24 appends v as 24-bit little endian.
func uint24(b []byte, v int) []byte {
	return append(b, binary.LittleEndian.AppendUint32(nil, u32(v))[:3]...)
}

// stillWebP is an opaque 16x8 two-colour lossless WebP in a VP8X container with an EXIF chunk
// (orientation 6, GPS) and an XMP chunk.
func stillWebP(tb testing.TB) []byte {
	tb.Helper()
	img := twoColour(16, 8, colourFirst, colourSecond)
	vp8x := uint24(uint24([]byte{0x08 | 0x04, 0, 0, 0}, 15), 7)
	return riffWebP(riffChunk("VP8X", vp8x), riffChunk("VP8L", vp8l(tb, img)), riffChunk("EXIF", exifTIFF(6, nil)),
		riffChunk("XMP ", []byte(`<x:xmpmeta xmlns:x="adobe:ns:meta/">`+canaryXMP+`</x:xmpmeta>`)))
}

// animatedWebP is a 16x8 WebP with the VP8X animation flag, an ANIM chunk and one ANMF frame.
func animatedWebP(tb testing.TB) []byte {
	tb.Helper()
	img := twoColour(16, 8, colourFirst, colourSecond)
	vp8x := uint24(uint24([]byte{0x02, 0, 0, 0}, 15), 7)
	frame := uint24(uint24(uint24(uint24(nil, 0), 0), 15), 7)
	frame = append(uint24(frame, 100), 0)
	return riffWebP(riffChunk("VP8X", vp8x), riffChunk("ANIM", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0, 0}),
		riffChunk("ANMF", cat(frame, riffChunk("VP8L", vp8l(tb, img)))))
}

// box builds an ISO base media box with a 32-bit size header (ISO/IEC 14496-12 4.2).
func box(typ string, payload ...[]byte) []byte {
	body := cat(payload...)
	return cat(binary.BigEndian.AppendUint32(nil, u32(8+len(body))), []byte(typ), body)
}

// fullBox builds a full box with version and zero flags.
func fullBox(typ string, version byte, payload ...[]byte) []byte {
	return box(typ, append([]byte{version, 0, 0, 0}, cat(payload...)...))
}

// be32 returns v as four big-endian bytes.
func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// The 16.16 fixed-point values of the tkhd matrix (ISO/IEC 14496-12 6.2.2).
const (
	fixedOne      = 0x00010000
	fixedMinusOne = 0xFFFF0000
	fixedW        = 0x40000000
)

// rotatedMP4 is an MP4 whose video track of 1920x1080 is turned by 90 degrees and whose movie
// lasts 4500 ms: ftyp, moov with mvhd version 0 (timescale 1000, duration 4500) and a trak with
// tkhd, mdia, mdhd and hdlr "vide", then mdat.
func rotatedMP4() []byte {
	var m []byte
	for _, v := range []uint32{0, fixedOne, 0, fixedMinusOne, 0, 0, 0, 0, fixedW} {
		m = append(m, be32(v)...)
	}
	mvhd := fullBox("mvhd", 0, be32(0), be32(0), be32(1000), be32(4500), make([]byte, 80))
	tkhd := fullBox("tkhd", 0, be32(0), be32(0), be32(1), be32(0), be32(4500), make([]byte, 16), m, be32(1920*fixedOne), be32(1080*fixedOne))
	hdlr := fullBox("hdlr", 0, be32(0), []byte("vide"), make([]byte, 12), []byte("Alex\x00"))
	trak := box("trak", tkhd, box("mdia", box("mdhd", make([]byte, 24)), hdlr, box("minf")))
	ftyp := box("ftyp", []byte("isom"), be32(0x200), []byte("isomiso2mp41"))
	return cat(ftyp, box("moov", mvhd, trak), box("mdat", bytes.Repeat([]byte{0xAB}, 256)))
}

// brokenMP4 is an MP4 whose moov holds a child larger than moov itself.
func brokenMP4() []byte {
	ftyp := box("ftyp", []byte("isom"), be32(0x200), []byte("isomiso2mp41"))
	child := cat(be32(4096), []byte("trak"), make([]byte, 16))
	return cat(ftyp, box("moov", child), box("mdat", bytes.Repeat([]byte{0xCD}, 256)))
}

// tiffImage is a little-endian TIFF header with an empty IFD.
func tiffImage() []byte {
	return cat([]byte("II*\x00"), []byte{8, 0, 0, 0}, make([]byte, 64))
}

// heicImage is an ftyp box with the major brand heic followed by filler.
func heicImage() []byte {
	return cat(box("ftyp", []byte("heic"), be32(0), []byte("mif1heic")), box("meta", make([]byte, 32)))
}

// svgWithScript is an SVG document with a script element.
func svgWithScript() []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><script>alert("example.com")</script><rect width="10" height="10"/></svg>
`)
}

// jpegPart is one marker segment of a parsed JPEG: the marker and the whole segment bytes (for
// SOS including the entropy-coded data).
type jpegPart struct {
	Marker byte
	Bytes  []byte
}

// payload returns the bytes after the marker and the length field.
func (p jpegPart) payload() []byte {
	if len(p.Bytes) < 4 {
		return nil
	}
	return p.Bytes[4:]
}

// parseJPEG splits b into segments up to the first EOI and returns the bytes after it; it fails
// the test on a structure it does not understand.
func parseJPEG(tb testing.TB, b []byte) ([]jpegPart, []byte) {
	tb.Helper()
	if len(b) < 2 || b[0] != 0xFF || b[1] != markerSOI {
		tb.Fatal("no SOI")
	}
	parts := []jpegPart{{Marker: markerSOI, Bytes: b[:2]}}
	i := 2
	for {
		start := i
		if i+1 >= len(b) || b[i] != 0xFF {
			tb.Fatalf("no marker at %d", i)
		}
		m := b[i+1]
		i += 2
		if m == markerEOI {
			return append(parts, jpegPart{Marker: m, Bytes: b[start:i]}), b[i:]
		}
		if i+2 > len(b) {
			tb.Fatal("truncated JPEG length")
		}
		end := i + int(binary.BigEndian.Uint16(b[i:]))
		if end > len(b) {
			tb.Fatal("JPEG segment beyond the end")
		}
		if m == markerSOS {
			end = scanEnd(tb, b, end)
		}
		parts = append(parts, jpegPart{Marker: m, Bytes: b[start:end]})
		i = end
	}
}

// scanEnd returns the offset of the first marker after the entropy-coded data at i that is no
// stuffed byte and no restart marker.
func scanEnd(tb testing.TB, b []byte, i int) int {
	tb.Helper()
	for ; i+1 < len(b); i++ {
		if b[i] == 0xFF && b[i+1] != 0x00 && (b[i+1] < 0xD0 || b[i+1] > 0xD7) {
			return i
		}
	}
	tb.Fatal("truncated JPEG scan")
	return i
}

// jpegICC reassembles the APP2 ICC_PROFILE parts of parts in order; nil when there are none.
func jpegICC(parts []jpegPart) []byte {
	var out []byte
	for _, p := range parts {
		if p.Marker == markerAPP2 && bytes.HasPrefix(p.payload(), []byte(iccIdentifier)) {
			out = append(out, p.payload()[len(iccIdentifier)+2:]...)
		}
	}
	return out
}
