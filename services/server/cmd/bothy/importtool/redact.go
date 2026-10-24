// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// The CDN fetcher of M8-02 names scheme, host and path of an attachment link in its error texts
// and log records (22 6.5 item 4), and Discord puts the file name of the attachment into that
// path. 22 10.2 rule 4 forbids file names in any output, so the run passes only the kind of a
// fetch error on and gives the fetcher a logger that cuts the path (docs/DECISIONS.md D-M8-04-10).

// cdnError is a fetch error that ends the run, reduced to its kind.
type cdnError struct{ kind error }

// Error returns "discord: attachment download: <kind>".
func (e *cdnError) Error() string { return "discord: attachment download: " + e.kind.Error() }

// Unwrap returns the kind.
func (e *cdnError) Unwrap() error { return e.kind }

// errFetchFailed is the kind of a fetch error without a known kind.
var errFetchFailed = errors.New("failed")

// redactFetch reduces a fetch error to its kind.
func redactFetch(err error) error {
	for _, k := range []error{discord.ErrRateWaitTooLong, discord.ErrRateLimited, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, k) {
			return &cdnError{kind: k}
		}
	}
	return &cdnError{kind: errFetchFailed}
}

// cdnLogKey is the attribute of the fetcher's records that holds method and link.
const cdnLogKey = "bothy.import.discord.op"

// cdnLog wraps the handler of the run's logger for the fetcher: the method and link attribute
// keeps only the method, scheme and host.
type cdnLog struct{ h slog.Handler }

// newCDNLogger returns the logger of the fetcher.
func newCDNLogger(l *slog.Logger) *slog.Logger { return slog.New(cdnLog{h: l.Handler()}) }

// Enabled reports the level of the wrapped handler.
func (c cdnLog) Enabled(ctx context.Context, l slog.Level) bool { return c.h.Enabled(ctx, l) }

// Handle rewrites the link attribute and passes the record on.
func (c cdnLog) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == cdnLogKey {
			a = slog.String(cdnLogKey, cutPath(a.Value.String()))
		}
		out.AddAttrs(a)
		return true
	})
	return c.h.Handle(ctx, out)
}

// WithAttrs wraps the handler with attributes.
func (c cdnLog) WithAttrs(as []slog.Attr) slog.Handler { return cdnLog{h: c.h.WithAttrs(as)} }

// WithGroup wraps the handler with a group.
func (c cdnLog) WithGroup(name string) slog.Handler { return cdnLog{h: c.h.WithGroup(name)} }

// cutPath keeps "METHOD scheme://host" of "METHOD scheme://host/path".
func cutPath(op string) string {
	i := strings.Index(op, "://")
	if i < 0 {
		return op
	}
	if j := strings.IndexByte(op[i+3:], '/'); j >= 0 {
		return op[:i+3+j]
	}
	return op
}
