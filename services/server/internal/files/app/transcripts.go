// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the Service is the transcript port of runs and the HTTP adapter (M3-09).
var _ files.Transcripts = (*Service)(nil)

// VoiceConfig holds the configuration of voice messages and transcripts (docs/spec/12-files.md 8,
// 18), bound in internal/wire (wire.VoiceConfig).
type VoiceConfig struct {
	// MaxDuration is voice.max_duration, the longest voice message.
	MaxDuration time.Duration `config:"voice.max_duration"`
	// Mode is voice.transcription_mode: off, agent or service.
	Mode string `config:"voice.transcription_mode"`
	// MaxAttempts is voice.transcription_max_attempts, the attempts of files.transcribe before the
	// transcript is failed.
	MaxAttempts int `config:"voice.transcription_max_attempts"`
	// MaxContentBytes is message.max_content_bytes, the longest transcript in UTF-8 bytes.
	MaxContentBytes int `config:"message.max_content_bytes"`
}

// check refuses a configuration the voice use cases cannot work with.
func (c VoiceConfig) check() error {
	switch files.Mode(c.Mode) {
	case files.ModeOff, files.ModeAgent, files.ModeService:
	default:
		return errors.New("files: voice.transcription_mode must be off, agent or service")
	}
	if c.MaxDuration <= 0 || c.MaxAttempts < 1 || c.MaxContentBytes < 1 {
		return errors.New("files: voice.max_duration, voice.transcription_max_attempts and message.max_content_bytes must be positive")
	}
	return nil
}

// TranscribeQueue inserts the job files.transcribe (adapters/jobs implements it over
// jobs.Inserter.InsertTx, so the job commits with the unit of work that sets the transcript
// pending).
type TranscribeQueue interface {
	// EnqueueTranscribe inserts files.transcribe for fileID in the ambient unit of work.
	EnqueueTranscribe(ctx context.Context, fileID ids.ID) error
}

// Principals answers the kind and the locale of a principal (identity.Directory, bound in
// internal/wire): the kind decides the voice formats of an uploader (12 8.1), the locale the
// language hint of the transcription service (12 8.2).
type Principals interface {
	// Principal returns the kind (user, agent, app) of id and, for a person, users.locale ("" for
	// every other kind). Errors: not_found for an unknown id; others are the directory's.
	Principal(ctx context.Context, id ids.ID) (kind, locale string, err error)
}

// VoiceDeps are the collaborators of the voice use cases of the Service.
type VoiceDeps struct {
	// Switch answers voice.enabled per principal.
	Switch VoiceSwitch
	// Queue inserts files.transcribe.
	Queue TranscribeQueue
	// Principals gives the kind of an uploader.
	Principals Principals
}

// voice is the voice part of the Service; nil until EnableVoice.
type voice struct {
	cfg VoiceConfig
	d   VoiceDeps
}

// EnableVoice adds voice messages and transcripts to s (M3-09). It is called once while the
// process is built, before s serves requests. Without it a voice message is refused with an
// internal error and Mode answers ModeOff. Errors: a missing collaborator or a value VoiceConfig
// refuses.
func (s *Service) EnableVoice(cfg VoiceConfig, d VoiceDeps) error {
	if d.Switch == nil || d.Queue == nil || d.Principals == nil {
		return errors.New("files: EnableVoice needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return err
	}
	s.voice = &voice{cfg: cfg, d: d}
	return nil
}

// The reason of 409 conflict of requestTranscription and submitTranscript (12 8.2, M3-09 design
// notes 8 b, 8 c and 9 a).
const reasonTranscriptionUnavailable = "transcription_unavailable"

// The field error codes of voice messages and transcripts (M3-09 design notes 6 b and 8 e).
const (
	codeVoiceFormatUnsupported = "voice_format_unsupported"
	codeVoiceTooLong           = "voice_too_long"
	codeTooLong                = "too_long"
	pointerText                = "/text"
	pointerLanguage            = "/language"
	codePatternMismatch        = "pattern_mismatch"
)

// The values of files.transcript_status and transcript_source (03 4.7).
const (
	transcriptFailed  = "failed"
	sourceAgent       = "agent"
	sourceService     = "service"
	transcriptPending = domain.TranscriptPending
)

// errVoiceOff is the internal error of a voice use case before EnableVoice.
var errVoiceOff = errs.New(errs.CodeInternalError, "files: voice messages are not enabled in this process")

// Mode implements files.Transcripts (M3-09 design note 2): voice.transcription_mode, with
// service answered as ModeOff while voice.enabled is off for the subject. Without EnableVoice it
// is ModeOff.
func (s *Service) Mode(ctx context.Context, subjectID ids.ID, subjectKind string) (files.Mode, error) {
	if s.voice == nil {
		return files.ModeOff, nil
	}
	return s.voice.mode(ctx, subjectID, subjectKind)
}

// mode is Mode with the voice part bound.
func (v *voice) mode(ctx context.Context, subjectID ids.ID, subjectKind string) (files.Mode, error) {
	m := files.Mode(v.cfg.Mode)
	if m != files.ModeService {
		return m, nil
	}
	on, err := v.d.Switch.Enabled(ctx, subjectID, subjectKind)
	if err != nil || !on {
		return files.ModeOff, err
	}
	return files.ModeService, nil
}

// checkVoice is the voice check of Attach (M3-09 design note 6 b) on the one locked target f with
// the answers at "<pointer>/0": 409 file_not_ready, 409 file_blocked, 400 validation_failed with
// voice_format_unsupported or voice_too_long.
func (s *Service) checkVoice(f FileRecord, pointer, authorKind string) error {
	if s.voice == nil {
		return errVoiceOff
	}
	switch domain.VoiceCheck(voiceFacts(f, authorKind, s.voice.cfg.MaxDuration)) {
	case domain.VoiceNotReady:
		return s.notReady()
	case domain.VoiceBlocked:
		return errs.New(errs.CodeFileBlocked, "")
	case domain.VoiceFormatUnsupported:
		return fieldError(pointer+"/0", codeVoiceFormatUnsupported, "the file is not a supported voice recording")
	case domain.VoiceTooLong:
		return fieldError(pointer+"/0", codeVoiceTooLong, "the recording is longer than voice.max_duration")
	case domain.VoiceOK:
	}
	return nil
}

// startServiceTranscript sets the transcript of the attached voice file f pending and inserts
// files.transcribe when the effective mode of the author is service (M3-09 design note 6 c); the
// attachment row exists, so file.updated goes to the scope of the message. It reports whether it
// started the transcript.
func (s *Service) startServiceTranscript(ctx context.Context, authorID ids.ID, authorKind string, f FileRecord) (bool, error) {
	if f.TranscriptStatus != transcriptNone && f.TranscriptStatus != transcriptFailed {
		// requestTranscription started it before the send; a pending or done transcript stays.
		return false, nil
	}
	m, err := s.voice.mode(ctx, authorID, authorKind)
	if err != nil || m != files.ModeService {
		return false, err
	}
	return true, s.setPending(ctx, actorOf(ctx), f)
}

// setPending writes transcript_status pending for the locked file f, file.updated through
// recordFileChange (which publishes nothing: pending is not final for the invocation) and
// files.transcribe.
func (s *Service) setPending(ctx context.Context, a actor.Actor, f FileRecord) error {
	if err := s.d.Repo.SetTranscript(ctx, f.ID, TranscriptWrite{Status: transcriptPending}); err != nil {
		return err
	}
	after := f
	after.TranscriptStatus, after.TranscriptText, after.TranscriptLanguage, after.TranscriptSource = transcriptPending, "", "", ""
	if err := s.changes().recordFileChange(ctx, a, f, after); err != nil {
		return err
	}
	return s.voice.d.Queue.EnqueueTranscribe(ctx, f.ID)
}

// actorOf is the actor of the request in ctx; the zero actor without one.
func actorOf(ctx context.Context) actor.Actor {
	a, _ := actor.FromContext(ctx)
	return a
}

// notReady is 409 file_not_ready, with Retry-After when downloads are enabled (M3-05).
func (s *Service) notReady() error {
	e := errs.New(errs.CodeFileNotReady, "")
	if s.dl != nil {
		e = e.WithRetryAfter(s.dl.cfg.NotReadyRetryAfter)
	}
	return e
}

// voiceFacts returns the facts of the voice check of f for an author of authorKind.
func voiceFacts(f FileRecord, authorKind string, maxDuration time.Duration) domain.VoiceFacts {
	return domain.VoiceFacts{UploadState: f.UploadState, ScanStatus: f.ScanStatus, Deleted: f.Deleted, MimeType: f.MIMEType,
		AuthorKind: authorKind, HasWidth: f.Width > 0, HasHeight: f.Height > 0, DurationMs: f.DurationMS,
		HasDuration: f.DurationMS > 0, MaxDuration: maxDuration}
}

// Request implements files.Transcripts (requestTranscription; M3-09 design note 9 a) in one unit
// of work: the file and file.transcribe (404 for an unknown or invisible file), voice.enabled of
// the caller (403 feature_disabled), mode service (409 transcription_unavailable), the locked row:
// not deliverable gives 409 file_not_ready or file_blocked, not a voice format of its uploader 409
// transcription_unavailable; none and failed become pending with a new job, pending and done stay.
func (s *Service) Request(ctx context.Context, a actor.Actor, fileID ids.ID) (files.TranscriptView, error) {
	if s.voice == nil {
		return files.TranscriptView{}, errVoiceOff
	}
	var out files.TranscriptView
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.request(ctx, a, fileID)
		return err
	})
	return out, err
}

// request is the unit of work of Request.
func (s *Service) request(ctx context.Context, a actor.Actor, fileID ids.ID) (files.TranscriptView, error) {
	if err := s.authorizeFile(ctx, a, authorize.ActionFileTranscribe, fileID); err != nil {
		return files.TranscriptView{}, err
	}
	on, err := s.voice.d.Switch.Enabled(ctx, a.ID, string(a.Kind))
	if err != nil {
		return files.TranscriptView{}, err
	}
	if !on {
		return files.TranscriptView{}, errs.New(errs.CodeFeatureDisabled, "voice messages are switched off")
	}
	if files.Mode(s.voice.cfg.Mode) != files.ModeService {
		return files.TranscriptView{}, conflict(reasonTranscriptionUnavailable, "")
	}
	f, err := s.lockVoiceFile(ctx, fileID)
	if err != nil {
		return files.TranscriptView{}, err
	}
	if f.TranscriptStatus == transcriptNone || f.TranscriptStatus == transcriptFailed {
		if err := s.setPending(ctx, a, f); err != nil {
			return files.TranscriptView{}, err
		}
		f.TranscriptStatus, f.TranscriptText, f.TranscriptLanguage, f.TranscriptSource = transcriptPending, "", "", ""
	}
	return transcriptView(f), nil
}

// authorizeFile loads the file by id without deleted_at and its attachment row and decides act
// on its descriptor (fileResource): unknown, deleted and invisible files give the same 404. An
// anonymous caller gets the 401 of authorize before any read, so the answer tells nothing about
// the id (the rule of getFile and deleteFile, M3-08).
func (s *Service) authorizeFile(ctx context.Context, a actor.Actor, act authorize.Action, fileID ids.ID) error {
	if a.IsAnonymous() {
		return s.anonymous(ctx, a, act, fileID)
	}
	f, found, err := s.d.Repo.File(ctx, fileID)
	if err != nil {
		return err
	}
	if !found {
		return notFound()
	}
	ref, err := s.attachmentRef(ctx, fileID)
	if err != nil {
		return err
	}
	d, err := s.decide(ctx, a, act, fileResource(f, ref))
	if err != nil {
		return err
	}
	return d.Err()
}

// lockVoiceFile locks the files row after authorize and checks it as a voice file: gone or
// deleted meanwhile is 404, not deliverable 409 file_not_ready or file_blocked, not a voice
// format of its uploader 409 transcription_unavailable.
func (s *Service) lockVoiceFile(ctx context.Context, fileID ids.ID) (FileRecord, error) {
	f, found, err := s.d.Repo.LockVoiceFile(ctx, fileID)
	if err != nil {
		return FileRecord{}, err
	}
	if !found || f.Deleted {
		return FileRecord{}, notFound()
	}
	if !domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted) {
		if domain.Final(f.UploadState, f.ScanStatus, f.Deleted) {
			return FileRecord{}, errs.New(errs.CodeFileBlocked, "")
		}
		return FileRecord{}, s.notReady()
	}
	kind, _, err := s.voice.d.Principals.Principal(ctx, f.UploadedBy)
	if err != nil {
		return FileRecord{}, err
	}
	if !domain.IsVoiceFormat(f.MIMEType, kind) {
		return FileRecord{}, conflict(reasonTranscriptionUnavailable, "")
	}
	return f, nil
}

// Submit implements files.Transcripts (submitTranscript; M3-09 design note 8) in one unit of
// work: the body checks, the file and file.transcript.submit (404 for an unknown or invisible file,
// the denial of the requirement for a visible caller who is not the operator agent), mode agent
// (409 transcription_unavailable), the locked row as for Request, then If-Match against the
// derived ETag (428 without one, 412 version_conflict with current and ETag otherwise), then
// done with the text, the language and source agent through recordFileChange. No audit entry.
func (s *Service) Submit(ctx context.Context, a actor.Actor, cmd files.SubmitCmd) (files.TranscriptView, error) {
	if s.voice == nil {
		return files.TranscriptView{}, errVoiceOff
	}
	if err := s.checkSubmit(cmd); err != nil {
		return files.TranscriptView{}, err
	}
	var out files.TranscriptView
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.submit(ctx, a, cmd)
		return err
	})
	return out, err
}

// submit is the unit of work of Submit.
func (s *Service) submit(ctx context.Context, a actor.Actor, cmd files.SubmitCmd) (files.TranscriptView, error) {
	if err := s.authorizeFile(ctx, a, authorize.ActionFileTranscriptSubmit, cmd.FileID); err != nil {
		return files.TranscriptView{}, err
	}
	if files.Mode(s.voice.cfg.Mode) != files.ModeAgent {
		return files.TranscriptView{}, conflict(reasonTranscriptionUnavailable, "")
	}
	f, err := s.lockVoiceFile(ctx, cmd.FileID)
	if err != nil {
		return files.TranscriptView{}, err
	}
	if err := precondition(cmd.IfMatch, transcriptView(f)); err != nil {
		return files.TranscriptView{}, err
	}
	w := TranscriptWrite{Status: transcriptDone, Text: cmd.Text, Language: cmd.Language, Source: sourceAgent}
	if err := s.d.Repo.SetTranscript(ctx, f.ID, w); err != nil {
		return files.TranscriptView{}, err
	}
	after := f
	after.TranscriptStatus, after.TranscriptText, after.TranscriptLanguage, after.TranscriptSource = w.Status, w.Text, w.Language, w.Source
	if err := s.changes().recordFileChange(ctx, a, f, after); err != nil {
		return files.TranscriptView{}, err
	}
	return transcriptView(after), nil
}

// precondition compares the raw If-Match value with the derived ETag of cur (M3-09 design note
// 8 d): none or "*" is 428 precondition_required, any other value than the ETag 412
// version_conflict with the current transcript and its ETag.
func precondition(ifMatch string, cur files.TranscriptView) error {
	ifMatch = strings.TrimSpace(ifMatch)
	if ifMatch == "" || ifMatch == "*" {
		return errs.PreconditionRequired()
	}
	if ifMatch != cur.ETag {
		return errs.VersionConflict(transcriptBody(cur)).WithHeader("ETag", cur.ETag)
	}
	return nil
}

// checkSubmit checks the body of submitTranscript (M3-09 design note 8 e): text without U+0000,
// valid UTF-8 and at most message.max_content_bytes; language a valid tag when sent. The text is
// never echoed in the problem.
func (s *Service) checkSubmit(cmd files.SubmitCmd) error {
	if strings.ContainsRune(cmd.Text, 0) || !utf8.ValidString(cmd.Text) {
		return fieldError(pointerText, codeInvalid, "the transcript contains an invalid character")
	}
	if len(cmd.Text) > s.voice.cfg.MaxContentBytes {
		return fieldError(pointerText, codeTooLong, "the transcript is longer than message.max_content_bytes")
	}
	if cmd.Language != "" && !domain.ValidLanguageTag(cmd.Language) {
		return fieldError(pointerLanguage, codePatternMismatch, "the language is not a valid language tag")
	}
	return nil
}

// transcriptView is the Transcript view of f with its derived ETag: text, language and source
// only when done.
func transcriptView(f FileRecord) files.TranscriptView {
	out := files.TranscriptView{FileID: f.ID, Status: f.TranscriptStatus, ETag: transcriptETag(f)}
	if f.TranscriptStatus == transcriptDone {
		out.Text, out.Language, out.Source = f.TranscriptText, f.TranscriptLanguage, f.TranscriptSource
	}
	return out
}

// transcriptETag is the derived ETag of the transcript columns of f (R-DM-48).
func transcriptETag(f FileRecord) string {
	return domain.TranscriptETag(domain.TranscriptFacts{Status: f.TranscriptStatus, Source: f.TranscriptSource,
		Language: f.TranscriptLanguage, Text: f.TranscriptText})
}

// transcriptJSON is the member current of 412 version_conflict (schema Transcript); absent
// members are omitted, never null (ADR 0053).
type transcriptJSON struct {
	FileID   string  `json:"file_id"`
	Status   string  `json:"status"`
	Text     *string `json:"text,omitempty"`
	Language string  `json:"language,omitempty"`
	Source   string  `json:"source,omitempty"`
}

// transcriptBody returns the current transcript of 412; text is present exactly when done, also
// when empty (a silent recording).
func transcriptBody(v files.TranscriptView) transcriptJSON {
	out := transcriptJSON{FileID: v.FileID.String(), Status: v.Status, Language: v.Language, Source: v.Source}
	if v.Status == transcriptDone {
		text := v.Text
		out.Text = &text
	}
	return out
}
