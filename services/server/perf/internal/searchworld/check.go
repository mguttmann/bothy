// SPDX-License-Identifier: Apache-2.0

package searchworld

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// threadView is the part of a Thread the checks compare.
type threadView struct {
	Title          string    `json:"title"`
	LastMessageSeq int64     `json:"last_message_seq"`
	MessageCount   int64     `json:"message_count"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

// messagePage is the part of a MessagePage the checks compare.
type messagePage struct {
	Items []struct {
		ID        string     `json:"id"`
		ThreadSeq int64      `json:"thread_seq"`
		ContentMD string     `json:"content_md"`
		CreatedAt time.Time  `json:"created_at"`
		DeletedAt *time.Time `json:"deleted_at"`
		Author    struct {
			ID string `json:"id"`
		} `json:"author"`
	} `json:"items"`
	HasMoreAfter bool `json:"has_more_after"`
}

// pageLimit is the page size the checks ask listThreadMessages for; the server caps it at
// api.page_size_max, and the checks follow has_more_after.
const pageLimit = 100

// CheckThreads compares n threads of rows, drawn with a generator seeded from Seed, with the
// product (design note 6, before phase messages): getThread returns the loaded title,
// last_message_seq, message_count and last_activity_at, and listThreadMessages returns the loaded
// messages in thread_seq order with the loaded content, creation time and author (a post in full,
// a main thread one page after a random position). n at or above the thread count checks every
// thread. The requests go round robin through the authors, who may read every space. The error
// names the thread, the seq and the field, never content.
func CheckThreads(ctx context.Context, w *World, rows *searchbench.Rows, n int) error {
	r := rand.New(rand.NewPCG(w.Plan.Seed, 7)) // #nosec G404 -- a reproducible sample
	picks := r.Perm(len(rows.Threads))
	if n < len(picks) {
		picks = picks[:n]
	}
	for k, t := range picks {
		c := w.Authors[k%len(w.Authors)].Client
		th := rows.Threads[t]
		var got threadView
		if err := do(ctx, c, http.MethodGet, "/threads/"+th.ID, nil, http.StatusOK, &got); err != nil {
			return err
		}
		title := th.Title
		if th.Post < 0 {
			title = "" // a main thread has no title
		}
		if got.Title != title || got.LastMessageSeq != th.LastMessageSeq || got.MessageCount != th.MessageCount ||
			!got.LastActivityAt.Equal(th.LastActivityAt) {
			return fmt.Errorf("searchworld: getThread %s: last_message_seq %d, message_count %d, last_activity_at %s, title equal %t; the loader wrote %d, %d, %s",
				th.ID, got.LastMessageSeq, got.MessageCount, got.LastActivityAt, got.Title == title, th.LastMessageSeq, th.MessageCount, th.LastActivityAt)
		}
		indexes := rows.ThreadMessages(t)
		after := th.BaseSeq
		if th.Post < 0 && len(indexes) > pageLimit {
			after = th.BaseSeq + int64(r.IntN(len(indexes)-pageLimit))
		}
		if err := checkMessages(ctx, c, rows, th, indexes, after, th.Post >= 0); err != nil {
			return err
		}
	}
	return nil
}

// checkMessages pages listThreadMessages of th after the seq after (all pages when full, one page
// otherwise) and compares every item with the loaded row of its seq.
func checkMessages(ctx context.Context, c *harness.Client, rows *searchbench.Rows, th searchbench.ThreadRow, indexes []int, after int64, full bool) error {
	seen := 0
	for {
		var page messagePage
		path := "/threads/" + th.ID + "/messages?after_seq=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(pageLimit)
		if err := do(ctx, c, http.MethodGet, path, nil, http.StatusOK, &page); err != nil {
			return err
		}
		for _, it := range page.Items {
			if it.ThreadSeq != after+1 {
				return fmt.Errorf("searchworld: listThreadMessages %s returned seq %d after %d", th.ID, it.ThreadSeq, after)
			}
			after = it.ThreadSeq
			pos := int(it.ThreadSeq - th.BaseSeq - 1)
			if pos < 0 || pos >= len(indexes) {
				return fmt.Errorf("searchworld: listThreadMessages %s returned seq %d, the loader wrote %d to %d", th.ID, it.ThreadSeq, th.BaseSeq+1, th.LastMessageSeq)
			}
			want := rows.Row(indexes[pos])
			if it.ContentMD != want.Content || !it.CreatedAt.Equal(want.CreatedAt) || it.Author.ID != want.AuthorID || (it.DeletedAt != nil) != want.Deleted {
				return fmt.Errorf("searchworld: listThreadMessages %s seq %d (message %s): content equal %t, created_at %s want %s, author %s want %s, deleted %t want %t",
					th.ID, it.ThreadSeq, it.ID, it.ContentMD == want.Content, it.CreatedAt, want.CreatedAt, it.Author.ID, want.AuthorID, it.DeletedAt != nil, want.Deleted)
			}
			seen++
		}
		if !full || !page.HasMoreAfter || len(page.Items) == 0 {
			break
		}
	}
	if full && seen != len(indexes) {
		return fmt.Errorf("searchworld: listThreadMessages %s returned %d messages, the loader wrote %d", th.ID, seen, len(indexes))
	}
	if !full && seen == 0 && len(indexes) > 0 {
		return fmt.Errorf("searchworld: listThreadMessages %s returned no message after seq %d", th.ID, after)
	}
	return nil
}

// WriteResult is what CheckWrites found.
type WriteResult struct {
	// Messages are the ids of the messages and threads the product wrote.
	Messages, Threads []string
}

// CheckWrites writes through the product after the measurement (design note 6) and compares with
// the loader: a German author posts into a loaded post and an English author into a loaded main
// thread (createMessage), each must get thread_seq = the loaded last_message_seq + 1 and the
// search_language (read through the admin pool db) the loader wrote for that author; a German and
// an English author each create a forum post (createThread), which must get the
// threads.search_language the loader wrote for that author. The error names ids and values, never
// content.
func CheckWrites(ctx context.Context, w *World, rows *searchbench.Rows, db *pgxpool.Pool) (WriteResult, error) {
	var res WriteResult
	p := w.Plan
	german, english := 0, p.Authors-1
	post := rows.Threads[0]
	main := rows.Threads[rows.MainThread(firstText(p))]
	for _, c := range []struct {
		author int
		th     searchbench.ThreadRow
	}{{german, post}, {english, main}} {
		a := w.Authors[c.author]
		var m struct {
			ID        string `json:"id"`
			ThreadSeq int64  `json:"thread_seq"`
		}
		body := map[string]any{"client_msg_id": ids.New().String(), "content_md": "Check " + a.Handle}
		if err := do(ctx, a.Client, http.MethodPost, "/threads/"+c.th.ID+"/messages", body, http.StatusCreated, &m); err != nil {
			return res, err
		}
		if m.ThreadSeq != c.th.LastMessageSeq+1 {
			return res, fmt.Errorf("searchworld: createMessage into %s got thread_seq %d, the loader ended at %d", c.th.ID, m.ThreadSeq, c.th.LastMessageSeq)
		}
		var lang string
		if err := db.QueryRow(ctx, `SELECT search_language::text FROM messages WHERE id = $1`, m.ID).Scan(&lang); err != nil {
			return res, err
		}
		if want := p.AuthorConfig(c.author); lang != want {
			return res, fmt.Errorf("searchworld: the product stored search_language %s for %s, the loader wrote %s", lang, a.Handle, want)
		}
		res.Messages = append(res.Messages, m.ID)
	}
	forum := p.PostChannel(0)
	for _, author := range []int{german, english} {
		a := w.Authors[author]
		body := map[string]any{"title": "Check " + a.Handle, "first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "Check " + a.Handle}}
		var th struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if err := do(ctx, a.Client, http.MethodPost, "/channels/"+w.Channels[forum]+"/threads", body, http.StatusCreated, &th); err != nil {
			return res, err
		}
		var lang string
		if err := db.QueryRow(ctx, `SELECT search_language::text FROM threads WHERE id = $1`, th.Thread.ID).Scan(&lang); err != nil {
			return res, err
		}
		if want := p.AuthorConfig(author); lang != want {
			return res, fmt.Errorf("searchworld: the product stored threads.search_language %s for %s, the loader wrote %s", lang, a.Handle, want)
		}
		res.Threads = append(res.Threads, th.Thread.ID)
	}
	return res, nil
}

// firstText returns the first text channel of the plan.
func firstText(p *searchbench.Plan) int {
	for c := range p.Channels {
		if p.ChannelKind(c) == searchbench.TextChannel {
			return c
		}
	}
	return 0
}

// SearchPath returns the path of GET /search with q and the defaults of openapi.yaml (design note
// 8); PrefixPath that of GET /search/thread-titles (design note 9).
func SearchPath(q string) string { return "/search?q=" + url.QueryEscape(q) }

// PrefixPath returns the path of the prefix search for q.
func PrefixPath(q string) string { return "/search/thread-titles?q=" + url.QueryEscape(q) }
