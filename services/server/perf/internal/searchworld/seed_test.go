// SPDX-License-Identifier: Apache-2.0

package searchworld_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
	"github.com/OWNER/bothy/services/server/perf/internal/searchworld"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// small is a seeded and loaded small corpus (brief M3-16 "Tests"): four spaces with one text and
// one forum channel each, four searchers who each see half of the spaces, a few posts and
// searchbench.SmallMessages messages, on a harness server with raised rate limits.
type small struct {
	srv    *harness.Server
	world  *searchworld.World
	corpus *searchbench.Corpus
	rows   *searchbench.Rows
}

// setup seeds the world through the public API, then loads the corpus through the admin pool.
func setup(t *testing.T) small {
	t.Helper()
	srv := harness.Start(t, harness.Options{Config: harness.RaisedRateLimits()})
	d, err := searchbench.LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := searchbench.NewPlan(searchbench.SmallParams(), searchbench.SmallMessages, d)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	w, err := searchworld.Seed(ctx, srv.Owner, srv.CreateUser, plan)
	if err != nil {
		t.Fatal(err)
	}
	var now time.Time
	if err := srv.AdminDB().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	corpus, err := searchbench.NewCorpus(plan, now)
	if err != nil {
		t.Fatal(err)
	}
	base, err := searchbench.MainBase(ctx, srv.AdminDB(), w.IDs())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := searchbench.BuildRows(corpus, w.IDs(), base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := searchbench.Load(ctx, srv.AdminDB(), rows); err != nil {
		t.Fatal(err)
	}
	return small{srv: srv, world: w, corpus: corpus, rows: rows}
}

// TestSmallCorpus_SeedLoadAndProductAgree: after Seed and the loader the aggregate check passes,
// getThread and listThreadMessages agree with the loader for every thread, and the product's own
// writes continue the loaded sequences with the languages the loader wrote.
func TestSmallCorpus_SeedLoadAndProductAgree(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	if err := searchbench.CheckLoad(ctx, s.srv.AdminDB(), s.rows); err != nil {
		t.Fatal(err)
	}
	if err := searchworld.CheckThreads(ctx, s.world, s.rows, len(s.rows.Threads)); err != nil {
		t.Fatal(err)
	}
	res, err := searchworld.CheckWrites(ctx, s.world, s.rows, s.srv.AdminDB())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 2 || len(res.Threads) != 2 {
		t.Fatalf("writes %+v", res)
	}
}

// search sends q as searcher k to path and returns the answer.
func search(ctx context.Context, s small, k int, path string) searchbench.Answer {
	r := s.world.Searchers[k].Client.Do(ctx, harness.Request{Method: http.MethodGet, Path: path})
	return searchbench.Answer{Status: r.Status, Body: r.Body}
}

// stemKey is a content stem of a language.
type stemKey struct {
	lang searchbench.Lang
	rank int
}

// phraseOf returns a quoted pair of adjacent rare content words of a live message of lang, a
// searcher who sees its channel and one who sees no channel where the two stems stand next to
// each other in a live message (any inflection), so the second must get no item.
func phraseOf(t *testing.T, s small, lang searchbench.Lang) (phrase string, sees, blind int) {
	t.Helper()
	c, p := s.corpus, s.corpus.Plan
	stems := map[string]stemKey{}
	for _, l := range searchbench.Langs {
		lv := c.Vocab.Langs[l]
		for rank := range lv.Stems {
			for k := range lv.Inflections {
				stems[lv.Form(rank, k)] = stemKey{l, rank}
			}
		}
	}
	// channels of every adjacent pair of stems in live messages
	where := map[[2]stemKey]map[int]bool{}
	for i := range p.Messages {
		m := c.Message(i)
		if m.Deleted {
			continue
		}
		w := searchbench.Words(m.Text)
		for j := 0; j+1 < len(w); j++ {
			a, okA := stems[w[j]]
			b, okB := stems[w[j+1]]
			if okA && okB {
				key := [2]stemKey{a, b}
				if where[key] == nil {
					where[key] = map[int]bool{}
				}
				where[key][m.Channel] = true
			}
		}
	}
	for i := range p.Messages {
		m := c.Message(i)
		if m.Deleted || m.Probe || m.Lang != lang {
			continue
		}
		w := searchbench.Words(m.Text)
		for j := 0; j+1 < len(w); j++ {
			a, okA := stems[w[j]]
			b, okB := stems[w[j+1]]
			if !okA || !okB || a.rank < p.BandMediumMax || b.rank < p.BandMediumMax {
				continue
			}
			sees, blind = -1, -1
			for k := range p.Searchers {
				hidden := true
				for ch := range where[[2]stemKey{a, b}] {
					if p.Visible(k, ch) {
						hidden = false
					}
				}
				if p.Visible(k, m.Channel) {
					sees = k
				} else if hidden {
					blind = k
				}
			}
			if sees >= 0 && blind >= 0 {
				return `"` + w[j] + " " + w[j+1] + `"`, sees, blind
			}
		}
	}
	t.Fatalf("no %s message with a rare pair that a searcher cannot see anywhere", lang)
	return "", 0, 0
}

// TestSmallCorpus_SearchFindsLoadedRows: a German and an English phrase of loaded messages are
// found by a searcher who sees their channel and give no item to one who does not; a prefix of a
// loaded title finds the post; for a searcher without the probe space the probe term and the
// absent term give the same empty answer.
func TestSmallCorpus_SearchFindsLoadedRows(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	p := s.corpus.Plan
	for _, lang := range searchbench.Langs {
		phrase, sees, blind := phraseOf(t, s, lang)
		q := searchbench.Query{Searcher: sees, Class: searchbench.ClassPhrase, Lang: lang, Text: phrase, MustFind: true}
		if _, err := searchbench.CheckSearch("small", q, search(ctx, s, sees, searchworld.SearchPath(phrase)), s.world.Visible(sees)); err != nil {
			t.Fatal(err)
		}
		q.Searcher, q.MustFind = blind, false
		a := search(ctx, s, blind, searchworld.SearchPath(phrase))
		if _, err := searchbench.CheckSearch("small", q, a, s.world.Visible(blind)); err != nil {
			t.Fatal(err)
		}
		var page struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(a.Body, &page); err != nil || len(page.Items) != 0 {
			t.Fatalf("%s phrase: searcher %d without the channel got %d items (%v)", lang, blind, len(page.Items), err)
		}
	}
	k := 0
	gen := searchbench.NewQueries(s.corpus, k, true)
	for range 20 {
		q, err := gen.NextPrefix()
		if err != nil {
			t.Fatal(err)
		}
		if err := searchbench.CheckPrefix("small", q, search(ctx, s, k, searchworld.PrefixPath(q.Text)), s.world.Visible(k)); err != nil {
			t.Fatal(err)
		}
	}
	gen = searchbench.NewQueries(s.corpus, k, false)
	for range 40 {
		q := gen.Next()
		if _, err := searchbench.CheckSearch("small", q, search(ctx, s, k, searchworld.SearchPath(q.Text)), s.world.Visible(k)); err != nil {
			t.Fatal(err)
		}
	}
	if len(p.ProbeSearchers) == 0 {
		t.Fatal("no probe searcher")
	}
	probe := p.ProbeSearchers[0]
	var bodies [][]byte
	for _, absent := range []bool{false, true} {
		q := s.corpus.ProbeQuery(probe, absent)
		a := search(ctx, s, probe, searchworld.SearchPath(q.Text))
		if err := searchbench.CheckProbe("small", q, a); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, a.Body)
	}
	if !slices.Equal(bodies[0], bodies[1]) {
		t.Errorf("probe term and absent term give different bodies: %d and %d bytes", len(bodies[0]), len(bodies[1]))
	}
	// The probe term is in the corpus: a searcher with the probe space finds it.
	for k := range p.Searchers {
		if !slices.Contains(p.ProbeSearchers, k) {
			a := search(ctx, s, k, searchworld.SearchPath(s.corpus.Vocab.Probe))
			if !strings.Contains(string(a.Body), `"kind":"message"`) {
				t.Errorf("searcher %d with the probe space does not find the probe term: status %d", k, a.Status)
			}
			break
		}
	}
}

// TestSmallCorpus_RewriteKeepsDefinitions: the rewrite of design note 12 keeps expression and
// index definitions, the fixed phrase finds the same messages, and every size is above zero.
func TestSmallCorpus_RewriteKeepsDefinitions(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	sizes, err := searchbench.MeasureSizes(ctx, s.srv.AdminDB())
	if err != nil {
		t.Fatal(err)
	}
	if sizes.MessagesTableBytes <= 0 || sizes.DatabaseBytes <= 0 || sizes.IndexShare <= 0 {
		t.Fatalf("sizes %+v", sizes)
	}
	for _, x := range append(sizes.VectorIndexes, sizes.TitleIndexes...) {
		if x.Bytes <= 0 {
			t.Errorf("index %s has %d bytes", x.Name, x.Bytes)
		}
	}
	rw, err := searchbench.MeasureRewrite(ctx, s.srv.AdminDB(), s.corpus.Plan.Defaults.Languages, s.corpus.FixedPhrase())
	if err != nil {
		t.Fatal(err)
	}
	if rw.Rows != int64(s.corpus.Plan.Messages) || rw.PhraseMatches == 0 || rw.RewriteSeconds <= 0 || rw.IndexBuildSeconds <= 0 || rw.MaintenanceWorkMem == "" {
		t.Fatalf("rewrite %+v", rw)
	}
}
