// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ValidateExportRange checks the half-open range [from, to) of exportChannel without any lookup,
// so the answer cannot reveal whether the channel exists (brief M3-11 note 3c, D-M3-11-1). There
// is no upper bound on the length: http.transfer_timeout bounds one response and clients split
// long ranges.
//
// Errors: validation_failed with one field error, pointer #query/to and code invalid, when from
// is not strictly before to.
//
// Example:
//
//	if err := domain.ValidateExportRange(from, to); err != nil {
//		return nil, err // 400 validation_failed
//	}
func ValidateExportRange(from, to time.Time) error {
	if from.Before(to) {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
		Pointer: "#query/to", Code: FieldInvalid, Detail: "The end of the range must be after its start.",
	})
}

// ExportCursor is the keyset position of a channel export: the thread being read and the last
// thread_seq emitted from it (0 before its first message). Threads are ordered by their UUIDv7 id
// and messages by thread_seq, so the pair is a total, resumable order (brief M3-11 note 5,
// D-M3-11-3).
type ExportCursor struct {
	ThreadID  ids.ID
	ThreadSeq int64
}

// ExportRead is what an ExportWalk needs read next.
type ExportRead int

// The steps of an ExportWalk.
const (
	// ExportReadThreads asks for the next page of thread ids after ExportWalk.AfterThread.
	ExportReadThreads ExportRead = iota + 1
	// ExportReadMessages asks for the next page of messages of ExportWalk.Cursor.
	ExportReadMessages
	// ExportReadDone ends the walk: every message of the selection was emitted.
	ExportReadDone
)

// ExportWalk is the pure step logic of the export over thread pages and message pages: the caller
// asks Next what to read, reads one page of at most the page size and hands it back with Threads
// or Messages. Every message of the selection is emitted exactly once and in order, also when
// messages are added while the walk runs: a message added to the thread being read or to a later
// thread is emitted when it lies in the range; one added to a finished thread is not. A walk is
// not safe for concurrent use.
//
// Example:
//
//	w := domain.NewExportWalk(pageSize)
//	for step := w.Next(); step != domain.ExportReadDone; step = w.Next() {
//		if step == domain.ExportReadThreads {
//			w.Threads(readThreads(w.AfterThread()))
//		} else {
//			w.Messages(emit(readMessages(w.Cursor())))
//		}
//	}
type ExportWalk struct {
	limit int
	cur   ExportCursor
	// reading reports that cur.ThreadID may have further messages.
	reading bool
	// queue are the read thread ids after cur.ThreadID, ascending.
	queue []ids.ID
	// after is the largest thread id read so far.
	after ids.ID
	// threadsDone reports that the last thread page was short, so no thread follows.
	threadsDone bool
}

// NewExportWalk returns a walk at the start of the selection with the page size limit (at least
// 1; api.page_size_max in the use case).
func NewExportWalk(limit int) *ExportWalk {
	return &ExportWalk{limit: max(limit, 1)}
}

// ResumeExportWalk returns a walk that continues after c: the messages of c.ThreadID above
// c.ThreadSeq, then the threads after it.
func ResumeExportWalk(limit int, c ExportCursor) *ExportWalk {
	return &ExportWalk{limit: max(limit, 1), cur: c, reading: true, after: c.ThreadID}
}

// Next returns what to read next. It moves to the next queued thread when the current one is
// finished.
func (w *ExportWalk) Next() ExportRead {
	if w.reading {
		return ExportReadMessages
	}
	if len(w.queue) > 0 {
		w.cur = ExportCursor{ThreadID: w.queue[0]}
		w.queue = w.queue[1:]
		w.reading = true
		return ExportReadMessages
	}
	if !w.threadsDone {
		return ExportReadThreads
	}
	return ExportReadDone
}

// AfterThread is the bound of the next thread page: thread ids greater than it (the zero ID at
// the start, below every UUIDv7).
func (w *ExportWalk) AfterThread() ids.ID { return w.after }

// Cursor is the position of the next message page: messages of Cursor().ThreadID with
// thread_seq greater than Cursor().ThreadSeq.
func (w *ExportWalk) Cursor() ExportCursor { return w.cur }

// Threads takes one page of thread ids read after AfterThread, ascending. Ids not above
// AfterThread are ignored, so a page can never repeat a thread; a page shorter than the page size
// ends the thread pages.
func (w *ExportWalk) Threads(page []ids.ID) {
	for _, id := range page {
		if bytes.Compare(id[:], w.after[:]) > 0 {
			w.queue = append(w.queue, id)
			w.after = id
		}
	}
	if len(page) < w.limit {
		w.threadsDone = true
	}
}

// Messages takes the thread_seq values of one message page of Cursor, ascending, after the caller
// emitted them; a page shorter than the page size finishes the thread.
func (w *ExportWalk) Messages(seqs []int64) {
	if n := len(seqs); n > 0 && seqs[n-1] > w.cur.ThreadSeq {
		w.cur.ThreadSeq = seqs[n-1]
	}
	if len(seqs) < w.limit {
		w.reading = false
	}
}
