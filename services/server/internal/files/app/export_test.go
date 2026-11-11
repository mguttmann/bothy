// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// ScheduleTextExtract exposes the trigger of the text extraction to the tests of app_test.
func ScheduleTextExtract(t *ExtractTrigger, ctx context.Context, f FileRecord) error {
	return t.schedule(ctx, f)
}

// VoiceFacts exposes the facts of the voice check to the tests of app_test.
func VoiceFacts(f FileRecord, authorKind string, maxDuration time.Duration) domain.VoiceFacts {
	return voiceFacts(f, authorKind, maxDuration)
}

// VoiceTranscribable exposes the format check of the transcription to the tests of app_test.
func VoiceTranscribable(f FileRecord, authorKind string) bool {
	return voiceTranscribable(f, authorKind)
}

// VoiceMIME exposes the refined type the voice checks and the transcriber read.
func VoiceMIME(f FileRecord) string { return voiceMIME(f) }

// ErrObjectLonger exposes the error of an object that is longer than the size of its row.
var ErrObjectLonger = errObjectLonger

// LimitObject exposes the reader between an object and a consumer that keeps it in memory.
func LimitObject(r io.Reader, limit int64, over error) io.Reader {
	return limitObject(r, limit, over)
}
