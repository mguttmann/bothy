// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// ErrExportCut is wrapped by the error of Export after the first batch was written: the response
// has started, and the stream ends short (detectable through prev_hash).
var ErrExportCut = errors.New("audit: export cut short")

// Export implements audit.Log: authorize audit.read, require both bounds, record the allow entry
// with the range, then stream the rows in ascending seq in batches of api.page_size_max, each
// batch read on its own and handed to w at once (no buffering of the range).
//
// The stream is the contiguous seq span from the smallest to the largest seq whose occurred_at
// lies in [from, to): every line links the line before by prev_hash, so the export verifies
// offline and a cut is visible. A row inside the span whose occurred_at lies outside the range
// (an aggregated denial keeps the time of its first denial, 05 "2.4") is part of it
// (docs/DECISIONS.md M1-50).
func (l *Log) Export(ctx context.Context, from, to time.Time, w audit.EventWriter) error {
	if err := l.check(ctx, authorize.ActionAuditRead); err != nil {
		return err
	}
	if err := checkBounds(from, to); err != nil {
		return err
	}
	if err := l.record(ctx, authorize.ActionAuditRead, auditing.Attrs{attrFrom: auditing.Time(from), attrTo: auditing.Time(to)}); err != nil {
		return err
	}
	first, last, ok, err := l.d.Store.ExportSpan(ctx, from, to)
	if err != nil || !ok {
		return err
	}
	return l.stream(ctx, first, last, w)
}

// checkBounds requires both bounds of an export (400 validation_failed).
func checkBounds(from, to time.Time) error {
	var fields []errs.FieldError
	if from.IsZero() {
		fields = append(fields, errs.FieldError{Pointer: "#query/from", Code: "required", Detail: "The start of the range is required."})
	}
	if to.IsZero() {
		fields = append(fields, errs.FieldError{Pointer: "#query/to", Code: "required", Detail: "The end of the range is required."})
	}
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return nil
}

// stream hands the rows first .. last to w in batches of api.page_size_max. An error after the
// first batch was handed over is reported and wraps ErrExportCut.
func (l *Log) stream(ctx context.Context, first, last int64, w audit.EventWriter) error {
	started := false
	for next := first; next <= last; {
		end := min(next+int64(l.cfg.PageSizeMax)-1, last)
		err := l.writeBatch(ctx, next, end, w, &started)
		if err != nil && started {
			l.d.Reporter.ExportCut(ctx, err)
			return errors.Join(ErrExportCut, err)
		}
		if err != nil {
			return err
		}
		next = end + 1
	}
	return nil
}

// writeBatch reads the rows from .. to and hands them to w; started is set once w was called.
func (l *Log) writeBatch(ctx context.Context, from, to int64, w audit.EventWriter, started *bool) error {
	rows, err := l.d.Store.Rows(ctx, from, to)
	if err != nil || len(rows) == 0 {
		return err
	}
	batch := make([]audit.Event, len(rows))
	for i, r := range rows {
		batch[i] = event(r)
	}
	*started = true
	return w.WriteEvents(ctx, batch)
}
