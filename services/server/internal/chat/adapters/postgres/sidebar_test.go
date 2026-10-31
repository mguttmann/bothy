// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestRepositorySidebarChannels (M4-51): the channel rows of the sidebar are inserted with
// version 1, replaced with version + 1 and the device of the write, an empty list stays an empty
// list, and the list carries the predicate of 07 section 7.4 rule 1 (visible channels, fully
// visible spaces, instance-wide) and leaves out archived channels and channels of archived
// spaces; writes need a unit of work.
func TestRepositorySidebarChannels(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex, dev := e.person(t)
	dev2 := ids.New()
	e.exec(t, `INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Phone', 'ios', 'active')`, dev2.String(), alex.String())
	sp := domain.NewSpace(ids.New(), "Dev", "", "", "a0", alex)
	sp.CreatedAt = t0
	gone := domain.NewSpace(ids.New(), "Old", "", "", "a1", alex)
	gone.CreatedAt = t0
	f1 := domain.NewChannel(ids.New(), sp.ID, domain.ChannelForum, "one", "", "a0", alex, 20)
	f2 := domain.NewChannel(ids.New(), sp.ID, domain.ChannelForum, "two", "", "a1", alex, 20)
	f3 := domain.NewChannel(ids.New(), gone.ID, domain.ChannelForum, "three", "", "a0", alex, 20)
	for _, c := range []*domain.Channel{&f1, &f2, &f3} {
		c.CreatedAt = t0
	}
	if err := e.write(t, func(ctx context.Context) error {
		for _, s := range []domain.Space{sp, gone} {
			if err := e.repo.InsertSpace(ctx, s); err != nil {
				return err
			}
		}
		for _, c := range []domain.Channel{f1, f2, f3} {
			if err := e.repo.InsertChannel(ctx, c); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var st domain.SidebarChannelState
	put := func(s domain.SidebarChannelState) {
		t.Helper()
		if err := e.write(t, func(ctx context.Context) error {
			var err error
			st, err = e.repo.PutSidebarChannelState(ctx, s)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	p1, p2 := ids.New(), ids.New()
	put(domain.SidebarChannelState{UserID: alex, ChannelID: f1.ID, ThreadOrder: []ids.ID{p2, p1}, UpdatedByDeviceID: dev})
	if st.Version != 1 || !slices.Equal(st.ThreadOrder, []ids.ID{p2, p1}) || st.UpdatedByDeviceID != dev || st.ChannelID != f1.ID {
		t.Fatalf("first put %+v", st)
	}
	put(domain.SidebarChannelState{UserID: alex, ChannelID: f1.ID, ThreadOrder: []ids.ID{}, UpdatedByDeviceID: dev2})
	if st.Version != 2 || st.ThreadOrder == nil || len(st.ThreadOrder) != 0 || st.UpdatedByDeviceID != dev2 {
		t.Fatalf("replace %+v", st)
	}
	put(domain.SidebarChannelState{UserID: alex, ChannelID: f2.ID, ThreadOrder: []ids.ID{p1}})
	put(domain.SidebarChannelState{UserID: alex, ChannelID: f3.ID, ThreadOrder: []ids.ID{p1}})
	count := func(all bool, visible, fully []ids.ID) int {
		t.Helper()
		list, err := e.repo.SidebarChannelStates(ctx, alex, all, visible, fully)
		if err != nil {
			t.Fatal(err)
		}
		return len(list)
	}
	if n := count(true, nil, nil); n != 3 {
		t.Fatalf("instance-wide %d rows", n)
	}
	if n := count(false, []ids.ID{f2.ID}, nil); n != 1 {
		t.Fatalf("visible channel %d rows", n)
	}
	if n := count(false, nil, []ids.ID{sp.ID}); n != 2 {
		t.Fatalf("fully visible space %d rows", n)
	}
	if n := count(false, nil, nil); n != 0 {
		t.Fatalf("empty scope %d rows", n)
	}
	if list, err := e.repo.SidebarChannelStates(ctx, ids.New(), true, nil, nil); err != nil || len(list) != 0 {
		t.Fatalf("rows of another person %+v, %v", list, err)
	}
	e.exec(t, `UPDATE channels SET archived_at = now() WHERE id = $1`, f2.ID.String())
	e.exec(t, `UPDATE spaces SET archived_at = $2 WHERE id = $1`, gone.ID.String(), t0.Add(time.Hour))
	list, err := e.repo.SidebarChannelStates(ctx, alex, true, nil, nil)
	if err != nil || len(list) != 1 || list[0].ChannelID != f1.ID {
		t.Fatalf("archived left out %+v, %v", list, err)
	}
	if _, err := e.repo.PutSidebarChannelState(ctx, st); err == nil {
		t.Fatal("sidebar channel write outside a unit of work")
	}
}
