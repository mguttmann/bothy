// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Keys of the trace fields the stdout JSON handler adds (signals.md "Logs"). They are no
// attributes of the catalog: the OTLP log record carries trace and span id as native fields.
const (
	keyTraceID = "trace_id"
	keySpanID  = "span_id"
)

// msgServerError is the constant message of ServerErrorLog.
const msgServerError = "http server error"

// ErrUnknownLevel reports a log level outside debug, info, warn and error.
var ErrUnknownLevel = errors.New("otel: unknown log level")

// ParseLevel maps the values of telemetry.logs.level (debug, info, warn, error) to a slog.Level.
// The configuration schema already restricts the key to these values; ParseLevel checks again
// so a caller outside the schema gets an error instead of a silent default.
//
// Errors: ErrUnknownLevel for every other text, also for other spellings such as "INFO".
func ParseLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("%w %q (want debug, info, warn or error)", ErrUnknownLevel, s)
}

// NewCommandLogger returns the logger of short-lived commands and of the time before the
// configuration is loaded: JSON lines to w at level and above, the same attribute allowlist as
// the server logger (attr.Allowed, groups flattened to dotted keys) and trace_id, span_id and
// bothy.correlation_id when the context carries a span. It never exports over OTLP.
//
// Example:
//
//	logger := otel.NewCommandLogger(os.Stderr, slog.LevelInfo)
//	logger.InfoContext(ctx, "secrets initialized", slog.String(attr.Outcome, "ok"))
func NewCommandLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(newLevelHandler(level, newAllowlistHandler(newStdoutHandler(w))))
}

// newStdoutHandler is the JSON handler of the stdout path with the trace fields of the context.
func newStdoutHandler(w io.Writer) slog.Handler {
	// The level filter sits in front of the chain; the JSON handler itself passes everything.
	json := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug - 4})
	return newContextFieldsHandler(json, true)
}

// levelHandler drops records below its level before any other handler works on them.
type levelHandler struct {
	level slog.Leveler
	next  slog.Handler
}

func newLevelHandler(level slog.Leveler, next slog.Handler) slog.Handler {
	return &levelHandler{level: level, next: next}
}

func (h *levelHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= h.level.Level() && h.next.Enabled(ctx, l)
}

func (h *levelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.next.Handle(ctx, r)
}

func (h *levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelHandler{level: h.level, next: h.next.WithAttrs(attrs)}
}

func (h *levelHandler) WithGroup(name string) slog.Handler {
	return &levelHandler{level: h.level, next: h.next.WithGroup(name)}
}

// allowlistHandler keeps only attributes whose flattened key is attr.Allowed. It resolves
// LogValuer values first and flattens groups (slog.Group, WithGroup) into dotted keys, so a key
// can only reach an output in the form the catalog and the collector allowlist name it, and no
// group can smuggle in a key the catalog does not list. It never drops a record. The handlers
// behind it never see a group.
type allowlistHandler struct {
	next   slog.Handler
	prefix string // the open groups of WithGroup, each followed by a dot
}

func newAllowlistHandler(next slog.Handler) slog.Handler {
	return &allowlistHandler{next: next}
}

func (h *allowlistHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *allowlistHandler) Handle(ctx context.Context, r slog.Record) error {
	kept := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		kept = appendAllowed(kept, h.prefix, a)
		return true
	})
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	out.AddAttrs(kept...)
	return h.next.Handle(ctx, out)
}

func (h *allowlistHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var kept []slog.Attr
	for _, a := range attrs {
		kept = appendAllowed(kept, h.prefix, a)
	}
	if len(kept) == 0 {
		return h
	}
	return &allowlistHandler{next: h.next.WithAttrs(kept), prefix: h.prefix}
}

func (h *allowlistHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &allowlistHandler{next: h.next, prefix: h.prefix + name + "."}
}

// appendAllowed appends a, flattened below prefix, to dst when its key is allowed.
func appendAllowed(dst []slog.Attr, prefix string, a slog.Attr) []slog.Attr {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		p := prefix
		if a.Key != "" {
			p = prefix + a.Key + "."
		}
		for _, m := range v.Group() {
			dst = appendAllowed(dst, p, m)
		}
		return dst
	}
	if a.Key == "" {
		return dst
	}
	key := prefix + a.Key
	if !attr.Allowed(key) || !scalar(v) {
		return dst
	}
	return append(dst, slog.Attr{Key: key, Value: v}) //nolint:sloglint // key passed attr.Allowed above, so it is a catalog key
}

// scalar reports whether v is a value a catalog attribute may carry: a string, number, bool,
// duration, time or a list of strings (gen_ai.response.finish_reasons). Any other value of kind
// Any (an error, a struct, a map) is dropped even under an allowed key, because a JSON or OTLP
// encoder would write its text, and that text is what signals.md "Logs" keeps out (an error is
// error.type with a class or code, never its message).
func scalar(v slog.Value) bool {
	if v.Kind() != slog.KindAny {
		return true
	}
	_, ok := v.Any().([]string)
	return ok
}

// contextFieldsHandler adds the trace fields of the span in the context: bothy.correlation_id
// always, trace_id and span_id when ids is set (the stdout JSON path; the OTLP record carries
// them natively).
type contextFieldsHandler struct {
	next slog.Handler
	ids  bool
}

func newContextFieldsHandler(next slog.Handler, ids bool) slog.Handler {
	return &contextFieldsHandler{next: next, ids: ids}
}

func (h *contextFieldsHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *contextFieldsHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		if h.ids {
			r.AddAttrs(slog.String(keyTraceID, sc.TraceID().String()))
			if sc.HasSpanID() {
				r.AddAttrs(slog.String(keySpanID, sc.SpanID().String()))
			}
		}
		r.AddAttrs(slog.String(attr.CorrelationID, uuidText(sc.TraceID())))
	}
	return h.next.Handle(ctx, r)
}

func (h *contextFieldsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextFieldsHandler{next: h.next.WithAttrs(attrs), ids: h.ids}
}

func (h *contextFieldsHandler) WithGroup(name string) slog.Handler {
	return &contextFieldsHandler{next: h.next.WithGroup(name), ids: h.ids}
}

// fanoutHandler hands every record to each of its handlers (stdout JSON and the OTLP bridge).
type fanoutHandler []slog.Handler

func newFanout(handlers ...slog.Handler) slog.Handler {
	if len(handlers) == 1 {
		return handlers[0]
	}
	return fanoutHandler(handlers)
}

func (f fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (f fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(fanoutHandler, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (f fanoutHandler) WithGroup(name string) slog.Handler {
	out := make(fanoutHandler, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}

// CorrelationID returns the trace id of the span in ctx as UUID text (16 bytes, 8-4-4-4-12
// lowercase hex; signals.md "Weitergabe" item 3). Every request through Middleware has one, also
// with telemetry.enabled = false and for unsampled traces. It is empty only when ctx carries no
// span context.
func CorrelationID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return uuidText(sc.TraceID())
}

// uuidText writes the 16 bytes of a trace id in the canonical UUID form.
func uuidText(id trace.TraceID) string {
	var buf [36]byte
	hex.Encode(buf[0:8], id[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], id[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], id[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], id[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], id[10:16])
	return string(buf[:])
}

// ServerErrorLog returns the logger for http.Server.ErrorLog. The lines net/http writes there
// carry client addresses and foreign error texts (19-sicherheit.md SEC-LOG-5), so every line
// becomes the constant message "http server error" at warn with error.type from a closed set:
// tls_handshake, accept, panic or other. The original text is discarded.
//
// Example:
//
//	srv := &http.Server{Handler: h, ErrorLog: otel.ServerErrorLog(tel.Logger())}
func ServerErrorLog(l *slog.Logger) *log.Logger {
	return log.New(serverErrorWriter{logger: l}, "", 0)
}

type serverErrorWriter struct {
	logger *slog.Logger
}

func (w serverErrorWriter) Write(p []byte) (int, error) {
	// The writer has no request context: net/http logs these lines outside any handler.
	w.logger.LogAttrs(context.Background(), slog.LevelWarn, msgServerError,
		slog.String(attr.ErrorType, serverErrorClass(string(p))))
	return len(p), nil
}

// serverErrorClass maps a line of net/http to its class by the constant prefixes net/http uses.
func serverErrorClass(line string) string {
	switch {
	case strings.Contains(line, "TLS handshake error"):
		return "tls_handshake"
	case strings.HasPrefix(line, "http: Accept error"):
		return "accept"
	case strings.HasPrefix(line, "http: panic serving"):
		return "panic"
	}
	return "other"
}
