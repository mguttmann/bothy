// SPDX-License-Identifier: Apache-2.0

package imaging

import (
	"encoding/binary"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// The parts of a TIFF block (TIFF 6.0 section 2, EXIF 2.32 section 4.6) the orientation reader
// and writer need.
const (
	// tiffHeaderLen is the byte order mark, the magic number 42 and the offset of IFD0.
	tiffHeaderLen = 8
	// tiffMagic is the number after the byte order mark.
	tiffMagic = 42
	// ifdCountLen is the entry count at the start of an IFD.
	ifdCountLen = 2
	// ifdEntryLen is one IFD entry: tag, type, count and the value or its offset.
	ifdEntryLen = 12
	// ifdNextLen is the offset of the next IFD after the entries.
	ifdNextLen = 4
	// tagOrientation is the EXIF tag "Orientation" (EXIF 2.32 table 4).
	tagOrientation = 0x0112
	// typeShort is the TIFF field type SHORT (16-bit unsigned).
	typeShort = 3
	// tiffLittleEndian and tiffBigEndian are the byte order marks.
	tiffLittleEndian = "II"
	tiffBigEndian    = "MM"
)

// orientationTIFFLen is the length of the TIFF block orientationAPP1 writes: header, IFD0 with
// one entry, next-IFD offset.
const orientationTIFFLen = tiffHeaderLen + ifdCountLen + ifdEntryLen + ifdNextLen

// orientationFromTIFF returns the tag Orientation (0x0112, type SHORT, count 1) of IFD0 of a TIFF
// block tiff, which starts at the byte order mark "II" or "MM". Both byte orders are read and
// every offset is checked against the block. The result is 1 for anything unreadable, missing,
// of the wrong type or count, or outside 1 to 8, because the block falls away anyway and an
// unreadable orientation must not reject the image (brief M3-06 design note 5). It never panics.
func orientationFromTIFF(tiff []byte) domain.Orientation {
	order, ok := tiffByteOrder(tiff)
	if !ok {
		return domain.OrientationNormal
	}
	ifd := int(order.Uint32(tiff[4:]))
	if ifd < tiffHeaderLen || ifd > len(tiff)-ifdCountLen {
		return domain.OrientationNormal
	}
	n := int(order.Uint16(tiff[ifd:]))
	for i := range n {
		entry := ifd + ifdCountLen + i*ifdEntryLen
		if entry > len(tiff)-ifdEntryLen {
			return domain.OrientationNormal
		}
		if order.Uint16(tiff[entry:]) == tagOrientation {
			return orientationEntry(order, tiff[entry:entry+ifdEntryLen])
		}
	}
	return domain.OrientationNormal
}

// tiffByteOrder returns the byte order of a TIFF header and false when tiff is shorter than the
// header, has no valid byte order mark or not the magic number 42.
func tiffByteOrder(tiff []byte) (binary.ByteOrder, bool) {
	if len(tiff) < tiffHeaderLen {
		return nil, false
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case tiffLittleEndian:
		order = binary.LittleEndian
	case tiffBigEndian:
		order = binary.BigEndian
	default:
		return nil, false
	}
	return order, order.Uint16(tiff[2:]) == tiffMagic
}

// orientationEntry returns the value of one Orientation entry: 1 unless the type is SHORT, the
// count is 1 and the value lies in 1 to 8. A SHORT with count 1 sits in the first two bytes of the
// value field (TIFF 6.0 section 2).
func orientationEntry(order binary.ByteOrder, entry []byte) domain.Orientation {
	if order.Uint16(entry[2:]) != typeShort || order.Uint32(entry[4:]) != 1 {
		return domain.OrientationNormal
	}
	v := order.Uint16(entry[8:])
	for o := domain.OrientationNormal; o.Valid(); o++ {
		if uint16(o) == v {
			return o
		}
	}
	return domain.OrientationNormal
}

// orientationAPP1 returns one complete APP1 segment holding only the orientation o: marker 0xFF
// 0xE1, big-endian length, "Exif\0\0", a big-endian TIFF header ("MM\0*", IFD0 at offset 8), IFD0
// with the single entry Orientation (SHORT, count 1, value o) and a next-IFD offset of 0, so no
// Exif, GPS or interoperability IFD can follow. It returns nil for 1 and for invalid values,
// because orientation 1 needs no tag (brief M3-06 design note 5).
func orientationAPP1(o domain.Orientation) []byte {
	if !o.Valid() || o == domain.OrientationNormal {
		return nil
	}
	be := binary.BigEndian
	seg := []byte{markerPrefix, markerAPP1}
	seg = be.AppendUint16(seg, segmentLengthSize+uint16(len(exifIdentifier))+orientationTIFFLen)
	seg = append(seg, exifIdentifier...)
	seg = append(seg, tiffBigEndian...)
	seg = be.AppendUint16(seg, tiffMagic)
	seg = be.AppendUint32(seg, tiffHeaderLen)
	seg = be.AppendUint16(seg, 1)
	seg = be.AppendUint16(seg, tagOrientation)
	seg = be.AppendUint16(seg, typeShort)
	seg = be.AppendUint32(seg, 1)
	seg = be.AppendUint16(seg, uint16(o))
	seg = be.AppendUint16(seg, 0) // the unused half of the value field
	return be.AppendUint32(seg, 0)
}
