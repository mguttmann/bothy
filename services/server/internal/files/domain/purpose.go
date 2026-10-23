// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"mime"
	"strings"
)

// Purpose is files.purpose (docs/spec/03-data-model.md 4.7): it decides the read path of a file
// (05 2.2, 12 3.7) and, for avatar and branding, which originals may be stored (brief M3-10).
type Purpose string

// The purposes of 03 4.7.
const (
	// PurposeAttachment is a file uploaded into a channel.
	PurposeAttachment Purpose = "attachment"
	// PurposeAvatar is the avatar of a person (setMyAvatar).
	PurposeAvatar Purpose = "avatar"
	// PurposeBranding is the logo of the instance (uploadBrandingLogo).
	PurposeBranding Purpose = "branding"
	// PurposeGDPRExport is a data export of a person (M7).
	PurposeGDPRExport Purpose = "gdpr_export"
)

// Image reports the purposes of the image operations, avatar and branding: their files come from
// the multipart part "file", run through the purpose rule and have a fixed original_name.
func (p Purpose) Image() bool { return p == PurposeAvatar || p == PurposeBranding }

// imageUploadTypes are the declared types the schema ImageUpload admits (docs/api/openapi.yaml).
var imageUploadTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

// DeclaredImageTypeAllowed reports whether the declared Content-Type of the part "file" is one the
// image operations take (brief M3-10 design note 2b): image/jpeg, image/png or image/webp, compared
// without parameters and case. It is the first, declared step of 12 6.1; the second step, the
// detected type, is StoredOriginalAllowed. Purposes other than avatar and branding are no image
// uploads and always false.
//
// Example:
//
//	domain.DeclaredImageTypeAllowed(domain.PurposeAvatar, "image/png") // true
func DeclaredImageTypeAllowed(p Purpose, declared string) bool {
	if !p.Image() {
		return false
	}
	mt, _, err := mime.ParseMediaType(declared)
	return err == nil && imageUploadTypes[strings.ToLower(mt)]
}

// OriginalKind is the outcome of the original step of 12 7.1 (M3-06): how the original of an
// image is stored.
type OriginalKind uint8

// The outcomes of the original step.
const (
	// OriginalUnchanged: stored as uploaded (every type without a plan, a GIF or WebP the job
	// could not decode, an animated WebP, a WebP above file.image.max_pixels).
	OriginalUnchanged OriginalKind = iota
	// OriginalFilteredJPEG: the lossless segment filter of JPEGMarkerKept.
	OriginalFilteredJPEG
	// OriginalFilteredPNG: the lossless chunk filter of PNGChunkKept (also an animated PNG).
	OriginalFilteredPNG
	// OriginalConvertedWebP: a still WebP converted to PNG or JPEG.
	OriginalConvertedWebP
	// OriginalReencodedGIF: a GIF re-encoded without comments and application blocks.
	OriginalReencodedGIF
)

// PlannedOriginal is the outcome plan aims at; the job may still keep a GIF or WebP unchanged.
//
// Example:
//
//	domain.PlannedOriginal(domain.PlanConvertWebP) // OriginalConvertedWebP
func PlannedOriginal(plan OriginalPlan) OriginalKind {
	switch plan {
	case PlanStripJPEG:
		return OriginalFilteredJPEG
	case PlanStripPNG:
		return OriginalFilteredPNG
	case PlanConvertWebP:
		return OriginalConvertedWebP
	case PlanReencodeGIF:
		return OriginalReencodedGIF
	case PlanNone:
	}
	return OriginalUnchanged
}

// StoredOriginalAllowed is the purpose rule of 12 7.1 for avatar and branding (brief M3-10 design
// note 4, docs/decisions.d/M3-10.md D-M3-10-6): their original may only be a filtered JPEG, a filtered PNG
// or a converted still WebP, because the promise "without metadata" holds only for originals the
// server filtered or converted. Any other outcome fails the file with file_type_denied before it
// can become deliverable. Attachments are never refused by this rule.
//
// Example:
//
//	domain.StoredOriginalAllowed(domain.PurposeAvatar, domain.OriginalReencodedGIF) // false
func StoredOriginalAllowed(p Purpose, k OriginalKind) bool {
	if !p.Image() {
		return true
	}
	return k == OriginalFilteredJPEG || k == OriginalFilteredPNG || k == OriginalConvertedWebP
}

// The keys of the image cap, named in the detail of 413 payload_too_large.
const (
	keyMaxUploadBytes = "file.max_upload_bytes"
	keyPartBytes      = "file.upload_part_bytes"
)

// ImageCap is the largest avatar or logo: the smaller of file.max_upload_bytes and
// file.upload_part_bytes, because the image is stored as one upload part read into memory (brief
// M3-10 design note 2c, docs/decisions.d/M3-10.md D-M3-10-2).
//
// Example:
//
//	domain.ImageCap(maxUploadBytes, partBytes)
func ImageCap(maxUploadBytes, partBytes int64) int64 { return min(maxUploadBytes, partBytes) }

// ImageCapSetting names the key that sets ImageCap; file.max_upload_bytes when both are equal.
func ImageCapSetting(maxUploadBytes, partBytes int64) string {
	if partBytes < maxUploadBytes {
		return keyPartBytes
	}
	return keyMaxUploadBytes
}

// FixedOriginalName is the original_name of an avatar ("avatar") or logo ("logo") file; "" for the
// other purposes. The multipart file name is ignored, because these files are readable by every
// signed-in principal and the name could carry personal data (12 1 point 5, D-M3-10-3).
func FixedOriginalName(p Purpose) string {
	switch p {
	case PurposeAvatar:
		return "avatar"
	case PurposeBranding:
		return "logo"
	case PurposeAttachment, PurposeGDPRExport:
	}
	return ""
}
