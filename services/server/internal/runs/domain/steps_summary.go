// SPDX-License-Identifier: Apache-2.0

package domain

// The summarization of old steps (09 5.3, 03 run_steps): a run keeps at most
// run.max_steps_persisted steps; above it the server folds the oldest finished steps into one
// summary step that counts them by kind.

import (
	"bytes"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SummaryStepName is the name of the summary step; with kind status and a detail that has the
// member summarized it marks the one step that counts the steps the server removed.
const SummaryStepName = "steps_summarized"

// The fixed values of the summary step.
const (
	summaryKind   = "status"
	summaryStatus = stepStatusCompleted
)

// summaryCounts is the member summarized of the detail of the summary step:
// {"by_kind":{"<kind>":n,...},"count":n}.
type summaryCounts struct {
	ByKind map[string]int64 `json:"by_kind"`
	Count  int64            `json:"count"`
}

// summaryOf reads the member summarized of a detail; false when the detail is absent (not
// loaded), not an object or has no such member of the right shape.
func summaryOf(detail json.RawMessage) (summaryCounts, bool) {
	if detail == nil {
		return summaryCounts{}, false
	}
	var d struct {
		Summarized *summaryCounts `json:"summarized"`
	}
	if json.Unmarshal(detail, &d) != nil || d.Summarized == nil {
		return summaryCounts{}, false
	}
	return *d.Summarized, true
}

// IsSummaryStep reports whether s is the summary step of its run: kind status, name
// steps_summarized and a detail with the member summarized ({"by_kind":{...},"count":n}). The
// use case loads the detail of such a step only (DocStep.Detail), so a step whose detail was not
// loaded is never the summary step.
//
// Example:
//
//	domain.IsSummaryStep(domain.DocStep{Kind: "status", Name: "steps_summarized",
//		Detail: []byte(`{"summarized":{"by_kind":{"tool_call":2},"count":2}}`)}) // true
func IsSummaryStep(s DocStep) bool {
	if s.Kind != summaryKind || s.Name != SummaryStepName {
		return false
	}
	_, ok := summaryOf(s.Detail)
	return ok
}

// isFinished reports the end states of a step; only finished steps may be summarized.
func isFinished(status string) bool {
	return status == stepStatusCompleted || status == stepStatusFailed || status == stepStatusCancelled
}

// summarize runs after the step adds of a patch. While the run has more steps than
// lim.MaxStepsPersisted (run.max_steps_persisted), it removes the finished step with the lowest
// ordinal that has no child and is not the summary step, and counts it into the summary step,
// which it creates at the lowest removed ordinal on the first removal (and moves there when a
// removed ordinal is lower than its own). Running steps and parents stay. A step the adapter
// added in this patch may be removed too: it is counted, never inserted, and its normalized ops
// are dropped, so the normalized ops still apply to the base document.
//
// It fails with limit_reached at the first step add of the patch when no step can be removed.
func (a *applier) summarize() error {
	order := slices.Clone(a.steps)
	slices.SortStableFunc(order, func(x, y *workStep) int { return x.Ordinal - y.Ordinal })
	summary, children, live := indexSteps(order)

	var removed []*workStep
	counts := summaryCounts{ByKind: map[string]int64{}}
	created := false
	for live > a.lim.MaxStepsPersisted {
		c := candidate(order, summary, children)
		if c == nil {
			return fail(a.firstAdd, "", fieldLimitReached, detailLimitReached)
		}
		c.deleted = true
		live--
		if !c.ParentStepID.IsZero() {
			children[c.ParentStepID]--
		}
		removed = append(removed, c)
		counts.ByKind[c.Kind]++
		counts.Count++
		if summary == nil {
			// The summary step takes the place of the steps it counts; its ordinal is set below,
			// once the lowest removed ordinal is known.
			summary = &workStep{}
			created = true
			live++
		}
	}
	if len(removed) == 0 {
		return nil
	}
	lowest := removed[0].Ordinal
	for _, r := range removed {
		lowest = min(lowest, r.Ordinal)
	}
	a.dropNormOpsOf(removed)
	for _, r := range removed {
		if r.base {
			a.emit(OpRemove, "/steps/"+r.ID.String(), nil)
		}
	}
	if created {
		return a.createSummary(summary, lowest, counts)
	}
	a.mergeSummary(summary, lowest, counts)
	return nil
}

// indexSteps walks the steps in ordinal order once and returns the summary step of the base
// document (nil when there is none), the number of live children per parent id and the number of
// live steps.
func indexSteps(order []*workStep) (*workStep, map[ids.ID]int, int) {
	var summary *workStep
	children := map[ids.ID]int{}
	live := 0
	for _, ws := range order {
		if ws.deleted {
			continue
		}
		live++
		if !ws.ParentStepID.IsZero() {
			children[ws.ParentStepID]++
		}
		if summary == nil && ws.base && IsSummaryStep(ws.DocStep) {
			summary = ws
		}
	}
	return summary, children, live
}

// createSummary fills the new summary step at ordinal lowest with counts, adds it to the run and
// emits its add.
func (a *applier) createSummary(summary *workStep, lowest int, counts summaryCounts) error {
	if a.env.NewID == nil {
		return errs.New(errs.CodeInternalError, "no id source for the summary step")
	}
	summary.DocStep = DocStep{
		ID: a.env.NewID(), Ordinal: lowest, Kind: summaryKind, Name: SummaryStepName,
		Status: summaryStatus, Detail: summaryDetail(nil, counts), StartedAt: storedTime(a.env.Now),
	}
	a.steps = append(a.steps, summary)
	a.byID[summary.ID] = summary
	a.emit(OpAdd, "/steps/"+summary.ID.String(), encodeStep(summary.DocStep))
	return nil
}

// mergeSummary adds counts to the existing summary step, moves it to ordinal lowest when that is
// lower than its own and emits the replaces.
func (a *applier) mergeSummary(summary *workStep, lowest int, counts summaryCounts) {
	old, _ := summaryOf(summary.Detail)
	merged := summaryCounts{ByKind: map[string]int64{}, Count: old.Count + counts.Count}
	for _, by := range []map[string]int64{old.ByKind, counts.ByKind} {
		for k, n := range by {
			merged.ByKind[k] += n
		}
	}
	summary.Detail, summary.detailSet = summaryDetail(summary.Detail, merged), true
	path := "/steps/" + summary.ID.String()
	a.emit(OpReplace, path+"/detail", summary.Detail)
	if lowest < summary.Ordinal {
		summary.Ordinal = lowest
		a.emit(OpReplace, path+"/ordinal", json.RawMessage(strconv.Itoa(lowest)))
	}
}

// candidate returns the finished step with the lowest ordinal that has no live child and is not
// the summary step, or nil.
func candidate(order []*workStep, summary *workStep, children map[ids.ID]int) *workStep {
	for _, ws := range order {
		if ws.deleted || ws == summary || children[ws.ID] > 0 || !isFinished(ws.Status) {
			continue
		}
		return ws
	}
	return nil
}

// dropNormOpsOf drops the normalized ops of removed steps the adapter added in this patch: those
// steps never reach the store, so their add and member replaces must not reach clients.
func (a *applier) dropNormOpsOf(removed []*workStep) {
	gone := map[string]bool{}
	for _, r := range removed {
		if !r.base {
			gone[r.ID.String()] = true
		}
	}
	if len(gone) == 0 {
		return
	}
	a.norm = slices.DeleteFunc(a.norm, func(n NormOp) bool {
		rest, ok := strings.CutPrefix(n.Path, "/steps/")
		if !ok {
			return false
		}
		id, _, _ := strings.Cut(rest, "/")
		return gone[id]
	})
}

// summaryDetail returns the detail of the summary step with the member summarized set to counts;
// other members of an existing detail are kept.
func summaryDetail(existing json.RawMessage, counts summaryCounts) json.RawMessage {
	m := map[string]any{}
	if existing != nil {
		if v, err := decodeJSON(existing); err == nil {
			if obj, ok := v.(map[string]any); ok {
				m = obj
			}
		}
	}
	if counts.ByKind == nil {
		counts.ByKind = map[string]int64{}
	}
	m["summarized"] = counts
	return bytes.Clone(encodeCanonical(m))
}
