// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// readOnlyUoW behaves like the unit of work in maintenance mode: a unit without the exemption of
// txn.WithWriteInMaintenance fails with 503 maintenance.
type readOnlyUoW struct{}

func (readOnlyUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	if !txn.WritesInMaintenance(ctx) {
		return errs.New(errs.CodeMaintenance, "")
	}
	return fn(ctx)
}

// TestConsoleSessionEndsInMaintenance (D-M7-08-12): in maintenance mode the authentication still
// ends an expired view (401 step_up_required) and an idle session (401 session_revoked) instead of
// answering 503 maintenance on every request of the console.
func TestConsoleSessionEndsInMaintenance(t *testing.T) {
	t.Parallel()
	t.Run("expired view", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		c.d.UoW = readOnlyUoW{}
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.ViewAsUserID = ids.New()
		f.store.st.Session.ViewAsStartedAt = consoleNow.Add(-2 * time.Hour)
		_, err := c.Authenticate(context.Background(), consoleCreds())
		_ = problemOf(t, err, errs.CodeStepUpRequired)
	})
	t.Run("idle session", func(t *testing.T) {
		t.Parallel()
		c, f := newTestConsole(t)
		c.d.UoW = readOnlyUoW{}
		f.store.st, f.store.found = openConsoleState(), true
		f.store.st.Session.LastSeenAt = consoleNow.Add(-24 * time.Hour)
		_, err := c.Authenticate(context.Background(), consoleCreds())
		_ = problemOf(t, err, errs.CodeSessionRevoked)
	})
}
