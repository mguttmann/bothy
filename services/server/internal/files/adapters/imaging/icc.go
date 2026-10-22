// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

// The capacity of APP2 ICC_PROFILE embedding (ICC.1:2022 Annex B.4): at most 255 parts, because
// the sequence number and the part count are one byte each.
const maxICCParts = 255

// iccPartPayload is the largest profile part of one APP2 segment: the segment length 65535 minus
// the two length bytes and the 14 bytes of identifier, sequence number and part count.
const iccPartPayload = 65519

// errProfileTooLarge reports a colour profile above the capacity of APP2 ICC_PROFILE segments;
// such an image gets no thumbnails (brief M3-06 design note 9).
var errProfileTooLarge = errors.New("files/imaging: colour profile too large for a JPEG")

// The iCCP chunk (PNG specification, third edition, 11.3.2.4).
const (
	// iccpProfileName is the profile name iccpChunkData writes; the name has no meaning for
	// decoders and says nothing about the uploader.
	iccpProfileName = "ICC Profile"
	// pngMaxKeyword is the longest keyword of an iCCP chunk; the name ends with a NUL within
	// the first pngMaxKeyword+1 bytes.
	pngMaxKeyword = 79
	// iccpMethodDeflate is the only defined compression method, zlib with deflate.
	iccpMethodDeflate = 0
	// iccHeaderSize is the fixed header of an ICC profile (ICC.1:2022 section 7.2).
	iccHeaderSize = 128
	// iccColourSpaceAt is the offset of the field "data colour space" in the header.
	iccColourSpaceAt = 16
	// iccColourSpaceLen is the length of that field, a four-character signature.
	iccColourSpaceLen = 4
)

// jpegICCSegments splits profile into complete APP2 segments (0xFF 0xE2, length, "ICC_PROFILE\0",
// sequence number from 1, part count, payload), each part as large as the segment allows, so a
// profile takes as few segments as possible. It returns nil for an empty profile and
// errProfileTooLarge when more than 255 parts would be needed.
func jpegICCSegments(profile []byte) ([][]byte, error) {
	if len(profile) == 0 {
		return nil, nil
	}
	parts := (len(profile) + iccPartPayload - 1) / iccPartPayload
	if parts > maxICCParts {
		return nil, errProfileTooLarge
	}
	segs := make([][]byte, 0, parts)
	// seq counts the parts as a byte; the check above keeps it within 255.
	var seq byte
	for off := 0; off < len(profile); off += iccPartPayload {
		seq++
		part := profile[off:min(off+iccPartPayload, len(profile))]
		seg := make([]byte, 0, 4+iccHeaderLen+len(part))
		seg = append(seg, markerPrefix, markerAPP2)
		seg = binary.BigEndian.AppendUint16(seg, uint16(segmentLengthSize+iccHeaderLen+len(part))) //nolint:gosec // G115: a part holds at most iccPartPayload bytes, so the length is at most 65535
		seg = append(seg, iccIdentifier...)
		seg = append(seg, seq, 0) // the part count is filled in below
		segs = append(segs, append(seg, part...))
	}
	for _, seg := range segs {
		seg[4+len(iccIdentifier)+1] = seq
	}
	return segs, nil
}

// pngChunk returns one complete PNG chunk: big-endian length, type, data and the CRC-32 over type
// and data (PNG specification 5.3). It panics when data is longer than a PNG chunk may be
// (2^31-1 bytes), which only a programming error can cause: callers pass profiles and headers.
func pngChunk(typ string, data []byte) []byte {
	if len(data) > math.MaxInt32 {
		panic("files/imaging: PNG chunk data above 2^31-1 bytes")
	}
	c := make([]byte, 0, 12+len(data))
	c = binary.BigEndian.AppendUint32(c, uint32(len(data))) //nolint:gosec // G115: checked against MaxInt32 above
	c = append(c, typ...)
	c = append(c, data...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

// iccpChunkData returns the data of an iCCP chunk for profile: the profile name "ICC Profile",
// NUL, compression method 0 and the zlib stream of the profile. Errors come from the compressor.
func iccpChunkData(profile []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(iccpProfileName)
	buf.WriteByte(0)
	buf.WriteByte(iccpMethodDeflate)
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(profile); err != nil {
		return nil, fmt.Errorf("files/imaging: compress profile: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("files/imaging: compress profile: %w", err)
	}
	return buf.Bytes(), nil
}

// iccpProfile decompresses the profile of iCCP chunk data and reads at most maxBytes bytes of it, so a
// small chunk cannot expand into a large allocation: errProfileTooLarge when the profile is
// longer than maxBytes. A missing or empty name, a method other than 0, a broken zlib stream or a
// wrong checksum wrap app.ErrImageMalformed.
func iccpProfile(data []byte, maxBytes int) ([]byte, error) {
	nul := bytes.IndexByte(data[:min(len(data), pngMaxKeyword+1)], 0)
	if nul < 1 || nul+1 >= len(data) || data[nul+1] != iccpMethodDeflate {
		return nil, malformedICC("iCCP header")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[nul+2:]))
	if err != nil {
		return nil, malformedICC("iCCP stream")
	}
	defer func() { _ = zr.Close() }() // a zlib reader over memory has nothing to release
	profile, err := io.ReadAll(io.LimitReader(zr, int64(maxBytes)+1))
	if err != nil {
		return nil, malformedICC("iCCP stream")
	}
	if len(profile) > maxBytes {
		return nil, errProfileTooLarge
	}
	return profile, nil
}

// malformedICC returns an error wrapping app.ErrImageMalformed that names the broken part.
func malformedICC(what string) error {
	return fmt.Errorf("%w: broken %s", app.ErrImageMalformed, what)
}

// profileColourSpace returns the field "data colour space" of an ICC header (bytes 16 to 19, for
// example "RGB ", "GRAY" or "CMYK"), which decides whether a thumbnail may carry the profile, or
// "" when the profile is shorter than the 128-byte header.
func profileColourSpace(profile []byte) string {
	if len(profile) < iccHeaderSize {
		return ""
	}
	return string(profile[iccColourSpaceAt : iccColourSpaceAt+iccColourSpaceLen])
}
