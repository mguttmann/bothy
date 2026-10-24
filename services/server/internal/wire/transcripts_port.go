// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"sync/atomic"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// LazyTranscripts is the port files.Transcripts for runs (M3-09), bound to the files module once
// its start step has run, for the same start order reason as LazyAttachments. Until Bind every
// method fails with an internal error; only a voice message calls it. It is safe for concurrent
// use.
type LazyTranscripts struct {
	p atomic.Pointer[files.Transcripts]
}

// Compile-time check: LazyTranscripts is the port.
var _ files.Transcripts = (*LazyTranscripts)(nil)

// Bind sets the transcripts of the files module; a second call replaces the first.
func (l *LazyTranscripts) Bind(t files.Transcripts) { l.p.Store(&t) }

// get returns the bound port or the internal error of an unbound one.
func (l *LazyTranscripts) get() (files.Transcripts, error) {
	if p := l.p.Load(); p != nil && *p != nil {
		return *p, nil
	}
	return nil, errs.New(errs.CodeInternalError, "wire: the transcripts of the files module are not bound yet")
}

// Mode implements files.Transcripts.
func (l *LazyTranscripts) Mode(ctx context.Context, subjectID ids.ID, subjectKind string) (files.Mode, error) {
	t, err := l.get()
	if err != nil {
		return files.ModeOff, err
	}
	return t.Mode(ctx, subjectID, subjectKind)
}

// Request implements files.Transcripts.
func (l *LazyTranscripts) Request(ctx context.Context, a actor.Actor, fileID ids.ID) (files.TranscriptView, error) {
	t, err := l.get()
	if err != nil {
		return files.TranscriptView{}, err
	}
	return t.Request(ctx, a, fileID)
}

// Submit implements files.Transcripts.
func (l *LazyTranscripts) Submit(ctx context.Context, a actor.Actor, cmd files.SubmitCmd) (files.TranscriptView, error) {
	t, err := l.get()
	if err != nil {
		return files.TranscriptView{}, err
	}
	return t.Submit(ctx, a, cmd)
}
