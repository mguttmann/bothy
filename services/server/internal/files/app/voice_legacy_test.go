// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// TestVoiceFacts_LegacyGenericBrandRow (FIX-voice-send, red team): a recording the scan stored as
// video/mp4 before the fix, declared audio/mp4, complete and clean with a duration and without a
// width or height, passes the voice check, so "Send again" of the refused message works. The
// same row with a width (a video track) still fails, and a row declared as video stays video.
func TestVoiceFacts_LegacyGenericBrandRow(t *testing.T) {
	t.Parallel()
	legacy := app.FileRecord{MIMEType: "video/mp4", DeclaredMIMEType: "audio/mp4", UploadState: "complete",
		ScanStatus: "clean", DurationMS: 16695}
	if v := domain.VoiceCheck(app.VoiceFacts(legacy, "user", 5*time.Minute)); v != domain.VoiceOK {
		t.Fatalf("legacy row: verdict %v, want VoiceOK", v)
	}
	withVideo := legacy
	withVideo.Width, withVideo.Height = 640, 480
	if v := domain.VoiceCheck(app.VoiceFacts(withVideo, "user", 5*time.Minute)); v != domain.VoiceFormatUnsupported {
		t.Fatalf("legacy row with a video track: verdict %v, want VoiceFormatUnsupported", v)
	}
	declaredVideo := legacy
	declaredVideo.DeclaredMIMEType = "video/mp4"
	if v := domain.VoiceCheck(app.VoiceFacts(declaredVideo, "user", 5*time.Minute)); v != domain.VoiceFormatUnsupported {
		t.Fatalf("row declared as video: verdict %v, want VoiceFormatUnsupported", v)
	}
}

// TestVoiceTranscribable_LegacyAndVideoTrack (FIX-voice-send, review): the transcription accepts
// the legacy row and sends the refined type to the transcriber, and refuses a declared-audio file
// with a video track as the voice check does (12 8.1 rule 4).
func TestVoiceTranscribable_LegacyAndVideoTrack(t *testing.T) {
	t.Parallel()
	legacy := app.FileRecord{MIMEType: "video/mp4", DeclaredMIMEType: "audio/mp4", DurationMS: 16695}
	if !app.VoiceTranscribable(legacy, "user") {
		t.Fatal("legacy row: not transcribable")
	}
	if got := app.VoiceMIME(legacy); got != "audio/mp4" {
		t.Fatalf("legacy row: transcriber type %q, want audio/mp4", got)
	}
	for _, f := range []app.FileRecord{
		{MIMEType: "video/mp4", DeclaredMIMEType: "audio/mp4", Width: 640, Height: 480},
		{MIMEType: "audio/mp4", DeclaredMIMEType: "audio/mp4", Width: 640},
		{MIMEType: "audio/mp4", DeclaredMIMEType: "audio/mp4", Height: 480},
	} {
		if app.VoiceTranscribable(f, "user") {
			t.Fatalf("%+v with a video track: transcribable", f)
		}
	}
}
