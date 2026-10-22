// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// WithPresence binds the presence port and sync.heartbeat_interval, the interval_ms of
// HeartbeatAck (M2-20), and returns h.
func (h *Agents) WithPresence(p agents.Presence, interval time.Duration) *Agents {
	h.presence, h.interval = p, interval
	return h
}

// SendHeartbeat implements rest.StrictServerInterface (POST /agents/me/heartbeat). The chain has
// checked the body against the schema Heartbeat; the generated object is encoded again and handed
// to agents.Presence, whose decoder is the only mapping of the wire form, so the gateway member
// agent and this body take the same path (brief M2-20 note 1). Errors pass to the problem writer.
func (h *Agents) SendHeartbeat(ctx context.Context, req rest.SendHeartbeatRequestObject) (rest.SendHeartbeatResponseObject, error) {
	if h.presence == nil {
		return nil, errs.New(errs.CodeInternalError, "agents: no presence bound")
	}
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "a body is required")
	}
	raw, err := json.Marshal(req.Body)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "agents: encoding a heartbeat")
	}
	st, err := h.presence.Heartbeat(ctx, raw)
	if err != nil {
		return nil, err
	}
	return rest.SendHeartbeat200JSONResponse{Body: rest.HeartbeatAck{Presence: presenceREST(st), IntervalMs: h.interval.Milliseconds()}}, nil
}

// presenceREST converts a presence (schema AgentRuntimeStatus); empty values become absent
// members, never null (ADR 0053).
func presenceREST(p agents.PresenceStatus) rest.AgentRuntimeStatus {
	out := rest.AgentRuntimeStatus{
		State: rest.AgentRuntimeState(p.State), RuntimeVersion: optional(p.RuntimeVersion), LastHeartbeatAt: optionalTime(p.LastHeartbeatAt),
	}
	matches := p.BillingModeMatches
	out.BillingModeMatches = &matches
	if p.ReportedBillingMode != "" {
		m := rest.BillingMode(p.ReportedBillingMode)
		out.ReportedBillingMode = &m
	}
	if p.ReportedToolClass != "" {
		c := rest.ToolClass(p.ReportedToolClass)
		out.ReportedToolClass = &c
	}
	if len(p.BillingModesSupported) > 0 {
		modes := make([]rest.BillingMode, 0, len(p.BillingModesSupported))
		for _, m := range p.BillingModesSupported {
			modes = append(modes, rest.BillingMode(m))
		}
		out.BillingModesSupported = &modes
	}
	if e := p.BillingEvidence; e != nil {
		out.BillingEvidence = &rest.BillingEvidence{KeyType: rest.BillingEvidenceKeyType(e.KeyType), Provider: optional(e.Provider), Method: optional(e.Method)}
	}
	if len(p.Warnings) > 0 {
		w := append([]string(nil), p.Warnings...)
		out.Warnings = &w
	}
	return out
}
