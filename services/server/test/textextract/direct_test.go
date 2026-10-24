// SPDX-License-Identifier: Apache-2.0

package textextract_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	filespg "github.com/OWNER/bothy/services/server/internal/files/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// The tests of this file build the use cases of the files module directly (brief M3-15 design
// note 14): the real repository on a pgtest database, the file system driver in a temporary root,
// the real extract.Registry or a wrapper around it, and a queue that records the jobs, which the
// test runs by calling the use cases. Production code has no option for a second extractor
// version; the wrapper stands in for a new binary.

// direct is one database with the files of Alex in Support/Allgemein and the use cases over it.
type direct struct {
	t      *testing.T
	conn   *pgx.Conn
	repo   *filespg.Repository
	uow    *pg.UnitOfWork
	store  app.ObjectStore
	clk    *clock.Fake
	queue  *jobQueue
	alex   ids.ID
	allg   ids.ID
	thread ids.ID
	seq    int
}

// jobQueue records the jobs the use cases insert, in order.
type jobQueue struct {
	mu      sync.Mutex
	extract []ids.ID
	refresh []ids.ID
}

func (q *jobQueue) EnqueueExtract(_ context.Context, id ids.ID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.extract = append(q.extract, id)
	return nil
}

func (q *jobQueue) EnqueueExtractRefresh(_ context.Context, after ids.ID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refresh = append(q.refresh, after)
	return nil
}

// take removes and returns the queued jobs.
func (q *jobQueue) take() (extracts, refreshes []ids.ID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	extracts, refreshes = q.extract, q.refresh
	q.extract, q.refresh = nil, nil
	return extracts, refreshes
}

// lang is the search language of the instance.
type lang string

func (l lang) InstanceSearchLanguage(context.Context) (string, error) { return string(l), nil }

func newDirect(t *testing.T) *direct {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	root, err := os.MkdirTemp("", "textextract-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	store, err := filesfs.Open(filesfs.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	d := &direct{t: t, conn: db.SuperConn(t), repo: filespg.NewRepository(pool), store: store,
		uow:   pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg),
		clk:   clock.NewFake(time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)),
		queue: &jobQueue{}, alex: ids.New(), allg: ids.New(), thread: ids.New()}
	space := ids.New()
	d.exec(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')`, d.alex)
	d.exec(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, 'Support', 'a', $2)`, space, d.alex)
	d.exec(`INSERT INTO channels (id, space_id, kind, name, sort_key, history_backfill_limit, created_by)
		VALUES ($1, $2, 'text', 'Allgemein', 'a', 20, $3)`, d.allg, space, d.alex)
	d.exec(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'thread', $3)`, d.thread, d.allg, d.alex)
	return d
}

func (d *direct) exec(q string, args ...any) {
	d.t.Helper()
	if _, err := d.conn.Exec(context.Background(), q, args...); err != nil {
		d.t.Fatalf("%s: %v", q, err)
	}
}

// extraction builds the use cases over x with the limits of cfg.
func (d *direct) extraction(x app.TextExtractor, cfg app.ExtractConfig) *app.Extraction {
	d.t.Helper()
	e, err := app.NewExtraction(cfg, app.ExtractDeps{Repo: d.repo, Store: d.store, UoW: d.uow, Extractor: x, Queue: d.queue,
		Languages: lang("german"), Clock: d.clk})
	if err != nil {
		d.t.Fatal(err)
	}
	return e
}

// attached stores content as a deliverable attachment of mime named name on a new message of Alex
// and returns its id.
func (d *direct) attached(name, mime string, content []byte) ids.ID {
	d.t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		d.t.Fatal(err)
	}
	if err := d.store.Put(context.Background(), key, bytes.NewReader(content), int64(len(content))); err != nil {
		d.t.Fatal(err)
	}
	d.exec(`INSERT INTO files (id, object_key, sha256, size_bytes, mime_type, original_name, uploaded_by, purpose, scope_channel_id,
		upload_state, scan_status, created_at)
		VALUES ($1, $2, '\x01', $3, $4, $5, $6, 'attachment', $7, 'complete', 'clean', $8)`,
		id, key, len(content), mime, name, d.alex, d.allg, d.clk.Now())
	d.seq++
	msg := ids.New()
	d.exec(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ($1, $2, $3, $4, $5, $6, '\x00', 'human', 'see the file')`, msg, d.thread, d.allg, d.seq, d.alex, ids.New())
	d.exec(`INSERT INTO message_attachments (message_id, file_id, position, thread_id, channel_id) VALUES ($1, $2, 1, $3, $4)`,
		msg, id, d.thread, d.allg)
	return id
}

// drain runs the queued jobs until none is left: files.extract_text with e.Text and
// files.extract_text_refresh with e.Refresh.
func (d *direct) drain(e *app.Extraction) {
	d.t.Helper()
	for range 100 {
		extracts, refreshes := d.queue.take()
		if len(extracts) == 0 && len(refreshes) == 0 {
			return
		}
		for _, id := range extracts {
			if err := e.Text.Run(context.Background(), id); err != nil {
				d.t.Fatalf("files.extract_text %s: %v", id, err)
			}
		}
		for _, after := range refreshes {
			if err := e.Refresh.Run(context.Background(), after); err != nil {
				d.t.Fatalf("files.extract_text_refresh: %v", err)
			}
		}
	}
	d.t.Fatal("the jobs did not settle")
}

// row is a file_text_extracts row as the tests read it.
type row struct {
	State, Extractor, Text, Error string
	ExtractedAt                   time.Time
	Truncated, Found              bool
}

func (d *direct) row(id ids.ID) row {
	d.t.Helper()
	var r row
	var extractor, text, errCode *string
	var at *time.Time
	err := d.conn.QueryRow(context.Background(), `SELECT state, extractor, text_content, error, extracted_at, truncated
		FROM file_text_extracts WHERE file_id = $1`, id).Scan(&r.State, &extractor, &text, &errCode, &at, &r.Truncated)
	if err == pgx.ErrNoRows { //nolint:errorlint // the sentinel of QueryRow
		return r
	}
	if err != nil {
		d.t.Fatal(err)
	}
	r.Found = true
	for _, p := range []struct {
		dst *string
		src *string
	}{{&r.Extractor, extractor}, {&r.Text, text}, {&r.Error, errCode}} {
		if p.src != nil {
			*p.dst = *p.src
		}
	}
	if at != nil {
		r.ExtractedAt = *at
	}
	return r
}

// found reports whether SearchFiles finds id for the tsquery tsq in Allgemein.
func (d *direct) found(tsq string, id ids.ID) bool {
	d.t.Helper()
	var p searchkit.Page
	err := d.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		p, err = d.repo.SearchFiles(ctx, searchkit.Query{TSQuery: tsq, ChannelIDs: []ids.ID{d.allg}, Sort: searchkit.SortNewest,
			RefTime: d.clk.Now().Add(time.Hour), Limits: searchkit.Limits{MaxCandidates: 100, RankNormalization: 1, RecencyScale: time.Hour,
				HeadlineInputMaxChars: 200, HeadlineMaxFragments: 1, HeadlineMaxWords: 10, HeadlineMinWords: 2, PageSize: 50}})
		return err
	})
	if err != nil {
		d.t.Fatal(err)
	}
	for _, h := range p.Hits {
		if h.ID == id {
			return true
		}
	}
	return false
}

// newVersion wraps the real registry as a binary with a new OOXML adapter would: it reports
// builtin-ooxml/2 for the OOXML types and supports text/rtf through the text adapter.
type newVersion struct{ reg *extract.Registry }

func (n newVersion) Version(mime string) (string, bool) {
	switch domain.MediaType(mime) {
	case "text/rtf":
		return extract.VersionText, true
	case extract.TypeDOCX, extract.TypeXLSX, extract.TypePPTX:
		return "builtin-ooxml/2", true
	}
	return n.reg.Version(mime)
}

func (n newVersion) Extract(ctx context.Context, r io.Reader, mime string) (domain.ExtractResult, error) {
	if domain.MediaType(mime) == "text/rtf" {
		return n.reg.Extract(ctx, r, "text/plain")
	}
	res, err := n.reg.Extract(ctx, r, mime)
	if v, ok := n.Version(mime); ok && err == nil {
		res.Version = v
	}
	return res, err
}

// directConfig are the limits of the direct tests.
var directConfig = app.ExtractConfig{MaxInputBytes: 1 << 20, Timeout: time.Minute, RefreshBatchSize: 100}

func newRegistry(t *testing.T) *extract.Registry {
	t.Helper()
	r, err := extract.New(extract.Config{MaxTextBytes: 1 << 16, MaxUncompressedBytes: 1 << 20, MaxArchiveEntries: 100})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// create creates the rows and jobs of the stored attachments: the walk creates the row of a
// deliverable attachment of a supported type exactly as the trigger after the scan pass does.
func (d *direct) create(e *app.Extraction) {
	d.t.Helper()
	if err := e.Refresh.Run(context.Background(), ids.ID{}); err != nil {
		d.t.Fatal(err)
	}
}

// TestNewExtractorVersionReextracts (plan row, 14 5.1 item 4): a DOCX and a text file are done
// under version 1 and an RTF file has no row; with a binary that reports builtin-ooxml/2 and
// supports text/rtf, and search.extract.refresh_batch_size 1, the walk chains its runs; the DOCX
// row is pending with its old text and still found meanwhile, then done under builtin-ooxml/2
// with a later extracted_at; the text row keeps its extracted_at; the RTF file gets a row and is
// extracted; a second walk changes nothing.
func TestNewExtractorVersionReextracts(t *testing.T) {
	t.Parallel()
	d := newDirect(t)
	reg := newRegistry(t)
	v1 := d.extraction(reg, directConfig)
	docx := d.attached("notes docx", extract.TypeDOCX, extracttest.DOCX("Der Lieferantenvertrag ist fertig."))
	txt := d.attached("plain text", "text/plain", []byte("Quartalsbericht"))
	rtf := d.attached("letter rtf", "text/rtf", []byte("Kuendigungsschreiben"))
	d.create(v1)
	d.drain(v1)
	first := d.row(docx)
	if first.State != "done" || first.Extractor != extract.VersionOOXML || d.row(txt).Extractor != extract.VersionText || d.row(rtf).Found {
		t.Fatalf("version 1: docx %+v text %+v rtf %+v", first, d.row(txt), d.row(rtf))
	}
	textAt := d.row(txt).ExtractedAt

	d.clk.Advance(time.Hour)
	cfg := directConfig
	cfg.RefreshBatchSize = 1
	v2 := d.extraction(newVersion{reg: reg}, cfg)
	if err := v2.Refresh.Run(context.Background(), ids.ID{}); err != nil {
		t.Fatal(err)
	}
	// Run only the walk first, so the reset rows are seen before their jobs run.
	runs := 1
	for {
		extracts, refreshes := d.queue.take()
		d.queue.mu.Lock()
		d.queue.extract = append(d.queue.extract, extracts...)
		d.queue.mu.Unlock()
		if len(refreshes) == 0 {
			break
		}
		for _, after := range refreshes {
			runs++
			if err := v2.Refresh.Run(context.Background(), after); err != nil {
				t.Fatal(err)
			}
		}
	}
	if runs != 4 {
		t.Fatalf("the walk ran %d times over three files with batch size 1; want 4", runs)
	}
	if r := d.row(docx); r.State != "pending" || r.Text != first.Text || r.Extractor != extract.VersionOOXML {
		t.Fatalf("docx during the refresh: %+v", r)
	}
	if !d.found("'lieferantenvertrag'", docx) {
		t.Fatal("the docx is not found by its old text during the refresh")
	}
	d.drain(v2)
	if r := d.row(docx); r.State != "done" || r.Extractor != "builtin-ooxml/2" || !r.ExtractedAt.After(first.ExtractedAt) {
		t.Fatalf("docx after the refresh: %+v", r)
	}
	if r := d.row(txt); !r.ExtractedAt.Equal(textAt) || r.Extractor != extract.VersionText {
		t.Fatalf("text row changed: %+v", r)
	}
	if r := d.row(rtf); r.State != "done" || r.Text != "Kuendigungsschreiben" {
		t.Fatalf("rtf: %+v", r)
	}
	before := []row{d.row(docx), d.row(txt), d.row(rtf)}
	if err := v2.Refresh.Run(context.Background(), ids.ID{}); err != nil {
		t.Fatal(err)
	}
	d.drain(v2)
	for i, id := range []ids.ID{docx, txt, rtf} {
		if d.row(id) != before[i] {
			t.Fatalf("a second walk changed %s: %+v", id, d.row(id))
		}
	}
}

// TestRefreshRecoversDiscardedJob: a pending row without a queued job is extracted after one walk;
// the walk also removes the row of a file that has deleted_at set.
func TestRefreshRecoversDiscardedJob(t *testing.T) {
	t.Parallel()
	d := newDirect(t)
	e := d.extraction(newRegistry(t), directConfig)
	id := d.attached("orphan job", "text/plain", []byte("Inventurliste"))
	gone := d.attached("deleted file", "text/plain", []byte("weg"))
	d.exec(`INSERT INTO file_text_extracts (file_id, search_language) VALUES ($1, 'german'), ($2, 'german')`, id, gone)
	d.exec(`UPDATE files SET deleted_at = $2 WHERE id = $1`, gone, d.clk.Now())
	if err := e.Refresh.Run(context.Background(), ids.ID{}); err != nil {
		t.Fatal(err)
	}
	d.drain(e)
	if r := d.row(id); r.State != "done" || r.Text != "Inventurliste" {
		t.Fatalf("%+v", r)
	}
	if d.row(gone).Found {
		t.Fatal("the row of the deleted file stayed")
	}
}

// stallingStore answers Get with a body that blocks until the context of the call ends.
type stallingStore struct{ app.ObjectStore }

func (s stallingStore) Get(ctx context.Context, _ string, _ *app.ByteRange) (io.ReadCloser, error) {
	return stallingBody{ctx: ctx}, nil
}

type stallingBody struct{ ctx context.Context } //nolint:containedctx // the body of one Get

func (b stallingBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}

func (stallingBody) Close() error { return nil }

// TestExtractTimeoutStallingStore (TestExtractLimits, timeout part): an object store whose body
// stalls ends the file failed with timeout under search.extract.timeout.
func TestExtractTimeoutStallingStore(t *testing.T) {
	t.Parallel()
	d := newDirect(t)
	id := d.attached("stalling", "text/plain", []byte("Text"))
	cfg := directConfig
	cfg.Timeout = time.Millisecond
	e, err := app.NewExtraction(cfg, app.ExtractDeps{Repo: d.repo, Store: stallingStore{d.store}, UoW: d.uow, Extractor: newRegistry(t),
		Queue: d.queue, Languages: lang("german"), Clock: d.clk})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Refresh.Run(context.Background(), ids.ID{}); err != nil {
		t.Fatal(err)
	}
	d.drain(e)
	if r := d.row(id); r.State != "failed" || r.Error != "timeout" || r.Text != "" {
		t.Fatalf("%+v", r)
	}
}
