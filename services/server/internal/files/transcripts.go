// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Mode is the effective transcription mode of a subject (docs/spec/12-files.md section 8.2): the
// value of voice.transcription_mode, except that service counts as off while the switch
// voice.enabled is off for the subject (M3-09 design note 2).
type Mode string

// The transcription modes (12 8.2, R-FI-10).
const (
	// ModeOff: no transcript; a triggering voice message has no transcription path.
	ModeOff Mode = "off"
	// ModeAgent: the operator agent of the thread transcribes and calls submitTranscript.
	ModeAgent Mode = "agent"
	// ModeService: the worker sends the audio to the configured transcription service.
	ModeService Mode = "service"
)

// Transcripts answers the transcription mode and stores transcripts (docs/spec/12-files.md 8.2,
// 09 section 13, M3-09). runs reads Mode at the acceptance of a voice message; the HTTP adapter
// calls Request (requestTranscription) and Submit (submitTranscript). Request and Submit authorize
// themselves (file.transcribe, file.transcript.submit) and run their own unit of work; Mode only
// reads. Errors are kernel/errs errors.
type Transcripts interface {
	// Mode returns the effective mode for the subject (the author of a voice message, the caller
	// of requestTranscription, the uploader in the job): voice.transcription_mode, with service
	// reported as ModeOff while voice.enabled is off for the subject. ModeAgent and ModeOff do
	// not depend on the switch.
	//
	// Example:
	//
	//	m, err := tr.Mode(ctx, alex, "user") // files.ModeAgent with the default configuration
	Mode(ctx context.Context, subjectID ids.ID, subjectKind string) (Mode, error)
	// Request starts the server transcript of a voice file (file.transcribe, 12 8.2): 404 for an
	// unknown or invisible file, 403 feature_disabled with voice.enabled off for the caller, 409
	// conflict transcription_unavailable outside mode service or for a file that is not a voice
	// format, 409 file_not_ready or file_blocked for a file that is not deliverable. none and
	// failed become pending with a new job files.transcribe; pending and done stay.
	Request(ctx context.Context, a actor.Actor, fileID ids.ID) (TranscriptView, error)
	// Submit stores the transcript of the operator agent (file.transcript.submit, mode agent;
	// 09 4.7) under the precondition cmd.IfMatch, the derived ETag of the transcript (R-DM-48).
	// It answers the new view with the new ETag.
	Submit(ctx context.Context, a actor.Actor, cmd SubmitCmd) (TranscriptView, error)
}

// SubmitCmd is the input of Transcripts.Submit (schema TranscriptSubmit with the If-Match header).
type SubmitCmd struct {
	// FileID is the file of the path.
	FileID ids.ID
	// IfMatch is the raw If-Match header value; Text and Language the body (Language "" when
	// absent).
	IfMatch, Text, Language string
}

// PendingReason tells why the attachments of a message are not final for the agent invocation
// (docs/spec/12-files.md sections 2 and 10, M3-09 design note 3).
type PendingReason uint8

// The pending reasons.
const (
	// PendingNone: every attachment is final for the invocation (or the message has none).
	PendingNone PendingReason = iota
	// PendingFiles: an attachment is not final in the sense of domain.Final (scan, container).
	PendingFiles
	// PendingTranscript: every attachment is final, but one has transcript_status pending.
	PendingTranscript
)
