// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestActingForModes_Golden (M2-14 design note 5): the exception and unbound sets of the mode
// table equal testdata/actingfor_modes.golden; every other catalogue action is bound.
func TestActingForModes_Golden(t *testing.T) {
	t.Parallel()
	var got []string
	for _, id := range domain.ActionIDs() {
		switch domain.ActingForModeOf(id) {
		case domain.ActingForOwnException:
			got = append(got, "exception "+id)
		case domain.ActingForUnbound:
			got = append(got, "unbound "+id)
		case domain.ActingForBound:
		default:
			t.Errorf("%s: unknown mode", id)
		}
	}
	slices.Sort(got)
	if want := readGolden(t, "actingfor_modes.golden"); !slices.Equal(got, want) {
		t.Fatalf("mode table\n got %v\nwant %v", got, want)
	}
	if domain.ActingForModeOf("no.such.action") != domain.ActingForBound {
		t.Fatal("an unknown action must be bound (fail closed)")
	}
}

var backticked = regexp.MustCompile("`([a-z_]+\\.[a-z_.]+)`")

// TestActingForExceptionsNamedInChapter07 (M2-14 design note 5): every backticked action id of
// the exception sentence of the SR9 section of chapter 07 is an own exception of the mode table.
func TestActingForExceptionsNamedInChapter07(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(chapter07(t))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	start := strings.Index(text, "### SR9")
	if start < 0 {
		t.Fatal("chapter 07 has no SR9 section")
	}
	section := text[start:]
	if end := strings.Index(section[len("### SR9"):], "\n### "); end >= 0 {
		section = section[:len("### SR9")+end]
	}
	// The exception sentence: "Ausnahmen, ..." in the German chapter, "Exceptions ..." in the
	// English one; it ends at the first full stop followed by a space or a line end.
	i := strings.Index(section, "Ausnahmen")
	if i < 0 {
		i = strings.Index(section, "Exceptions")
	}
	if i < 0 {
		t.Fatal("the SR9 section has no exception sentence")
	}
	sentence := section[i:]
	if j := regexp.MustCompile(`\.(\s|$)`).FindStringIndex(sentence); j != nil {
		sentence = sentence[:j[0]]
	}
	found := backticked.FindAllStringSubmatch(sentence, -1)
	if len(found) == 0 {
		t.Fatalf("no action id in the exception sentence %q", sentence)
	}
	for _, m := range found {
		if domain.ActingForModeOf(m[1]) != domain.ActingForOwnException {
			t.Errorf("%s is named as an SR9 exception in chapter 07 but is not ActingForOwnException", m[1])
		}
	}
}

// TestActingForApplies (M2-14 design note 4d, 5 and 6): the intersection applies to bound
// actions outside delivery mode, never to unbound actions, and to an own exception only off the
// own thread and run.
func TestActingForApplies(t *testing.T) {
	t.Parallel()
	run, thread, other := ids.New(), ids.New(), ids.New()
	af := domain.ActingForFacts{RunID: run, ThreadID: thread}
	own := domain.Path{Kind: domain.PathChannel, ThreadID: thread}
	foreign := domain.Path{Kind: domain.PathChannel, ThreadID: other}
	msg := domain.ResourceFacts{Kind: "thread", ID: thread}
	ownRun := domain.ResourceFacts{Kind: "run", ID: run}
	earlierRun := domain.ResourceFacts{Kind: "run", ID: ids.New()}
	cases := []struct {
		name     string
		mode     domain.ActingForMode
		path     domain.Path
		r        domain.ResourceFacts
		delivery bool
		want     bool
	}{
		{"bound", domain.ActingForBound, own, msg, false, true},
		{"bound in delivery mode", domain.ActingForBound, own, msg, true, false},
		{"unbound", domain.ActingForUnbound, foreign, msg, false, false},
		{"exception own thread", domain.ActingForOwnException, own, msg, false, false},
		{"exception foreign thread", domain.ActingForOwnException, foreign, msg, false, true},
		{"exception without thread", domain.ActingForOwnException, domain.Path{Kind: domain.PathChannel}, domain.ResourceFacts{Kind: "channel"}, false, true},
		{"exception own run", domain.ActingForOwnException, own, ownRun, false, false},
		{"exception earlier run of the own thread", domain.ActingForOwnException, own, earlierRun, false, true},
		{"exception own run in delivery mode", domain.ActingForOwnException, foreign, ownRun, true, false},
	}
	for _, c := range cases {
		if got := domain.ActingForApplies(c.mode, af, c.path, c.r, c.delivery); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if domain.ActingForApplies(domain.ActingForOwnException, domain.ActingForFacts{}, domain.Path{Kind: domain.PathChannel}, msg, false) != true {
		t.Error("a zero thread on both sides is not the own thread")
	}
}

// TestCheckActingFor (M2-14 design notes 4b and 4c): invalid denies every action, run_inactive
// every action but the unbound ones, both with the status as Detail.
func TestCheckActingFor(t *testing.T) {
	t.Parallel()
	inv := domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonInvocationTokenInvalid, Detail: "invalid"}
	ended := domain.Decision{Outcome: domain.OutcomeForbidden, Reason: domain.ReasonInvocationTokenInvalid, Detail: "run_inactive"}
	for _, c := range []struct {
		status string
		mode   domain.ActingForMode
		want   domain.Decision
	}{
		{"", domain.ActingForBound, domain.Allow},
		{"bound", domain.ActingForBound, domain.Allow},
		{"bound", domain.ActingForUnbound, domain.Allow},
		{"invalid", domain.ActingForBound, inv},
		{"invalid", domain.ActingForOwnException, inv},
		{"invalid", domain.ActingForUnbound, inv},
		{"garbage", domain.ActingForUnbound, inv},
		{"run_inactive", domain.ActingForBound, ended},
		{"run_inactive", domain.ActingForOwnException, ended},
		{"run_inactive", domain.ActingForUnbound, domain.Allow},
	} {
		if got := domain.CheckActingFor(c.status, c.mode); got != c.want {
			t.Errorf("%q mode %d: %+v, want %+v", c.status, c.mode, got, c.want)
		}
	}
}
