// SPDX-License-Identifier: Apache-2.0

package domain

// The summarization of old steps (09 5.3, 03 run_steps, R-DM-53): a run keeps at most
// run.max_steps_persisted steps; above it the server folds the oldest finished steps into one
// summary step per parent that counts them by kind.

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
	return status == stepStatusCompleted || status == "failed" || status == "cancelled"
}

// group is the summary step of one parent during a summarization (09 5.3, addendum of
// 2026-10-24, D-RV-6): the stored or new summary step, the counts this patch adds to it and the
// lowest ordinal of the steps removed into it.
type group struct {
	parent ids.ID
	// step is the summary step; nil until the first removal into a group without one.
	step *workStep
	// created reports that this patch creates the step; gone that the summarization removed it
	// together with its parent.
	created, gone bool
	add           summaryCounts
	lowest        int
}

// total returns the stored counts of the summary step plus the counts this patch adds.
func (g *group) total() summaryCounts {
	out := summaryCounts{ByKind: map[string]int64{}, Count: g.add.Count}
	if g.step != nil && !g.created {
		old, _ := summaryOf(g.step.Detail)
		out.Count += old.Count
		for k, n := range old.ByKind {
			out.ByKind[k] += n
		}
	}
	for k, n := range g.add.ByKind {
		out.ByKind[k] += n
	}
	return out
}

// summarizer is the state of one summarization: the steps in ordinal order, the live children
// per parent, the summary step per parent (the zero id is the root) and the number of live steps.
type summarizer struct {
	order    []*workStep
	children map[ids.ID]int
	groups   map[ids.ID]*group
	// summaries marks the summary steps, so they are never candidates of their own.
	summaries map[*workStep]bool
	live      int
	removed   []*workStep
	// touched are the groups in the order of their first removal.
	touched []*group
}

// summarize runs after the step adds of a patch. While the run has more steps than
// lim.MaxStepsPersisted (run.max_steps_persisted), it removes the finished step with the lowest
// ordinal that can go (candidate) and counts it into the summary step of its own parent (09 5.3,
// addendum of 2026-10-24: one summary step per parent, the root counting as one parent). The
// summary step of a group is created on the first removal into it, at the lowest ordinal removed
// into it and with that parent, and moves down when a lower ordinal is removed into it. A finished
// step whose only child is its own summary step goes together with that summary step, and both
// counts go into the summary of the step's parent, so a finished subtree collapses upward.
// Running steps, their ancestors and every other parent stay. A step the adapter added in this
// patch may be removed too: it is counted, never inserted, and its normalized ops are dropped, so
// the normalized ops still apply to the base document. Summary steps stored before the addendum
// sit at the root and are simply the root's summary step.
//
// It fails with limit_reached at the first step add of the patch when no step can be removed.
func (a *applier) summarize() error {
	z := newSummarizer(a.steps)
	for z.live > a.lim.MaxStepsPersisted {
		c := z.candidate()
		if c == nil {
			return fail(a.firstAdd, "", fieldLimitReached, detailLimitReached)
		}
		z.remove(c)
	}
	if len(z.removed) == 0 {
		return nil
	}
	a.dropNormOpsOf(z.removed)
	for _, r := range z.removed {
		if r.base {
			a.emit(OpRemove, "/steps/"+r.ID.String(), nil)
		}
	}
	for _, g := range z.touched {
		if g.gone {
			continue
		}
		if g.created {
			if err := a.createSummary(g); err != nil {
				return err
			}
			continue
		}
		a.mergeSummary(g)
	}
	return nil
}

// newSummarizer indexes the live steps in ordinal order once: the live children per parent and
// the summary step of each parent found among the base steps (the first one per parent).
func newSummarizer(steps []*workStep) *summarizer {
	z := &summarizer{
		order:    slices.Clone(steps),
		children: map[ids.ID]int{}, groups: map[ids.ID]*group{}, summaries: map[*workStep]bool{},
	}
	slices.SortStableFunc(z.order, func(x, y *workStep) int { return x.Ordinal - y.Ordinal })
	for _, ws := range z.order {
		if ws.deleted {
			continue
		}
		z.live++
		if !ws.ParentStepID.IsZero() {
			z.children[ws.ParentStepID]++
		}
		if ws.base && IsSummaryStep(ws.DocStep) {
			z.summaries[ws] = true
			if _, ok := z.groups[ws.ParentStepID]; !ok {
				z.groups[ws.ParentStepID] = &group{parent: ws.ParentStepID, step: ws, lowest: ws.Ordinal}
			}
		}
	}
	return z
}

// candidate returns the finished step with the lowest ordinal that is not a summary step and has
// no live child, or whose only live child is its own summary step; nil when there is none.
func (z *summarizer) candidate() *workStep {
	for _, ws := range z.order {
		if ws.deleted || z.summaries[ws] || !isFinished(ws.Status) {
			continue
		}
		switch n := z.children[ws.ID]; {
		case n == 0:
			return ws
		case n == 1:
			if g, ok := z.groups[ws.ID]; ok && g.step != nil && !g.gone {
				return ws
			}
		}
	}
	return nil
}

// remove removes c (and its own summary step, if it has one) and counts both into the summary
// step of c's parent, creating that step on the first removal into the group.
func (z *summarizer) remove(c *workStep) {
	c.deleted = true
	z.live--
	z.removed = append(z.removed, c)
	parent := c.ParentStepID
	if !parent.IsZero() {
		z.children[parent]--
	}
	g := z.groups[parent]
	if g == nil {
		g = &group{parent: parent, lowest: c.Ordinal}
		z.groups[parent] = g
	}
	if g.step == nil {
		// The summary step takes the place of the steps it counts; it is a child of the parent.
		g.step, g.created = &workStep{}, true
		z.summaries[g.step] = true
		z.live++
		if !parent.IsZero() {
			z.children[parent]++
		}
	}
	if !slices.Contains(z.touched, g) {
		z.touched = append(z.touched, g)
	}
	if g.add.ByKind == nil {
		g.add.ByKind = map[string]int64{}
	}
	g.add.ByKind[c.Kind]++
	g.add.Count++
	g.lowest = min(g.lowest, c.Ordinal)
	if own, ok := z.groups[c.ID]; ok && own.step != nil && !own.gone {
		// c's only child is its own summary step: it goes with c, and its counts move up. Its
		// step is never counted as a step of its own.
		own.gone = true
		z.children[c.ID]--
		z.live--
		if !own.created {
			own.step.deleted = true
			z.removed = append(z.removed, own.step)
		}
		t := own.total()
		g.add.Count += t.Count
		for k, n := range t.ByKind {
			g.add.ByKind[k] += n
		}
	}
}

// createSummary fills the new summary step of g at its lowest ordinal with its counts and its
// parent, adds it to the run and emits its add.
func (a *applier) createSummary(g *group) error {
	if a.env.NewID == nil {
		return errs.New(errs.CodeInternalError, "no id source for the summary step")
	}
	summary := g.step
	summary.DocStep = DocStep{
		ID: a.env.NewID(), Ordinal: g.lowest, ParentStepID: g.parent, Kind: summaryKind, Name: SummaryStepName,
		Status: summaryStatus, Detail: summaryDetail(nil, g.total()), StartedAt: storedTime(a.env.Now),
	}
	a.steps = append(a.steps, summary)
	a.byID[summary.ID] = summary
	a.emit(OpAdd, "/steps/"+summary.ID.String(), encodeStep(summary.DocStep))
	return nil
}

// mergeSummary adds the counts of g to its stored summary step, moves it to the lowest ordinal
// removed into it when that is lower than its own and emits the replaces.
func (a *applier) mergeSummary(g *group) {
	summary := g.step
	summary.Detail, summary.detailSet = summaryDetail(summary.Detail, g.total()), true
	path := "/steps/" + summary.ID.String()
	a.emit(OpReplace, path+"/detail", summary.Detail)
	if g.lowest < summary.Ordinal {
		summary.Ordinal = g.lowest
		a.emit(OpReplace, path+"/ordinal", json.RawMessage(strconv.Itoa(g.lowest)))
	}
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
