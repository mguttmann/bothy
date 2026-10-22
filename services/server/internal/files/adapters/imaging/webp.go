// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/image/webp"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// RIFF container and WebP chunk layout (RFC 9649, WebP Image Format).
const (
	// webpRIFFHeaderSize is "RIFF", the 32-bit payload size and "WEBP".
	webpRIFFHeaderSize = 12
	// webpRIFFSizeOffset is where the payload size starts; it counts from "WEBP" on.
	webpRIFFSizeOffset = 8
	// webpChunkHeaderSize is the FourCC and the 32-bit payload size of a chunk.
	webpChunkHeaderSize = 8
	// webpVP8XSize is the payload size of the VP8X chunk: flags, 3 reserved bytes, canvas
	// width-1 and height-1 as 24-bit little endian.
	webpVP8XSize = 10
	// webpVP8XAnimation is the animation flag in the first VP8X byte.
	webpVP8XAnimation = 0x02
	// webpVP8HeaderSize is the 3-byte frame tag, the 3-byte start code and the 16-bit width and
	// height of a VP8 key frame (RFC 6386 section 9.1).
	webpVP8HeaderSize = 10
	// webpVP8KeyFrameBit is clear in the first frame tag byte for a key frame.
	webpVP8KeyFrameBit = 0x01
	// webpVP8SizeMask keeps the 14 size bits of a VP8 dimension; the top 2 bits are scaling.
	webpVP8SizeMask = 0x3FFF
	// webpVP8LHeaderSize is the signature byte and the 32 bits of width-1, height-1, alpha and
	// version of a VP8L bitstream.
	webpVP8LHeaderSize = 5
	// webpVP8LSignature is the first byte of a VP8L bitstream.
	webpVP8LSignature = 0x2F
	// webpVP8LSizeBits is the width of the width-1 and height-1 fields of VP8L.
	webpVP8LSizeBits = 14
	// webpVP8LVersionShift is the position of the 3-bit version field, which must be 0.
	webpVP8LVersionShift = 29
)

// The chunk FourCCs scanWebP looks at.
const (
	webpFourCCVP8X = "VP8X"
	webpFourCCVP8  = "VP8 "
	webpFourCCVP8L = "VP8L"
	webpFourCCANIM = "ANIM"
	webpFourCCANMF = "ANMF"
	webpFourCCEXIF = "EXIF"
	webpFourCCICCP = "ICCP"
)

// webpExifPrefix is the JPEG APP1 identifier some writers leave in front of the TIFF block.
var webpExifPrefix = []byte("Exif\x00\x00")

// webpInfo is what scanWebP reads from the RIFF chunks of a WebP.
type webpInfo struct {
	Width, Height int    // canvas size from VP8X, or the size of the VP8 / VP8L bitstream header
	Animated      bool   // VP8X animation flag set or an ANIM / ANMF chunk present
	EXIF          []byte // payload of the EXIF chunk (TIFF block, "Exif\0\0" stripped), nil when none
	ICC           []byte // payload of the ICCP chunk, nil when none
}

// webpScan is the state of one scan: the result so far and whether a VP8X or an image chunk
// was seen.
type webpScan struct {
	info     webpInfo
	hasVP8X  bool
	hasImage bool
}

// scanWebP scans the RIFF chunks of data ("RIFF", size, "WEBP", then chunks padded to even
// length) without decoding pixels. The size comes from the VP8X canvas when there is one,
// otherwise from the first VP8 or VP8L header. EXIF and ICC are copies, so the caller may keep
// them. Errors wrap app.ErrImageMalformed: bad header, a chunk beyond the input, a broken
// bitstream header or no image chunk. It never panics on any input.
func scanWebP(data []byte) (webpInfo, error) {
	body, err := webpRIFFBody(data)
	if err != nil {
		return webpInfo{}, err
	}
	var s webpScan
	for len(body) > 0 {
		fourcc, payload, rest, err := webpNextChunk(body)
		if err != nil {
			return webpInfo{}, err
		}
		if err := s.chunk(fourcc, payload); err != nil {
			return webpInfo{}, err
		}
		body = rest
	}
	if !s.hasImage {
		return webpInfo{}, fmt.Errorf("%w: webp without image chunk", app.ErrImageMalformed)
	}
	return s.info, nil
}

// webpRIFFBody checks the RIFF header and returns the chunks it covers; bytes after the RIFF
// payload are ignored, as decoders do.
func webpRIFFBody(data []byte) ([]byte, error) {
	if len(data) < webpRIFFHeaderSize || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, fmt.Errorf("%w: webp riff header", app.ErrImageMalformed)
	}
	size := uint64(binary.LittleEndian.Uint32(data[4:webpRIFFSizeOffset]))
	avail := uint64(len(data) - webpRIFFSizeOffset) //nolint:gosec // G115: len(data) is at least the header size, checked above
	if size < webpRIFFHeaderSize-webpRIFFSizeOffset || size > avail {
		return nil, fmt.Errorf("%w: webp riff size", app.ErrImageMalformed)
	}
	return data[webpRIFFHeaderSize : webpRIFFSizeOffset+size], nil
}

// webpNextChunk splits the first chunk off body: its FourCC, its payload and the bytes after
// it and its padding byte. A missing padding byte at the very end is tolerated.
func webpNextChunk(body []byte) (string, []byte, []byte, error) {
	if len(body) < webpChunkHeaderSize {
		return "", nil, nil, fmt.Errorf("%w: webp chunk header cut", app.ErrImageMalformed)
	}
	size := uint64(binary.LittleEndian.Uint32(body[4:webpChunkHeaderSize]))
	end := webpChunkHeaderSize + size
	if end > uint64(len(body)) {
		return "", nil, nil, fmt.Errorf("%w: webp chunk beyond the input", app.ErrImageMalformed)
	}
	next := min(end+size%2, uint64(len(body)))
	return string(body[:4]), body[webpChunkHeaderSize:end], body[next:], nil
}

// chunk records what one chunk says about the image.
func (s *webpScan) chunk(fourcc string, payload []byte) error {
	switch fourcc {
	case webpFourCCVP8X:
		return s.vp8x(payload)
	case webpFourCCANIM:
		s.info.Animated = true
	case webpFourCCANMF:
		s.info.Animated = true
		s.hasImage = true
	case webpFourCCEXIF:
		if s.info.EXIF == nil {
			s.info.EXIF = bytes.Clone(bytes.TrimPrefix(payload, webpExifPrefix))
		}
	case webpFourCCICCP:
		if s.info.ICC == nil {
			s.info.ICC = bytes.Clone(payload)
		}
	case webpFourCCVP8:
		w, h, err := webpVP8Size(payload)
		return s.image(w, h, err)
	case webpFourCCVP8L:
		w, h, err := webpVP8LSize(payload)
		return s.image(w, h, err)
	}
	return nil
}

// vp8x reads the animation flag and the canvas size of the VP8X chunk.
func (s *webpScan) vp8x(payload []byte) error {
	if len(payload) < webpVP8XSize {
		return fmt.Errorf("%w: webp vp8x size", app.ErrImageMalformed)
	}
	// VP8X must come first: x/image/webp decodes the first bitstream at its own size and never
	// reads a later VP8X, so a later canvas could hide the size from the pixel guard.
	if s.hasVP8X || s.hasImage {
		return fmt.Errorf("%w: webp vp8x after an image chunk or twice", app.ErrImageMalformed)
	}
	s.hasVP8X = true
	if payload[0]&webpVP8XAnimation != 0 {
		s.info.Animated = true
	}
	s.info.Width = webpUint24(payload[4:7]) + 1
	s.info.Height = webpUint24(payload[7:10]) + 1
	return nil
}

// image records a bitstream header: its size counts only without VP8X and for the first image
// chunk; a broken header fails the scan.
func (s *webpScan) image(w, h int, err error) error {
	if err != nil {
		return err
	}
	if !s.hasVP8X && !s.hasImage {
		s.info.Width, s.info.Height = w, h
	}
	s.hasImage = true
	return nil
}

// webpUint24 reads a 24-bit little-endian value.
func webpUint24(b []byte) int {
	return int(b[0]) | int(b[1])<<8 | int(b[2])<<16
}

// webpVP8Size reads width and height from the header of a VP8 key frame. Errors wrap
// app.ErrImageMalformed: short payload, not a key frame, wrong start code.
func webpVP8Size(p []byte) (int, int, error) {
	if len(p) < webpVP8HeaderSize || p[0]&webpVP8KeyFrameBit != 0 || p[3] != 0x9D || p[4] != 0x01 || p[5] != 0x2A {
		return 0, 0, fmt.Errorf("%w: webp vp8 header", app.ErrImageMalformed)
	}
	w := int(binary.LittleEndian.Uint16(p[6:8]) & webpVP8SizeMask)
	h := int(binary.LittleEndian.Uint16(p[8:10]) & webpVP8SizeMask)
	return w, h, nil
}

// webpVP8LSize reads width and height from the header of a VP8L bitstream. Errors wrap
// app.ErrImageMalformed: short payload, wrong signature, version other than 0.
func webpVP8LSize(p []byte) (int, int, error) {
	if len(p) < webpVP8LHeaderSize || p[0] != webpVP8LSignature {
		return 0, 0, fmt.Errorf("%w: webp vp8l header", app.ErrImageMalformed)
	}
	// Read the 32 header bits as int from single bytes, so no conversion can overflow.
	bits := int(p[1]) | int(p[2])<<8 | int(p[3])<<16 | int(p[4])<<24
	if bits>>webpVP8LVersionShift != 0 {
		return 0, 0, fmt.Errorf("%w: webp vp8l version", app.ErrImageMalformed)
	}
	const mask = 1<<webpVP8LSizeBits - 1
	w := bits&mask + 1
	h := bits>>webpVP8LSizeBits&mask + 1
	return w, h, nil
}

// webpConfig returns the size of a still WebP through golang.org/x/image/webp DecodeConfig,
// which reads the headers only. An error wraps app.ErrImageMalformed because the header cannot
// be read.
func webpConfig(data []byte) (width, height int, err error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", app.ErrImageMalformed, err)
	}
	return cfg.Width, cfg.Height, nil
}
