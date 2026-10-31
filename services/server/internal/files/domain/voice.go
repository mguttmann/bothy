// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
)

// The detected names of MP4 audio as the pinned type detection reports them (12 8.1, M3-09
// design note 4 a): brand "M4A " gives audio/x-m4a, brand "M4B " audio/mp4. An MP4 of a generic
// brand (isom, mp42) is named video/mp4 whatever its tracks are; the scan pass stores it as
// audio/mp4 when the uploader declared MP4 audio (RefineMP4Type, FIX-voice-send), and only then
// is it a voice format. A video track still fails the voice check (rule 4).
const (
	mimeAudioXM4A = "audio/x-m4a"
	mimeAudioMP4  = "audio/mp4"
	// mimeAudioMPEG is MP3, a voice format for agents only (12 8.1).
	mimeAudioMPEG = "audio/mpeg"
	// authorAgent is the principal kind of an agent.
	authorAgent = "agent"
	// mimeVideoMP4 is the name the type detection gives every MP4 of a generic brand.
	mimeVideoMP4 = "video/mp4"
)

// RefineMP4Type returns the type the scan pass stores for the detected type detected and the
// declared type declared (12 6.1, FIX-voice-send): audio/mp4 when detected is video/mp4 and
// declared is MP4 audio (audio/mp4 or audio/x-m4a, parameters and case ignored), else detected
// unchanged. The detection names MP4 by its major brand only, and recorders write generic brands
// (mp42 on Windows, isom on Android) for an audio-only file; the declaration tells them apart.
// It never turns a non-MP4 into audio, and the container phase still reads width and height, so a
// declared-audio file with a video track fails the voice check.
//
// Example:
//
//	domain.RefineMP4Type("video/mp4", "audio/mp4") // "audio/mp4"
//	domain.RefineMP4Type("video/mp4", "video/mp4") // "video/mp4"
func RefineMP4Type(detected, declared string) string {
	if detected == mimeVideoMP4 && isMP4Audio(MediaType(declared)) {
		return mimeAudioMP4
	}
	return detected
}

// isMP4Audio reports the MP4 audio names of the type detection.
func isMP4Audio(mimeType string) bool {
	return mimeType == mimeAudioXM4A || mimeType == mimeAudioMP4
}

// IsVoiceFormat reports whether a file of the detected type mimeType may be the attachment of a
// voice message of an author of authorKind (user, agent, app; docs/spec/12-files.md 8.1): MP4
// audio for every author, MP3 (audio/mpeg) in addition for agents.
//
// Example:
//
//	domain.IsVoiceFormat("audio/x-m4a", "user") // true
//	domain.IsVoiceFormat("audio/mpeg", "user")  // false
func IsVoiceFormat(mimeType, authorKind string) bool {
	return isMP4Audio(mimeType) || (mimeType == mimeAudioMPEG && authorKind == authorAgent)
}

// DeferContainerComplete reports whether the final write of the scan pass leaves upload_state
// pending for the container phase of files.thumbnail (M3-09 design note 5): exactly for MP4
// audio, so a voice file is deliverable only together with its duration. Video and QuickTime keep
// the order of M3-06.
//
// Example:
//
//	domain.DeferContainerComplete("audio/x-m4a") // true
func DeferContainerComplete(mimeType string) bool { return isMP4Audio(mimeType) }

// VoiceFacts are the columns of the locked files row the voice check decides on, with the author
// of the message and voice.max_duration.
type VoiceFacts struct {
	// UploadState and ScanStatus are files.upload_state and files.scan_status; Deleted is
	// deleted_at IS NOT NULL.
	UploadState, ScanStatus string
	Deleted                 bool
	// MimeType is files.mime_type, the detected type.
	MimeType string
	// AuthorKind is the principal kind of the author of the message.
	AuthorKind string
	// HasWidth and HasHeight report files.width and files.height set (a video track).
	HasWidth, HasHeight bool
	// DurationMs is files.duration_ms; HasDuration reports it set.
	DurationMs  int64
	HasDuration bool
	// MaxDuration is voice.max_duration.
	MaxDuration time.Duration
}

// VoiceVerdict is the answer of VoiceCheck.
type VoiceVerdict int

// The verdicts in the order VoiceCheck tests them.
const (
	// VoiceOK: the file may be the attachment of the voice message.
	VoiceOK VoiceVerdict = iota
	// VoiceNotReady: upload or scan still pending (409 file_not_ready).
	VoiceNotReady
	// VoiceBlocked: final but not deliverable (409 file_blocked).
	VoiceBlocked
	// VoiceFormatUnsupported: not a voice format, an MP4 with a video track or without a
	// duration (400 voice_format_unsupported).
	VoiceFormatUnsupported
	// VoiceTooLong: longer than voice.max_duration (400 voice_too_long).
	VoiceTooLong
)

// VoiceCheck returns the first failing verdict for the attachment of a voice message (12 8.1,
// M3-09 design note 4 b): not final gives VoiceNotReady, final but not deliverable VoiceBlocked,
// not a voice format VoiceFormatUnsupported, an MP4 with width or height or without a duration
// VoiceFormatUnsupported, a duration above MaxDuration VoiceTooLong (equal passes), else VoiceOK.
// MP3 has no duration and passes the duration rule. The AAC codec inside the MP4 is not checked
// (design note 4 c).
//
// Example:
//
//	v := domain.VoiceCheck(domain.VoiceFacts{UploadState: "complete", ScanStatus: "clean",
//		MimeType: "audio/x-m4a", AuthorKind: "user", DurationMs: 4500, HasDuration: true,
//		MaxDuration: maxDuration}) // VoiceOK
func VoiceCheck(f VoiceFacts) VoiceVerdict {
	switch {
	case !Final(f.UploadState, f.ScanStatus, f.Deleted) && !f.Deleted:
		return VoiceNotReady
	case !IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted):
		return VoiceBlocked
	case !IsVoiceFormat(f.MimeType, f.AuthorKind):
		return VoiceFormatUnsupported
	case isMP4Audio(f.MimeType) && (f.HasWidth || f.HasHeight || !f.HasDuration):
		return VoiceFormatUnsupported
	case f.HasDuration && f.DurationMs > f.MaxDuration.Milliseconds():
		return VoiceTooLong
	}
	return VoiceOK
}

// TranscriptPending is files.transcript_status of a transcript that is being made.
const TranscriptPending = "pending"

// InvocationFinal reports whether a file is final for the agent invocation (12 sections 2 and
// 10, M3-09 design note 10): Final and its transcript not pending. Final and IsDeliverable keep
// their meaning for delivery.
//
// Example:
//
//	domain.InvocationFinal(domain.StateFacts{UploadState: "complete", ScanStatus: "clean",
//		TranscriptStatus: "pending"}) // false
func InvocationFinal(f StateFacts) bool {
	return Final(f.UploadState, f.ScanStatus, f.Deleted) && f.TranscriptStatus != TranscriptPending
}

// TranscriptFacts are the transcript columns of a files row (03 R-DM-48).
type TranscriptFacts struct {
	Status, Source, Language, Text string
}

// etagChars is the length of the derived ETag in base64url characters (the form of the ETag of
// getInstance, M1-46).
const etagChars = 22

// TranscriptETag is the derived strong ETag of a transcript (03 R-DM-48, 06 8 point 1; M3-09
// design note 8 d): the first 22 characters of the base64url SHA-256 over transcript_status,
// transcript_source, transcript_language and transcript_text, each followed by one zero byte, in
// quotes. An absent column counts as the empty string.
//
// Example:
//
//	e := domain.TranscriptETag(domain.TranscriptFacts{Status: "none"}) // "\"...22 chars...\""
func TranscriptETag(f TranscriptFacts) string {
	h := sha256.New()
	for _, s := range []string{f.Status, f.Source, f.Language, f.Text} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return `"` + base64.RawURLEncoding.EncodeToString(h.Sum(nil))[:etagChars] + `"`
}

// LanguageHint returns the language hint for the transcription service from a users.locale
// value (M3-09 design note 9 b): the primary subtag in lower case, "" when the locale is no
// valid tag.
//
// Example:
//
//	domain.LanguageHint("en-GB") // "en"
func LanguageHint(locale string) string {
	if !ValidLanguageTag(locale) {
		return ""
	}
	primary, _, _ := strings.Cut(locale, "-")
	return strings.ToLower(primary)
}

// ValidLanguageTag reports whether s is a BCP 47 tag of the form the transcript accepts (M3-09
// design note 8 e): a primary subtag of two or three letters, then subtags of two to eight
// letters or digits, separated by hyphens.
//
// Example:
//
//	domain.ValidLanguageTag("en-GB")   // true
//	domain.ValidLanguageTag("english") // false
func ValidLanguageTag(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts[0]) < 2 || len(parts[0]) > 3 || !allOf(parts[0], isLetter) {
		return false
	}
	for _, p := range parts[1:] {
		if len(p) < 2 || len(p) > 8 || !allOf(p, func(c byte) bool { return isLetter(c) || (c >= '0' && c <= '9') }) {
			return false
		}
	}
	return true
}

// allOf reports whether every byte of s passes ok.
func allOf(s string, ok func(byte) bool) bool {
	for i := range len(s) {
		if !ok(s[i]) {
			return false
		}
	}
	return true
}

// isLetter reports an ASCII letter.
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
