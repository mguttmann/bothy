// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestClassify: one row per line of the table of M2-13 design note 4, plus an import authored by
// the triggering person, a system line authored by a person and the triggering message itself.
// Origin decides before the author (ruling D-M2-13-3).
func TestClassify(t *testing.T) {
	t.Parallel()
	agent, other, alex, sam, writer := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	parties := domain.RunParties{AgentID: agent, TriggeredBy: alex}
	holders := map[ids.ID]bool{sam: true}
	cases := []struct {
		name   string
		origin string
		author ids.ID
		role   domain.Role
		trust  domain.Trust
	}{
		{"system", "system", other, domain.RoleOther, domain.TrustSystem},
		{"import", "import", sam, domain.RoleOther, domain.TrustUnverified},
		{"app", "app", other, domain.RoleOther, domain.TrustUnverified},
		{"own agent", "agent", agent, domain.RoleAssistant, domain.TrustSelf},
		{"other agent", "agent", other, domain.RoleOther, domain.TrustUnverified},
		{"triggering person", "human", alex, domain.RoleUser, domain.TrustTrigger},
		{"holder of trigger", "human", sam, domain.RoleUser, domain.TrustAuthorized},
		{"person without trigger", "human", writer, domain.RoleUser, domain.TrustUnverified},
		{"import by the triggering person", "import", alex, domain.RoleOther, domain.TrustUnverified},
		{"system line by a person", "system", alex, domain.RoleOther, domain.TrustSystem},
		{"an agent message whose author holds trigger", "agent", sam, domain.RoleOther, domain.TrustUnverified},
		{"unknown origin", "carrier-pigeon", alex, domain.RoleOther, domain.TrustUnverified},
	}
	for _, c := range cases {
		role, trust := domain.Classify(domain.AuthorFacts{AuthorID: c.author, Origin: c.origin}, parties, holders)
		if role != c.role || trust != c.trust {
			t.Errorf("%s: got %s/%s, want %s/%s", c.name, role, trust, c.role, c.trust)
		}
	}
}

// TestPlanContext: the session modes of M2-13 design note 5 with floor, own messages and channel
// context (ruling D-M2-13-5).
func TestPlanContext(t *testing.T) {
	t.Parallel()
	inT := func(seq int64) domain.EarlierRun {
		return domain.EarlierRun{HasTrigger: true, TriggerInThread: true, TriggerSeq: seq}
	}
	cases := []struct {
		name string
		f    domain.PlanFacts
		want domain.ContextPlan
	}{
		{"guest", domain.PlanFacts{Operator: false, HasSession: true, Started: true, Earlier: []domain.EarlierRun{inT(4)}},
			domain.ContextPlan{Mode: domain.SessionGuest, WantChannel: true}},
		{"no session", domain.PlanFacts{Operator: true},
			domain.ContextPlan{Mode: domain.SessionNew, WantChannel: true}},
		{"unstarted session with an earlier failed run", domain.PlanFacts{Operator: true, HasSession: true, Earlier: []domain.EarlierRun{inT(3)}},
			domain.ContextPlan{Mode: domain.SessionNew, WantChannel: true}},
		{"bind", domain.PlanFacts{Operator: true, HasSession: true, Started: true},
			domain.ContextPlan{Mode: domain.SessionBind, WantChannel: true}},
		{"resume", domain.PlanFacts{Operator: true, HasSession: true, Started: true, Earlier: []domain.EarlierRun{inT(7), inT(2)}},
			domain.ContextPlan{Mode: domain.SessionResume, FloorSeq: 7, ExcludeOwn: true}},
		{"resume after the parent message", domain.PlanFacts{Operator: true, HasSession: true, Started: true,
			Earlier: []domain.EarlierRun{{HasTrigger: true, TriggerInThread: false, TriggerSeq: 12}}},
			domain.ContextPlan{Mode: domain.SessionResume, ExcludeOwn: true}},
		{"resume whose latest run has no trigger", domain.PlanFacts{Operator: true, HasSession: true, Started: true,
			Earlier: []domain.EarlierRun{{HasTrigger: false}, inT(5)}},
			domain.ContextPlan{Mode: domain.SessionResume, FloorSeq: 5, ExcludeOwn: true}},
		{"resume without any trigger", domain.PlanFacts{Operator: true, HasSession: true, Started: true,
			Earlier: []domain.EarlierRun{{HasTrigger: false}}},
			domain.ContextPlan{Mode: domain.SessionResume, ExcludeOwn: true}},
	}
	for _, c := range cases {
		if got := domain.PlanContext(c.f); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// TestCutContent: below, equal to and above the limit; a multi-byte character across the limit
// is not split; the result is valid UTF-8.
func TestCutContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		max     int
		want    string
		trimmed bool
	}{
		{"abc", 4, "abc", false},
		{"abcd", 4, "abcd", false},
		{"abcde", 4, "abcd", true},
		{"abé", 3, "ab", true},      // é is two bytes: 'a','b' then 0xC3 0xA9
		{"a€b", 3, "a", true},       // € is three bytes and does not fit after 'a'
		{"a€b", 4, "a€", true},      // it fits exactly
		{"\U0001F600", 3, "", true}, // a four-byte rune never splits
		{"", 0, "", false},
		{"x", 0, "", true},
	}
	for _, c := range cases {
		got, trimmed := domain.CutContent(c.in, c.max)
		if got != c.want || trimmed != c.trimmed || !utf8.ValidString(got) {
			t.Errorf("CutContent(%q, %d) = %q, %v; want %q, %v", c.in, c.max, got, trimmed, c.want, c.trimmed)
		}
	}
}

// FuzzCutContent: arbitrary bytes and limits never give invalid UTF-8, a result above the limit,
// or a result that is not a prefix of the input.
func FuzzCutContent(f *testing.F) {
	f.Add("hello € world", 7)
	f.Add("\xff\xfeabc", 2)
	f.Add("\U0001F600\U0001F600", 5)
	f.Fuzz(func(t *testing.T, s string, limit int) {
		if limit < 0 {
			limit = -limit
		}
		limit %= 1 << 16
		got, trimmed := domain.CutContent(s, limit)
		if !utf8.ValidString(got) || len(got) > limit || !strings.HasPrefix(s, got) {
			t.Fatalf("CutContent(%q, %d) = %q", s, limit, got)
		}
		if !trimmed && got != s {
			t.Fatalf("CutContent(%q, %d) changed the content without trimmed", s, limit)
		}
	})
}

// item is a candidate of the budget with its position and size.
type item struct {
	seq  int64
	size int
}

// size is the byte count of the budget.
func size(i item) int { return i.size }

// seqs lists the positions of items.
func seqs(in []item) []int64 {
	out := make([]int64, 0, len(in))
	for _, i := range in {
		out = append(out, i.seq)
	}
	return out
}

// TestFitBudget: the newest are kept, the oldest dropped; channel elements are dropped before
// thread elements; the kept elements are contiguous; the sum stays within the limit; an element
// larger than the whole budget drops everything older too.
func TestFitBudget(t *testing.T) {
	t.Parallel()
	ch := []item{{1, 10}, {2, 10}}
	th := []item{{1, 10}, {2, 10}, {3, 10}}
	cases := []struct {
		name         string
		ch, th       []item
		max          int
		wantCh, want []int64
	}{
		{"everything fits", ch, th, 50, []int64{1, 2}, []int64{1, 2, 3}},
		{"oldest channel dropped", ch, th, 40, []int64{2}, []int64{1, 2, 3}},
		{"all channel and oldest thread dropped", ch, th, 25, []int64{}, []int64{2, 3}},
		{"nothing fits", ch, th, 5, []int64{}, []int64{}},
		{"zero budget", ch, th, 0, []int64{}, []int64{}},
		{"an oversized element drops everything older", ch, []item{{1, 1}, {2, 100}, {3, 1}}, 50, []int64{}, []int64{3}},
		{"empty", nil, nil, 10, []int64{}, []int64{}},
		{"empty items count zero", []item{{1, 0}}, []item{{1, 10}}, 10, []int64{1}, []int64{1}},
	}
	for _, c := range cases {
		gotCh, gotTh := domain.FitBudget(c.ch, c.th, size, c.max)
		if !slices.Equal(seqs(gotCh), c.wantCh) || !slices.Equal(seqs(gotTh), c.want) {
			t.Errorf("%s: got %v %v, want %v %v", c.name, seqs(gotCh), seqs(gotTh), c.wantCh, c.want)
		}
		sum := 0
		for _, i := range append(slices.Clone(gotCh), gotTh...) {
			sum += i.size
		}
		if sum > c.max {
			t.Errorf("%s: sum %d above %d", c.name, sum, c.max)
		}
		if len(gotCh) > 0 && len(gotTh) != len(c.th) {
			t.Errorf("%s: a channel element is kept while a thread element is dropped", c.name)
		}
		if !slices.Equal(seqs(gotTh), seqs(c.th[len(c.th)-len(gotTh):])) || !slices.Equal(seqs(gotCh), seqs(c.ch[len(c.ch)-len(gotCh):])) {
			t.Errorf("%s: the kept elements are not a newest suffix", c.name)
		}
	}
}

// TestTruncatedBeforeSeq: HasOlder only, a budget drop only, nothing kept, nothing missing.
func TestTruncatedBeforeSeq(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		hasOlder, drop bool
		kept           []int64
		want           int64
		set            bool
	}{
		{"older in the database", true, false, []int64{5, 6}, 5, true},
		{"budget drop", false, true, []int64{8}, 8, true},
		{"nothing kept", true, true, nil, 10, true},
		{"nothing missing", false, false, []int64{3, 4}, 0, false},
		{"nothing missing and nothing there", false, false, nil, 0, false},
	}
	for _, c := range cases {
		got, set := domain.TruncatedBeforeSeq(c.hasOlder, c.drop, c.kept, 10)
		if got != c.want || set != c.set {
			t.Errorf("%s: got %d %v, want %d %v", c.name, got, set, c.want, c.set)
		}
	}
}
