// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// heartbeat is the handler of heartbeat (note 14 of the brief): last_seq is at least 0; the
// credential is revalidated (a revoked session, device or token closes with 4012) and
// gateway.connect authorized again (the authorizer answers from its cache; a principal made
// inactive closes with 4012); the heartbeat deadline starts again and heartbeat_ack carries the
// server time. The member agent is ignored until M2-20.
// heartbeat consumes nothing of rl.gateway_connect (05 section 3).
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
	return nil
}
