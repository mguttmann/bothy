// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
)

// followStore keeps the rows of the automatic following.
type followStore struct {
	app.Store
	rows map[[2]ids.ID]notify.Preference
}

func (s *followStore) InsertPreferenceIfAbsent(_ context.Context, user ids.ID, p notify.Preference, _ time.Time) (notify.Preference, bool, error) {
	k := [2]ids.ID{user, p.Target.ID}
	if _, ok := s.rows[k]; ok {
		return notify.Preference{}, false, nil
	}
	p.Version = 1
	s.rows[k] = p
	return p, true, nil
}

// TestOnMessagePosted_Following: a human writing in a thread follows it with all once; the main
// thread, agents and an own row give nothing (13 3.2, R-PN-04).
func TestOnMessagePosted_Following(t *testing.T) {
	t.Parallel()
	alex, thread, own := ids.New(), ids.New(), ids.New()
	store := &followStore{rows: map[[2]ids.ID]notify.Preference{{alex, own}: {Level: "mute"}}}
	sub := app.NewSubscribers(store)
	ctx := context.Background()
	if err := sub.OnMessagePosted(ctx, messaging.MessagePosted{AuthorID: alex, ThreadID: thread, ThreadKind: "thread", Origin: "human"}); err == nil {
		t.Fatal("unbound subscribers accepted a message")
	}
	events := &appender{}
	if err := sub.Bind(events, &fakePaths{}); err != nil {
		t.Fatal(err)
	}
	for _, ev := range []messaging.MessagePosted{
		{AuthorID: alex, ThreadID: thread, ThreadKind: "thread", Origin: "human"},
		{AuthorID: alex, ThreadID: thread, ThreadKind: "thread", Origin: "human"},
		{AuthorID: alex, ThreadID: ids.New(), ThreadKind: "main", Origin: "human"},
		{AuthorID: ids.New(), ThreadID: ids.New(), ThreadKind: "thread", Origin: "agent"},
		{AuthorID: alex, ThreadID: own, ThreadKind: "thread", Origin: "human"},
	} {
		if err := sub.OnMessagePosted(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if len(events.events) != 1 || events.events[0].AudiencePrincipalID != alex || events.events[0].Type != "notification_pref.updated" {
		t.Fatalf("events %+v", events.events)
	}
	if p := store.rows[[2]ids.ID{alex, thread}]; p.Level != "all" || p.Target.Kind != "thread" {
		t.Fatalf("row %+v", p)
	}
	if p := store.rows[[2]ids.ID{alex, own}]; p.Level != "mute" {
		t.Fatalf("own row changed: %+v", p)
	}
}
