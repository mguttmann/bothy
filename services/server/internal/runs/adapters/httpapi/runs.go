// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the runs module as part of rest.StrictServerInterface
// (M2-05): getRun, listThreadRuns, listRunSteps and listActiveRuns of docs/api/openapi.yaml, the
// lifecycle of M2-09 and createInteraction, respondToInteraction and updateComponent (M2-15). The
// use cases authorize; the handlers only convert, set the ETag of getRun and pass errors to the
// problem writer of the chain unchanged. Cursors is the codec of platform/httpserver for the use
// cases.
//
// Example:
//
//	h := httpapi.New(svc)
//	built, err := routes.Build(wire.Handlers{Runs: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// Runs implements the REST operations of runs.
type Runs struct {
	runs         runs.Runs
	interactions runs.Interactions
}

// Ports are the incoming ports of runs the handlers call: the runs (M2-05, M2-09) and the
// components and interactions (M2-15).
type Ports interface {
	runs.Runs
	runs.Interactions
}

// New returns the handlers over the ports of runs.
func New(p Ports) *Runs { return &Runs{runs: p, interactions: p} }

// Cursors is the codec of the list cursors (platform/httpserver), the outgoing port app.Cursors.
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// GetRun implements rest.StrictServerInterface (GET /runs/{run_id}): the run document with the
// ETag of its version.
func (h *Runs) GetRun(ctx context.Context, req rest.GetRunRequestObject) (rest.GetRunResponseObject, error) {
	r, err := h.runs.Get(ctx, ids.ID(req.RunId))
	if err != nil {
		return nil, err
	}
	body, err := RunREST(r)
	if err != nil {
		return nil, err
	}
	etag := httpserver.FormatETag(r.Version)
	return rest.GetRun200JSONResponse{Body: body, Headers: rest.GetRun200ResponseHeaders{ETag: &etag}}, nil
}

// ListThreadRuns implements rest.StrictServerInterface (GET /threads/{thread_id}/runs).
func (h *Runs) ListThreadRuns(ctx context.Context, req rest.ListThreadRunsRequestObject) (rest.ListThreadRunsResponseObject, error) {
	page, err := h.runs.ListByThread(ctx, runs.ThreadRunsQuery{
		ThreadID: ids.ID(req.ThreadId), Cursor: deref(req.Params.Cursor), Limit: derefInt(req.Params.Limit),
	})
	if err != nil {
		return nil, err
	}
	return rest.ListThreadRuns200JSONResponse{Body: summaryPage(page)}, nil
}

// ListRunSteps implements rest.StrictServerInterface (GET /runs/{run_id}/steps).
func (h *Runs) ListRunSteps(ctx context.Context, req rest.ListRunStepsRequestObject) (rest.ListRunStepsResponseObject, error) {
	page, err := h.runs.ListSteps(ctx, runs.RunStepsQuery{RunID: ids.ID(req.RunId), Cursor: deref(req.Params.Cursor), Limit: derefInt(req.Params.Limit)})
	if err != nil {
		return nil, err
	}
	out := rest.RunStepPage{Items: make([]rest.RunStep, 0, len(page.Items))}
	for _, s := range page.Items {
		out.Items = append(out.Items, stepREST(s))
	}
	if page.NextCursor != "" {
		c := page.NextCursor
		out.NextCursor = &c
	}
	return rest.ListRunSteps200JSONResponse{Body: out}, nil
}

// ListActiveRuns implements rest.StrictServerInterface (GET /runs/active).
func (h *Runs) ListActiveRuns(ctx context.Context, req rest.ListActiveRunsRequestObject) (rest.ListActiveRunsResponseObject, error) {
	q := runs.ActiveRunsQuery{Cursor: deref(req.Params.Cursor), Limit: derefInt(req.Params.Limit)}
	if req.Params.AgentId != nil {
		q.AgentID = ids.ID(*req.Params.AgentId)
	}
	page, err := h.runs.ListActive(ctx, q)
	if err != nil {
		return nil, err
	}
	return rest.ListActiveRuns200JSONResponse{Body: summaryPage(page)}, nil
}

// summaryPage converts a page of runs (schema RunSummaryPage).
func summaryPage(p runs.RunSummaryPage) rest.RunSummaryPage {
	out := rest.RunSummaryPage{Items: make([]rest.RunSummary, 0, len(p.Items))}
	for _, r := range p.Items {
		out.Items = append(out.Items, SummaryREST(r))
	}
	if p.NextCursor != "" {
		c := p.NextCursor
		out.NextCursor = &c
	}
	return out
}

// SummaryREST converts a run summary (schema RunSummary); absent values stay absent members.
func SummaryREST(r runs.RunSummary) rest.RunSummary {
	out := rest.RunSummary{
		Id: rest.Uuid(r.ID), ThreadId: rest.Uuid(r.ThreadID), ChannelId: rest.Uuid(r.ChannelID), AgentId: rest.Uuid(r.AgentID),
		TriggeredBy: rest.Uuid(r.TriggeredBy), State: rest.RunState(r.State), StateReason: optionalString(r.StateReason),
		Version: r.Version, BillingModeExpected: rest.BillingMode(r.BillingModeExpected), BillingModeUsed: optionalMode(r.BillingModeUsed),
		StatusLine: optionalString(r.StatusLine), Summary: optionalString(r.Summary), Cost: money(r.Cost), StepCount: r.StepCount,
		QueuedAt: timestamp(r.QueuedAt), StartedAt: optionalTime(r.StartedAt), FinishedAt: optionalTime(r.FinishedAt),
	}
	if r.HasDuration {
		d := r.DurationMS
		out.DurationMs = &d
	}
	return out
}

// RunREST converts a run document (schema Run): the steps keyed by step ID, followups the empty
// object until M2-11, metrics and plan from the stored objects (the plan only with items).
// Errors: internal when a stored object does not fit its schema.
func RunREST(r runs.Run) (rest.Run, error) {
	s := SummaryREST(r.RunSummary)
	out := rest.Run{
		Id: s.Id, ThreadId: s.ThreadId, ChannelId: s.ChannelId, AgentId: s.AgentId, TriggeredBy: s.TriggeredBy, State: s.State,
		StateReason: s.StateReason, Version: s.Version, BillingModeExpected: s.BillingModeExpected, BillingModeUsed: s.BillingModeUsed,
		StatusLine: s.StatusLine, Summary: s.Summary, Cost: s.Cost, DurationMs: s.DurationMs, QueuedAt: s.QueuedAt, StartedAt: s.StartedAt,
		FinishedAt: s.FinishedAt, AgentSessionId: optionalID(r.AgentSessionID), TriggeringMessageId: optionalID(r.TriggeringMessageID),
		TriggeringCommandId: optionalID(r.TriggeringCommandID), TriggeringInteractionId: optionalID(r.TriggeringInteractionID),
		Model: optionalString(r.Model), LastTextExcerpt: optionalString(r.LastTextExcerpt),
		Steps: make(map[string]rest.RunStep, len(r.Steps)), Followups: map[string]rest.RunFollowup{},
	}
	if r.TriggerKind != "" {
		k := rest.RunTriggerKind(r.TriggerKind)
		out.TriggerKind = &k
	}
	for _, st := range r.Steps {
		out.Steps[st.ID.String()] = stepREST(st)
	}
	if err := convert(r.Metrics, &out.Metrics); err != nil {
		return rest.Run{}, err
	}
	if r.Plan != nil {
		var plan rest.RunPlan
		if err := convert(r.Plan, &plan); err != nil {
			return rest.Run{}, err
		}
		out.Plan = &plan
	}
	return out, nil
}

// convert moves a stored JSON object into its schema type.
func convert(in map[string]any, out any) error {
	b, err := json.Marshal(in)
	if err == nil {
		err = json.Unmarshal(b, out)
	}
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "runs: a stored object does not fit its schema")
	}
	return nil
}

// stepREST converts a step (schema RunStep); the detail is always an object.
func stepREST(s runs.RunStep) rest.RunStep {
	detail := rest.JsonObject(s.Detail)
	if detail == nil {
		detail = rest.JsonObject{}
	}
	return rest.RunStep{
		Id: rest.Uuid(s.ID), Ordinal: s.Ordinal, ParentStepId: optionalID(s.ParentStepID), Kind: rest.RunStepKind(s.Kind), Name: s.Name,
		Title: s.Title, Status: rest.RunStepStatus(s.Status), Detail: &detail, StartedAt: timestamp(s.StartedAt),
		FinishedAt: optionalTime(s.FinishedAt),
	}
}

// money converts an amount (schema Money).
func money(m runs.Money) rest.Money {
	return rest.Money{AmountMicros: m.AmountMicros, Currency: m.Currency, IsEstimate: m.IsEstimate}
}

// timestamp cuts t to the milliseconds of the API (06 section 2).
func timestamp(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }

// optionalTime is nil for the zero time.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := timestamp(t)
	return &v
}

// optionalString is nil for "".
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// optionalMode is nil for "".
func optionalMode(s string) *rest.BillingMode {
	if s == "" {
		return nil
	}
	m := rest.BillingMode(s)
	return &m
}

// optionalID is nil for the zero ID.
func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// deref is "" for nil.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// derefInt is 0 for nil.
func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
