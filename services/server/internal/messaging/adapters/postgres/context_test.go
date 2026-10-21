// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestRepository_ContextWindow (M2-13 design note 3, ruling D-M2-13-7): the newest Limit
// selectable messages strictly between the bounds, ascending; tombstones and the excluded author
// are neither selected nor counted; HasOlder is exact at both edges, also for an empty page and a
// limit of zero; content is whole.
func TestRepository_ContextWindow(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	space, _, main := w.TextChannel()
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	byID := map[int64]ids.ID{}
	long := strings.Repeat("x", mw.MaxContent)
	for i := range 8 {
		author, content := "owner", fmt.Sprintf("m%d", i+1)
		if i == 3 || i == 5 {
			author = "sam"
		}
		if i == 7 {
			content = long
		}
		m := w.Send(author, main, content, http.StatusCreated)
		byID[int64(i+1)] = ids.MustParse(m["id"].(string))
	}
	w.Tombstone(byID[2])
	repo := postgres.NewRepository(w.Pool)
	ctx := context.Background()
	for _, c := range []struct {
		name          string
		after, before int64
		limit         int
		exclude       ids.ID
		want          string
		older         bool
	}{
		{"all", 0, 9, 10, ids.ID{}, "[1 3 4 5 6 7 8]", false},
		{"newest three", 0, 9, 3, ids.ID{}, "[6 7 8]", true},
		{"exactly to the oldest", 0, 9, 7, ids.ID{}, "[1 3 4 5 6 7 8]", false},
		{"one short of the oldest", 0, 9, 6, ids.ID{}, "[3 4 5 6 7 8]", true},
		{"a tombstone is not counted", 1, 9, 6, ids.ID{}, "[3 4 5 6 7 8]", false},
		{"below the trigger", 0, 5, 2, ids.ID{}, "[3 4]", true},
		{"above the floor", 4, 9, 10, ids.ID{}, "[5 6 7 8]", false},
		{"excluded author not counted", 0, 9, 5, sam, "[1 3 5 7 8]", false},
		{"excluded author, one short", 0, 9, 4, sam, "[3 5 7 8]", true},
		{"limit zero", 0, 9, 0, ids.ID{}, "[]", true},
		{"limit zero over nothing", 1, 3, 0, ids.ID{}, "[]", false},
		{"empty window", 8, 9, 5, ids.ID{}, "[]", false},
	} {
		rows, err := repo.ContextWindow(ctx, app.ContextRead{ThreadID: main, AfterSeq: c.after, BeforeSeq: c.before, Limit: c.limit, ExcludeAuthorID: c.exclude})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if fmt.Sprint(seqsOf(rows.Items)) != c.want || rows.HasOlder != c.older {
			t.Errorf("%s: %v older %v, want %s older %v", c.name, seqsOf(rows.Items), rows.HasOlder, c.want, c.older)
		}
	}
	rows, err := repo.ContextWindow(ctx, app.ContextRead{ThreadID: main, AfterSeq: 7, BeforeSeq: 9, Limit: 1})
	if err != nil || len(rows.Items) != 1 || rows.Items[0].ContentMD != long {
		t.Fatalf("whole content: %v %v", len(rows.Items), err)
	}
	if _, err := repo.ContextWindow(ctx, app.ContextRead{ThreadID: main, BeforeSeq: 9, Limit: -1}); err == nil {
		t.Error("a negative limit was accepted")
	}
}
