// SPDX-License-Identifier: Apache-2.0

package filestest

import (
	"bytes"
	"encoding/binary"
)

// The generated audio fixtures of the voice tests (brief M3-09 "Boundaries"): minimal box trees
// of ISO/IEC 14496-12 and a few MPEG audio frame headers, never recorded speech. The box layout
// constants below are format constants of the specification, not tuning numbers.

// AudioOptions shapes an MP4 fixture of M4A.
type AudioOptions struct {
	// Brand is the major brand of ftyp; "" means "M4A ".
	Brand string
	// NoMovieHeader leaves mvhd out, so the box reader finds no duration.
	NoMovieHeader bool
	// VideoTrack adds a second track with handler vide and a display size of 640 by 360.
	VideoTrack bool
	// Broken cuts the moov box in the middle of mvhd, which the box reader refuses.
	Broken bool
}

// M4A returns an MP4 audio file: ftyp with brand "M4A " (or opts.Brand), moov with mvhd of the
// given timescale and duration, one trak with hdlr soun and an mp4a sample entry, and a small
// mdat. The duration in milliseconds is duration * 1000 / timescale.
//
// Example:
//
//	body := filestest.M4A(1000, 4500, filestest.AudioOptions{}) // 4.5 seconds
func M4A(timescale, duration uint32, opts AudioOptions) []byte {
	brand := opts.Brand
	if brand == "" {
		brand = "M4A "
	}
	ft := box("ftyp", []byte(brand), be32(0), []byte(brand), []byte("isom"))
	var moov []byte
	if !opts.NoMovieHeader {
		moov = append(moov, fullBox("mvhd", be32(0), be32(0), be32(timescale), be32(duration), make([]byte, 80))...)
	}
	moov = append(moov, trak(1, 0, 0, "soun", mp4aEntry())...)
	if opts.VideoTrack {
		moov = append(moov, trak(2, 640, 360, "vide", nil)...)
	}
	mv := box("moov", moov)
	if opts.Broken {
		// A moov whose declared size runs past the end of the file.
		mv = mv[:len(mv)/2]
		return cat(ft, mv)
	}
	return cat(ft, mv, box("mdat", make([]byte, 64)))
}

// MP3 returns n MPEG-1 Layer III frames (128 kbit/s, 44.1 kHz) behind an ID3v2 header, which the
// type detection names audio/mpeg.
//
// Example:
//
//	body := filestest.MP3(8)
func MP3(n int) []byte {
	// An ID3v2.4 header with an empty tag, then frames with header FF FB 90 00 (417 bytes each).
	out := []byte{'I', 'D', '3', 4, 0, 0, 0, 0, 0, 0}
	for range n {
		frame := make([]byte, 417)
		copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})
		out = append(out, frame...)
	}
	return out
}

// WAV returns a RIFF WAVE file with a short silent PCM block.
//
// Example:
//
//	body := filestest.WAV()
func WAV() []byte {
	data := make([]byte, 64)
	fmtChunk := cat([]byte("fmt "), le32(16), le16(1), le16(1), le32(8000), le32(16000), le16(2), le16(16))
	dataChunk := cat([]byte("data"), le32(u32(len(data))), data)
	body := cat([]byte("WAVE"), fmtChunk, dataChunk)
	return cat([]byte("RIFF"), le32(u32(len(body))), body)
}

// trak builds a track with tkhd (id, size) and mdia/hdlr of the handler type; entry is the
// sample entry put into minf/stbl/stsd, nil for none.
func trak(id, width, height uint32, handler string, entry []byte) []byte {
	ident := []uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000}
	m := make([]byte, 0, 36)
	for _, v := range ident {
		m = binary.BigEndian.AppendUint32(m, v)
	}
	tkhd := fullBox("tkhd", be32(0), be32(0), be32(id), be32(0), be32(0), make([]byte, 8),
		make([]byte, 8), m, be32(width<<16), be32(height<<16))
	hdlr := fullBox("hdlr", be32(0), []byte(handler), make([]byte, 12), []byte{0})
	var minf []byte
	if entry != nil {
		minf = box("minf", box("stbl", fullBox("stsd", be32(1), entry)))
	}
	return box("trak", tkhd, box("mdia", hdlr, minf))
}

// mp4aEntry is an mp4a audio sample entry (ISO/IEC 14496-12 12.2.3) without esds.
func mp4aEntry() []byte {
	return box("mp4a", make([]byte, 6), be16(1), make([]byte, 8), be16(1), be16(16), be16(0), be16(0), be32(44100<<16))
}

// u32 converts the length of a fixture, which is far below 4 GiB.
func u32(n int) uint32 {
	return uint32(n) //nolint:gosec // G115: fixtures are a few hundred bytes.
}

func be16(v uint16) []byte       { return binary.BigEndian.AppendUint16(nil, v) }
func be32(v uint32) []byte       { return binary.BigEndian.AppendUint32(nil, v) }
func le16(v uint16) []byte       { return binary.LittleEndian.AppendUint16(nil, v) }
func le32(v uint32) []byte       { return binary.LittleEndian.AppendUint32(nil, v) }
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// box builds a box with a 32-bit size header around the joined payload.
func box(typ string, payload ...[]byte) []byte {
	body := cat(payload...)
	return cat(be32(u32(8+len(body))), []byte(typ), body)
}

// fullBox builds a version 0 full box: version, 24-bit flags (zero) and the payload.
func fullBox(typ string, payload ...[]byte) []byte {
	return box(typ, append([]byte{0, 0, 0, 0}, cat(payload...)...))
}
