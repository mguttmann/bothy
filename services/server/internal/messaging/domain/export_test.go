// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestValidateExportRange (brief M3-11 note 3c): from strictly before to; equal and reversed
// ranges are one field error on #query/to with code invalid.
func TestValidateExportRange(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name     string
		from, to time.Time
		ok       bool
	}{
		{"from before to", at, at.Add(time.Hour), true},
		{"one millisecond", at, at.Add(time.Millisecond), true},
		{"equal", at, at, false},
		{"reversed", at.Add(time.Hour), at, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := domain.ValidateExportRange(c.from, c.to)
			if c.ok {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != "#query/to" || e.Fields[0].Code != "invalid" {
				t.Fatalf("err = %#v", err)
			}
		})
	}
}

// cmpID orders ids by their bytes, the creation order of UUIDv7.
func cmpID(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }

// pair is one emitted message position.
type pair struct {
	thread ids.ID
	seq    int64
}

// walkAll runs w over data (thread -> ascending seqs) with page size limit and returns the
// emitted positions in order.
func walkAll(t *testing.T, w *domain.ExportWalk, data map[ids.ID][]int64, limit int) []pair {
	t.Helper()
	threads := make([]ids.ID, 0, len(data))
	for id := range data {
		threads = append(threads, id)
	}
	slices.SortFunc(threads, cmpID)
	var out []pair
	for range 10000 {
		switch w.Next() {
		case domain.ExportReadThreads:
			var page []ids.ID
			for _, id := range threads {
				if cmpID(id, w.AfterThread()) > 0 && len(page) < limit {
					page = append(page, id)
				}
			}
			w.Threads(page)
		case domain.ExportReadMessages:
			c := w.Cursor()
			var page []int64
			for _, s := range data[c.ThreadID] {
				if s > c.ThreadSeq && len(page) < limit {
					page = append(page, s)
				}
			}
			for _, s := range page {
				out = append(out, pair{c.ThreadID, s})
			}
			w.Messages(page)
		case domain.ExportReadDone:
			return out
		}
	}
	t.Fatal("the walk did not end")
	return nil
}

// threadIDs returns n ascending thread ids.
func threadIDs(n int) []ids.ID {
	out := make([]ids.ID, n)
	for i := range out {
		out[i] = ids.New()
	}
	slices.SortFunc(out, cmpID)
	return out
}

// TestExportCursor (brief M3-11 note 5): every (thread_id, thread_seq) of the selection exactly
// once, threads ascending and thread_seq ascending within each, across thread and message pages.
func TestExportCursor(t *testing.T) {
	t.Parallel()
	th := threadIDs(7)
	seqs := func(list ...int64) []int64 { return list }
	run := func(from, to int64) []int64 {
		var out []int64
		for i := from; i <= to; i++ {
			out = append(out, i)
		}
		return out
	}
	for _, c := range []struct {
		name  string
		data  map[ids.ID][]int64
		limit int
	}{
		{"empty selection", map[ids.ID][]int64{}, 3},
		{"one thread inside one page", map[ids.ID][]int64{th[0]: seqs(1, 2)}, 3},
		{"page border inside a thread", map[ids.ID][]int64{th[0]: run(1, 7)}, 3},
		{"page border at the end of a thread", map[ids.ID][]int64{th[0]: run(1, 6), th[1]: seqs(4)}, 3},
		{"thread page ends exactly at the limit", map[ids.ID][]int64{th[0]: seqs(1), th[1]: seqs(2), th[2]: seqs(5, 9)}, 3},
		{"several threads over several thread pages", map[ids.ID][]int64{
			th[0]: run(1, 4), th[1]: seqs(3), th[2]: run(10, 15), th[3]: seqs(1, 2), th[4]: seqs(8), th[5]: run(1, 3), th[6]: seqs(2),
		}, 2},
		{"page size one", map[ids.ID][]int64{th[0]: seqs(1, 3), th[1]: seqs(2)}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := walkAll(t, domain.NewExportWalk(c.limit), c.data, c.limit)
			var want []pair
			for _, id := range th {
				for _, s := range c.data[id] {
					want = append(want, pair{id, s})
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("got %v\nwant %v", got, want)
			}
		})
	}
	t.Run("resuming after a cursor", func(t *testing.T) {
		data := map[ids.ID][]int64{th[0]: run(1, 5), th[1]: run(1, 4), th[2]: seqs(2)}
		got := walkAll(t, domain.ResumeExportWalk(2, domain.ExportCursor{ThreadID: th[1], ThreadSeq: 2}), data, 2)
		want := []pair{{th[1], 3}, {th[1], 4}, {th[2], 2}}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("got %v\nwant %v", got, want)
		}
	})
	t.Run("a message added to the current thread during the walk", func(t *testing.T) {
		data := map[ids.ID][]int64{th[0]: run(1, 3), th[1]: seqs(1)}
		w := domain.NewExportWalk(2)
		var got []pair
		for step := w.Next(); step != domain.ExportReadDone; step = w.Next() {
			switch step {
			case domain.ExportReadThreads:
				var page []ids.ID
				for _, id := range []ids.ID{th[0], th[1]} {
					if cmpID(id, w.AfterThread()) > 0 && len(page) < 2 {
						page = append(page, id)
					}
				}
				w.Threads(page)
			case domain.ExportReadMessages:
				c := w.Cursor()
				var page []int64
				for _, s := range data[c.ThreadID] {
					if s > c.ThreadSeq && len(page) < 2 {
						page = append(page, s)
					}
				}
				for _, s := range page {
					got = append(got, pair{c.ThreadID, s})
				}
				w.Messages(page)
				if c.ThreadID == th[0] && len(got) == 2 {
					data[th[0]] = append(data[th[0]], 4)
				}
			}
		}
		want := []pair{{th[0], 1}, {th[0], 2}, {th[0], 3}, {th[0], 4}, {th[1], 1}}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("got %v\nwant %v", got, want)
		}
	})
}
