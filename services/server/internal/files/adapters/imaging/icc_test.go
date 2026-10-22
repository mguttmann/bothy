// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image/color"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
)

func TestJPEGICCSegmentsRoundTrip(t *testing.T) {
	plain := encodeTestJPEG(t, twoColour(16, 8, color.Black, color.White))
	for name, c := range map[string]struct{ size, parts int }{
		"one part":    {size: 700, parts: 1},
		"three parts": {size: 2*iccPartPayload + 10, parts: 3},
	} {
		t.Run(name, func(t *testing.T) {
			profile := syntheticICC(testColourRGB, c.size)
			segs, err := jpegICCSegments(profile)
			if err != nil || len(segs) != c.parts {
				t.Fatalf("%d segments, err %v; want %d", len(segs), err, c.parts)
			}
			in := []byte{0xFF, 0xD8}
			for i, s := range segs {
				checkICCSegment(t, s, i, c.parts, i < c.parts-1)
				in = append(in, s...)
			}
			info, err := scanJPEG(bytes.NewReader(append(in, plain[2:]...)))
			if err != nil || !bytes.Equal(info.ICC, profile) {
				t.Fatalf("round trip failed (err %v)", err)
			}
		})
	}
}

// checkICCSegment fails unless s is a complete APP2 ICC_PROFILE segment with sequence number
// i+1 of count parts, full-sized when full is set.
func checkICCSegment(t *testing.T, s []byte, i, count int, full bool) {
	t.Helper()
	if s[0] != 0xFF || s[1] != 0xE2 || int(binary.BigEndian.Uint16(s[2:])) != len(s)-2 {
		t.Fatalf("part %d: bad segment header", i)
	}
	if !bytes.HasPrefix(s[4:], []byte(iccIdentifier)) || int(s[16]) != i+1 || int(s[17]) != count {
		t.Fatalf("part %d: bad ICC header", i)
	}
	if full && len(s)-4-iccHeaderLen != iccPartPayload {
		t.Fatalf("part %d: %d payload bytes, want the largest part", i, len(s)-4-iccHeaderLen)
	}
}

func TestJPEGICCSegmentsLimits(t *testing.T) {
	segs, err := jpegICCSegments(nil)
	if err != nil || segs != nil {
		t.Fatalf("empty profile: %d segments, err %v", len(segs), err)
	}
	segs, err = jpegICCSegments(make([]byte, maxICCParts*iccPartPayload))
	if err != nil || len(segs) != maxICCParts {
		t.Fatalf("largest profile: %d segments, err %v", len(segs), err)
	}
	if _, err := jpegICCSegments(make([]byte, maxICCParts*iccPartPayload+1)); !errors.Is(err, errProfileTooLarge) {
		t.Fatalf("too large: err %v", err)
	}
}

func TestICCPRoundTrip(t *testing.T) {
	profile := syntheticICC("GRAY", 5000)
	data, err := iccpChunkData(profile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("ICC Profile\x00\x00")) {
		t.Fatalf("bad iCCP header %q", data[:14])
	}
	got, err := iccpProfile(data, len(profile))
	if err != nil || !bytes.Equal(got, profile) {
		t.Fatalf("round trip failed (err %v)", err)
	}
	if _, err := iccpProfile(data, len(profile)-1); !errors.Is(err, errProfileTooLarge) {
		t.Fatalf("max stop: err %v", err)
	}
}

func TestICCPProfileMalformed(t *testing.T) {
	data, err := iccpChunkData(syntheticICC(testColourRGB, 300))
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte{}, data...)
	corrupt[len(corrupt)-1] ^= 0xFF
	method := append([]byte{}, data...)
	method[len(iccpProfileName)+1] = 1
	for name, d := range map[string][]byte{
		"no NUL":         bytes.Repeat([]byte("a"), 100),
		"empty name":     append([]byte{0, 0}, data[len(iccpProfileName)+2:]...),
		"no method":      []byte("ICC Profile\x00"),
		"method 1":       method,
		"bad zlib":       []byte("ICC Profile\x00\x00not zlib at all"),
		"bad checksum":   corrupt,
		"truncated zlib": data[:len(data)-6],
		"no data":        nil,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := iccpProfile(d, 1<<20); !errors.Is(err, app.ErrImageMalformed) {
				t.Fatalf("err %v, want ErrImageMalformed", err)
			}
		})
	}
}

func TestProfileColourSpace(t *testing.T) {
	for _, cs := range []string{testColourRGB, "GRAY", "CMYK"} {
		if got := profileColourSpace(syntheticICC(cs, 200)); got != cs {
			t.Fatalf("colour space %q, want %q", got, cs)
		}
	}
	if got := profileColourSpace(make([]byte, 127)); got != "" {
		t.Fatalf("short profile: %q", got)
	}
}

func TestPNGChunk(t *testing.T) {
	c := pngChunk("tEXt", []byte("k\x00v"))
	if binary.BigEndian.Uint32(c) != 3 || string(c[4:8]) != "tEXt" || string(c[8:11]) != "k\x00v" {
		t.Fatalf("chunk %x", c)
	}
	if binary.BigEndian.Uint32(c[11:]) != crc32.ChecksumIEEE(c[4:11]) || len(c) != 15 {
		t.Fatalf("bad CRC or length")
	}
}
