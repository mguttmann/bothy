// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// heartbeat is the handler of heartbeat (note 14 of the brief M1-64): last_seq is at least 0; the
// credential is revalidated (a revoked session, device or token closes with 4012) and
// gateway.connect authorized again (the authorizer answers from its cache; a principal made
// inactive closes with 4012); the heartbeat deadline starts again and heartbeat_ack carries the
// server time. heartbeat consumes nothing of rl.gateway_connect (05 section 3).
//
// Then the member agent (brief M2-20 note 10, docs/DECISIONS.md D-M2-20-9): rl.agent_runtime
// keyed by the principal, then agents.Presence.Heartbeat with the raw member in the session's
// actor context. Its errors (forbidden, validation_failed, rate_limited, internal_error) answer
// error {op: "heartbeat", code} and keep the connection. Without the member nothing changes the
// presence: it needs the state report.
func (g *Gateway) heartbeat(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(Heartbeat)
	if !ok || in.LastSeq < 0 {
		return closeWith(domain.CloseDecodeError)
	}
	_, a := c.current()
	// The credential is read again (identity, one query): the actor of identify carries the state
	// of session, device and token at that time, which authorize would trust.
	if err := g.d.Authenticator.Revalidate(ctx, a); err != nil {
		if errs.KindOf(err) == errs.KindUnauthenticated {
			return closeWith(domain.CloseRevoked)
		}
		e, _ := errs.As(err)
		return &OpError{Code: string(e.PublicCode())}
	}
	if err := g.authorizeConnect(actor.WithActor(ctx, a), a); err != nil {
		return err
	}
	now := g.d.Clock.Now()
	c.beat(now)
	_ = c.conn.Send(HeartbeatAck{ServerTime: now})
	if len(in.Agent) == 0 || g.d.Presence == nil {
		return nil
	}
	return g.agentHeartbeat(actor.WithActor(ctx, a), a, in.Agent)
}

// agentHeartbeat is the agent part of heartbeat: the rate limit, then the presence. Every error
// becomes an OpError with its public code, so the connection stays; internal failures answer
// internal_error.
func (g *Gateway) agentHeartbeat(ctx context.Context, a actor.Actor, body json.RawMessage) error {
	if g.d.AgentLimiter != nil {
		allowed, retry, err := g.d.AgentLimiter.AllowAgent(a)
		if err != nil {
			return &OpError{Code: string(errs.CodeInternalError)}
		}
		if !allowed {
			return &OpError{Code: string(errs.CodeRateLimited), Detail: "retry_after=" + strconv.Itoa(retry)}
		}
	}
	if _, err := g.d.Presence.Heartbeat(ctx, body); err != nil {
		if e, ok := errs.As(err); ok {
			return &OpError{Code: string(e.PublicCode())}
		}
		return &OpError{Code: string(errs.CodeInternalError)}
	}
	return nil
}
