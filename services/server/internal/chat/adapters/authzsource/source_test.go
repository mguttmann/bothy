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
