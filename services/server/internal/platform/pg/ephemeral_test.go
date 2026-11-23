// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// syncBuffer is a bytes.Buffer safe for the listener's goroutine and the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// nopRoute is an EphemeralRoute that does nothing.
type nopRoute struct{}

func (nopRoute) HandleEphemeral(context.Context, []byte) {}
func (nopRoute) Reconcile(context.Context)               {}

// TestEphemeralListener_LostLineNamesTheClass: a connection that fails logs "ephemeral listener
// lost" with the class of the error that the pool's own log uses (the SQLSTATE 28P01 of a wrong
// password here), not only the step that failed, so an operator tells a wrong password from a
// network problem.
func TestEphemeralListener_LostLineNamesTheClass(t *testing.T) {
	t.Parallel()
	var out syncBuffer
	log := slog.New(slog.NewJSONHandler(&out, nil))
	cfg := pg.EphemeralConfig{
		ReconnectBackoffCap: time.Hour,
		Gate: func(context.Context) error {
			return &pgconn.PgError{Code: "28P01", Message: "password authentication failed"}
		},
	}
	l, err := pg.NewEphemeralListener(&pg.Pool{}, cfg, []pg.EphemeralRoute{nopRoute{}}, func(err error) { t.Errorf("fail: %v", err) }, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	testkit.Until(t, "the lost line", func() bool { return strings.Contains(out.String(), "ephemeral listener lost") })
	if err := l.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, `"error.type":"28P01"`) {
		t.Fatalf("log %s, want error.type 28P01", got)
	}
}
