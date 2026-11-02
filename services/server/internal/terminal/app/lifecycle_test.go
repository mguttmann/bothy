// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestOwnerAccountSessionWindows (FIX-terminal-session-lifecycle, D-TSL-1): a session in the owner
// account of its host gets terminal.owner_account_detach_grace in the hello of its data connection
// and is marked in the times the relay and the sweep measure; a session of a separate session user
// keeps terminal.detach_grace.
func TestOwnerAccountSessionWindows(t *testing.T) {
	t.Parallel()
	for _, ownerAccount := range []bool{true, false} {
		w := newSWorld(t, nil, func(c *app.Config) {
			c.DetachGrace, c.OwnerAccountDetachGrace = 15*time.Minute, 24*time.Hour
		})
		w.store.own[0].OwnerAccount = ownerAccount
		w.store.users = []domain.SessionUserRow{{ID: w.store.own[0].ID, UserID: w.alex.ID, OSUser: "bothy-s-alex", OwnerAccount: ownerAccount}}
		g, err := w.open()
		if err != nil {
			t.Fatal(err)
		}
		id := g.Session.ID
		grace, err := w.svc.HelloDetachGrace(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		want := 15 * time.Minute
		if ownerAccount {
			want = 24 * time.Hour
		}
		if grace != want {
			t.Errorf("owner account %v: hello detach grace %v, want %v", ownerAccount, grace, want)
		}
		e, ok := w.svc.RelayExpectation(context.Background(), id)
		if !ok || e.Times.OwnerAccount != ownerAccount {
			t.Errorf("owner account %v: expectation times %+v (%v)", ownerAccount, e.Times, ok)
		}
		// An unknown session gets the plain grace.
		if grace, err := w.svc.HelloDetachGrace(context.Background(), ids.New()); err != nil || grace != 15*time.Minute {
			t.Errorf("unknown session: %v %v", grace, err)
		}
		if win := w.svc.Windows(); win.OwnerDetachGrace != 24*time.Hour {
			t.Errorf("windows %+v", win)
		}
	}
}
