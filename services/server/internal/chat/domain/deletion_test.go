// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var (
	delAt  = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	window = 30 * time.Second
)

// wantConflict checks that err is 409 conflict with reason.
func wantConflict(t *testing.T, err error, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeConflict || e.Reason != reason {
		t.Fatalf("error %v (%+v), want conflict %s", err, e, reason)
	}
}

// TestDeletion_State: live before a deletion, pending until deleted_at plus the window, final
// from then on and once purged (D-DEL-1, D-DEL-6).
func TestDeletion_State(t *testing.T) {
	t.Parallel()
	var d domain.Deletion
	if d.Deleted() || d.State(delAt, window) != domain.DeletionLive || !d.UndoUntil(window).IsZero() {
		t.Fatalf("zero deletion %+v", d)
	}
	d = domain.Deletion{At: delAt, By: ids.New()}
	for _, c := range []struct {
		now  time.Time
		want domain.DeletionState
	}{
		{delAt, domain.DeletionPending},
		{delAt.Add(window - time.Microsecond), domain.DeletionPending},
		{delAt.Add(window), domain.DeletionFinal},
		{delAt.Add(time.Hour), domain.DeletionFinal},
	} {
		if got := d.State(c.now, window); got != c.want {
			t.Errorf("state at %v = %s, want %s", c.now, got, c.want)
		}
	}
	if !d.UndoUntil(window).Equal(delAt.Add(window)) {
		t.Errorf("undo until %v", d.UndoUntil(window))
	}
	d.PurgedAt = delAt.Add(time.Minute)
	if d.State(delAt, window) != domain.DeletionFinal || !d.Purged() {
		t.Errorf("a purged deletion is final, %+v", d)
	}
}

// TestDeletion_CheckRestore: a restore is allowed strictly before the end of the window and never
// after a purge; otherwise 409 conflict undo_window_passed.
func TestDeletion_CheckRestore(t *testing.T) {
	t.Parallel()
	d := domain.Deletion{At: delAt, By: ids.New()}
	if err := d.CheckRestore(delAt.Add(window-time.Microsecond), window); err != nil {
		t.Fatalf("within the window: %v", err)
	}
	wantConflict(t, d.CheckRestore(delAt.Add(window), window), domain.ReasonUndoWindowPassed)
	d.PurgedAt = delAt
	wantConflict(t, d.CheckRestore(delAt, window), domain.ReasonUndoWindowPassed)
}

// TestSpaceDeleteRestore: a delete archives a live space with the same time stamp and bumps the
// version once; a restore clears both; a space archived before keeps its archive.
func TestSpaceDeleteRestore(t *testing.T) {
	t.Parallel()
	by := ids.New()
	sp := domain.NewSpace(ids.New(), "Support", "", "", "a0", by)
	sp.Delete(by, delAt)
	if !sp.Deleted() || !sp.ArchivedAt.Equal(delAt) || !sp.Deletion.At.Equal(delAt) || sp.Deletion.By != by || sp.Version != 2 {
		t.Fatalf("deleted space %+v", sp)
	}
	sp.Restore()
	if sp.Deleted() || sp.Archived() || sp.Version != 3 {
		t.Fatalf("restored space %+v", sp)
	}
	earlier := delAt.Add(-time.Hour)
	sp.Archive(earlier)
	sp.Delete(by, delAt)
	if !sp.ArchivedAt.Equal(earlier) {
		t.Fatalf("an earlier archive was replaced: %v", sp.ArchivedAt)
	}
	sp.Restore()
	if sp.Deleted() || !sp.ArchivedAt.Equal(earlier) {
		t.Fatalf("an earlier archive must stay: %+v", sp)
	}
}

// TestChannelDeleteRestore: the same rules for a channel; a dm channel is refused with 409
// conflict dm_channel.
func TestChannelDeleteRestore(t *testing.T) {
	t.Parallel()
	by := ids.New()
	c := domain.NewChannel(ids.New(), ids.New(), domain.ChannelForum, "website", "", "a0", by, 20)
	if err := c.Delete(by, delAt); err != nil {
		t.Fatal(err)
	}
	if !c.Deleted() || !c.ArchivedAt.Equal(delAt) || c.Version != 2 {
		t.Fatalf("deleted channel %+v", c)
	}
	c.Restore()
	if c.Deleted() || c.Archived() || c.Version != 3 {
		t.Fatalf("restored channel %+v", c)
	}
	dm := domain.Channel{ID: ids.New(), Kind: domain.ChannelDM, Version: 1}
	wantConflict(t, dm.Delete(by, delAt), domain.ReasonDMChannel)
	if dm.Deleted() || dm.Version != 1 {
		t.Fatalf("a refused delete changed the channel: %+v", dm)
	}
}

// TestThreadDeleteRestore: a thread is deleted and restored with a version bump each; a main
// thread is refused with 409 conflict main_thread.
func TestThreadDeleteRestore(t *testing.T) {
	t.Parallel()
	by := ids.New()
	th := domain.Thread{ID: ids.New(), Kind: domain.ThreadKindThread, Version: 1}
	if err := th.Delete(by, delAt); err != nil {
		t.Fatal(err)
	}
	if !th.Deleted() || th.Deletion.By != by || th.Version != 2 {
		t.Fatalf("deleted thread %+v", th)
	}
	th.Restore()
	if th.Deleted() || th.Version != 3 {
		t.Fatalf("restored thread %+v", th)
	}
	main := domain.NewMainThread(ids.New(), ids.New(), by, delAt)
	wantConflict(t, main.Delete(by, delAt), domain.ReasonMainThread)
}
