// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strings"
)

// VariantOriginal names the original of a file as a download variant (docs/spec/12-files.md
// "9.2", the variant of a ticket). It has no key of its own: VariantKey refuses it, and the
// original lives under OriginalKey.
const VariantOriginal Variant = "original"

// ParseVariant returns the variant named s, one of original, small, medium, large and poster.
// ok is false for every other string, so a path or body value reaches the rules only as one of
// the five constants.
//
// Example:
//
//	v, ok := domain.ParseVariant("small") // domain.VariantSmall, true
func ParseVariant(s string) (Variant, bool) {
	switch v := Variant(s); v {
	case VariantOriginal, VariantSmall, VariantMedium, VariantLarge, VariantPoster:
		return v, true
	}
	return "", false
}

// IsThumbnail reports whether v is one of the four thumbnail variants small, medium, large and
// poster; it is false for VariantOriginal and for unknown values.
func (v Variant) IsThumbnail() bool {
	switch v {
	case VariantSmall, VariantMedium, VariantLarge, VariantPoster:
		return true
	case VariantOriginal:
		return false
	}
	return false
}

// Readiness is the answer of the download path to the states of a file (12 "9.3",
// "Nicht auslieferbare Dateien"). The zero value is ReadinessInvalidZero, so a forgotten
// assignment never means deliverable.
type Readiness uint8

// The values of Readiness.
const (
	// ReadinessInvalidZero is the unset value; no function returns it.
	ReadinessInvalidZero Readiness = iota
	// Deliverable means the bytes may be served.
	Deliverable
	// NotReady means the checks still run: 409 file_not_ready with Retry-After.
	NotReady
	// Blocked means the file will never be served: 409 file_blocked.
	Blocked
)

// ReadinessOf maps files.upload_state and files.scan_status to a Readiness (12 "2"):
// upload_state failed or scan_status infected or error is Blocked, because those states are
// final; upload_state complete with scan_status clean or skipped is Deliverable; every other
// pair, unknown values included, is NotReady. A deleted file is the caller's unknown-id 404 and
// never reaches this function.
//
// Example:
//
//	r := domain.ReadinessOf("complete", "pending") // domain.NotReady
func ReadinessOf(uploadState, scanStatus string) Readiness {
	switch {
	case uploadState == UploadFailed || scanStatus == ScanInfected || scanStatus == ScanError:
		return Blocked
	case uploadState == UploadComplete && (scanStatus == ScanClean || scanStatus == ScanSkipped):
		return Deliverable
	}
	return NotReady
}

// The media types the download rules name.
const (
	// attachmentType is the Content-Type of every response outside the inline list.
	attachmentType = "application/octet-stream"
	mediaJPEG      = "image/jpeg"
	mediaPNG       = "image/png"
)

// inlineOriginals is the inline list of 12 "9.3" for originals. Every other type, text/html and
// image/svg+xml above all, leaves the server as an attachment, so an upload never runs as a page
// of the instance's origin (SEC-T03).
var inlineOriginals = map[string]bool{
	mediaJPEG:   true,
	mediaPNG:    true,
	"image/gif": true,
	"audio/mp4": true,
	// audio/x-m4a is the detected name of MP4 audio of brand "M4A " (Apple recordings),
	// FIX-voice-send.
	"audio/x-m4a": true,
	"audio/mpeg":  true,
	"video/mp4":   true,
}

// mediaType returns the media type of a MIME type in lower case, without parameters and
// surrounding white space.
func mediaType(mimeType string) string {
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}
	return strings.ToLower(strings.TrimSpace(mimeType))
}

// Presentation returns the Content-Type of a download and whether it is shown inline
// (12 "9.3"). mimeType is files.mime_type for the original and file_thumbnails.mime_type for a
// thumbnail; its media type is compared case-insensitively and without parameters. An original
// in the inline list and a JPEG or PNG thumbnail keep their media type and are inline; everything
// else, unknown and empty types and unknown variants included, is application/octet-stream and
// an attachment.
//
// Example:
//
//	ct, inline := domain.Presentation("text/html", domain.VariantOriginal) // "application/octet-stream", false
func Presentation(mimeType string, v Variant) (contentType string, inline bool) {
	mt := mediaType(mimeType)
	switch {
	case v == VariantOriginal && inlineOriginals[mt]:
		return mt, true
	case v.IsThumbnail() && (mt == mediaJPEG || mt == mediaPNG):
		return mt, true
	}
	return attachmentType, false
}

// ContentDisposition returns the Content-Disposition header of a download (RFC 6266 with
// RFC 8187; 12 "9.3"): "inline" or "attachment", then filename with an ASCII fallback, then
// filename* with the UTF-8 bytes of name percent-encoded. In the fallback every rune outside
// printable ASCII and every '"', '\', '%' and ';' becomes one '_'; in filename* every byte
// outside the attr-char set of RFC 8187 becomes %XX. The header therefore holds only printable
// ASCII, never CR, LF or an unescaped quote. An empty name becomes FallbackName. The name is a
// canary value (SEC-T10): callers never log it.
//
// Example:
//
//	domain.ContentDisposition("Résumé for Sam.pdf", false)
//	// attachment; filename="R_sum_ for Sam.pdf"; filename*=UTF-8''R%C3%A9sum%C3%A9%20for%20Sam.pdf
func ContentDisposition(name string, inline bool) string {
	if name == "" {
		name = FallbackName
	}
	var b strings.Builder
	if inline {
		b.WriteString("inline")
	} else {
		b.WriteString("attachment")
	}
	b.WriteString(`; filename="`)
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '%' || r == ';' {
			r = '_'
		}
		b.WriteRune(r)
	}
	b.WriteString(`"; filename*=UTF-8''`)
	const hexDigits = "0123456789ABCDEF"
	for i := 0; i < len(name); i++ {
		if c := name[i]; isAttrChar(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0x0f])
		}
	}
	return b.String()
}

// isAttrChar reports the attr-char set of RFC 8187 section 3.2.1: ALPHA, DIGIT and
// "!#$&+-.^_`|~".
func isAttrChar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("!#$&+-.^_`|~", c) >= 0
}

// ThumbnailName returns the download name of a thumbnail: originalName without its last
// extension (the part from the last '.', unless that dot is the first character), then
// "-<variant>", then ".png" for an image/png thumbnail and ".jpg" for every other type.
//
// Example:
//
//	domain.ThumbnailName("Beach day.heic", domain.VariantSmall, "image/jpeg") // "Beach day-small.jpg"
func ThumbnailName(originalName string, v Variant, mimeType string) string {
	stem := originalName
	if dot := strings.LastIndexByte(stem, '.'); dot > 0 {
		stem = stem[:dot]
	}
	ext := ".jpg"
	if mediaType(mimeType) == mediaPNG {
		ext = ".png"
	}
	return stem + "-" + string(v) + ext
}
