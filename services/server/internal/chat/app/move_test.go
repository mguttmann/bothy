// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// moveEnv is a space with three forum channels and one post in the first.
type moveEnv struct {
	*env
	from, to, third chat.Channel
	post            chat.Thread
}

func newMoveEnv(t *testing.T) moveEnv {
	t.Helper()
	e := newEnv(t)
	sp := e.create(t, "Support", ids.ID{})
	m := moveEnv{env: e, from: e.channel(t, sp.ID, "forum", "help"), to: e.channel(t, sp.ID, "forum", "bugs"),
		third: e.channel(t, sp.ID, "forum", "ideas")}
	post, err := e.newThread(e.owner, chat.CreateThreadCmd{ChannelID: m.from.ID, Title: ptr("Crash")})
	if err != nil {
		t.Fatal(err)
	}
	m.post = post
	return m
}

// move moves the post into target as the owner.
func (m moveEnv) move(target ids.ID) (chat.Thread, error) {
	return m.svc.Move(context.Background(), m.owner, chat.MoveThread{ThreadID: m.post.ID, TargetChannelID: target, Reason: "Wrong channel"})
}

// TestMoveOverFakes: the move use case over the fakes, the happy path with its events and audit
// entry, and every failure of a step it calls stops the move with that failure (brief M1-62
// note 1; the rules of authz and the tables run over PostgreSQL in test/integration).
func TestMoveOverFakes(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")

	t.Run("moves", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		got, err := m.move(m.to.ID)
		if err != nil || got.ChannelID != m.to.ID || got.ID != m.post.ID {
			t.Fatalf("move %+v, %v", got, err)
		}
		if m.eventCount(string(authorize.EventThreadMovedOut)) != 1 || m.eventCount(string(authorize.EventThreadMoved)) != 1 {
			t.Fatalf("events %+v", m.events.evs)
		}
		last := m.rec.entries[len(m.rec.entries)-1]
		if last.Action != string(authorize.ActionThreadMove) || last.ResourceID != m.post.ID {
			t.Fatalf("audit %+v", last)
		}
	})

	t.Run("another principal in the context", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
		_, err := m.svc.Move(ctx, m.owner, chat.MoveThread{ThreadID: m.post.ID, TargetChannelID: m.to.ID, Reason: "x"})
		wantCode(t, err, errs.CodeInternalError)
	})

	t.Run("unknown thread", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		_, err := m.svc.Move(context.Background(), m.owner, chat.MoveThread{ThreadID: ids.New(), TargetChannelID: m.to.ID, Reason: "x"})
		wantCode(t, err, errs.CodeNotFound)
	})

	t.Run("unknown target", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		_, err := m.move(ids.New())
		wantCode(t, err, errs.CodeNotFound)
	})

	t.Run("target space gone", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		m.store.spaceGone = true
		_, err := m.move(m.to.ID)
		wantCode(t, err, errs.CodeInternalError)
	})

	t.Run("moved concurrently and refused at its new place", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		m.store.movedTo = m.third.ID
		calls := 0
		m.az.rules[authorize.ActionThreadMove] = func(authorize.Principal, authorize.Resource) authorize.Decision {
			calls++
			if calls > 1 {
				return forbidden
			}
			return allow
		}
		_, err := m.move(m.to.ID)
		wantCode(t, err, errs.CodeForbidden)
		if calls != 2 {
			t.Fatalf("authorize calls %d, want a second decision after the lock", calls)
		}
	})

	t.Run("read fails", func(t *testing.T) {
		t.Parallel()
		m := newMoveEnv(t)
		m.store.fail = boom
		if _, err := m.move(m.to.ID); !errors.Is(err, boom) {
			t.Fatalf("err %v", err)
		}
	})

	for _, method := range []string{"LockThread", "Channel", "ShareChannel", "ShareSpace", "MoveThread", "DeleteThreadTags"} {
		t.Run(method+" fails", func(t *testing.T) {
			t.Parallel()
			m := newMoveEnv(t)
			m.store.inject(method, boom)
			if _, err := m.move(m.to.ID); !errors.Is(err, boom) {
				t.Fatalf("err %v", err)
			}
		})
	}

	for _, typ := range []authorize.EventType{authorize.EventThreadMovedOut, authorize.EventThreadMoved} {
		t.Run(string(typ)+" append fails", func(t *testing.T) {
			t.Parallel()
			m := newMoveEnv(t)
			m.events.mu.Lock()
			m.events.fail = map[string]error{string(typ): boom}
			m.events.mu.Unlock()
			if _, err := m.move(m.to.ID); !errors.Is(err, boom) {
				t.Fatalf("err %v", err)
			}
		})
	}
}
