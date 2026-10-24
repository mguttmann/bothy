// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

// worldOf returns made-up ids for the plan of c and the base of its main threads (seq 3, version
// 4, as if three messages had been posted before the load).
func worldOf(c *searchbench.Corpus) (searchbench.WorldIDs, map[string][2]int64) {
	w := searchbench.WorldIDs{}
	base := map[string][2]int64{}
	for ch := range c.Plan.Channels {
		w.Channels = append(w.Channels, fmt.Sprintf("channel-%d", ch))
		main := ""
		if c.Plan.ChannelKind(ch) == searchbench.TextChannel {
			main = fmt.Sprintf("main-%d", ch)
			base[main] = [2]int64{3, 4}
		}
		w.MainThreads = append(w.MainThreads, main)
	}
	for a := range c.Plan.Authors {
		w.Authors = append(w.Authors, fmt.Sprintf("author-%d", a))
	}
	return w, base
}

// TestLoaderRows: thread_seq is gapless per thread (from 1 in a post, after the base in a main
// thread); counters equal the aggregates of the messages and unread_floor_seq the last seq; a
// deleted row is a tombstone of version 2; ids are UUIDv7 at created_at, ascending; the payload
// hash is the SHA-256 of the text.
func TestLoaderRows(t *testing.T) {
	t.Parallel()
	c := corpusOf(t, searchbench.SmallParams(), searchbench.SmallMessages)
	w, base := worldOf(c)
	rows, err := searchbench.BuildRows(c, w, base)
	if err != nil {
		t.Fatal(err)
	}
	type agg struct {
		maxSeq, n, live, deleted int64
		last                     time.Time
	}
	got := map[string]*agg{}
	next := map[string]int64{}
	for _, th := range rows.Threads {
		got[th.ID] = &agg{}
		next[th.ID] = th.BaseSeq + 1
		if th.Post >= 0 && th.BaseSeq != 0 {
			t.Fatalf("post %d starts after %d", th.Post, th.BaseSeq)
		}
	}
	var prevID, prevCM string
	var prevAt time.Time
	n := 0
	for row := range rows.Messages() {
		n++
		a := got[row.ThreadID]
		if row.ThreadSeq != next[row.ThreadID] {
			t.Fatalf("message %d: thread_seq %d, want %d", row.Index, row.ThreadSeq, next[row.ThreadID])
		}
		next[row.ThreadID]++
		a.maxSeq, a.n, a.last = row.ThreadSeq, a.n+1, row.CreatedAt
		if row.Deleted {
			a.deleted++
			if row.Content != "" {
				t.Fatalf("deleted message %d keeps its content", row.Index)
			}
		} else {
			a.live++
		}
		text := c.Message(row.Index).Text
		if row.PayloadSHA256 != sha256.Sum256([]byte(text)) {
			t.Fatalf("message %d: payload hash is not the SHA-256 of the text", row.Index)
		}
		for _, id := range []string{row.ID.String(), row.ClientMsgID.String()} {
			if id[14] != '7' {
				t.Fatalf("message %d: %s is no UUIDv7", row.Index, id)
			}
		}
		if !row.ID.Time().Equal(row.CreatedAt) || !row.ClientMsgID.Time().Equal(row.CreatedAt) {
			t.Fatalf("message %d: id time %s, created_at %s", row.Index, row.ID.Time(), row.CreatedAt)
		}
		if row.ID.String() <= prevID || row.ClientMsgID.String() <= prevCM || !row.CreatedAt.After(prevAt) {
			t.Fatalf("message %d: ids or created_at do not ascend", row.Index)
		}
		prevID, prevCM, prevAt = row.ID.String(), row.ClientMsgID.String(), row.CreatedAt
	}
	if n != c.Plan.Messages {
		t.Fatalf("%d rows, want %d", n, c.Plan.Messages)
	}
	for _, th := range rows.Threads {
		a := got[th.ID]
		if th.LastMessageSeq != th.BaseSeq+a.n || th.LastMessageSeq != a.maxSeq || th.MessageCount != a.live || !th.LastActivityAt.Equal(a.last) {
			t.Errorf("thread %s: counters %d/%d/%s, aggregates %+v", th.ID, th.LastMessageSeq, th.MessageCount, th.LastActivityAt, *a)
		}
		if th.Version != th.BaseVersion+a.n+a.deleted {
			t.Errorf("thread %s: version %d, want %d", th.ID, th.Version, th.BaseVersion+a.n+a.deleted)
		}
		if th.Post >= 0 {
			if !th.CreatedAt.Equal(c.CreatedAt(th.Post)) || th.Title != c.Title(th.Post) || th.SearchLanguage != c.Plan.AuthorConfig(c.PostCreator(th.Post)) {
				t.Errorf("post %d: created_at, title or language differ from the corpus", th.Post)
			}
		}
	}
}
