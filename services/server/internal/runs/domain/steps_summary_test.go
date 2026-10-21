// SPDX-License-Identifier: Apache-2.0

package domain_test

// Tests of the summarization of old steps into the summary step.

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// mkStep builds a stored step for the summary tests.
func mkStep(n, ordinal int, kind, status string, parent int) domain.DocStep {
	s := domain.DocStep{
		ID: tid(n), Ordinal: ordinal, Kind: kind, Name: "read_file", Status: status,
		Detail: json.RawMessage(`{}`), StartedAt: time.Date(2026, 10, 21, 9, 0, n, 0, time.UTC),
	}
	if parent != 0 {
		s.ParentStepID = tid(parent)
	}
	return s
}

// summaryOf returns the summary step of state and fails when there is none.
func summaryOf(t *testing.T, st domain.DocumentState) domain.DocStep {
	t.Helper()
	for _, s := range st.Steps {
		if domain.IsSummaryStep(s) {
			return s
		}
	}
	t.Fatalf("no summary step")
	return domain.DocStep{}
}

// TestSummarizeSteps covers the summarization of 09 5.3 and 03: oldest finished leaf first, the
// summary step at the lowest removed ordinal, its growth and move in later patches, running
// steps and parents kept, and limit_reached without a candidate.
func TestSummarizeSteps(t *testing.T) {
	t.Parallel()
	lim := domain.Limits{ExcerptMaxChars: 10, StepDetailMaxBytes: 200, MaxStepsPersisted: 3}
	base := baseState()
	base.Steps = []domain.DocStep{
		mkStep(11, 1, "tool_call", "completed", 0),
		mkStep(12, 2, "tool_call", "running", 0),
		mkStep(13, 3, "workflow", "completed", 0),
		mkStep(14, 4, "subagent", "failed", 13),
	}
	sumID := tid(900) // first id of testEnv(900)

	// Patch 1: one add makes five steps; s11 goes first, then the leaf s14, then its parent s13.
	ch, norm, err := mustApply(t, base, lim, rawOp("add", "/steps/"+sid(15), `{"kind":"status","name":"wait"}`))
	if err != nil {
		t.Fatal(err)
	}
	sameNorm(t, "patch 1", norm, []domain.NormOp{
		nop(domain.OpAdd, "/steps/"+sid(15), `{"id":"`+sid(15)+`","ordinal":5,"kind":"status","name":"wait","title":"","status":"running","detail":{},"started_at":"2026-10-21T10:00:00.123Z"}`),
		nop(domain.OpRemove, "/steps/"+sid(11), ""),
		nop(domain.OpRemove, "/steps/"+sid(14), ""),
		nop(domain.OpRemove, "/steps/"+sid(13), ""),
		nop(domain.OpAdd, "/steps/"+sumID.String(), `{"id":"`+sumID.String()+`","ordinal":1,"kind":"status","name":"steps_summarized","title":"","status":"completed","detail":{"summarized":{"by_kind":{"subagent":1,"tool_call":1,"workflow":1},"count":3}},"started_at":"2026-10-21T10:00:00.123Z"}`),
	})
	if !reflect.DeepEqual(ch.DeleteSteps, []ids.ID{tid(11), tid(13), tid(14)}) {
		t.Errorf("deletes = %v", ch.DeleteSteps)
	}
	if len(ch.InsertSteps) != 2 || ch.InsertSteps[0].ID != sumID || ch.InsertSteps[1].ID != tid(15) {
		t.Errorf("inserts = %+v", ch.InsertSteps)
	}
	if len(ch.AddedSteps) != 1 || ch.AddedSteps[0].ID != tid(15) {
		t.Errorf("added = %+v", ch.AddedSteps)
	}
	if len(ch.After.Steps) != 3 {
		t.Errorf("after has %d steps", len(ch.After.Steps))
	}
	sum := summaryOf(t, ch.After)
	if sum.Ordinal != 1 || sum.Kind != "status" || sum.Name != domain.SummaryStepName || sum.Status != "completed" || !sum.FinishedAt.IsZero() {
		t.Errorf("summary = %+v", sum)
	}

	// Patch 2: a finished step added in this patch is a candidate too: it is counted, never
	// inserted and never announced.
	st2 := ch.After
	ch, norm, err = mustApply(t, st2, lim, rawOp("add", "/steps/"+sid(16), `{"kind":"workflow","name":"build","status":"completed"}`))
	if err != nil {
		t.Fatal(err)
	}
	sameNorm(t, "patch 2", norm, []domain.NormOp{
		nop(domain.OpReplace, "/steps/"+sumID.String()+"/detail", `{"summarized":{"by_kind":{"subagent":1,"tool_call":1,"workflow":2},"count":4}}`),
	})
	if len(ch.InsertSteps) != 0 || len(ch.AddedSteps) != 0 || len(ch.DeleteSteps) != 0 {
		t.Errorf("patch 2: inserts %v, added %v, deletes %v", ch.InsertSteps, ch.AddedSteps, ch.DeleteSteps)
	}
	if len(ch.UpdateSteps) != 1 || ch.UpdateSteps[0].ID != sumID || ch.UpdateSteps[0].Ordinal != nil {
		t.Errorf("patch 2: updates = %+v", ch.UpdateSteps)
	}

	// Patch 3: the summary step sits above a finished leaf and moves down to its ordinal.
	st3 := baseState()
	st3.Steps = []domain.DocStep{
		mkStep(21, 1, "tool_call", "cancelled", 0),
		mkStep(22, 2, "tool_call", "running", 0),
		{ID: tid(23), Ordinal: 3, Kind: "status", Name: domain.SummaryStepName, Status: "completed",
			Detail: json.RawMessage(`{"summarized":{"by_kind":{"tool_call":5},"count":5}}`), StartedAt: testNow},
	}
	ch, norm, err = mustApply(t, st3, lim, rawOp("add", "/steps/"+sid(24), `{"kind":"tool_call","name":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	sameNorm(t, "patch 3", norm, []domain.NormOp{
		nop(domain.OpAdd, "/steps/"+sid(24), `{"id":"`+sid(24)+`","ordinal":4,"kind":"tool_call","name":"x","title":"","status":"running","detail":{},"started_at":"2026-10-21T10:00:00.123Z"}`),
		nop(domain.OpRemove, "/steps/"+sid(21), ""),
		nop(domain.OpReplace, "/steps/"+sid(23)+"/detail", `{"summarized":{"by_kind":{"tool_call":6},"count":6}}`),
		nop(domain.OpReplace, "/steps/"+sid(23)+"/ordinal", `1`),
	})
	if len(ch.UpdateSteps) != 1 || ch.UpdateSteps[0].Ordinal == nil || *ch.UpdateSteps[0].Ordinal != 1 {
		t.Errorf("patch 3: updates = %+v", ch.UpdateSteps)
	}

	// No candidate: running steps and parents stay, the patch fails at its first step add.
	st4 := baseState()
	st4.Steps = []domain.DocStep{
		mkStep(31, 1, "tool_call", "completed", 0),
		mkStep(32, 2, "tool_call", "running", 31),
	}
	lim.MaxStepsPersisted = 2
	ch, norm, err = mustApply(t, st4, lim,
		rawOp("replace", "/status_line", `"x"`),
		rawOp("add", "/steps/"+sid(33), `{"kind":"tool_call","name":"x","parent_step_id":"`+sid(32)+`"}`),
		rawOp("add", "/steps/"+sid(34), `{"kind":"tool_call","name":"y","parent_step_id":"`+sid(32)+`"}`))
	if got := patchErrFields(t, err); !reflect.DeepEqual(got, []fe{{"/ops/1", "limit_reached"}}) {
		t.Errorf("errors = %v", got)
	}
	if norm != nil || !reflect.DeepEqual(ch, domain.Change{}) {
		t.Errorf("a failed patch returned a change")
	}

	// Without a step add the patch never summarizes, even above the limit.
	ch, _, err = mustApply(t, st4, domain.Limits{ExcerptMaxChars: 10, StepDetailMaxBytes: 200, MaxStepsPersisted: 1},
		rawOp("replace", "/steps/"+sid(32)+"/status", `"completed"`))
	if err != nil || len(ch.DeleteSteps) != 0 {
		t.Errorf("err %v, deletes %v", err, ch.DeleteSteps)
	}
}

// TestIsSummaryStep checks the recognition of the summary step.
func TestIsSummaryStep(t *testing.T) {
	t.Parallel()
	ok := domain.DocStep{Kind: "status", Name: domain.SummaryStepName, Detail: json.RawMessage(`{"summarized":{"by_kind":{},"count":0}}`)}
	if !domain.IsSummaryStep(ok) {
		t.Errorf("summary step not recognized")
	}
	for name, s := range map[string]domain.DocStep{
		"other kind":     {Kind: "tool_call", Name: domain.SummaryStepName, Detail: ok.Detail},
		"other name":     {Kind: "status", Name: "wait", Detail: ok.Detail},
		"detail missing": {Kind: "status", Name: domain.SummaryStepName},
		"no member":      {Kind: "status", Name: domain.SummaryStepName, Detail: json.RawMessage(`{"count":1}`)},
		"bad member":     {Kind: "status", Name: domain.SummaryStepName, Detail: json.RawMessage(`{"summarized":"many"}`)},
	} {
		if domain.IsSummaryStep(s) {
			t.Errorf("%s: recognized as summary step", name)
		}
	}
}
