// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
)

// extracted gives the file of p a file_text_extracts row in state with text in lang.
func (s *searchWorld) extracted(p placed, state, text, lang string) {
	s.t.Helper()
	var content any
	if state == "done" || state == "pending" {
		content = text
	}
	s.exec(`INSERT INTO file_text_extracts (file_id, state, text_content, search_language, extractor)
		VALUES ($1, $2, $3, $4::regconfig, 'builtin-text/1')`, p.file, state, content, lang)
}

// TestSearchFilesCombinesVectors holds 14 2 and 4.5 for the extracted text (brief M3-15 design
// note 11): a file found only by its text is a hit under the same visibility and deliverable
// rules; a name hit ranks above a text hit of the same age; a file found both ways appears once;
// text-only hits count toward the candidate cap; the headline shows the text with a stem match;
// rows in any state take part with their vector.
func TestSearchFilesCombinesVectors(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	at := s.base.Add(time.Hour)
	byName := s.add(spec{name: "invoice march", at: at})
	byText := s.add(spec{name: "notes.txt", at: at})
	s.extracted(byText, "done", "The invoice for March", "english")
	both := s.add(spec{name: "invoice two", at: s.base})
	s.extracted(both, "done", "another invoice inside", "english")
	refreshing := s.add(spec{name: "plan.txt", at: s.base.Add(-time.Hour)})
	s.extracted(refreshing, "pending", "old invoice text", "english")
	failed := s.add(spec{name: "broken.docx", at: s.base.Add(-2 * time.Hour)})
	s.extracted(failed, "failed", "", "english")
	hidden := s.add(spec{name: "secret.txt", at: at, thread: s.eskThread})
	s.extracted(hidden, "done", "invoice in another channel", "english")
	blocked := s.add(spec{name: "blocked.txt", at: at, scan: "infected"})
	s.extracted(blocked, "done", "invoice of a blocked file", "english")

	q := s.query(searchkit.SortRelevance, s.allgemein)
	// The coordinator builds the query over the configured languages plus simple (14 4.2): the
	// english stem and the word.
	q.TSQuery = "'invoic' | 'invoice'"
	p := s.search(q)
	got := hitIDs(p)
	if !sameSet(got, fileIDs(byName, byText, both, refreshing)) {
		t.Fatalf("hits %v; want name, text, both and the refreshing file", got)
	}
	if slices.Index(got, byName.file) > slices.Index(got, byText.file) {
		t.Fatalf("order %v: the name hit must rank above the text hit of the same age", got)
	}
	for _, h := range p.Hits {
		if h.ID != byText.file {
			continue
		}
		joined := fmt.Sprint(h.Segments)
		if !strings.Contains(joined, "{invoice true}") || !strings.Contains(joined, "March") {
			t.Fatalf("headline of the text hit %+v", h.Segments)
		}
	}

	// The stem of the language of the text (weight C) matches where the word does not: the
	// german stem of "Rechnungen" is "rechnung".
	var german bool
	stem := s.add(spec{name: "brief.txt", at: at})
	s.extracted(stem, "done", "Die Rechnungen sind fertig", "german")
	q.TSQuery = "'rechnung'"
	for _, h := range s.search(q).Hits {
		german = german || h.ID == stem.file
	}
	if !german {
		t.Fatal("the stem of the extracted text (weight C) does not match")
	}

	// The cap counts text-only files like name hits.
	for _, sort := range sorts {
		q := s.query(sort, s.allgemein)
		q.Limits.MaxCandidates = 3
		if p := s.search(q); !p.CandidatesCapped || len(p.Hits) != 3 {
			t.Fatalf("%s: capped %v hits %d", sort, p.CandidatesCapped, len(p.Hits))
		}
		q.Limits.MaxCandidates = 4
		if p := s.search(q); p.CandidatesCapped || len(p.Hits) != 4 {
			t.Fatalf("%s max 4: capped %v hits %d", sort, p.CandidatesCapped, len(p.Hits))
		}
	}
}

// TestRepository_TextExtracts holds the queries of the extraction (brief M3-15 design notes 2, 4,
// 8, 9 and 10) against PostgreSQL.
func TestRepository_TextExtracts(t *testing.T) {
	t.Parallel()
	s := newSearchWorld(t)
	ctx := context.Background()
	f := s.add(spec{name: "a.txt", at: s.base})
	inserted := false
	do := func(fn func(ctx context.Context) error) {
		t.Helper()
		if err := s.uow.Do(ctx, fn); err != nil {
			t.Fatal(err)
		}
	}
	do(func(ctx context.Context) error {
		var err error
		inserted, err = s.repo.InsertTextExtract(ctx, f.file, "german")
		return err
	})
	second := true
	do(func(ctx context.Context) error {
		var err error
		second, err = s.repo.InsertTextExtract(ctx, f.file, "english")
		return err
	})
	if !inserted || second {
		t.Fatalf("inserted %v second %v", inserted, second)
	}
	target, found, err := s.repo.ExtractTarget(ctx, f.file)
	if err != nil || !found || target.State != "pending" || target.File.MIMEType != "text/plain" || target.File.Purpose != "attachment" ||
		target.File.UploadState != "complete" || target.File.Deleted {
		t.Fatalf("target %+v %v %v", target, found, err)
	}
	var lang string
	if err := s.conn.QueryRow(ctx, `SELECT search_language::text FROM file_text_extracts WHERE file_id = $1`, f.file).Scan(&lang); err != nil || lang != "german" {
		t.Fatalf("language %q %v", lang, err)
	}
	// The tsvector limit: many distinct long tokens exceed it (SQLSTATE 54000).
	var b strings.Builder
	for i := range 200000 {
		fmt.Fprintf(&b, "tok%08xabcdefabcdefabcdefabcdefabcdef ", i)
	}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		_, err := s.repo.WriteExtract(ctx, f.file, app.ExtractWrite{State: "done", Text: b.String(), Version: "builtin-text/1", At: s.base})
		return err
	})
	if err != app.ErrVectorTooLarge { //nolint:errorlint // the sentinel itself
		t.Fatalf("large vector: %v", err)
	}
	var changed bool
	do(func(ctx context.Context) error {
		var err error
		changed, err = s.repo.WriteExtract(ctx, f.file, app.ExtractWrite{State: "done", Text: "", Version: "builtin-text/1", At: s.base})
		return err
	})
	var text *string
	if err := s.conn.QueryRow(ctx, `SELECT text_content FROM file_text_extracts WHERE file_id = $1`, f.file).Scan(&text); err != nil ||
		!changed || text == nil || *text != "" {
		t.Fatalf("empty text stored as %v (changed %v, %v)", text, changed, err)
	}
	do(func(ctx context.Context) error {
		var err error
		changed, err = s.repo.WriteExtract(ctx, f.file, app.ExtractWrite{State: "failed", Code: "timeout", At: s.base})
		return err
	})
	if changed {
		t.Fatal("a second write changed a row that is not pending")
	}
	// Reset is a compare-and-set that keeps the text.
	var reset, stale bool
	do(func(ctx context.Context) error {
		var err error
		if stale, err = s.repo.ResetTextExtract(ctx, f.file, "done", "builtin-text/0"); err != nil {
			return err
		}
		reset, err = s.repo.ResetTextExtract(ctx, f.file, "done", "builtin-text/1")
		return err
	})
	if stale || !reset {
		t.Fatalf("reset stale %v reset %v", stale, reset)
	}
	other := s.add(spec{name: "b.txt", at: s.base})
	var cands []app.RefreshCandidate
	do(func(ctx context.Context) error {
		var err error
		cands, err = s.repo.RefreshCandidates(ctx, ids.ID{}, 10)
		return err
	})
	if len(cands) != 2 {
		t.Fatalf("candidates %+v", cands)
	}
	for _, c := range cands {
		if (c.FileID == f.file && (!c.HasRow || c.State != "pending" || c.Extractor != "builtin-text/1")) || (c.FileID == other.file && c.HasRow) {
			t.Fatalf("candidate %+v", c)
		}
	}
	// Deleting the file through markDeleted removes the row; the orphan sweep removes rows the
	// sweep finds.
	do(func(ctx context.Context) error {
		if _, err := s.repo.InsertTextExtract(ctx, other.file, "simple"); err != nil {
			return err
		}
		if _, err := s.repo.MarkFilesDeleted(ctx, app.Deletion{FileIDs: []ids.ID{f.file}}, s.base); err != nil {
			return err
		}
		return s.repo.DeleteTextExtractsOfDeleted(ctx, []ids.ID{f.file, other.file})
	})
	if _, found, _ := s.repo.ExtractTarget(ctx, f.file); found {
		t.Fatal("the row of the deleted file stayed")
	}
	if _, found, _ := s.repo.ExtractTarget(ctx, other.file); !found {
		t.Fatal("the row of a live file went")
	}
	s.exec(`UPDATE files SET deleted_at = $2 WHERE id = $1`, other.file, s.base)
	var n int
	do(func(ctx context.Context) error {
		var err error
		n, err = s.repo.DeleteOrphanTextExtracts(ctx, 10)
		return err
	})
	if n != 1 {
		t.Fatalf("orphans %d", n)
	}
}
