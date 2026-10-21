// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// AgentSessions implements listAgentSessions, resetAgentSession, bindAgentSession and
// reportAgentSession (M2-12, docs/spec/09-agenten.md section 9) over agents.Sessions. The use
// cases authorize; the handlers convert, set ETag from the version of the answered row and pass
// errors to the problem writer of the chain unchanged. Idempotency-Key is handled by the chain
// (M1-20).
//
// Example:
//
//	h := httpapi.NewAgentSessions(sessions)
//	built, err := routes.Build(wire.Handlers{AgentSessions: h}, wire.RouteOptions(), cat)
type AgentSessions struct {
	sessions agents.Sessions
}

// NewAgentSessions returns the handlers over the use cases s.
func NewAgentSessions(s agents.Sessions) *AgentSessions { return &AgentSessions{sessions: s} }

// sessionETag returns the ETag header value of s.
func sessionETag(s agents.Session) *string {
	v := httpserver.FormatETag(s.Version)
	return &v
}

// sessionREST converts a session row into the schema AgentSession; empty values become absent
// members, never null (ADR 0053). cost_baseline_micros is never shown.
func sessionREST(s agents.Session) rest.AgentSession {
	out := rest.AgentSession{
		Id: rest.Uuid(s.ID), ThreadId: rest.Uuid(s.ThreadID), AgentId: rest.Uuid(s.AgentID),
		ExternalSessionRef: optional(s.ExternalSessionRef), IsCurrent: s.IsCurrent, Started: s.Started, Model: optional(s.Model),
		Cost:      rest.Money{AmountMicros: s.CumulativeCostMicros, Currency: s.Currency, IsEstimate: s.CostIsEstimate},
		CreatedAt: s.CreatedAt.UTC().Truncate(time.Millisecond), EndedAt: optionalTime(s.EndedAt), Version: s.Version,
	}
	if s.EndReason != "" {
		r := rest.AgentSessionEndReason(s.EndReason)
		out.EndReason = &r
	}
	return out
}

// ListAgentSessions implements listAgentSessions: the sessions of the thread, newest first, not
// paginated (06 section 6 point 6).
func (h *AgentSessions) ListAgentSessions(ctx context.Context, req rest.ListAgentSessionsRequestObject) (rest.ListAgentSessionsResponseObject, error) {
	list, err := h.sessions.List(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	items := make([]rest.AgentSession, 0, len(list))
	for _, s := range list {
		items = append(items, sessionREST(s))
	}
	return rest.ListAgentSessions200JSONResponse{Body: rest.AgentSessionList{Items: items}}, nil
}

// ResetAgentSession implements resetAgentSession: 201 with the ended row and its ETag.
func (h *AgentSessions) ResetAgentSession(ctx context.Context, req rest.ResetAgentSessionRequestObject) (rest.ResetAgentSessionResponseObject, error) {
	s, err := h.sessions.Reset(ctx, principal(ctx), ids.ID(req.ThreadId))
	if err != nil {
		return nil, err
	}
	return rest.ResetAgentSession201JSONResponse{Body: sessionREST(s), Headers: rest.ResetAgentSession201ResponseHeaders{ETag: sessionETag(s)}}, nil
}

// BindAgentSession implements bindAgentSession: 201 with the new current row and its ETag.
func (h *AgentSessions) BindAgentSession(ctx context.Context, req rest.BindAgentSessionRequestObject) (rest.BindAgentSessionResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "the request body is required")
	}
	cmd := agents.BindCmd{ExternalSessionRef: req.Body.ExternalSessionRef}
	if req.Body.ExpectedCurrentSessionId != nil {
		cmd.ExpectedCurrentSessionID = ids.ID(*req.Body.ExpectedCurrentSessionId)
	}
	if req.Body.TakeOver != nil {
		cmd.TakeOver = *req.Body.TakeOver
	}
	s, err := h.sessions.Bind(ctx, principal(ctx), ids.ID(req.ThreadId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.BindAgentSession201JSONResponse{Body: sessionREST(s), Headers: rest.BindAgentSession201ResponseHeaders{ETag: sessionETag(s)}}, nil
}

// ReportAgentSession implements reportAgentSession: 200 with the session row of the run and its
// ETag; If-Match is the compare-and-swap of the report (absent: the run has no row yet).
func (h *AgentSessions) ReportAgentSession(ctx context.Context, req rest.ReportAgentSessionRequestObject) (rest.ReportAgentSessionResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "the request body is required")
	}
	cmd := agents.ReportCmd{ExternalSessionRef: req.Body.ExternalSessionRef}
	if req.Body.Model != nil {
		cmd.Model = *req.Body.Model
	}
	if req.Body.Started != nil {
		cmd.Started = *req.Body.Started
	}
	if req.Body.BindFailed != nil {
		cmd.BindFailed = string(*req.Body.BindFailed)
	}
	if req.Params.IfMatch != nil {
		cmd.IfMatch = precondition(*req.Params.IfMatch)
	}
	s, err := h.sessions.Report(ctx, principal(ctx), ids.ID(req.ThreadId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.ReportAgentSession200JSONResponse{Body: sessionREST(s), Headers: rest.ReportAgentSession200ResponseHeaders{ETag: sessionETag(s)}}, nil
}
