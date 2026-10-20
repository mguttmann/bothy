// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// testByteOrder reads and appends in one byte order.
type testByteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// exifTestBlock returns a TIFF block in byte order order with IFD0 at ifdOffset holding the
// given entries (tag, type, count, inline value) and a next-IFD offset of zero.
func exifTestBlock(order testByteOrder, ifdOffset uint32, entries ...[4]uint32) []byte {
	b := []byte("II")
	if order == testByteOrder(binary.BigEndian) {
		b = []byte("MM")
	}
	b = order.AppendUint16(b, 42)
	b = order.AppendUint32(b, ifdOffset)
	for len(b) < int(ifdOffset) {
		b = append(b, 0)
	}
	b = order.AppendUint16(b, uint16(len(entries))) //nolint:gosec // G115: a handful of entries
	for _, e := range entries {
		b = order.AppendUint16(b, uint16(e[0])) //nolint:gosec // G115: test tags fit 16 bits
		b = order.AppendUint16(b, uint16(e[1])) //nolint:gosec // G115: test types fit 16 bits
		b = order.AppendUint32(b, e[2])
		v := order.AppendUint16(nil, uint16(e[3])) //nolint:gosec // G115: test values fit 16 bits
		b = append(append(b, v...), 0, 0)
	}
	return order.AppendUint32(b, 0)
}

func TestEXIFOrientation(t *testing.T) {
	le, be := binary.LittleEndian, binary.BigEndian
	orient := func(v uint32) [4]uint32 { return [4]uint32{0x0112, 3, 1, v} }
	other := [4]uint32{0x010F, 2, 4, 0x4142}
	valid := exifTestBlock(be, 8, orient(6))
	cases := map[string]struct {
		tiff []byte
		want domain.Orientation
	}{
		"little endian":      {exifTestBlock(le, 8, other, orient(6)), 6},
		"big endian":         {exifTestBlock(be, 8, orient(3)), 3},
		"ifd later in block": {exifTestBlock(le, 20, orient(8)), 8},
		"value 1":            {exifTestBlock(le, 8, orient(1)), 1},
		"tag missing":        {exifTestBlock(le, 8, other), 1},
		"no entries":         {exifTestBlock(be, 8), 1},
		"wrong type LONG":    {exifTestBlock(le, 8, [4]uint32{0x0112, 4, 1, 6}), 1},
		"count 2":            {exifTestBlock(le, 8, [4]uint32{0x0112, 3, 2, 6}), 1},
		"value 0":            {exifTestBlock(le, 8, orient(0)), 1},
		"value 9":            {exifTestBlock(be, 8, orient(9)), 1},
		"value 0xFFFF":       {exifTestBlock(be, 8, orient(0xFFFF)), 1},
		"ifd offset outside": {append(append([]byte{}, valid[:4]...), 0, 0, 0xFF, 0xFF), 1},
		"ifd offset huge":    {append(append([]byte{}, valid[:4]...), 0xFF, 0xFF, 0xFF, 0xFF), 1},
		"entry beyond block": {valid[:len(valid)-8], 1},
		"count beyond block": {append(append([]byte{}, valid[:8]...), 0xFF, 0xFF), 1},
		"bad byte order":     {append([]byte("XX"), valid[2:]...), 1},
		"mixed byte order":   {append([]byte("IM"), valid[2:]...), 1},
		"bad magic":          {append(append([]byte{}, valid[:2]...), append([]byte{0, 43}, valid[4:]...)...), 1},
		"short header":       {valid[:7], 1},
		"nil block":          {nil, 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := orientationFromTIFF(c.tiff); got != c.want {
				t.Fatalf("orientation %d, want %d", got, c.want)
			}
		})
	}
}

func TestWriteOrientationAPP1(t *testing.T) {
	for o := domain.Orientation(2); o <= 8; o++ {
		seg := orientationAPP1(o)
		if len(seg) < 4 || seg[0] != 0xFF || seg[1] != 0xE1 || int(binary.BigEndian.Uint16(seg[2:])) != len(seg)-2 {
			t.Fatalf("orientation %d: bad segment %x", o, seg)
		}
		payload := seg[4:]
		if !bytes.HasPrefix(payload, []byte(exifIdentifier+"MM\x00\x2a\x00\x00\x00\x08")) {
			t.Fatalf("orientation %d: bad header %x", o, payload)
		}
		tiff := payload[len(exifIdentifier):]
		if got := orientationFromTIFF(tiff); got != o {
			t.Fatalf("read back %d, want %d", got, o)
		}
		if binary.BigEndian.Uint16(tiff[8:]) != 1 || binary.BigEndian.Uint32(tiff[8+2+12:]) != 0 || len(tiff) != 8+2+12+4 {
			t.Fatalf("orientation %d: more than one entry or a next IFD", o)
		}
	}
	for _, o := range []domain.Orientation{0, 1, 9, 255} {
		if seg := orientationAPP1(o); seg != nil {
			t.Fatalf("orientation %d: segment %x, want none", o, seg)
		}
	}
}

// FuzzEXIFOrientation checks that the EXIF reader never panics and always yields 1 to 8.
func FuzzEXIFOrientation(f *testing.F) {
	f.Add(exifTestBlock(binary.LittleEndian, 8, [4]uint32{0x0112, 3, 1, 6}))
	f.Add(exifTestBlock(binary.BigEndian, 8, [4]uint32{0x0112, 3, 1, 8}))
	f.Add(testTIFF(f, jpegExtras{Orientation: 5, EXIF: true}))
	f.Add(orientationAPP1(7)[4+len(exifIdentifier):])
	f.Add([]byte("MM\x00\x2a\xff\xff\xff\xf0"))
	f.Fuzz(func(t *testing.T, tiff []byte) {
		if o := orientationFromTIFF(tiff); !o.Valid() {
			t.Fatalf("orientation %d", o)
		}
	})
}
