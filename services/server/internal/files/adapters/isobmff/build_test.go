// SPDX-License-Identifier: Apache-2.0

package isobmff_test

import (
	"bytes"
	"encoding/binary"
)

// fixedOne is 1.0 in 16.16 fixed point, the unit of the tkhd matrix and size fields.
const fixedOne = 0x00010000

// fixedMinusOne is -1.0 in 16.16 fixed point as the unsigned bits of the field.
const fixedMinusOne = 0xFFFF0000

// identity is the tkhd matrix {a, b, u, c, d, v, x, y, w} that keeps the picture upright.
var identity = [9]uint32{fixedOne, 0, 0, 0, fixedOne, 0, 0, 0, 0x40000000}

// rotate90 is the tkhd matrix of a picture turned by 90 degrees (a = 0, b = 1, c = -1, d = 0).
var rotate90 = [9]uint32{0, fixedOne, 0, fixedMinusOne, 0, 0, 0, 0, 0x40000000}

// rotate270 is the tkhd matrix of a picture turned by 270 degrees (a = 0, b = -1, c = 1, d = 0).
var rotate270 = [9]uint32{0, fixedMinusOne, 0, fixedOne, 0, 0, 0, 0, 0x40000000}

// u32 converts the length of a test box, which is a few hundred bytes at most.
func u32(n int) uint32 {
	return uint32(n) //nolint:gosec // G115: test boxes are far below 4 GiB.
}

// be32 returns v as 4 big-endian bytes.
func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// be64 returns v as 8 big-endian bytes.
func be64(v uint64) []byte { return binary.BigEndian.AppendUint64(nil, v) }

// cat joins byte slices.
func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

// box builds a box with a 32-bit size header around the joined payload.
func box(typ string, payload ...[]byte) []byte {
	body := cat(payload...)
	return cat(be32(u32(8+len(body))), []byte(typ), body)
}

// largeBox builds a box with size 1 and a 64-bit largesize (its high word is zero).
func largeBox(typ string, payload ...[]byte) []byte {
	body := cat(payload...)
	return cat(be32(1), []byte(typ), be32(0), be32(u32(16+len(body))), body)
}

// openBox builds a box with size 0, which extends to the end of its parent.
func openBox(typ string, payload ...[]byte) []byte {
	return cat(be32(0), []byte(typ), cat(payload...))
}

// fullBox builds a full box: version, 24-bit flags (zero) and the payload.
func fullBox(typ string, version byte, payload ...[]byte) []byte {
	return box(typ, append([]byte{version, 0, 0, 0}, cat(payload...)...))
}

// ftyp builds a file type box with the major brand and no compatible brands.
func ftyp(brand string) []byte { return box("ftyp", []byte(brand), be32(0)) }

// mvhd0 builds a version 0 movie header with 32-bit times.
func mvhd0(timescale, duration uint32) []byte {
	return fullBox("mvhd", 0, be32(0), be32(0), be32(timescale), be32(duration), make([]byte, 80))
}

// mvhd1 builds a version 1 movie header with 64-bit times.
func mvhd1(timescale uint32, duration uint64) []byte {
	return fullBox("mvhd", 1, be64(0), be64(0), be32(timescale), be64(duration), make([]byte, 80))
}

// matrix encodes the nine 32-bit matrix entries.
func matrix(m [9]uint32) []byte {
	out := make([]byte, 0, 36)
	for _, v := range m {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

// tkhd builds a track header of the given version with width and height in whole pixels.
func tkhd(version byte, width, height uint32, m [9]uint32) []byte {
	times := cat(be32(0), be32(0), be32(1), be32(0), be32(0))
	if version == 1 {
		times = cat(be64(0), be64(0), be32(1), be32(0), be64(0))
	}
	return fullBox("tkhd", version, times, make([]byte, 16), matrix(m), be32(width*fixedOne), be32(height*fixedOne))
}

// hdlr builds a handler reference box with the four-letter handler type.
func hdlr(handler string) []byte {
	return fullBox("hdlr", 0, be32(0), []byte(handler), make([]byte, 12), []byte("Alex\x00"))
}

// mehd0 builds a version 0 movie extends header.
func mehd0(fragment uint32) []byte { return fullBox("mehd", 0, be32(fragment)) }

// mehd1 builds a version 1 movie extends header.
func mehd1(fragment uint64) []byte { return fullBox("mehd", 1, be64(fragment)) }

// trak builds a track with the given header and handler type.
func trak(header []byte, handler string) []byte {
	return box("trak", header, box("mdia", box("mdhd", make([]byte, 24)), hdlr(handler), box("minf")))
}

// mdat builds a media data box with filler bytes the reader must never need.
func mdat(n int) []byte { return box("mdat", bytes.Repeat([]byte{0xAB}, n)) }
