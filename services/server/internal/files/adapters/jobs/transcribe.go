// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindTranscribe is the kind of files.transcribe (02 12.3, M3-09).
const KindTranscribe = "files.transcribe"

// TranscribeArgs are the arguments of files.transcribe: only the file id (02 12.1 "Usage").
type TranscribeArgs struct {
	// FileID is the voice file whose transcript is pending.
	FileID ids.ID `json:"file_id"`
}

// Kind implements jobs.Args.
func (TranscribeArgs) Kind() string { return KindTranscribe }

// TranscribeRunner is the use case the handler runs (app.Transcription).
type TranscribeRunner interface {
	Run(ctx context.Context, fileID ids.ID, attempt int) error
}

// TranscribeHandler runs files.transcribe with the attempt number, so the use case writes failed
// on the last attempt (M3-09 design note 9 d). The span job files.transcribe comes from the
// handler wrapper of platform/jobs.
type TranscribeHandler struct {
	// Transcription is the use case; nil in a role that does not run jobs.
	Transcription TranscribeRunner
}

// errNoTranscription reports a handler without its use case (a role that never runs jobs).
var errNoTranscription = errors.New("files: files.transcribe has no transcription in this process")

// Work implements jobs.Handler.
func (h TranscribeHandler) Work(ctx context.Context, job jobs.Job[TranscribeArgs]) error {
	if h.Transcription == nil {
		return errNoTranscription
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return h.Transcription.Run(ctx, job.Args.FileID, job.Attempt)
}

// RegisterTranscribe adds files.transcribe on the queue files to reg (every role builds the same
// registry) with maxAttempts (voice.transcription_max_attempts) and River's default backoff. It
// has no River job timeout: every attempt is bounded by voice.transcription_timeout of the
// egress client, and River's default could cut the result unit of work that follows.
func RegisterTranscribe(reg *jobs.Registry, t TranscribeRunner, maxAttempts int) error {
	return jobs.Register[TranscribeArgs](reg, TranscribeHandler{Transcription: t},
		jobs.HandlerOptions{Queue: jobs.QueueFiles, MaxAttempts: maxAttempts, Timeout: -1})
}

// TranscribeQueue implements app.TranscribeQueue over jobs.Inserter.InsertTx; internal/wire binds
// the inserter once the job runtime exists (Bind). It shares the inserter of Queue.
type TranscribeQueue struct {
	// Q is the scan queue whose inserter it uses.
	Q *Queue
}

// Compile-time check: TranscribeQueue is the outgoing port of the use cases.
var _ app.TranscribeQueue = TranscribeQueue{}

// EnqueueTranscribe implements app.TranscribeQueue: files.transcribe for fileID in the ambient
// unit of work.
func (t TranscribeQueue) EnqueueTranscribe(ctx context.Context, fileID ids.ID) error {
	t.Q.mu.RLock()
	ins := t.Q.ins
	t.Q.mu.RUnlock()
	if ins == nil {
		return errs.New(errs.CodeInternalError, "files: the transcribe queue has no job runtime")
	}
	return ins.InsertTx(ctx, TranscribeArgs{FileID: fileID}, nil)
}
