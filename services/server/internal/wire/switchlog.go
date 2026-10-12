// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// switchLogger is the slog.Handler of a serve process. It forwards every record to the logger
// set last: the command logger (stdout JSON) before telemetry starts and after it shut down,
// the server logger of otel.Setup in between. Loggers derived with With or WithGroup keep
// following the switch, so the pool, the listeners and the runner hold one logger for the whole
// process life.
type switchLogger struct {
	cur *atomic.Pointer[slog.Logger]
	ops []func(slog.Handler) slog.Handler
}

func newSwitchLogger(l *slog.Logger) *switchLogger {
	p := &atomic.Pointer[slog.Logger]{}
	p.Store(l)
	return &switchLogger{cur: p}
}

// set makes l the target of every record from now on.
func (h *switchLogger) set(l *slog.Logger) { h.cur.Store(l) }

func (h *switchLogger) handler() slog.Handler {
	out := h.cur.Load().Handler()
	for _, op := range h.ops {
		out = op(out)
	}
	return out
}

// Enabled implements slog.Handler.
func (h *switchLogger) Enabled(ctx context.Context, l slog.Level) bool {
	return h.handler().Enabled(ctx, l)
}

// Handle implements slog.Handler.
func (h *switchLogger) Handle(ctx context.Context, r slog.Record) error {
	return h.handler().Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *switchLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.with(func(next slog.Handler) slog.Handler { return next.WithAttrs(attrs) })
}

// WithGroup implements slog.Handler.
func (h *switchLogger) WithGroup(name string) slog.Handler {
	return h.with(func(next slog.Handler) slog.Handler { return next.WithGroup(name) })
}

func (h *switchLogger) with(op func(slog.Handler) slog.Handler) *switchLogger {
	ops := make([]func(slog.Handler) slog.Handler, len(h.ops), len(h.ops)+1)
	copy(ops, h.ops)
	return &switchLogger{cur: h.cur, ops: append(ops, op)}
}
