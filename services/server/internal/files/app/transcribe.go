// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TranscribeDeps are the collaborators of the job files.transcribe (M3-09 design note 9).
type TranscribeDeps struct {
	// Repo reads and locks the files row and writes the transcript.
	Repo Repository
	// UoW runs the result write.
	UoW txn.UnitOfWork
	// Store opens the audio.
	Store ObjectStore
	// Transcriber is the adapter openai_compatible; nil outside mode service, where a pending
	// transcript of an earlier mode service ends failed.
	Transcriber Transcriber
	// Switch answers voice.enabled for the uploader.
	Switch VoiceSwitch
	// Principals gives the kind and locale of the uploader.
	Principals Principals
	// Events, Channels and Purge are the collaborators of recordFileChange.
	Events   eventlog.Appender
	Channels ChannelLocator
	Purge    PurgeQueue
	// Log receives one Warn line per failed attempt with bothy.file.id; nil means slog.Default().
	Log *slog.Logger
}

// Transcription is the use case of the job files.transcribe (12 8.2, 02 12.3, 02 14.4 row
// "transcription service"). It is safe for concurrent use.
type Transcription struct {
	cfg VoiceConfig
	d   TranscribeDeps
	v   *voice
}

// NewTranscription returns the use case. Errors: a missing collaborator or a value VoiceConfig
// refuses.
func NewTranscription(cfg VoiceConfig, d TranscribeDeps) (*Transcription, error) {
	if d.Repo == nil || d.UoW == nil || d.Store == nil || d.Switch == nil || d.Principals == nil ||
		d.Events == nil || d.Channels == nil || d.Purge == nil {
		return nil, errors.New("files: NewTranscription needs every collaborator")
	}
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Transcription{cfg: cfg, d: d, v: &voice{cfg: cfg, d: VoiceDeps{Switch: d.Switch, Principals: d.Principals}}}, nil
}

// The Info and Warn lines of the job; they carry only bothy.file.id (12 14).
const (
	msgTranscribeFailed = "files: transcription failed; the job retries or the transcript becomes failed"
	logKeyAttemptClass  = "error.type"
	classTransient      = "transient"
	classPermanent      = "permanent"
)

// Run is one attempt of files.transcribe for fileID (M3-09 design note 9 b to e); attempt counts
// from 1. The row is read outside a transaction: gone, deleted or not pending returns nil (02
// 12.4 rule 1). A mode that is no longer service for the uploader, or a file that is not
// deliverable, writes failed. Otherwise the audio goes to the Transcriber outside any
// transaction; a success writes done, ErrTranscriptionPermanent writes failed, any other error is
// returned so River retries, and on the last attempt (voice.transcription_max_attempts) it writes
// failed instead. A database error never writes failed.
func (t *Transcription) Run(ctx context.Context, fileID ids.ID, attempt int) error {
	f, found, err := t.d.Repo.TranscriptFile(ctx, fileID)
	if err != nil || !found || f.Deleted || f.TranscriptStatus != transcriptPending {
		return err
	}
	kind, locale, err := t.d.Principals.Principal(ctx, f.UploadedBy)
	if err != nil {
		return err
	}
	mode, err := t.v.mode(ctx, f.UploadedBy, kind)
	if err != nil {
		return err
	}
	if mode != files.ModeService || t.d.Transcriber == nil || !domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted) {
		return t.finish(ctx, f, TranscriptWrite{Status: transcriptFailed})
	}
	hint := domain.LanguageHint(locale)
	res, err := t.transcribe(ctx, f, hint)
	switch {
	case err == nil:
		lang := res.Language
		if !domain.ValidLanguageTag(lang) {
			lang = hint
		}
		return t.finish(ctx, f, TranscriptWrite{Status: transcriptDone, Text: res.Text, Language: lang, Source: sourceService})
	case errors.Is(err, ErrTranscriptionPermanent):
		t.warn(ctx, f.ID, classPermanent)
		return t.finish(ctx, f, TranscriptWrite{Status: transcriptFailed})
	case ctx.Err() == nil && attempt >= t.cfg.MaxAttempts:
		t.warn(ctx, f.ID, classTransient)
		return t.finish(ctx, f, TranscriptWrite{Status: transcriptFailed})
	default:
		t.warn(ctx, f.ID, classTransient)
		return err
	}
}

// transcribe opens the object and calls the Transcriber outside any transaction (02 8.5).
func (t *Transcription) transcribe(ctx context.Context, f FileRecord, hint string) (TranscribeResult, error) {
	body, err := t.d.Store.Get(ctx, f.ObjectKey, nil)
	if err != nil {
		return TranscribeResult{}, err
	}
	defer func() { _ = body.Close() }()
	return t.d.Transcriber.Transcribe(ctx, TranscribeRequest{FileID: f.ID, MIMEType: voiceMIME(f), Audio: body, LanguageHint: hint})
}

// warn logs one failed attempt with the file id and its class only, never text, URL or key.
func (t *Transcription) warn(ctx context.Context, fileID ids.ID, class string) {
	t.d.Log.LogAttrs(ctx, slog.LevelWarn, msgTranscribeFailed, slog.String(logKeyFileID, fileID.String()),
		slog.String(logKeyAttemptClass, class))
}

// finish is the result unit of work (design notes 9 c and e): lock the row, require pending, not
// deleted and the object read, then write w through recordFileChange, which publishes
// files.FileFinalized for done and failed on a final file, so a held invocation is released in
// the same transaction. Anything else writes nothing.
func (t *Transcription) finish(ctx context.Context, read FileRecord, w TranscriptWrite) error {
	return t.d.UoW.Do(ctx, func(ctx context.Context) error {
		cur, found, err := t.d.Repo.LockTranscriptFile(ctx, read.ID)
		if err != nil || !found || cur.Deleted || cur.TranscriptStatus != transcriptPending || cur.ObjectKey != read.ObjectKey {
			return err
		}
		if err := t.d.Repo.SetTranscript(ctx, cur.ID, w); err != nil {
			return err
		}
		after := cur
		after.TranscriptStatus, after.TranscriptText, after.TranscriptLanguage, after.TranscriptSource = w.Status, w.Text, w.Language, w.Source
		ch := fileChanges{attachments: t.d.Repo, channels: t.d.Channels, events: t.d.Events, purge: t.d.Purge}
		return ch.recordFileChange(ctx, actor.Actor{}, cur, after)
	})
}
