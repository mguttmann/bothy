// SPDX-License-Identifier: Apache-2.0

package domain_test

// Tests of the summary step per parent (09 5.3, addendum of 2026-10-24, R-DM-53, D-RV-6): each
// parent gets its own summary step, stored root summaries stay, and finished subtrees collapse
// upward so a long run with many finished subagents never hits limit_reached.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// summaryCount is the member summarized of a summary step's detail.
type summaryCount struct {
	ByKind map[string]int64 `json:"by_kind"`
	Count  int64            `json:"count"`
}

// countsOf decodes the counts of a summary step.
func countsOf(t *testing.T, s domain.DocStep) summaryCount {
	t.Helper()
	var d struct {
		Summarized summaryCount `json:"summarized"`
	}
	if err := json.Unmarshal(s.Detail, &d); err != nil {
		t.Fatalf("summary detail %s: %v", s.Detail, err)
	}
	return d.Summarized
}

// summariesByParent returns the summary steps of st by parent (zero for the root); it fails when
// a parent has two.
func summariesByParent(t *testing.T, st domain.DocumentState) map[ids.ID]domain.DocStep {
	t.Helper()
	out := map[ids.ID]domain.DocStep{}
	for _, s := range st.Steps {
		if !domain.IsSummaryStep(s) {
			continue
		}
		if _, dup := out[s.ParentStepID]; dup {
			t.Fatalf("two summary steps under parent %s", s.ParentStepID)
		}
		out[s.ParentStepID] = s
	}
	return out
}

// TestSummary_PerParent: a run with finished leaves under two running subagents and at the root
// crosses run.max_steps_persisted; each parent gets its own summary step with that parent, at the
// lowest ordinal removed into it; running steps and parents stay; the normalized ops are a remove
// per removed step and the add of each summary step.
func TestSummary_PerParent(t *testing.T) {
	t.Parallel()
	lim := domain.Limits{ExcerptMaxChars: 10, StepDetailMaxBytes: 200, MaxStepsPersisted: 7}
	base := baseState()
	base.Steps = []domain.DocStep{
		mkStep(1, 1, "tool_call", "running", 0),
		mkStep(2, 2, "subagent", "running", 1),
		mkStep(3, 3, "tool_call", "completed", 2),
		mkStep(4, 4, "tool_call", "failed", 2),
		mkStep(5, 5, "tool_call", "completed", 0),
		mkStep(6, 6, "subagent", "running", 0),
		mkStep(7, 7, "tool_call", "completed", 6),
		mkStep(8, 8, "tool_call", "cancelled", 6),
	}
	ch, norm, err := mustApply(t, base, lim, rawOp("add", "/steps/"+sid(9), `{"kind":"tool_call","name":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	sa, sr, sb := tid(900), tid(901), tid(902)
	step := func(id ids.ID, ordinal int, parent string, counts string) string {
		p := ""
		if parent != "" {
			p = `"parent_step_id":"` + parent + `",`
		}
		return fmt.Sprintf(`{"id":"%s","ordinal":%d,%s"kind":"status","name":"steps_summarized","title":"","status":"completed","detail":{"summarized":%s},"started_at":"2026-10-21T10:00:00.123Z"}`,
			id, ordinal, p, counts)
	}
	sameNorm(t, "per parent", norm, []domain.NormOp{
		nop(domain.OpAdd, "/steps/"+sid(9), `{"id":"`+sid(9)+`","ordinal":9,"kind":"tool_call","name":"x","title":"","status":"running","detail":{},"started_at":"2026-10-21T10:00:00.123Z"}`),
		nop(domain.OpRemove, "/steps/"+sid(3), ""),
		nop(domain.OpRemove, "/steps/"+sid(4), ""),
		nop(domain.OpRemove, "/steps/"+sid(5), ""),
		nop(domain.OpRemove, "/steps/"+sid(7), ""),
		nop(domain.OpRemove, "/steps/"+sid(8), ""),
		nop(domain.OpAdd, "/steps/"+sa.String(), step(sa, 3, sid(2), `{"by_kind":{"tool_call":2},"count":2}`)),
		nop(domain.OpAdd, "/steps/"+sr.String(), step(sr, 5, "", `{"by_kind":{"tool_call":1},"count":1}`)),
		nop(domain.OpAdd, "/steps/"+sb.String(), step(sb, 7, sid(6), `{"by_kind":{"tool_call":2},"count":2}`)),
	})
	got := summariesByParent(t, ch.After)
	if len(got) != 3 || got[tid(2)].ID != sa || got[ids.ID{}].ID != sr || got[tid(6)].ID != sb {
		t.Errorf("summaries by parent = %+v", got)
	}
	if len(ch.After.Steps) != 7 {
		t.Errorf("after has %d steps, want 7", len(ch.After.Steps))
	}
	kept := map[ids.ID]bool{}
	for _, s := range ch.After.Steps {
		kept[s.ID] = true
	}
	for _, n := range []int{1, 2, 6, 9} {
		if !kept[tid(n)] {
			t.Errorf("running step or parent %d was removed", n)
		}
	}

	// No further candidate: running steps, their parents and their summary steps alone are
	// above the bound, so the next add fails with limit_reached.
	lim.MaxStepsPersisted = 6
	_, _, err = mustApply(t, ch.After, lim, rawOp("add", "/steps/"+sid(10), `{"kind":"tool_call","name":"y"}`))
	if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/0", "limit_reached"}}) {
		t.Errorf("errors = %v", got)
	}
}

// TestSummary_OldRootSummaryStays: a summary step stored before the addendum (at the root) stays
// as it is when the later merges happen under another parent; a later root merge grows it.
func TestSummary_OldRootSummaryStays(t *testing.T) {
	t.Parallel()
	lim := domain.Limits{ExcerptMaxChars: 10, StepDetailMaxBytes: 200, MaxStepsPersisted: 4}
	base := baseState()
	root := domain.DocStep{ID: tid(1), Ordinal: 1, Kind: "status", Name: domain.SummaryStepName, Status: "completed",
		Detail: json.RawMessage(`{"summarized":{"by_kind":{"tool_call":5},"count":5}}`), StartedAt: testNow}
	base.Steps = []domain.DocStep{
		root,
		mkStep(2, 2, "subagent", "running", 0),
		mkStep(3, 3, "tool_call", "completed", 2),
		mkStep(4, 4, "tool_call", "completed", 2),
	}
	ch, norm, err := mustApply(t, base, lim, rawOp("add", "/steps/"+sid(5), `{"kind":"tool_call","name":"x","parent_step_id":"`+sid(2)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range norm {
		if n.Path == "/steps/"+sid(1) || n.Path == "/steps/"+sid(1)+"/detail" || n.Path == "/steps/"+sid(1)+"/ordinal" {
			t.Errorf("the stored root summary changed: %s %s", n.Op, n.Path)
		}
	}
	got := summariesByParent(t, ch.After)
	if got[ids.ID{}].ID != tid(1) || countsOf(t, got[ids.ID{}]).Count != 5 {
		t.Errorf("root summary = %+v", got[ids.ID{}])
	}
	sub, ok := got[tid(2)]
	if !ok || sub.ID != tid(900) || sub.Ordinal != 3 || countsOf(t, sub).Count != 2 {
		t.Errorf("summary under the subagent = %+v", sub)
	}

	// A finished root step later goes into the stored root summary.
	st := ch.After
	ch, norm, err = mustApply(t, st, lim, rawOp("add", "/steps/"+sid(6), `{"kind":"tool_call","name":"y","status":"completed"}`))
	if err != nil {
		t.Fatal(err)
	}
	sameNorm(t, "root merge", norm, []domain.NormOp{
		nop(domain.OpReplace, "/steps/"+sid(1)+"/detail", `{"summarized":{"by_kind":{"tool_call":6},"count":6}}`),
	})
	if len(ch.After.Steps) != 4 {
		t.Errorf("after has %d steps", len(ch.After.Steps))
	}
}

// TestSummary_FinishedSubtreesCollapse: more than run.max_steps_persisted / 3 finished
// subagents, each an Agent tool call with a subagent step and tool calls under it, never give
// limit_reached; the run stays at or below the bound, and a finished step whose only child is
// its summary step goes with it into its parent's summary, so the counts add up to every step
// ever added.
func TestSummary_FinishedSubtreesCollapse(t *testing.T) {
	t.Parallel()
	const bound = 30
	lim := domain.Limits{ExcerptMaxChars: 10, StepDetailMaxBytes: 200, MaxStepsPersisted: bound}
	st := baseState()
	st.Steps = nil
	next := 100
	newID := func() string { next++; return sid(next) }
	added := map[string]int64{}
	var total int64
	env := testEnv(5000) // one id source for the whole run, so summary ids never repeat
	apply := func(st domain.DocumentState, raws ...domain.RawOp) (domain.Change, error) {
		t.Helper()
		ops, fieldErrs := domain.ParseOps(raws)
		if len(fieldErrs) != 0 {
			t.Fatalf("ParseOps: %v", fieldErrs)
		}
		ch, norm, err := domain.Apply(st, ops, lim, env)
		if err == nil {
			checkProperty(t, st, ops, ch, norm, lim)
		}
		return ch, err
	}
	for agent := range bound/3 + 5 {
		call, sub := newID(), newID()
		raws := []domain.RawOp{
			rawOp("add", "/steps/"+call, `{"kind":"tool_call","name":"Agent"}`),
			rawOp("add", "/steps/"+sub, `{"kind":"subagent","name":"local_agent","parent_step_id":"`+call+`"}`),
		}
		for range 3 {
			raws = append(raws, rawOp("add", "/steps/"+newID(), `{"kind":"tool_call","name":"Read","status":"completed","parent_step_id":"`+sub+`"}`))
		}
		added["tool_call"] += 4
		added["subagent"]++
		total += 5
		ch, err := apply(st, raws...)
		if err != nil {
			t.Fatalf("agent %d: %v", agent, err)
		}
		if len(ch.After.Steps) > bound {
			t.Fatalf("agent %d: %d steps above the bound", agent, len(ch.After.Steps))
		}
		// The subagent and its call end; a patch without a step add never summarizes.
		ch, err = apply(ch.After,
			rawOp("replace", "/steps/"+sub+"/status", `"completed"`),
			rawOp("replace", "/steps/"+call+"/status", `"completed"`))
		if err != nil {
			t.Fatalf("agent %d end: %v", agent, err)
		}
		st = ch.After
	}
	byKind := map[string]int64{}
	var counted int64
	for _, s := range st.Steps {
		if domain.IsSummaryStep(s) {
			c := countsOf(t, s)
			counted += c.Count
			for k, n := range c.ByKind {
				byKind[k] += n
			}
			continue
		}
		counted++
		byKind[s.Kind]++
	}
	if counted != total || !reflect.DeepEqual(byKind, added) {
		t.Errorf("counted %d %v, want %d %v", counted, byKind, total, added)
	}
	sums := summariesByParent(t, st)
	if _, ok := sums[ids.ID{}]; !ok {
		t.Errorf("no root summary after the collapse: %+v", sums)
	}
}
