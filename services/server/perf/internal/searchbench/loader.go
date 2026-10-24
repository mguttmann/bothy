// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"iter"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// WorldIDs are the ids of a seeded world that the content rows refer to, by the numbers of the
// plan: the channel and its main thread ("" for a forum channel) per channel, the principal per
// author.
type WorldIDs struct {
	Channels    []string
	MainThreads []string
	Authors     []string
}

// ThreadRow is a thread with its counters after the load: a post (Post >= 0) is inserted with
// them, a main thread (Post == -1) is updated to them. BaseSeq and BaseVersion are the
// last_message_seq and version of a main thread before the load.
type ThreadRow struct {
	Post           int
	Channel        int
	ID             string
	Title          string
	CreatedBy      string
	CreatedAt      time.Time
	SearchLanguage string
	LastMessageSeq int64
	MessageCount   int64
	LastActivityAt time.Time
	Version        int64
	BaseSeq        int64
	BaseVersion    int64
	// appended and deleted count the loaded messages and deletions of the thread.
	appended, deleted int64
}

// MessageRow is one row of messages as the loader writes it (design note 6).
type MessageRow struct {
	Index          int
	ID             ids.ID
	ThreadID       string
	ChannelID      string
	ThreadSeq      int64
	AuthorID       string
	ClientMsgID    ids.ID
	PayloadSHA256  [32]byte
	Content        string
	CreatedAt      time.Time
	SearchLanguage string
	Deleted        bool
}

// Rows are the content rows of a corpus in a world: the threads with their final counters and,
// per message, its thread and thread_seq. BuildRows computes them in one pass over the meta data;
// Messages builds the rows with their texts and ids in a second pass.
type Rows struct {
	Corpus  *Corpus
	World   WorldIDs
	Threads []ThreadRow
	// thread and seq are the thread (index into Threads) and the thread_seq of each message.
	thread []int32
	seq    []int32
	// mainOf maps a text channel to the index of its main thread in Threads.
	mainOf map[int]int
}

// The id columns the loader fills, each with its own generator (design note 6).
const (
	columnThreadID = iota + 1
	columnMessageID
	columnClientMsgID
)

// idGenerator returns a UUIDv7 generator of column whose clock the caller sets to the row's
// created_at before each id, with a ChaCha8 reader seeded from (seed, column), so the ids follow
// time (03 principle 1) and every pass gives the same ids.
func idGenerator(seed uint64, column int) (*ids.Generator, *clock.Fake) {
	var key [32]byte
	binary.LittleEndian.PutUint64(key[:8], seed)
	binary.LittleEndian.PutUint64(key[8:16], uint64(column))
	clk := clock.NewFake(time.Unix(0, 0))
	return ids.NewGenerator(clk, rand.NewChaCha8(key)), clk
}

// BuildRows computes the post threads (ids, titles, creators, languages) and, over the meta data
// of every message in index order, each message's thread_seq (gapless per thread, from 1 in a post
// and after base[mainThread] in a main thread) and the final counters of every thread:
// last_message_seq, message_count without deleted messages, last_activity_at of the newest
// message, version raised once per message and once per deletion as the product does. base holds
// the last_message_seq and version of each main thread before the load. The error reports a world
// that does not match the plan.
func BuildRows(c *Corpus, w WorldIDs, base map[string][2]int64) (*Rows, error) {
	p := c.Plan
	if len(w.Channels) != p.Channels || len(w.MainThreads) != p.Channels || len(w.Authors) != p.Authors {
		return nil, fmt.Errorf("searchbench: the world has %d channels and %d authors, the plan %d and %d", len(w.Channels), len(w.Authors), p.Channels, p.Authors)
	}
	r := &Rows{Corpus: c, World: w, thread: make([]int32, p.Messages), seq: make([]int32, p.Messages), mainOf: map[int]int{}}
	gen, clk := idGenerator(p.Seed, columnThreadID)
	for post := range p.Posts {
		creator := c.PostCreator(post)
		at := c.CreatedAt(post)
		clk.Set(at)
		r.Threads = append(r.Threads, ThreadRow{Post: post, Channel: p.PostChannel(post), ID: gen.New().String(), Title: c.Title(post),
			CreatedBy: w.Authors[creator], CreatedAt: at, SearchLanguage: p.AuthorConfig(creator), Version: 1, BaseVersion: 1})
	}
	for ch := range p.Channels {
		if p.ChannelKind(ch) != TextChannel {
			continue
		}
		id := w.MainThreads[ch]
		b, ok := base[id]
		if id == "" || !ok {
			return nil, fmt.Errorf("searchbench: text channel %d has no main thread with a base", ch)
		}
		r.mainOf[ch] = len(r.Threads)
		r.Threads = append(r.Threads, ThreadRow{Post: -1, Channel: ch, ID: id, LastMessageSeq: b[0], BaseSeq: b[0], Version: b[1], BaseVersion: b[1]})
	}
	for i := range p.Messages {
		m := c.Meta(i)
		t := r.mainOf[m.Channel]
		if m.Post >= 0 {
			t = m.Post
		}
		th := &r.Threads[t]
		th.LastMessageSeq++
		th.appended++
		th.Version++
		if m.Deleted {
			th.deleted++
			th.Version++
		} else {
			th.MessageCount++
		}
		th.LastActivityAt = m.CreatedAt
		r.thread[i], r.seq[i] = int32(t), int32(th.LastMessageSeq) // #nosec G115 -- a thread holds far fewer than 2^31 messages
	}
	return r, nil
}

// Row returns the row of message i without its ids.
func (r *Rows) Row(i int) MessageRow {
	m := r.Corpus.Message(i)
	th := r.Threads[r.thread[i]]
	content := m.Text
	if m.Deleted {
		content = ""
	}
	return MessageRow{Index: i, ThreadID: th.ID, ChannelID: r.World.Channels[m.Channel], ThreadSeq: int64(r.seq[i]),
		AuthorID: r.World.Authors[m.Author], PayloadSHA256: sha256.Sum256([]byte(m.Text)), Content: content, CreatedAt: m.CreatedAt,
		SearchLanguage: r.Corpus.Plan.AuthorConfig(m.Author), Deleted: m.Deleted}
}

// Messages yields the rows of every message in index order with their ids: id and client_msg_id
// are UUIDv7 from their own generators at the row's created_at; client_payload_sha256 is the
// SHA-256 of the text as sent; a deleted row has an empty content.
func (r *Rows) Messages() iter.Seq[MessageRow] {
	return func(yield func(MessageRow) bool) {
		idGen, idClk := idGenerator(r.Corpus.Plan.Seed, columnMessageID)
		cmGen, cmClk := idGenerator(r.Corpus.Plan.Seed, columnClientMsgID)
		for i := range r.Corpus.Plan.Messages {
			row := r.Row(i)
			idClk.Set(row.CreatedAt)
			cmClk.Set(row.CreatedAt)
			row.ID, row.ClientMsgID = idGen.New(), cmGen.New()
			if !yield(row) {
				return
			}
		}
	}
}

// ThreadMessages returns the indexes of the messages of thread t (an index into Threads) in
// thread_seq order.
func (r *Rows) ThreadMessages(t int) []int {
	var out []int
	for i, th := range r.thread {
		if int(th) == t {
			out = append(out, i)
		}
	}
	return out
}

// ThreadOf returns the index into Threads and the thread_seq of message i.
func (r *Rows) ThreadOf(i int) (int, int64) { return int(r.thread[i]), int64(r.seq[i]) }

// MainThread returns the index into Threads of the main thread of text channel ch.
func (r *Rows) MainThread(ch int) int { return r.mainOf[ch] }

// LoadStats are the durations and counts of a load.
type LoadStats struct {
	Threads, Messages, Deleted int
	CopyThreads, CopyMessages  time.Duration
	UpdateMainThreads, Vacuum  time.Duration
	Check                      time.Duration
}

// Map returns the stats for extra.search of results.json.
func (s LoadStats) Map() map[string]any {
	sec := func(d time.Duration) float64 { return d.Seconds() }
	return map[string]any{"post_threads": s.Threads, "messages": s.Messages, "deleted_messages": s.Deleted,
		"copy_threads_seconds": sec(s.CopyThreads), "copy_messages_seconds": sec(s.CopyMessages),
		"update_main_threads_seconds": sec(s.UpdateMainThreads), "vacuum_analyze_seconds": sec(s.Vacuum), "check_seconds": sec(s.Check)}
}

// The columns the loader writes; every other column keeps its database default and the generated
// columns compute the vectors.
const (
	threadColumns  = "id, channel_id, kind, title, title_source, state, created_by, created_at, search_language, last_message_seq, message_count, unread_floor_seq, last_activity_at, version"
	messageColumns = "id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind, content_md, created_at, search_language, deleted_at, deleted_by, version"
)

// MainBase reads last_message_seq and version of the main threads of w through the admin pool db,
// the base of BuildRows.
func MainBase(ctx context.Context, db *pgxpool.Pool, w WorldIDs) (map[string][2]int64, error) {
	var main []string
	for _, id := range w.MainThreads {
		if id != "" {
			main = append(main, id)
		}
	}
	rows, err := db.Query(ctx, `SELECT id::text, last_message_seq, version FROM threads WHERE id = ANY($1::uuid[]) AND kind = 'main'`, main)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][2]int64{}
	for rows.Next() {
		var id string
		var seq, version int64
		if err := rows.Scan(&id, &seq, &version); err != nil {
			return nil, err
		}
		out[id] = [2]int64{seq, version}
	}
	return out, rows.Err()
}

// Load writes the rows through the admin pool db of a throwaway database (design note 6): COPY of
// the post threads, then the messages in batches of LoadBatch rows (one transaction each), then one
// UPDATE of the counters of the main threads, then VACUUM (ANALYZE) of messages and threads, then
// the checks: the count of messages equals the plan's N, and per thread last_message_seq equals
// the largest thread_seq and the number of rows, message_count the number of rows that are not
// deleted, and last_activity_at the newest created_at. Checks, foreign keys and unique keys are
// enforced by the database during COPY. No server may serve a request on the database meanwhile.
// The error names counts and ids, never content.
func Load(ctx context.Context, db *pgxpool.Pool, r *Rows) (LoadStats, error) {
	p := r.Corpus.Plan
	st := LoadStats{Threads: p.Posts, Messages: p.Messages}
	t0 := time.Now()
	var buf bytes.Buffer
	for _, th := range r.Threads[:p.Posts] {
		writeLine(&buf, th.ID, r.World.Channels[th.Channel], "thread", th.Title, "manual", "open", th.CreatedBy, stamp(th.CreatedAt),
			th.SearchLanguage, itoa(th.LastMessageSeq), itoa(th.MessageCount), itoa(th.LastMessageSeq), stamp(th.LastActivityAt), itoa(th.Version))
	}
	if err := copyRows(ctx, db, "threads", threadColumns, &buf); err != nil {
		return st, err
	}
	st.CopyThreads = time.Since(t0)
	t0 = time.Now()
	n := 0
	for row := range r.Messages() {
		deletedAt, deletedBy, version := null, null, "1"
		if row.Deleted {
			deletedAt, deletedBy, version = stamp(row.CreatedAt), row.AuthorID, "2"
			st.Deleted++
		}
		writeLine(&buf, row.ID.String(), row.ThreadID, row.ChannelID, itoa(row.ThreadSeq), row.AuthorID, row.ClientMsgID.String(),
			`\x`+hex.EncodeToString(row.PayloadSHA256[:]), "human", "text", row.Content, stamp(row.CreatedAt), row.SearchLanguage,
			deletedAt, deletedBy, version)
		if n++; n%p.LoadBatch == 0 {
			if err := copyRows(ctx, db, "messages", messageColumns, &buf); err != nil {
				return st, fmt.Errorf("%w (batch ending at message %d)", err, n)
			}
		}
	}
	if buf.Len() > 0 {
		if err := copyRows(ctx, db, "messages", messageColumns, &buf); err != nil {
			return st, err
		}
	}
	st.CopyMessages = time.Since(t0)
	t0 = time.Now()
	if err := updateMainThreads(ctx, db, r); err != nil {
		return st, err
	}
	st.UpdateMainThreads = time.Since(t0)
	t0 = time.Now()
	for _, table := range []string{"messages", "threads"} {
		if _, err := db.Exec(ctx, "VACUUM (ANALYZE) "+table); err != nil {
			return st, fmt.Errorf("searchbench: VACUUM (ANALYZE) %s: %w", table, err)
		}
	}
	st.Vacuum = time.Since(t0)
	t0 = time.Now()
	err := CheckLoad(ctx, db, r)
	st.Check = time.Since(t0)
	return st, err
}

// updateMainThreads sets the counters of the main threads in one UPDATE: last_message_seq and
// unread_floor_seq (the import rule of 03 threads), message_count, last_activity_at and version.
func updateMainThreads(ctx context.Context, db *pgxpool.Pool, r *Rows) error {
	var idsList []string
	var seqs, counts, versions []int64
	var last []time.Time
	for _, th := range r.Threads[r.Corpus.Plan.Posts:] {
		if th.appended == 0 {
			continue
		}
		idsList, seqs, counts, versions, last = append(idsList, th.ID), append(seqs, th.LastMessageSeq), append(counts, th.MessageCount), append(versions, th.Version), append(last, th.LastActivityAt)
	}
	tag, err := db.Exec(ctx, `UPDATE threads t SET last_message_seq = v.seq, unread_floor_seq = v.seq, message_count = t.message_count + v.n,
		last_activity_at = v.last, version = v.version
		FROM unnest($1::uuid[], $2::bigint[], $3::bigint[], $4::timestamptz[], $5::bigint[]) AS v(id, seq, n, last, version)
		WHERE t.id = v.id AND t.kind = 'main'`, idsList, seqs, counts, last, versions)
	if err != nil {
		return fmt.Errorf("searchbench: updating the main threads: %w", err)
	}
	if int(tag.RowsAffected()) != len(idsList) {
		return fmt.Errorf("searchbench: updated %d of %d main threads", tag.RowsAffected(), len(idsList))
	}
	return nil
}

// CheckLoad runs the checks of Load after the load: the count of messages equals N, and the
// counters of every thread of r agree with its rows. The error names counts and up to ten thread
// ids.
func CheckLoad(ctx context.Context, db *pgxpool.Pool, r *Rows) error {
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count); err != nil {
		return err
	}
	if count != r.Corpus.Plan.Messages {
		return fmt.Errorf("searchbench: messages holds %d rows, %s is %d", count, KeyBenchmarkMessages, r.Corpus.Plan.Messages)
	}
	threadIDs := make([]string, len(r.Threads))
	for i, th := range r.Threads {
		threadIDs[i] = th.ID
	}
	rows, err := db.Query(ctx, `WITH a AS (
		SELECT thread_id, max(thread_seq) AS max_seq, count(*) AS n, count(*) FILTER (WHERE deleted_at IS NULL) AS live, max(created_at) AS last
		FROM messages GROUP BY thread_id)
	SELECT t.id::text FROM threads t LEFT JOIN a ON a.thread_id = t.id
	WHERE t.id = ANY($1::uuid[])
	  AND (t.last_message_seq <> coalesce(a.max_seq, 0) OR t.last_message_seq <> coalesce(a.n, 0) OR t.message_count <> coalesce(a.live, 0)
	       OR t.unread_floor_seq <> t.last_message_seq OR (a.last IS NOT NULL AND t.last_activity_at <> a.last))
	ORDER BY t.id LIMIT 10`, threadIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	var bad []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		bad = append(bad, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(bad) > 0 {
		return fmt.Errorf("searchbench: the counters of threads %s do not match their messages", strings.Join(bad, ", "))
	}
	return nil
}

// copyRows sends the COPY text lines of buf into table and empties buf.
func copyRows(ctx context.Context, db *pgxpool.Pool, table, columns string, buf *bytes.Buffer) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, err = conn.Conn().PgConn().CopyFrom(ctx, buf, "COPY "+table+" ("+columns+") FROM STDIN")
	buf.Reset()
	if err != nil {
		return fmt.Errorf("searchbench: COPY %s: %w", table, err)
	}
	return nil
}

// null is the field value writeLine writes as NULL; PostgreSQL text never holds a NUL byte, so no
// value can be mistaken for it.
const null = "\x00"

// writeLine appends one row in the text format of COPY: the fields escaped and separated by tabs,
// null as \N. A bytea field is passed as `\x` and hex digits; the escaping doubles its backslash
// as COPY expects.
func writeLine(buf *bytes.Buffer, fields ...string) {
	for k, f := range fields {
		if k > 0 {
			buf.WriteByte('\t')
		}
		if f == null {
			buf.WriteString(`\N`)
			continue
		}
		buf.WriteString(copyEscaper.Replace(f))
	}
	buf.WriteByte('\n')
}

// copyEscaper escapes the characters the text format of COPY treats specially.
var copyEscaper = strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\n", `\n`, "\r", `\r`)

// stamp formats t for COPY with microseconds in UTC.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05.000000+00") }

// itoa formats n.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }
