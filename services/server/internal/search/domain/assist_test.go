// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search/domain"
)

// The questions of the owner's example with their non-ASCII letters written out: U+00DC capital U
// with diaeresis.
// language-check: off

const (
	takeoverQuestion = "Ich suche die Übernahme von Larkspur auf Hermes"
	moveQuestion     = "Umzug der Steuersitzung von Discord nach Larkspur"
)

// language-check: on

// language-check: off

func TestAssistTerms_DropsStopWordsShortWordsAndDuplicates(t *testing.T) {
	got := domain.AssistTerms(takeoverQuestion, nil, 3, 12)
	want := []string{"übernahme", "larkspur", "hermes"}
	if !slices.Equal(got, want) {
		t.Fatalf("terms = %q, want %q", got, want)
	}
	got = domain.AssistTerms(moveQuestion, nil, 3, 12)
	want = []string{"umzug", "steuersitzung", "discord", "larkspur"}
	if !slices.Equal(got, want) {
		t.Fatalf("terms = %q, want %q", got, want)
	}
}

// language-check: on

func TestAssistTerms_FollowUpKeepsEarlierTermsAfterTheNewOnes(t *testing.T) {
	got := domain.AssistTerms("no, the older one about Discord", []string{"larkspur", "discord", "hermes"}, 3, 4)
	want := []string{"older", "discord", "larkspur", "hermes"}
	if !slices.Equal(got, want) {
		t.Fatalf("terms = %q, want %q", got, want)
	}
}

func TestAssistTerms_SplitsAtEveryNonLetter(t *testing.T) {
	got := domain.AssistTerms(`"quoted" -minus or ERR_CONN_RESET deploy-v2`, nil, 3, 12)
	want := []string{"quoted", "minus", "err", "conn", "reset", "deploy", "v2"}
	if !slices.Equal(got, slices.DeleteFunc(want, func(s string) bool { return len(s) < 3 })) {
		t.Fatalf("terms = %q", got)
	}
}

func TestAssistQueryText_IsAnOrOfTheTerms(t *testing.T) {
	if got := domain.AssistQueryText([]string{"larkspur", "hermes"}); got != "larkspur or hermes" {
		t.Fatalf("query = %q", got)
	}
	if got := domain.AssistQueryText(nil); got != "" {
		t.Fatalf("query of no terms = %q", got)
	}
}

func TestFuseThreads_TitleHitsCountTwiceAndTiesGoToTheNewer(t *testing.T) {
	a, b, c, d := ids.New(), ids.New(), ids.New(), ids.New()
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	titles := []domain.Ranked{{Thread: b, At: base}}
	messages := []domain.Ranked{
		{Thread: a, At: base.Add(time.Hour)}, {Thread: a, At: base}, {Thread: c, At: base}, {Thread: b, At: base},
		{Thread: d, At: base.Add(2 * time.Hour)},
	}
	got := domain.FuseThreads(titles, messages)
	// b: 2/(60+1) + 1/(60+3); a: 1/(60+1); c: 1/(60+2); d: 1/(60+4).
	want := []ids.ID{b, a, c, d}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	// A title hit outranks a message hit of the same rank.
	x, y := ids.New(), ids.New()
	got = domain.FuseThreads([]domain.Ranked{{Thread: x, At: base}}, []domain.Ranked{{Thread: y, At: base.Add(time.Minute)}})
	if !slices.Equal(got, []ids.ID{x, y}) {
		t.Fatalf("title first = %v", got)
	}
	got = domain.FuseThreads(nil, nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}

// language-check: off

func TestAssistExcerpt_CollapsesSpaceAndCutsAtRunes(t *testing.T) {
	if got := domain.AssistExcerpt("  one\n\ttwo   three ", 100); got != "one two three" {
		t.Fatalf("excerpt = %q", got)
	}
	if got := domain.AssistExcerpt("äöü abc", 3); got != "äöü..." {
		t.Fatalf("cut = %q", got)
	}
}

// language-check: on

func TestValidateAssistAnswer(t *testing.T) {
	refs := map[string]bool{"c1": true, "c2": true}
	ok := domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c2", Reason: "The move."}}, Terms: []string{"migration"}}
	if err := domain.ValidateAssistAnswer(ok, refs, 2, 2, 50); err != nil {
		t.Fatalf("valid answer: %v", err)
	}
	cases := []struct {
		name    string
		a       domain.AssistAnswer
		pointer string
		code    string
	}{
		{"unknown ref", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c9", Reason: "x"}}}, "/picks/0/ref", "unknown_ref"},
		{"duplicate", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c1", Reason: "x"}, {Ref: "c1", Reason: "y"}}}, "/picks/1/ref", "duplicate"},
		{"too many picks", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c1", Reason: "x"}, {Ref: "c2", Reason: "y"}, {Ref: "c3", Reason: "z"}}}, "/picks", "too_many_items"},
		{"empty reason", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c1"}}}, "/picks/0/reason", "too_short"},
		{"long reason", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c1", Reason: strings.Repeat("x", 51)}}}, "/picks/0/reason", "too_long"},
		{"control in reason", domain.AssistAnswer{Picks: []domain.Pick{{Ref: "c1", Reason: "a\nb"}}}, "/picks/0/reason", "control_character"},
		{"too many terms", domain.AssistAnswer{Terms: []string{"a", "b", "c"}}, "/terms", "too_many_items"},
		{"empty term", domain.AssistAnswer{Terms: []string{""}}, "/terms/0", "too_short"},
		{"long term", domain.AssistAnswer{Terms: []string{strings.Repeat("t", 65)}}, "/terms/0", "too_long"},
		{"bidi term", domain.AssistAnswer{Terms: []string{"a\u202eb"}}, "/terms/0", "bidi_control"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateAssistAnswer(tc.a, refs, 2, 2, 50)
			e, okErr := errs.As(err)
			if !okErr || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
				t.Fatalf("err = %v", err)
			}
			if f := e.Fields[0]; f.Pointer != tc.pointer || f.Code != tc.code {
				t.Fatalf("field = %+v, want %s %s", f, tc.pointer, tc.code)
			}
		})
	}
}

func TestAssistPayload_HoldsNoIDsAndDropsCandidatesThatDoNotFit(t *testing.T) {
	req := ids.New()
	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	cands := []domain.AssistCandidate{
		{Ref: "c1", Title: "Takeover", ChannelName: "larkspur", SpaceName: "Projects", At: at, Excerpt: "moved", ShownBefore: true},
		{Ref: "c2", ChannelName: "general", At: at, Excerpt: strings.Repeat("x", 400)},
	}
	in := domain.AssistPayloadInput{RequestID: req, Q: "takeover", History: []string{"larkspur"}, Candidates: cands, MaxPicks: 5,
		MaxTerms: 8, ExpiresAt: at}
	b, kept, err := domain.AssistPayload(in, 4096)
	if err != nil || kept != 2 {
		t.Fatalf("payload: kept %d, %v", kept, err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["request_id"] != req.String() || got["q"] != "takeover" || got["expires_at"] != "2026-11-02T10:00:00.000Z" {
		t.Fatalf("payload = %s", b)
	}
	c2 := got["candidates"].([]any)[1].(map[string]any)
	if _, has := c2["title"]; has || c2["shown_before"] != false {
		t.Fatalf("candidate without a title = %v", c2)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("payload has null: %s", b)
	}
	small, kept, err := domain.AssistPayload(in, 400)
	if err != nil || kept != 1 || len(small) >= 400 {
		t.Fatalf("small payload: kept %d, len %d, %v", kept, len(small), err)
	}
	if _, kept, err := domain.AssistPayload(in, 10); err == nil || kept != 0 {
		t.Fatalf("a payload that never fits: kept %d, %v", kept, err)
	}
}

func TestAssistRequest_HasExactlyOneOutcome(t *testing.T) {
	end := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	r := domain.NewAssistRequest(end)
	if r.State() != domain.AssistOpen {
		t.Fatal("not open")
	}
	if !r.Answer(domain.AssistAnswer{Terms: []string{"a"}}) || r.Answer(domain.AssistAnswer{}) || r.Expire() {
		t.Fatal("answer is not a single transition")
	}
	if r.State() != domain.AssistAnswered || r.Answered().Terms[0] != "a" {
		t.Fatal("answer lost")
	}
	e := domain.NewAssistRequest(end)
	if !e.Expire() || e.Answer(domain.AssistAnswer{}) || e.State() != domain.AssistExpired {
		t.Fatal("expire is not a single transition")
	}
	if e.Forgotten(end.Add(time.Second), 2*time.Second) || !e.Forgotten(end.Add(2*time.Second), 2*time.Second) {
		t.Fatal("tombstone rule")
	}
}
