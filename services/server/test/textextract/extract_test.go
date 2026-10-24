// SPDX-License-Identifier: Apache-2.0

package textextract_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/extract/extracttest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// invoice is the sentence of the plan row: the second letter of "fuer" is U+00FC.
// language-check: off
const invoice = "Die Rechnung für den Lieferantenvertrag ist fertig."

// language-check: on

// TestDOCXFoundByContentWords (plan row, 14 2): a DOCX attached to a message is found by the
// words inside it, through the word (D) and through the german stem (C), by Alex who reads
// Allgemein and never by Robin who reads nothing of Support; a word that is nowhere finds nothing.
// The variant with the scripted clamd answering stream: OK proves the trigger for clean.
func TestDOCXFoundByContentWords(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		clamd bool
	}{{"skipped", false}, {"clean", true}} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := start(t, options{clamd: c.clamd, locale: "de"})
			id, _ := e.attached(t, "notes.docx", extract.TypeDOCX, extracttest.DOCX(invoice))
			r := e.extracted(t, id)
			if r.State != "done" || r.Extractor != extract.VersionOOXML || r.Language != "german" || r.Truncated || r.Text != invoice {
				t.Fatalf("row %+v", r)
			}
			hits := fileHits(t, e.alex, "Lieferantenvertrag")
			if seg, ok := hits[id]; !ok || !strings.Contains(seg, "[Lieferantenvertrag]") {
				t.Fatalf("Lieferantenvertrag: %v", hits)
			}
			if _, ok := fileHits(t, e.alex, "Rechnungen")[id]; !ok {
				t.Fatal("the stem Rechnungen does not find the document")
			}
			if hits := fileHits(t, e.alex, "Steuerbescheid"); len(hits) != 0 {
				t.Fatalf("a word in no part of the file: %v", hits)
			}
			for _, q := range []string{"Lieferantenvertrag", "Rechnungen", "notes"} {
				if hits := fileHits(t, e.robin, q); len(hits) != 0 {
					t.Fatalf("Robin found %v for %q", hits, q)
				}
			}
		})
	}
}

// TestOfficeAndTextFormatsFound: XLSX, PPTX, ODT, ODS, ODP, a plain text, a CSV and an HTML file
// are each found by a word only in their content; the HTML file is not found by a word only in its
// script element.
func TestOfficeAndTextFormatsFound(t *testing.T) {
	t.Parallel()
	e := start(t, options{})
	html := []byte("<!DOCTYPE html><html><head><script>var Skriptgeheimnis = 1;</script></head><body><p>Hafenrundfahrt</p></body></html>")
	for _, c := range []struct {
		name, mime, word string
		data             []byte
	}{
		{"sheet.xlsx", extract.TypeXLSX, "Tabellenwort", extracttest.XLSX("Kunde", "Tabellenwort")},
		{"deck.pptx", extract.TypePPTX, "Folienwort", extracttest.PPTX("Einleitung", "Folienwort")},
		{"letter.odt", extract.TypeODT, "Briefwort", extracttest.ODT("Briefwort")},
		{"calc.ods", extract.TypeODS, "Zellenwort", extracttest.ODS("Zellenwort")},
		{"talk.odp", extract.TypeODP, "Seitenwort", extracttest.ODP("Seitenwort")},
		{"notes.txt", "text/plain", "Klartextwort", []byte("ein Klartextwort steht hier")},
		{"list.csv", "text/csv", "Spaltenwort", []byte("name,wert\nSpaltenwort,1\nanderes,2\n")},
		{"page.html", "text/html", "Hafenrundfahrt", html},
	} {
		id, _ := e.attached(t, c.name, c.mime, c.data)
		if r := e.extracted(t, id); r.State != "done" {
			t.Fatalf("%s: %+v", c.name, r)
		}
		if _, ok := fileHits(t, e.alex, c.word)[id]; !ok {
			t.Errorf("%s not found by %s", c.name, c.word)
		}
	}
	if hits := fileHits(t, e.alex, "Skriptgeheimnis"); len(hits) != 0 {
		t.Fatalf("a word of the script element found %v", hits)
	}
}

// TestZipBombFailedWithoutInflating (plan row, 14 12, SEC-T08 file text part): with the archive
// limits set low the bomb variants end failed with their code, no text and the version; the file
// stays deliverable and is still found by its name. The adapter tests (TestZipLimits) prove with
// the counting seam that no part inflated beyond its limit.
func TestZipBombFailedWithoutInflating(t *testing.T) {
	t.Parallel()
	e := start(t, options{cfg: map[string]string{"search.extract.max_uncompressed_bytes": "65536", "search.extract.max_archive_entries": "16"}})
	for _, c := range []struct {
		name string
		code string
		data []byte
	}{
		{"bomb declared.docx", "archive_size_exceeded", extracttest.BombDeclared(1 << 20)},
		{"bomb lying.docx", "archive_invalid", extracttest.BombLyingHeader(1000, 50000)},
		{"bomb entries.docx", "archive_entries_exceeded", extracttest.TooManyEntries(40)},
	} {
		id, _ := e.attached(t, c.name, extract.TypeDOCX, c.data)
		r := e.extracted(t, id)
		if r.State != "failed" || r.Error != c.code || r.HasText || r.Extractor != extract.VersionOOXML {
			t.Fatalf("%s: %+v", c.name, r)
		}
		if st := e.state(t, id); !strings.HasPrefix(st, "complete/skipped/") {
			t.Fatalf("%s: file state %s", c.name, st)
		}
		if _, ok := fileHits(t, e.alex, "bomb")[id]; !ok {
			t.Fatalf("%s is no longer found by its name", c.name)
		}
	}
}

// TestExtractOnlyForDeliverableAttachments: a file clamd reports FOUND or ERROR for, a file whose
// digest does not match and a PNG get no row and no job files.extract_text; a text file does.
func TestExtractOnlyForDeliverableAttachments(t *testing.T) {
	t.Parallel()
	e := start(t, options{clamd: true})
	jobs := func(id string) int {
		return e.count(t, `SELECT count(*) FROM jobs.river_job WHERE kind = 'files.extract_text' AND args->>'file_id' = $1`, id)
	}
	content := []byte("Infiziertes Dokument")
	for _, c := range []struct {
		name  string
		reply string
	}{{"found.txt", "stream: Win.Test.EICAR_HDB-1 FOUND"}, {"error.txt", "stream: Can't allocate memory ERROR"}} {
		e.clamd.SetScript(clamdtest.Script{Reply: c.reply})
		id := e.upload(t, c.name, content, nil)
		if st := e.final(t, id); strings.HasPrefix(st, "complete/clean/") || strings.HasPrefix(st, "complete/skipped/") {
			t.Fatalf("%s ended %s", c.name, st)
		}
		if e.row(t, id).Found || jobs(id) != 0 {
			t.Fatalf("%s got a row or a job", c.name)
		}
	}
	e.clamd.SetScript(clamdtest.Script{})
	mismatch := e.upload(t, "mismatch.txt", content, []byte("other bytes"))
	if st := e.final(t, mismatch); !strings.HasPrefix(st, "failed/") {
		t.Fatalf("checksum mismatch ended %s", st)
	}
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde")
	img := e.upload(t, "image.png", png, nil)
	e.final(t, img)
	ok, _ := e.attached(t, "ok.txt", "text/plain", []byte("Lesbarer Text"))
	// At least one job: the walk files.extract_text_refresh, which runs once at the start of the
	// worker, may meet the row while it is pending and enqueue it again (a duplicate finds the
	// row done and does nothing).
	if r := e.extracted(t, ok); r.State != "done" || jobs(ok) < 1 {
		t.Fatalf("the clean text file: %+v jobs %d", r, jobs(ok))
	}
	for _, id := range []string{mismatch, img} {
		if e.row(t, id).Found || jobs(id) != 0 {
			t.Fatalf("file %s (%s) got a row or a job", id, e.state(t, id))
		}
	}
}

// TestExtractLimits: a text file over search.extract.max_input_bytes ends failed input_too_large;
// a text over search.extract.max_text_bytes ends done with truncated and the limit; the timeout is
// TestExtractTimeoutStallingStore.
func TestExtractLimits(t *testing.T) {
	t.Parallel()
	e := start(t, options{cfg: map[string]string{"search.extract.max_input_bytes": "2000", "search.extract.max_text_bytes": "100"}})
	big, _ := e.attached(t, "big.txt", "text/plain", []byte(strings.Repeat("Wort ", 500)))
	if r := e.extracted(t, big); r.State != "failed" || r.Error != "input_too_large" || r.Extractor != extract.VersionText || r.HasText {
		t.Fatalf("too large: %+v", r)
	}
	long, _ := e.attached(t, "long.txt", "text/plain", []byte(strings.Repeat("abcd ", 300)))
	if r := e.extracted(t, long); r.State != "done" || !r.Truncated || len(r.Text) != 100 {
		t.Fatalf("truncated: %+v (%d bytes)", r, len(r.Text))
	}
}

// TestExtractVectorLimit: a text file of search.extract.max_text_bytes bytes of distinct hex
// tokens, whose search vector exceeds the size limit of PostgreSQL, ends done with truncated and
// a non-empty text, without an endless retry.
func TestExtractVectorLimit(t *testing.T) {
	t.Parallel()
	e := start(t, options{})
	var b strings.Builder
	for i := 0; b.Len() < 1048576-18; i++ {
		fmt.Fprintf(&b, "%016x ", uint64(i)*0x9e3779b97f4a7c15)
	}
	id, _ := e.attached(t, "hashes.txt", "text/plain", []byte(b.String()))
	r := e.extracted(t, id)
	if r.State != "done" || !r.Truncated || r.Text == "" || len(r.Text) >= b.Len() {
		t.Fatalf("state %s truncated %v text bytes %d of %d", r.State, r.Truncated, len(r.Text), b.Len())
	}
	if n := e.count(t, `SELECT count(*) FROM file_text_extracts WHERE file_id = $1 AND length(search_vector::text) > 0`, id); n != 1 {
		t.Fatal("empty vector")
	}
}

// TestExtractRowRemovedWithFile (DP-T01 file text part, 14 5.3): deleting the message that
// carries an extracted DOCX removes the extract row in the same transaction (the path of M3-07
// that sets files.deleted_at) and search finds nothing; deleting a files row as the owner role
// removes its row by cascade. The walk part is TestRefreshRecoversDiscardedJob.
func TestExtractRowRemovedWithFile(t *testing.T) {
	t.Parallel()
	e := start(t, options{locale: "de"})
	id, msg := e.attached(t, "contract.docx", extract.TypeDOCX, extracttest.DOCX(invoice))
	if r := e.extracted(t, id); r.State != "done" {
		t.Fatalf("%+v", r)
	}
	r, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + msg})
	if err != nil || r.Status != http.StatusNoContent {
		t.Fatalf("deleteMessage: %v %d %s", err, r.Status, r.Body)
	}
	if e.row(t, id).Found {
		t.Fatal("the extract row stayed after the deletion of the message")
	}
	if hits := fileHits(t, e.alex, "Lieferantenvertrag"); len(hits) != 0 {
		t.Fatalf("search after the deletion: %v", hits)
	}
	other, _ := e.attached(t, "other.txt", "text/plain", []byte("Kaskadenwort"))
	if r := e.extracted(t, other); r.State != "done" {
		t.Fatalf("%+v", r)
	}
	e.exec(t, `DELETE FROM message_attachments WHERE file_id = $1`, other)
	e.exec(t, `DELETE FROM files WHERE id = $1`, other)
	if n := e.count(t, `SELECT count(*) FROM file_text_extracts WHERE file_id = $1`, other); n != 0 {
		t.Fatalf("rows after deleting the files row: %d", n)
	}
}
