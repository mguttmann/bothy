// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// resume is the handler of resume (note 17 of the brief, ruling in docs/DECISIONS.md M1-64): the
// credentials are checked, rate limited and authorized as for identify; an unknown session, one
// detached for sync.resume_window or longer and one of another principal or device all close with
// the same 4006. The old connection of the session is closed with 4017 first; resumed carries the
// head and epoch of catch-up, and delivery continues as after ready (note 12).
func (g *Gateway) resume(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(Resume)
	if !ok || (in.AccessToken == "") == (in.APIToken == "") || in.Cursor < 0 {
		return closeWith(domain.CloseDecodeError)
	}
	a, err := g.connect(ctx, in.AccessToken, in.APIToken, in.DPoP, func(actor.Actor) bool { return true })
	if err != nil {
		return err
	}
	ctx = actor.WithActor(ctx, a)
	s, ok := g.sessions.resumable(in.SessionID, a.ID, a.DeviceID, g.d.Clock.Now())
	if !ok {
		return closeWith(domain.CloseSessionInvalid)
	}
	g.sessions.update(func() { s.AuthSessionID, s.TokenID = a.SessionID, a.TokenID })
	replace(g.sessions.bind(s, c))
	if !c.become(s, a, g.d.Clock.Now()) {
		// The connection closed while resume ran: the session is detached again and expires
		// with sync.resume_window.
		g.sessions.release(s, c, true)
		return nil
	}
	g.d.Delivery.Register(s)
	res, err := g.d.CatchUp.Check(ctx, sync.CheckInput{After: in.Cursor, Epoch: in.SyncEpoch})
	if err != nil {
		return setupClose(err)
	}
	if err := c.conn.Send(Resumed{SessionID: s.ID, HeadSeq: res.HeadSeq, SyncEpoch: res.SyncEpoch}); err != nil {
		return nil
	}
	c.beat(g.d.Clock.Now())
	from := in.Cursor
	if res.Reason != "" {
		_ = c.conn.Send(ResyncRequired{Reason: res.Reason, HeadSeq: res.HeadSeq, SyncEpoch: res.SyncEpoch})
		from = res.HeadSeq
	}
	if err := g.d.Delivery.Attach(ctx, s, from); err != nil {
		return setupClose(err)
	}
	return nil
}
