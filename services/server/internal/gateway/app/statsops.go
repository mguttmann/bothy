// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// The ops and the ephemeral type of the live stream (M6-15).
const (
	opStatsSubscribe   = "stats.subscribe"
	opStatsUnsubscribe = "stats.unsubscribe"
	eventStatsLive     = "stats.live"
)

// Log message and error type of a live frame that could not be delivered.
const (
	msgLiveFrameFailed = "gateway stats.live frame not delivered"
	errorTypeLive      = "live_frame"
)

// StatsSubscribe is the body of stats.subscribe (schema GatewayStatsSubscribe): SpaceID zero
// subscribes to the instance.
type StatsSubscribe struct {
	SpaceID ids.ID `json:"space_id,omitzero"`
}

// StatsUnsubscribe is the body of stats.unsubscribe (schema GatewayStatsUnsubscribe, empty).
type StatsUnsubscribe struct{}

// liveSub is the stats.live subscription of one connection; its identity tells a replaced or
// ended subscription from the current one.
type liveSub struct {
	q stats.LiveQuery
}

// directOfferer is the part of the hub the live stream needs: the direct offer of one ephemeral
// event to one connection and the member counts. Deliveries of tests that do not implement it
// get no live frames.
type directOfferer interface {
	OfferEphemeralTo(ctx context.Context, c *Connection, ev eventlog.EphemeralEvent, payload func() (any, error)) (DirectOutcome, error)
}

// liveState returns the subscription the session holds for resume; nil for none.
func (s *Session) liveState() *stats.LiveQuery {
	s.focusMu.Lock()
	defer s.focusMu.Unlock()
	if s.live == nil {
		return nil
	}
	q := *s.live
	return &q
}

// setLive stores q (nil clears) as the subscription resume restores.
func (s *Session) setLive(q *stats.LiveQuery) {
	s.focusMu.Lock()
	defer s.focusMu.Unlock()
	if q == nil {
		s.live = nil
		return
	}
	v := *q
	s.live = &v
}

// statsSubscribe is the handler of stats.subscribe (M6-15 design note 1, 25 5.1 item 1): rl.read
// keyed by the principal, then the check of stats.Live (stats.live_enabled, stats.read on the
// instance or the space, people only), then a slot of the node unless the connection already
// holds one (a new subscription replaces the old one). Every denial answers error with its code
// (feature_disabled, not_found, forbidden, rate_limited) and keeps the connection and an earlier
// subscription. The subscription is stored in the gateway session, so resume restores it.
func (g *Gateway) statsSubscribe(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(StatsSubscribe)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	s, a := c.current()
	if err := g.allowRead(a.ID); err != nil {
		return err
	}
	if g.d.Live == nil {
		return &OpError{Code: string(errs.CodeFeatureDisabled)}
	}
	q := stats.LiveQuery{SpaceID: in.SpaceID}
	if err := g.d.Live.Check(ctx, a, q); err != nil {
		return liveOpError(err)
	}
	g.liveMu.Lock()
	defer g.liveMu.Unlock()
	if _, held := g.liveSubs[c]; !held {
		if err := g.d.Live.Acquire(); err != nil {
			return liveOpError(err)
		}
	}
	g.liveSubs[c] = &liveSub{q: q}
	if s != nil {
		s.setLive(&q)
	}
	return nil
}

// statsUnsubscribe is the handler of stats.unsubscribe: rl.read keyed by the principal, then the
// subscription of the connection ends; without one nothing happens and no error is sent. It names
// no resource, so there is nothing to authorize (noResource).
func (g *Gateway) statsUnsubscribe(ctx context.Context, c *Connection, d any) error {
	if _, ok := d.(StatsUnsubscribe); !ok {
		return closeWith(domain.CloseDecodeError)
	}
	s, a := c.current()
	if err := g.allowRead(a.ID); err != nil {
		return err
	}
	noResource(ctx)
	g.dropLive(c)
	if s != nil {
		s.setLive(nil)
	}
	return nil
}

// liveOpError maps an error of stats.Live to the error frame of stats.subscribe; a failure that
// is no problem of the request (a database error) ends the connection like every other op.
func liveOpError(err error) error {
	e, ok := errs.As(err)
	if !ok {
		return err
	}
	if k := e.Kind(); k == errs.KindForbidden || k == errs.KindNotFound || k == errs.KindRateLimited {
		return &OpError{Code: string(e.PublicCode())}
	}
	if e.Kind() == errs.KindUnauthenticated {
		return closeWith(domain.CloseRevoked)
	}
	return err
}

// dropLive ends the subscription of c in this process, when it has one, and frees its slot. The
// session keeps its state (a closed connection may be resumed).
func (g *Gateway) dropLive(c *Connection) {
	g.liveMu.Lock()
	_, held := g.liveSubs[c]
	delete(g.liveSubs, c)
	g.liveMu.Unlock()
	if held && g.d.Live != nil {
		g.d.Live.Release()
	}
}

// restoreLive restores the subscription of a resumed session on c (04 6.3, M6-15 design note 1):
// the check of stats.subscribe again with the rights of now, then a slot of the node. A denial
// sends one error {op: stats.subscribe} with its code and clears the session's subscription; a
// failure of the check itself is logged and leaves it for the next resume.
func (g *Gateway) restoreLive(ctx context.Context, c *Connection, s *Session, a actor.Actor) {
	q := s.liveState()
	if q == nil || g.d.Live == nil {
		return
	}
	err := g.d.Live.Check(ctx, a, *q)
	if err == nil {
		g.liveMu.Lock()
		if _, held := g.liveSubs[c]; !held {
			err = g.d.Live.Acquire()
		}
		if err == nil {
			g.liveSubs[c] = &liveSub{q: *q}
		}
		g.liveMu.Unlock()
	}
	if err == nil {
		return
	}
	var oe *OpError
	if errors.As(liveOpError(err), &oe) {
		s.setLive(nil)
		_ = c.conn.Send(ErrorFrame{Op: opStatsSubscribe, Code: oe.Code})
		return
	}
	g.d.Log.LogAttrs(ctx, slog.LevelWarn, msgLiveFrameFailed, slog.String(logKeyErrorType, errorTypeLive))
}

// LiveBuilt implements stats.LiveSink (M6-15 design note 5): for every subscribed connection one
// stats.live frame, filtered for its actor and subscription, offered to that connection only
// through the hub after sync.Admit with stats.read on the instance or the subscribed space. A
// denial ends the subscription with exactly one error {op: stats.subscribe} (forbidden, or
// not_found when the space became unreadable) and no further frame; a connection that is not
// live or has a full queue just misses this frame.
func (g *Gateway) LiveBuilt(ctx context.Context, r stats.LiveRound) {
	direct, ok := g.d.Delivery.(directOfferer)
	if !ok {
		return
	}
	g.liveMu.Lock()
	subs := make(map[*Connection]*liveSub, len(g.liveSubs))
	for c, sub := range g.liveSubs {
		subs[c] = sub
	}
	g.liveMu.Unlock()
	for c, sub := range subs {
		g.deliverLive(ctx, direct, c, sub, r)
	}
}

// deliverLive offers the frame of r for sub to c.
func (g *Gateway) deliverLive(ctx context.Context, direct directOfferer, c *Connection, sub *liveSub, r stats.LiveRound) {
	_, a := c.current()
	ev := eventlog.EphemeralEvent{Type: eventStatsLive, SchemaVersion: 1, OccurredAt: r.At(), Audience: a.ID,
		Scope: eventlog.Scope{SpaceID: sub.q.SpaceID}, Resource: eventlog.ResourceRef{Kind: string(authorize.ResourceInstance)}}
	if !sub.q.Instance() {
		ev.Resource = eventlog.ResourceRef{Kind: string(authorize.ResourceSpace), ID: sub.q.SpaceID}
	}
	dctx := authorize.Delivery(ctx)
	out, err := direct.OfferEphemeralTo(ctx, c, ev, func() (any, error) {
		if !g.liveCurrent(c, sub) {
			return nil, errLiveEnded
		}
		f, err := r.For(dctx, a, sub.q)
		return f.Payload, err
	})
	switch {
	case out == DirectDenied:
		g.endLive(dctx, c, sub, a)
	case err != nil && !errors.Is(err, errLiveEnded):
		g.d.Log.LogAttrs(ctx, slog.LevelWarn, msgLiveFrameFailed, slog.String(logKeyErrorType, errorTypeLive))
	}
}

// errLiveEnded stops the offer of a frame whose subscription ended while the round ran.
var errLiveEnded = errs.New(errs.CodeConflict, "the stats.live subscription ended")

// liveCurrent reports whether sub is still the subscription of c.
func (g *Gateway) liveCurrent(c *Connection, sub *liveSub) bool {
	g.liveMu.Lock()
	defer g.liveMu.Unlock()
	return g.liveSubs[c] == sub
}

// endLive ends sub of c after a denial of its frame: once, when it is still current, with one
// error {op: stats.subscribe} whose code is the decision of the subscription check now (not_found
// for an unreadable space, forbidden otherwise).
func (g *Gateway) endLive(ctx context.Context, c *Connection, sub *liveSub, a actor.Actor) {
	g.liveMu.Lock()
	current := g.liveSubs[c] == sub
	if current {
		delete(g.liveSubs, c)
	}
	g.liveMu.Unlock()
	if !current {
		return
	}
	g.d.Live.Release()
	if s, _ := c.current(); s != nil {
		s.setLive(nil)
	}
	code := string(errs.CodeForbidden)
	if e, ok := errs.As(g.d.Live.Check(ctx, a, sub.q)); ok && e.Kind() == errs.KindNotFound {
		code = string(errs.CodeNotFound)
	}
	_ = c.conn.Send(ErrorFrame{Op: opStatsSubscribe, Code: code})
}

// LiveConnections implements stats.LiveConnectionCounter: the registered connections of this
// node by the kind of their principal (25 5.2 row connections).
func (h *Hub) LiveConnections() stats.LiveConnections {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out stats.LiveConnections
	for c := range h.members {
		switch _, a := c.current(); a.Kind {
		case actor.KindUser:
			out.User++
		case actor.KindAgent:
			out.Agent++
		case actor.KindApp:
			out.App++
		case actor.KindHost, actor.KindAnonymous:
			// Host services use their own channel (M2-43); an anonymous connection is not counted.
		}
	}
	return out
}
