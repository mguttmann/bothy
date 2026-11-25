// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestTitleRetention_EmptiesOldTitles (AUD-PKG-R2-owner-server, #685): retention.content empties
// the title of a thread whose last activity lies before the cutoff, keeps its title source, raises
// the version and appends thread.updated; a thread with newer activity and a deleted thread keep
// their titles, and a second run finds nothing more.
func TestTitleRetention_EmptiesOldTitles(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := e.create(t, "Support", ids.ID{})
	forum := e.channel(t, sp.ID, "forum", "forum")
	author := actor.Actor{ID: ids.New(), Kind: actor.KindAgent}
	var made []chat.Thread
	for _, title := range []string{"Old plan", "New plan", "Deleted plan"} {
		th, err := e.newThread(author, chat.CreateThreadCmd{ChannelID: forum.ID, Title: ptr(title)})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, th)
	}
	old, recent, deleted := made[0], made[1], made[2]
	start := e.store.threads[old.ID].LastActivityAt
	cutoff := start.Add(time.Hour)
	e.store.mu.Lock()
	th := e.store.threads[recent.ID]
	th.LastActivityAt = cutoff.Add(time.Minute)
	e.store.threads[recent.ID] = th
	th = e.store.threads[deleted.ID]
	th.Deletion.At = start
	e.store.threads[deleted.ID] = th
	e.store.mu.Unlock()
	before := len(e.events.evs)

	n, more, err := e.svc.TitleRetention().Purge(ctx, cutoff, 10)
	if err != nil || n != 1 || more {
		t.Fatalf("Purge = %d, %v, %v; want 1 thread", n, more, err)
	}
	got := e.store.threads[old.ID]
	if got.Title != "" || got.TitleSource != e.store.threads[recent.ID].TitleSource || got.Version != old.Version+1 {
		t.Errorf("old thread after retention: %+v", got)
	}
	if e.store.threads[recent.ID].Title != "New plan" || e.store.threads[deleted.ID].Title != "Deleted plan" {
		t.Errorf("titles of the recent and the deleted thread changed")
	}
	evs := e.events.evs[before:]
	if len(evs) != 1 || evs[0].Type != "thread.updated" || evs[0].ResourceID != old.ID || evs[0].ActorPrincipalID != (ids.ID{}) {
		t.Fatalf("events %+v, want one thread.updated without an actor", evs)
	}
	if n, _, err := e.svc.TitleRetention().Purge(ctx, cutoff, 10); err != nil || n != 0 {
		t.Fatalf("second Purge = %d, %v; want nothing", n, err)
	}
}
