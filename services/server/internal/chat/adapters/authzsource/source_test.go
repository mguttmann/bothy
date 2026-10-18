// SPDX-License-Identifier: Apache-2.0

package authzsource_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/adapters/authzsource"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// store answers the space reads of the source; the writes are not used.
type store struct {
	app.Store
	spaces map[ids.ID]domain.Space
}

func (s store) Space(_ context.Context, id ids.ID) (domain.Space, bool, error) {
	sp, ok := s.spaces[id]
	return sp, ok, nil
}

func (s store) SpaceIDs(context.Context) ([]ids.ID, error) {
	out := make([]ids.ID, 0, len(s.spaces))
	for id := range s.spaces {
		out = append(out, id)
	}
	return out, nil
}

// TestSource: a space exists archived or not, SpaceArchived is false for an unknown space, and
// Spaces lists the archived ones too.
func TestSource(t *testing.T) {
	t.Parallel()
	open, closed := ids.New(), ids.New()
	arch := domain.NewSpace(closed, "Research", "", "", "a1", ids.New())
	arch.Archive(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC))
	src := authzsource.New(store{spaces: map[ids.ID]domain.Space{open: domain.NewSpace(open, "Support", "", "", "a0", ids.New()), closed: arch}})
	ctx := context.Background()
	for _, c := range []struct {
		id              ids.ID
		exists, archive bool
	}{{open, true, false}, {closed, true, true}, {ids.New(), false, false}} {
		ok, err := src.Space(ctx, c.id)
		a, err2 := src.SpaceArchived(ctx, c.id)
		if err != nil || err2 != nil || ok != c.exists || a != c.archive {
			t.Errorf("%v: exists %v archived %v (%v, %v)", c.id, ok, a, err, err2)
		}
	}
	if list, err := src.Spaces(ctx); err != nil || len(list) != 2 {
		t.Fatalf("spaces %v, %v", list, err)
	}
}

// TestSourceSpaceDetails: the details of the known spaces among the ids, archived or not; an
// unknown id is absent (M1-49 on the spaces of M1-52, INT-W15/2).
func TestSourceSpaceDetails(t *testing.T) {
	t.Parallel()
	open, closed, op := ids.New(), ids.New(), ids.New()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	sp := domain.NewSpace(open, "Support", "chat", "teal", "a0", ids.New())
	sp.DefaultOperatorAgentID, sp.CreatedAt, sp.Version = op, at, 3
	arch := domain.NewSpace(closed, "Research", "", "", "a1", ids.New())
	arch.Archive(at)
	src := authzsource.New(store{spaces: map[ids.ID]domain.Space{open: sp, closed: arch}})
	got, err := src.SpaceDetails(context.Background(), []ids.ID{open, closed, ids.New()})
	if err != nil || len(got) != 2 {
		t.Fatalf("SpaceDetails = %+v, %v", got, err)
	}
	want := authz.SpaceDetail{ID: open, Name: "Support", Icon: "chat", Color: "teal", SortKey: "a0", DefaultOperatorID: op, Version: 3, CreatedAt: at}
	if got[open] != want {
		t.Fatalf("open = %+v, want %+v", got[open], want)
	}
	if d := got[closed]; d.Name != "Research" || !d.ArchivedAt.Equal(at) {
		t.Fatalf("archived = %+v", d)
	}
}

// chanStore answers the channel reads of the source.
type chanStore struct {
	app.Store
	rows []app.ChannelRow
}

func (s chanStore) Channel(_ context.Context, id ids.ID) (app.ChannelRow, bool, error) {
	for _, c := range s.rows {
		if c.Channel.ID == id {
			return c, true, nil
		}
	}
	return app.ChannelRow{}, false, nil
}

func (s chanStore) AllChannels(context.Context) ([]app.ChannelRow, error) { return s.rows, nil }

// TestSourceChannels (M1-53): a channel is archived when it or its space is; the effective
// operator follows inheritance; ChannelsOperatedBy finds the inheriting channel and nothing for
// the zero id; ChannelDetails omits unknown ids.
func TestSourceChannels(t *testing.T) {
	t.Parallel()
	space, op := ids.New(), ids.New()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	live := domain.NewChannel(ids.New(), space, domain.ChannelText, "tickets", "", "a0", ids.New(), 20)
	gone := domain.NewChannel(ids.New(), space, domain.ChannelForum, "bugs", "", "a1", ids.New(), 20)
	gone.Archive(at)
	inSpace := domain.NewChannel(ids.New(), ids.New(), domain.ChannelText, "old", "", "a0", ids.New(), 20)
	src := authzsource.New(chanStore{rows: []app.ChannelRow{
		{Channel: live, SpaceDefaultOperatorID: op}, {Channel: gone}, {Channel: inSpace, SpaceArchived: true},
	}})
	ctx := context.Background()
	for _, c := range []struct {
		id       ids.ID
		archived bool
	}{{live.ID, false}, {gone.ID, true}, {inSpace.ID, true}} {
		f, ok, err := src.Channel(ctx, c.id)
		if err != nil || !ok || f.Archived != c.archived {
			t.Errorf("%v: %+v %v %v", c.id, f, ok, err)
		}
	}
	if f, _, _ := src.Channel(ctx, live.ID); f.EffectiveOperatorID != op || f.Kind != authz.ChannelText || !f.RequireMention {
		t.Fatalf("facts %+v", f)
	}
	if _, ok, err := src.Channel(ctx, ids.New()); ok || err != nil {
		t.Fatal("unknown channel found")
	}
	if got, _ := src.ChannelsOperatedBy(ctx, op); len(got) != 1 || got[0] != live.ID {
		t.Fatalf("operated %v", got)
	}
	if got, _ := src.ChannelsOperatedBy(ctx, ids.ID{}); len(got) != 0 {
		t.Fatalf("zero agent operates %v", got)
	}
	if all, _ := src.Channels(ctx); len(all) != 3 {
		t.Fatalf("channels %v", all)
	}
	d, err := src.ChannelDetails(ctx, []ids.ID{live.ID, ids.New()})
	if err != nil || len(d) != 1 || d[live.ID].OperatorMode != "inherit" || d[live.ID].EffectiveOperatorID != op {
		t.Fatalf("details %+v, %v", d, err)
	}
}

// opStore answers the served-target reads of the source (M1-54).
type opStore struct {
	app.Store
	served []app.ServedTarget
}

func (s opStore) ServedTargets(context.Context, ids.ID) ([]app.ServedTarget, error) {
	return s.served, nil
}

// TestSourceOperators: ServedTargets converts the targets; the zero agent serves nothing. The
// thread facts are TestSourceThread.
func TestSourceOperators(t *testing.T) {
	t.Parallel()
	space, channel := ids.New(), ids.New()
	src := authzsource.New(opStore{served: []app.ServedTarget{{SpaceID: space}, {ChannelID: channel, SpaceID: space}}})
	ctx := context.Background()
	refs, err := src.ServedTargets(ctx, ids.New())
	if err != nil || len(refs) != 2 || refs[0] != (authz.ChannelRef{SpaceID: space}) || refs[1] != (authz.ChannelRef{ChannelID: channel, SpaceID: space}) {
		t.Fatalf("served %+v %v", refs, err)
	}
	if refs, err := src.ServedTargets(ctx, ids.ID{}); err != nil || refs != nil {
		t.Fatalf("zero agent %+v %v", refs, err)
	}
}

// threadStore answers the thread reads of the source.
type threadStore struct {
	app.Store
	threads map[ids.ID]domain.Thread
}

func (s threadStore) Thread(_ context.Context, id ids.ID) (domain.Thread, bool, error) {
	th, ok := s.threads[id]
	return th, ok, nil
}

// TestSourceThread (M1-55 D8): the facts of a thread are its channel, kind, creator and fixed
// operator; a main thread has no operator; an unknown thread is not found.
func TestSourceThread(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	ch, by, op := ids.New(), ids.New(), ids.New()
	main := domain.NewMainThread(ids.New(), ch, by, at)
	post := domain.Thread{ID: ids.New(), ChannelID: ch, Kind: domain.ThreadKindThread, OperatorAgentID: op, CreatedBy: by}
	src := authzsource.New(threadStore{threads: map[ids.ID]domain.Thread{main.ID: main, post.ID: post}})
	ctx := context.Background()
	if f, ok, err := src.Thread(ctx, main.ID); err != nil || !ok || f != (authz.ThreadFacts{ID: main.ID, ChannelID: ch, Main: true, CreatedBy: by}) {
		t.Fatalf("main %+v %v %v", f, ok, err)
	}
	if f, ok, err := src.Thread(ctx, post.ID); err != nil || !ok || f != (authz.ThreadFacts{ID: post.ID, ChannelID: ch, OperatorAgentID: op, CreatedBy: by}) {
		t.Fatalf("post %+v %v %v", f, ok, err)
	}
	if _, ok, err := src.Thread(ctx, ids.New()); ok || err != nil {
		t.Fatal("unknown thread found")
	}
}
