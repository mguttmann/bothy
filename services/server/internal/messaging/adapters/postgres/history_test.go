// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// seqsOf returns the thread_seq of list.
func seqsOf(list []domain.Message) []int64 {
	out := []int64{}
	for _, m := range list {
		out = append(out, m.ThreadSeq)
	}
	return out
}

// TestRepository_History (brief M1-57 note 4): each statement returns its sides in reading order
// (below descending, above ascending) with at most RowLimit rows, the opposite flag also for an
// empty page, nothing outside the visible channels, and VisibleMessage only inside them.
func TestRepository_History(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	_, ch, main := w.TextChannel()
	var first ids.ID
	for i := range 6 {
		m := w.Send("owner", main, fmt.Sprintf("m%d", i+1), http.StatusCreated)
		if i == 0 {
			first = ids.MustParse(m["id"].(string))
		}
	}
	repo := postgres.NewRepository(w.Pool)
	ctx := context.Background()
	visible := []ids.ID{ch}
	for _, c := range []struct {
		name         string
		mode         domain.HistoryMode
		anchor       int64
		below, above string
		opposite     bool
	}{
		{"newest", domain.HistoryNewest, 0, "[6 5 4]", "[]", false},
		{"before", domain.HistoryBefore, 4, "[3 2 1]", "[]", true},
		{"before empty", domain.HistoryBefore, 1, "[]", "[]", true},
		{"before past the end", domain.HistoryBefore, 9, "[6 5 4]", "[]", false},
		{"after", domain.HistoryAfter, 2, "[]", "[3 4 5]", true},
		{"after empty", domain.HistoryAfter, 6, "[]", "[]", true},
		{"after from zero", domain.HistoryAfter, 0, "[]", "[1 2 3]", false},
		{"around", domain.HistoryAround, 4, "[3 2 1]", "[4 5 6]", false},
		{"around the start", domain.HistoryAround, 1, "[]", "[1 2 3]", false},
	} {
		rows, err := repo.History(ctx, app.HistoryRead{ThreadID: main, Visible: visible, Mode: c.mode, Anchor: c.anchor, RowLimit: 3})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if fmt.Sprint(seqsOf(rows.Below)) != c.below || fmt.Sprint(seqsOf(rows.Above)) != c.above || rows.Opposite != c.opposite {
			t.Errorf("%s: below %v above %v opposite %v", c.name, seqsOf(rows.Below), seqsOf(rows.Above), rows.Opposite)
		}
	}
	rows, err := repo.History(ctx, app.HistoryRead{ThreadID: main, Visible: visible, Mode: domain.HistoryNewest, RowLimit: 1})
	if err != nil || len(rows.Below) != 1 {
		t.Fatalf("newest: %v %v", rows, err)
	}
	if m := rows.Below[0]; m.ContentMD != "m6" || m.ChannelID != ch || m.ThreadID != main || m.AuthorID != w.OwnerID ||
		m.Version != 1 || m.Kind != domain.KindText || m.Origin != domain.OriginHuman || m.CreatedAt.IsZero() || m.ClientMsgID.IsZero() {
		t.Errorf("row %+v", m)
	}
	for _, mode := range []domain.HistoryMode{domain.HistoryNewest, domain.HistoryBefore, domain.HistoryAfter, domain.HistoryAround} {
		rows, err := repo.History(ctx, app.HistoryRead{ThreadID: main, Visible: nil, Mode: mode, Anchor: 3, RowLimit: 3})
		if err != nil || len(rows.Below)+len(rows.Above) != 0 || rows.Opposite {
			t.Errorf("mode %d without visible channels: %+v %v", mode, rows, err)
		}
	}
	if _, err := repo.History(ctx, app.HistoryRead{ThreadID: main, Visible: visible, Mode: domain.HistoryMode(99), RowLimit: 3}); err == nil {
		t.Error("unknown mode accepted")
	}
	if _, err := repo.History(ctx, app.HistoryRead{ThreadID: main, Visible: visible, RowLimit: 0}); err == nil {
		t.Error("row limit 0 accepted")
	}
	if m, found, err := repo.VisibleMessage(ctx, first, visible); err != nil || !found || m.ContentMD != "m1" {
		t.Errorf("VisibleMessage = %+v %v %v", m, found, err)
	}
	if _, found, err := repo.VisibleMessage(ctx, first, []ids.ID{ids.New()}); err != nil || found {
		t.Errorf("VisibleMessage in another channel: %v %v", found, err)
	}
	if _, found, err := repo.VisibleMessage(ctx, ids.New(), visible); err != nil || found {
		t.Errorf("VisibleMessage of an unknown id: %v %v", found, err)
	}
}
