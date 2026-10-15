// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func (s *store) ShareSpace(ctx context.Context, id ids.ID) (domain.Space, bool, error) {
	return s.Space(ctx, id)
}

func (s *store) ShareChannel(ctx context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	return s.Channel(ctx, id)
}

func (s *store) ChannelIDsOfSpace(_ context.Context, space ids.ID) ([]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []ids.ID{}
	for _, c := range s.channels {
		if c.SpaceID == space {
			out = append(out, c.ID)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return cmp.Compare(a.String(), b.String()) })
	return out, s.fail
}

func (s *store) ThreadIDsOfChannels(_ context.Context, channels []ids.ID) ([]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureThreads()
	out := []ids.ID{}
	for _, th := range s.threads {
		if slices.Contains(channels, th.ChannelID) {
			out = append(out, th.ID)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return cmp.Compare(a.String(), b.String()) })
	return out, s.fail
}

// TestNotificationPath (M3-18): the path of a space, a channel and a thread with the channel
// default and the archive state of the channel or its space; an id of another kind and an
// unknown id are not_found; the id lists of the cleanup.
func TestNotificationPath(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sp := domain.Space{ID: ids.New(), Name: "Support"}
	ch := domain.Channel{ID: ids.New(), SpaceID: sp.ID, Kind: domain.ChannelText, DefaultNotificationLevel: "mute"}
	other := domain.Channel{ID: ids.New(), SpaceID: sp.ID, Kind: domain.ChannelForum, ArchivedAt: time.Now()}
	th := domain.Thread{ID: ids.New(), ChannelID: ch.ID}
	e.store.spaces[sp.ID] = sp
	e.store.channels[ch.ID], e.store.channels[other.ID] = ch, other
	e.store.ensureThreads()
	e.store.threads[th.ID] = th

	for _, read := range []func(context.Context, string, ids.ID) (chat.NotificationPath, error){e.svc.NotificationPath, e.svc.LockNotificationPath} {
		got, err := read(ctx, chat.TargetThread, th.ID)
		want := chat.NotificationPath{Kind: "thread", SpaceID: sp.ID, ChannelID: ch.ID, ThreadID: th.ID, ChannelKind: "text", DefaultNotificationLevel: "mute"}
		if err != nil || got != want {
			t.Fatalf("thread %+v, %v", got, err)
		}
		if got, err := read(ctx, chat.TargetChannel, other.ID); err != nil || !got.Archived || got.ChannelKind != "forum" || got.ThreadID != (ids.ID{}) {
			t.Fatalf("archived channel %+v, %v", got, err)
		}
		if got, err := read(ctx, chat.TargetSpace, sp.ID); err != nil || got.SpaceID != sp.ID || got.ChannelID != (ids.ID{}) || got.Archived {
			t.Fatalf("space %+v, %v", got, err)
		}
		for _, tc := range []struct {
			kind string
			id   ids.ID
		}{{chat.TargetThread, ch.ID}, {chat.TargetChannel, sp.ID}, {chat.TargetSpace, th.ID}, {chat.TargetThread, ids.New()}, {"dm", ch.ID}} {
			_, err := read(ctx, tc.kind, tc.id)
			wantCode(t, err, errs.CodeNotFound)
		}
	}
	sp.ArchivedAt = time.Now()
	e.store.spaces[sp.ID] = sp
	if got, err := e.svc.NotificationPath(ctx, chat.TargetThread, th.ID); err != nil || !got.Archived {
		t.Fatalf("thread in an archived space %+v, %v", got, err)
	}

	chans, err := e.svc.ChannelIDsOfSpace(ctx, sp.ID)
	if err != nil || len(chans) != 2 || !slices.Contains(chans, other.ID) {
		t.Fatalf("channels %v, %v", chans, err)
	}
	if got, err := e.svc.ThreadIDsOfChannels(ctx, chans); err != nil || !slices.Equal(got, []ids.ID{th.ID}) {
		t.Fatalf("threads %v, %v", got, err)
	}
}
