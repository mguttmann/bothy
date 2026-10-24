// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// seedExtract stores a deliverable attachment of mime with content and, with row, its pending
// extract row.
func (e *extractEnv) seedExtract(t *testing.T, mime string, content []byte, row bool) app.FileRecord {
	t.Helper()
	id := ids.New()
	key, err := domain.OriginalKey(id, 1)
	if err != nil {
		t.Fatal(err)
	}
	f := app.FileRecord{ID: id, ObjectKey: key, SizeBytes: int64(len(content)), MIMEType: mime, Purpose: "attachment",
		UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean}
	if err := e.store.Put(context.Background(), key, bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	e.repo.mu.Lock()
	e.repo.files[id] = f
	if row {
		e.repo.rows[id] = &extractRow{ExtractWrite: app.ExtractWrite{State: domain.ExtractPending}, Lang: "german"}
	}
	e.repo.mu.Unlock()
	return f
}

func TestScheduleTextExtract(t *testing.T) {
	t.Parallel()
	e := newExtractEnv(t, extractCfg)
	ctx := context.Background()
	schedule := func(f app.FileRecord) error {
		return e.uow.Do(ctx, func(ctx context.Context) error { return app.ScheduleTextExtract(e.ext.Trigger(), ctx, f) })
	}
	file := func(purpose, mime string) app.FileRecord {
		return app.FileRecord{ID: ids.New(), Purpose: purpose, MIMEType: mime, UploadState: domain.UploadComplete, ScanStatus: domain.ScanSkipped}
	}
	ok := file("attachment", "text/plain")
	for _, f := range []app.FileRecord{ok, ok, file("attachment", "image/png"), file("avatar", "text/plain"), file("gdpr_export", extractDOCX)} {
		if err := schedule(f); err != nil {
			t.Fatal(err)
		}
	}
	jobs, _ := e.queue.jobs()
	if !slices.Equal(jobs, []ids.ID{ok.ID}) {
		t.Fatalf("jobs %v; want one for %s", jobs, ok.ID)
	}
	row, found := e.repo.row(ok.ID)
	if !found || row.State != domain.ExtractPending || row.Lang != "german" || len(e.repo.rows) != 1 {
		t.Fatalf("row %+v found %v rows %d", row, found, len(e.repo.rows))
	}
	if err := app.ScheduleTextExtract(e.ext.Trigger(), ctx, file("attachment", "text/plain")); err == nil {
		t.Fatal("schedule outside a unit of work succeeded")
	}
}

// TestScanPassSchedulesExtract: the scan pass that makes an attachment deliverable creates the
// row and the job in its unit of work; a blocked file gets none.
func TestScanPassSchedulesExtract(t *testing.T) {
	t.Parallel()
	content := []byte("Rechnung von Alex")
	for _, c := range []struct {
		verdict domain.VerdictKind
		want    bool
	}{{domain.VerdictClean, true}, {domain.VerdictInfected, false}, {domain.VerdictError, false}} {
		e := newScanEnvTypes(t, scanCfg, fixedType("text/plain"))
		e.scanner.verdict = domain.Verdict{Kind: c.verdict, Signature: "Test.Sig", Reason: domain.ReasonMalwareDetected}
		s := e.seed(t, content, content, true)
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
		jobs, _ := e.xqueue.jobs()
		_, row := e.extracts.row(s.FileID)
		if row != c.want || (len(jobs) == 1) != c.want {
			t.Errorf("verdict %v: row %v jobs %v", c.verdict, row, jobs)
		}
	}
}

func TestExtractTextRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("not pending", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		e.repo.rows[f.ID].State = domain.ExtractDone
		if err := e.ext.Text.Run(ctx, f.ID); err != nil || len(e.repo.writes) != 0 || e.store.count("Get") != 0 {
			t.Fatalf("%v writes %d", err, len(e.repo.writes))
		}
		if err := e.ext.Text.Run(ctx, ids.New()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("deleted", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		f.Deleted = true
		e.repo.files[f.ID] = f
		if err := e.ext.Text.Run(ctx, f.ID); err != nil || e.store.count("Get") != 0 {
			t.Fatal(err)
		}
		if _, ok := e.repo.row(f.ID); ok {
			t.Fatal("row kept")
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/rtf", []byte("x"), true)
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.State != domain.ExtractUnsupported || row.Version != "" || row.Code != "" || row.At.IsZero() || e.store.count("Get") != 0 {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("too large", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", bytes.Repeat([]byte("a"), 1001), true)
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.State != domain.ExtractFailed || row.Code != "input_too_large" || row.Version != "builtin-text/1" || e.store.count("Get") != 0 {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("bytes beyond the stored size", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", bytes.Repeat([]byte("a"), 2000), true)
		f.SizeBytes = 10
		e.repo.files[f.ID] = f
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.Code != "input_too_large" || e.x.read > extractCfg.MaxInputBytes {
			t.Fatalf("%+v read %d", row, e.x.read)
		}
	})
	t.Run("done", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		e.x.result = app.ExtractResult{Text: "Rechnung", Truncated: true}
		f := e.seedExtract(t, "text/plain", []byte("Rechnung"), true)
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.State != domain.ExtractDone || row.Text != "Rechnung" || !row.Truncated || row.Version != "builtin-text/1" ||
			!row.At.Equal(fakeClock().Now()) || row.Code != "" {
			t.Fatalf("%+v", row)
		}
		// A repeated job finds the row done.
		if err := e.ext.Text.Run(ctx, f.ID); err != nil || len(e.repo.writes) != 1 {
			t.Fatalf("repeat: %v writes %d", err, len(e.repo.writes))
		}
	})
	t.Run("extractor code", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		e.x.err = domain.NewExtractError(domain.CodeXMLDoctype)
		f := e.seedExtract(t, extractDOCX, []byte("PK"), true)
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.State != domain.ExtractFailed || row.Code != "xml_doctype" || row.Text != "" || row.Version != "builtin-ooxml/1" {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("object missing", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		if err := e.store.Delete(ctx, f.ObjectKey); err != nil {
			t.Fatal(err)
		}
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		row, _ := e.repo.row(f.ID)
		if row.State != domain.ExtractFailed || row.Code != "object_missing" {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("transient read error", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		boom := errors.New("storage down while reading")
		e.x.err = boom
		if err := e.ext.Text.Run(ctx, f.ID); !errors.Is(err, boom) {
			t.Fatalf("%v", err)
		}
		if row, _ := e.repo.row(f.ID); row.State != domain.ExtractPending {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		cfg := extractCfg
		cfg.Timeout = time.Nanosecond
		e := newExtractEnv(t, cfg)
		e.x.stall = true
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		if err := e.ext.Text.Run(ctx, f.ID); err != nil {
			t.Fatal(err)
		}
		if row, _ := e.repo.row(f.ID); row.State != domain.ExtractFailed || row.Code != "timeout" {
			t.Fatalf("%+v", row)
		}
	})
	t.Run("cancelled job", func(t *testing.T) {
		t.Parallel()
		e := newExtractEnv(t, extractCfg)
		e.x.stall, e.x.started = true, make(chan struct{})
		f := e.seedExtract(t, "text/plain", []byte("x"), true)
		cctx, cancel := context.WithCancel(ctx)
		go func() {
			<-e.x.started
			cancel()
		}()
		if err := e.ext.Text.Run(cctx, f.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
		if row, _ := e.repo.row(f.ID); row.State != domain.ExtractPending {
			t.Fatalf("%+v", row)
		}
	})
}

func TestExtractTextVectorLimit(t *testing.T) {
	t.Parallel()
	e := newExtractEnv(t, extractCfg)
	text := strings.Repeat("aü", 10) // 30 bytes
	e.x.result = app.ExtractResult{Text: text}
	e.repo.tooLarge = 2
	f := e.seedExtract(t, "text/plain", []byte("x"), true)
	if err := e.ext.Text.Run(context.Background(), f.ID); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.writes) != 3 {
		t.Fatalf("writes %d", len(e.repo.writes))
	}
	row, _ := e.repo.row(f.ID)
	want := domain.HalveText(domain.HalveText(text))
	if row.Text != want || !row.Truncated || !utf8.ValidString(row.Text) || e.repo.writes[0].Text == e.repo.writes[1].Text {
		t.Fatalf("%q truncated %v", row.Text, row.Truncated)
	}
}

func TestExtractRefreshRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := extractCfg
	cfg.RefreshBatchSize = 10
	e := newExtractEnv(t, cfg)
	set := func(f app.FileRecord, w app.ExtractWrite) {
		e.repo.rows[f.ID] = &extractRow{ExtractWrite: w, Lang: "german"}
	}
	noRow := e.seedExtract(t, "text/plain", []byte("a"), false)
	noRowUnsupported := e.seedExtract(t, "image/png", []byte("a"), false)
	pending := e.seedExtract(t, "text/plain", []byte("a"), true)
	old := e.seedExtract(t, extractDOCX, []byte("a"), false)
	set(old, app.ExtractWrite{State: domain.ExtractDone, Version: "builtin-ooxml/0", Text: "alt"})
	current := e.seedExtract(t, extractDOCX, []byte("a"), false)
	set(current, app.ExtractWrite{State: domain.ExtractDone, Version: "builtin-ooxml/1", Text: "neu"})
	tooLarge := e.seedExtract(t, "text/plain", []byte("a"), false)
	set(tooLarge, app.ExtractWrite{State: domain.ExtractFailed, Version: "builtin-text/1", Code: "input_too_large"})
	changed := e.seedExtract(t, extractDOCX, []byte("a"), false)
	set(changed, app.ExtractWrite{State: domain.ExtractDone, Version: "builtin-ooxml/0"})
	gone := e.seedExtract(t, "text/plain", []byte("a"), false)
	set(gone, app.ExtractWrite{State: domain.ExtractDone, Version: "builtin-text/1"})
	g := e.repo.files[gone.ID]
	g.Deleted = true
	e.repo.files[gone.ID] = g
	e.repo.beforeReset = func(id ids.ID) {
		if id == changed.ID {
			e.repo.mu.Lock()
			e.repo.rows[id].Version = "builtin-ooxml/1"
			e.repo.mu.Unlock()
		}
	}
	if err := e.ext.Refresh.Run(ctx, ids.ID{}); err != nil {
		t.Fatal(err)
	}
	jobs, next := e.queue.jobs()
	want := []ids.ID{noRow.ID, pending.ID, old.ID, tooLarge.ID}
	slices.SortFunc(want, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	if !slices.Equal(jobs, want) || len(next) != 0 {
		t.Fatalf("jobs %v; want %v; next %v", jobs, want, next)
	}
	if row, _ := e.repo.row(old.ID); row.State != domain.ExtractPending || row.Text != "alt" || row.Version != "builtin-ooxml/0" {
		t.Fatalf("old row %+v: reset keeps the text", row)
	}
	if row, _ := e.repo.row(noRow.ID); row.State != domain.ExtractPending || row.Lang != "german" {
		t.Fatalf("created row %+v", row)
	}
	if _, ok := e.repo.row(noRowUnsupported.ID); ok {
		t.Fatal("unsupported file got a row")
	}
	if row, _ := e.repo.row(current.ID); row.State != domain.ExtractDone {
		t.Fatalf("current row %+v", row)
	}
	if row, _ := e.repo.row(changed.ID); row.State != domain.ExtractDone {
		t.Fatalf("changed row %+v: the compare-and-set must skip it", row)
	}
	if _, ok := e.repo.row(gone.ID); ok {
		t.Fatal("the row of a deleted file stayed")
	}

	// A full batch inserts the next run after the last file id.
	small := extractCfg
	small.RefreshBatchSize = 2
	e2 := newExtractEnv(t, small)
	a, b := e2.seedExtract(t, "text/plain", nil, false), e2.seedExtract(t, "text/plain", nil, false)
	c := e2.seedExtract(t, "text/plain", nil, false)
	order := []ids.ID{a.ID, b.ID, c.ID}
	slices.SortFunc(order, func(x, y ids.ID) int { return bytes.Compare(x[:], y[:]) })
	if err := e2.ext.Refresh.Run(ctx, ids.ID{}); err != nil {
		t.Fatal(err)
	}
	if _, next := e2.queue.jobs(); !slices.Equal(next, []ids.ID{order[1]}) {
		t.Fatalf("next %v; want after %s", next, order[1])
	}
	if err := e2.ext.Refresh.Run(ctx, order[1]); err != nil {
		t.Fatal(err)
	}
	if jobs, next := e2.queue.jobs(); len(jobs) != 3 || len(next) != 1 {
		t.Fatalf("second run: jobs %v next %v", jobs, next)
	}
}
