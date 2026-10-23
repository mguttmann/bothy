// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// Values of run.state_changed (05 section 6).
const (
	resourceRun    = "run"
	receiveRunRead = string(authorize.ActionRunRead)
)

// summary converts a row into the port DTO with the channel of its thread now and its number of
// steps.
func summary(r RunRow, channelID ids.ID, steps int) runs.RunSummary {
	return runs.RunSummary{
		ID: r.ID, ThreadID: r.ThreadID, ChannelID: channelID, AgentID: r.AgentID, TriggeredBy: r.TriggeredBy,
		State: string(r.State), StateReason: r.StateReason, Version: r.Version,
		BillingModeExpected: r.BillingModeExpected, BillingModeUsed: r.BillingModeUsed,
		StatusLine: r.StatusLine, Summary: r.Summary,
		Cost:      runs.Money{AmountMicros: r.CostMicros, Currency: r.Currency, IsEstimate: r.CostIsEstimate},
		StepCount: steps, DurationMS: r.DurationMS, HasDuration: r.HasDuration,
		QueuedAt: r.QueuedAt, StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

// object decodes a stored JSON object; an empty or invalid value is the empty object, so a
// document never carries null (ADR 0053).
func object(raw []byte) map[string]any {
	out := map[string]any{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// step converts a step row into the port DTO.
func step(r StepRow) runs.RunStep {
	return runs.RunStep{
		ID: r.ID, Ordinal: r.Ordinal, ParentStepID: r.ParentStepID, Kind: r.Kind, Name: r.Name, Title: r.Title,
		Status: r.Status, Detail: object(r.Detail), StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

// document builds the run document of r (schema Run): the summary, the trigger, the stored JSON
// objects and the steps; the plan only when it has items.
func document(r RunRow, channelID ids.ID, rows []StepRow) runs.Run {
	steps := make([]runs.RunStep, 0, len(rows))
	for _, s := range rows {
		steps = append(steps, step(s))
	}
	doc := runs.Run{
		RunSummary: summary(r, channelID, len(rows)), AgentSessionID: r.AgentSessionID, TriggerKind: r.TriggerKind,
		TriggeringMessageID: r.TriggeringMessageID, TriggeringCommandID: r.TriggeringCommandID,
		TriggeringInteractionID: r.TriggeringInteractionID, Model: r.Model, LastTextExcerpt: r.LastTextExcerpt,
		Metrics: object(r.Metrics), Steps: steps, Followups: []runs.Followup{},
	}
	if plan := object(r.Plan); hasItems(plan) {
		doc.Plan = plan
	}
	return doc
}

// hasItems reports a plan with a non-empty items list (schema RunPlan).
func hasItems(plan map[string]any) bool {
	items, ok := plan["items"].([]any)
	return ok && len(items) > 0
}

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// optionalID formats id; the zero ID is absent.
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// moneyJSON is the schema Money.
type moneyJSON struct {
	AmountMicros int64  `json:"amount_micros"`
	Currency     string `json:"currency"`
	IsEstimate   bool   `json:"is_estimate"`
}

// summaryJSON is the schema RunSummary in an event; optional members are left out, never null.
type summaryJSON struct {
	ID                  string    `json:"id"`
	ThreadID            string    `json:"thread_id"`
	ChannelID           string    `json:"channel_id"`
	AgentID             string    `json:"agent_id"`
	TriggeredBy         string    `json:"triggered_by"`
	State               string    `json:"state"`
	StateReason         string    `json:"state_reason,omitempty"`
	Version             int64     `json:"version"`
	BillingModeExpected string    `json:"billing_mode_expected"`
	BillingModeUsed     string    `json:"billing_mode_used,omitempty"`
	StatusLine          string    `json:"status_line,omitempty"`
	Summary             string    `json:"summary,omitempty"`
	Cost                moneyJSON `json:"cost"`
	StepCount           int       `json:"step_count"`
	DurationMS          *int64    `json:"duration_ms,omitempty"`
	QueuedAt            string    `json:"queued_at"`
	StartedAt           string    `json:"started_at,omitempty"`
	FinishedAt          string    `json:"finished_at,omitempty"`
}

// summaryBody converts a summary into the JSON form of the schema RunSummary.
func summaryBody(r runs.RunSummary) summaryJSON {
	out := summaryJSON{
		ID: r.ID.String(), ThreadID: r.ThreadID.String(), ChannelID: r.ChannelID.String(), AgentID: r.AgentID.String(),
		TriggeredBy: r.TriggeredBy.String(), State: r.State, StateReason: r.StateReason, Version: r.Version,
		BillingModeExpected: r.BillingModeExpected, BillingModeUsed: r.BillingModeUsed, StatusLine: r.StatusLine,
		Summary: r.Summary, Cost: moneyJSON{AmountMicros: r.Cost.AmountMicros, Currency: r.Cost.Currency, IsEstimate: r.Cost.IsEstimate},
		StepCount: r.StepCount, QueuedAt: stamp(r.QueuedAt), StartedAt: stamp(r.StartedAt), FinishedAt: stamp(r.FinishedAt),
	}
	if r.HasDuration {
		d := r.DurationMS
		out.DurationMS = &d
	}
	return out
}

// stateChangedJSON is the payload RunStateChangedPayload.
type stateChangedJSON struct {
	Run                 summaryJSON `json:"run"`
	TriggeringMessageID string      `json:"triggering_message_id,omitempty"`
	TriggerState        string      `json:"trigger_state,omitempty"`
}

// encode marshals v without HTML escaping, so the size of a payload is the size of its content.
func encode(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "runs: encoding an event payload")
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// emitStateChanged appends run.state_changed for the run sum in the thread scope of space (04
// section 11.3 point 1): resource run with runs.version, receive action run.read, no audience,
// the payload with the triggering message and the trigger state this change set (absent when it
// set none).
func (s *Service) emitStateChanged(ctx context.Context, spaceID ids.ID, sum runs.RunSummary, triggering ids.ID, trigger domain.TriggerState) error {
	p, err := encode(stateChangedJSON{Run: summaryBody(sum), TriggeringMessageID: optionalID(triggering), TriggerState: string(trigger)})
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventRunStateChanged), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: spaceID, ChannelID: sum.ChannelID, ThreadID: sum.ThreadID, ResourceKind: resourceRun, ResourceID: sum.ID,
		ResourceVersion: sum.Version, ReceiveAction: receiveRunRead, Payload: p,
	})
}

// eventActor returns the actor of the request in ctx; none outside a request (a job).
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}
