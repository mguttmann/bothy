// SPDX-License-Identifier: Apache-2.0

package domain

import "math/bits"

// OriginalPlan is what the job files.thumbnail does with the original of a file before it
// becomes deliverable (docs/spec/12-dateien.md 7.1, ADR 0047, docs/DECISIONS.md D-M3-06-1).
type OriginalPlan uint8

// The plans of 12 7.1.
const (
	// PlanNone: the original is stored unchanged (SVG, HEIC, HEIF, AVIF, TIFF, every other type).
	PlanNone OriginalPlan = iota
	// PlanStripJPEG: the lossless segment filter with the allow list of JPEGMarkerKept.
	PlanStripJPEG
	// PlanStripPNG: the lossless chunk filter with the allow list of PNGChunkKept.
	PlanStripPNG
	// PlanReencodeGIF: lossless re-encoding with the palette, without comments and application
	// blocks except the loop count.
	PlanReencodeGIF
	// PlanConvertWebP: a still WebP becomes PNG (alpha) or JPEG (no alpha).
	PlanConvertWebP
)

// OriginalPlanFor returns the plan of a detected media type, exactly as the type detection of
// adapters/mimetype names it (lower case, without parameters). image/apng is the name of the
// pinned detection for an animated PNG; it is a PNG and gets the PNG filter, because otherwise its
// text chunks would reach the store unfiltered (docs/DECISIONS.md D-M3-06-1).
//
// Example:
//
//	domain.OriginalPlanFor("image/jpeg") // PlanStripJPEG
func OriginalPlanFor(mimeType string) OriginalPlan {
	switch mimeType {
	case "image/jpeg":
		return PlanStripJPEG
	case "image/png", "image/apng":
		return PlanStripPNG
	case "image/gif":
		return PlanReencodeGIF
	case "image/webp":
		return PlanConvertWebP
	default:
		return PlanNone
	}
}

// HasContainerMetadata reports the MP4 and QuickTime types whose width, height and duration the
// job files.thumbnail reads from the boxes mvhd and tkhd (12 7.3): video/mp4, video/quicktime,
// video/x-m4v, audio/mp4 and audio/x-m4a, as the pinned type detection names them. HEIC, HEIF
// and AVIF are ISO base media files too but images, and do not count.
//
// Example:
//
//	domain.HasContainerMetadata("video/quicktime") // true
func HasContainerMetadata(mimeType string) bool {
	switch mimeType {
	case "video/mp4", "video/quicktime", "video/x-m4v", "audio/mp4", "audio/x-m4a":
		return true
	default:
		return false
	}
}

// ExceedsPixelLimit reports whether width times height times frames is above maxPixels
// (file.image.max_pixels, 12 7.2, SEC-UP-4). The multiplication cannot overflow: a product beyond
// int64 counts as exceeding, and so do negative factors. frames 0 counts as one frame.
//
// Example:
//
//	domain.ExceedsPixelLimit(8000, 8000, 1, 50_000_000) // true
func ExceedsPixelLimit(width, height, frames, maxPixels int64) bool {
	if width < 0 || height < 0 || frames < 0 || maxPixels < 0 {
		return true
	}
	if frames == 0 {
		frames = 1
	}
	hi, area := bits.Mul64(uint64(width), uint64(height))
	if hi != 0 {
		return true
	}
	hi, total := bits.Mul64(area, uint64(frames))
	if hi != 0 || total > uint64(maxPixels) {
		return true
	}
	return false
}

// Orientation is the EXIF tag "Orientation" (values 1 to 8): how the stored pixels are turned for
// display. 0 and values above 8 are invalid and count as 1.
type Orientation uint8

// OrientationNormal is the value 1: the stored pixels are shown as they are.
const OrientationNormal Orientation = 1

// Valid reports a value from 1 to 8.
func (o Orientation) Valid() bool { return o >= 1 && o <= 8 }

// DisplaySize returns the size of an image of the stored size width x height after the
// orientation is applied: the values 5 to 8 turn by 90 or 270 degrees and swap the edges.
//
// Example:
//
//	w, h := domain.Orientation(6).DisplaySize(4000, 3000) // 3000, 4000
func (o Orientation) DisplaySize(width, height int) (int, int) {
	if o >= 5 && o <= 8 {
		return height, width
	}
	return width, height
}

// RenderedVariants returns the variants the server renders for every decodable image, in the
// order small, medium, large (12 7.3); poster is the client's and never rendered.
func RenderedVariants() []Variant {
	return []Variant{VariantSmall, VariantMedium, VariantLarge}
}

// VariantSize returns the size of a variant whose longest edge is longestEdge for an image of
// width x height: never larger than the image (no upscaling), the other edge rounded to the
// nearest pixel and at least one pixel. Non-positive input gives 0, 0.
//
// Example:
//
//	w, h := domain.VariantSize(4000, 3000, 320) // 320, 240
func VariantSize(width, height, longestEdge int) (int, int) {
	if width <= 0 || height <= 0 || longestEdge <= 0 {
		return 0, 0
	}
	long, short := width, height
	if height > width {
		long, short = height, width
	}
	if long <= longestEdge {
		return width, height
	}
	scaled := int((int64(short)*int64(longestEdge)*2 + int64(long)) / (int64(long) * 2))
	scaled = max(scaled, 1)
	if width >= height {
		return longestEdge, scaled
	}
	return scaled, longestEdge
}

// JPEGMarkerKept reports whether the JPEG filter keeps segments of marker (the byte after 0xFF):
// SOI, EOI, the frame headers SOF0 to SOF15 except JPG (0xC8), the tables DQT, DHT, DAC, DRI, DNL,
// DHP and EXP, SOS and RST0 to RST7 (every marker ITU-T T.81 needs to decode), and the
// application segments APP0, APP1, APP2 and APP14, which the filter keeps only with their
// identifiers JFIF, a rewritten Exif with only the orientation, ICC_PROFILE and Adobe (12 7.1,
// ADR 0047 point 1). Every other APPn, COM, JPGn and the reserved markers fall away.
//
// Example:
//
//	domain.JPEGMarkerKept(0xFE) // false: COM
func JPEGMarkerKept(marker byte) bool {
	switch {
	case marker >= 0xC0 && marker <= 0xCF:
		return marker != 0xC8
	case marker >= 0xD0 && marker <= 0xD9: // RST0 to RST7, SOI, EOI
		return true
	case marker >= 0xDA && marker <= 0xDF: // SOS, DQT, DNL, DRI, DHP, EXP
		return true
	case marker == 0xE0, marker == 0xE1, marker == 0xE2, marker == 0xEE:
		return true
	default:
		return false
	}
}

// PNGChunkKept reports whether the PNG filter keeps a chunk of type chunkType: the critical
// chunks and the ancillary chunks for colour, transparency, density and animation of 12 7.1
// (ADR 0047 point 2). Every other chunk falls away, also one marked safe to copy.
//
// Example:
//
//	domain.PNGChunkKept("tEXt") // false
func PNGChunkKept(chunkType string) bool {
	switch chunkType {
	case "IHDR", "PLTE", "IDAT", "IEND", "tRNS", "cHRM", "gAMA", "iCCP", "sBIT", "sRGB", "cICP", "mDCV", "cLLI",
		"bKGD", "pHYs", "acTL", "fcTL", "fdAT":
		return true
	default:
		return false
	}
}
