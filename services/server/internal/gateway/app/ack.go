// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// AgentLimiter is the rate-limit class rl.agent_runtime for the ack op (platform/ratelimit, keyed
// by the agent; 06 section 9 item 3). Allow takes one token; when the bucket is empty it reports
// false and the seconds until one is there.
type AgentLimiter interface {
	AllowAgent(a actor.Actor) (allowed bool, retryAfterSeconds int, err error)
}

// opErrorCodes are the problem codes an ack answers with error {op: "ack", code} while the
// connection stays (04 section 6.2; brief M2-07 note 12).
var opErrorCodes = map[errs.Kind]bool{errs.KindBadRequest: true, errs.KindForbidden: true, errs.KindNotFound: true}

// ack is the handler of ack (brief M2-07 note 8): rl.agent_runtime for the agent, then
// agents.Consumers.AcceptAck (authorize agent.events.consume, then the bounds of CheckAck against
// the head of this process), then the acknowledgement goes to the window of the connection and is
// saved by the next tick. There is no reply frame. validation_failed, forbidden (people and apps,
// subject_kind_not_allowed) and rate_limited answer error and keep the connection; a credential
// revoked since identify closes with 4012.
func (g *Gateway) ack(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(Ack)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	s, a := c.current()
	ctx = actor.WithActor(ctx, a)
	if g.d.AgentLimiter != nil {
		allowed, retry, err := g.d.AgentLimiter.AllowAgent(a)
		if err != nil {
			return err
		}
		if !allowed {
			return &OpError{Code: string(errs.CodeRateLimited), Detail: "retry_after=" + strconv.Itoa(retry)}
		}
	}
	if err := g.d.Consumers.cursors().AcceptAck(ctx, s.Consumer, in.Seq, g.d.Consumers.AckHead(c)); err != nil {
		e, ok := errs.As(err)
		switch {
		case ok && opErrorCodes[e.Kind()]:
			return &OpError{Code: string(e.PublicCode())}
		case ok && e.Kind() == errs.KindUnauthenticated:
			// The credential was valid at identify; it was revoked since (04 section 6.4).
			return closeWith(domain.CloseRevoked)
		}
		return err
	}
	g.d.Consumers.Ack(c, in.Seq)
	return nil
}
