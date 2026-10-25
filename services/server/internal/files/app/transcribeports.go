// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrTranscriptionPermanent marks a failure of the transcription service that a retry cannot
// heal (M3-09 design note 11 d): a blocked egress target, a response over
// egress.max_response_bytes, a redirect, a 4xx other than 408 and 429, a malformed or invalid
// body. The job writes transcript_status failed at once. Every other error is transient.
var ErrTranscriptionPermanent = errors.New("files: permanent transcription failure")

// TranscribeRequest is one call of Transcriber.Transcribe.
type TranscribeRequest struct {
	// FileID is the file; adapters log only this id (bothy.file.id).
	FileID ids.ID
	// MIMEType is the stored, detected type of the audio (audio/mp4 and the like, audio/mpeg).
	MIMEType string
	// Audio streams the object; the caller closes it after the call.
	Audio io.Reader
	// LanguageHint is the primary language subtag of the uploader's locale in lower case; "" for
	// none (domain.LanguageHint).
	LanguageHint string
}

// TranscribeResult is the answer of the transcription service.
type TranscribeResult struct {
	// Text is the transcript: valid UTF-8 without U+0000, at most message.max_content_bytes bytes.
	Text string
	// Language is the language the service reported; "" when absent. The job keeps it only when
	// domain.ValidLanguageTag accepts it.
	Language string
}

// Transcriber is the outgoing port of the server transcript (docs/spec/12-files.md 8.2, M3-09
// design note 11); adapters/transcription implements it with openai_compatible over the egress
// client. It is called outside any transaction (02 8.5).
type Transcriber interface {
	// Transcribe sends the audio to the configured service and returns its transcript. Errors:
	// an error wrapping ErrTranscriptionPermanent for a failure a retry cannot heal, any other
	// error for a transient one (timeout, connection, 408, 429, 5xx).
	Transcribe(ctx context.Context, req TranscribeRequest) (TranscribeResult, error)
}

// VoiceSwitch is the outgoing port of the switch voice.enabled (M1-47, M3-09 design notes 1 and
// 15): internal/wire binds it to instance.Flags, so files needs no edge to instance.
type VoiceSwitch interface {
	// Enabled reports whether voice.enabled is on for the principal of the given kind.
	Enabled(ctx context.Context, principalID ids.ID, principalKind string) (bool, error)
}
