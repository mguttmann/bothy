// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// riverJobKindKey is the attribute under which River names the kind of a job in its log lines.
const riverJobKindKey = "job_kind"

// riverLogHandler is the handler of River's logger: River writes its routine lines (producer
// counts, maintenance runs, a job that errored and is retried) at Info, so they go to the inner
// handler at Debug, and Warn and Error stay as they are. River's job_kind becomes bothy.job.kind;
// every other attribute of River (job IDs, queue names, error texts) is dropped here, as the
// server logger would drop it (docs/observability/signals.md "Logs"). The wrapper of every
// handler logs a failed attempt itself with its error class (worker.logFailed).
type riverLogHandler struct {
	inner slog.Handler
}

// newRiverLogger returns the logger River gets: log with riverLogHandler.
func newRiverLogger(log *slog.Logger) *slog.Logger {
	return slog.New(riverLogHandler{inner: log.Handler()})
}

// riverLevel lowers Info to Debug.
func riverLevel(l slog.Level) slog.Level {
	if l == slog.LevelInfo {
		return slog.LevelDebug
	}
	return l
}

// Enabled reports whether the inner handler takes a line of River at level l.
func (h riverLogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, riverLevel(l))
}

// Handle passes r on at its lowered level with bothy.job.kind as its only attribute.
func (h riverLogHandler) Handle(ctx context.Context, r slog.Record) error {
	level := riverLevel(r.Level)
	if !h.inner.Enabled(ctx, level) {
		return nil
	}
	out := slog.NewRecord(r.Time, level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == riverJobKindKey {
			out.AddAttrs(slog.String(attr.JobKind, a.Value.String()))
		}
		return true
	})
	return h.inner.Handle(ctx, out)
}

// WithAttrs drops River's attributes like Handle does.
func (h riverLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup keeps the handler: River's groups carry no catalogue attribute.
func (h riverLogHandler) WithGroup(string) slog.Handler { return h }
