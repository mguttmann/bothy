// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The ephemeral event of typing (05 "Ephemeral events").
const (
	eventTypingStarted  = "typing.started"
	typingSchemaVersion = 1
	resourceKindThread  = "thread"
	timestampMillisUTC  = "2006-01-02T15:04:05.000Z07:00"
)

// typingStartedBody is the payload of typing.started (schema TypingStartedPayload).
type typingStartedBody struct {
	ThreadID    ids.ID `json:"thread_id"`
	PrincipalID ids.ID `json:"principal_id"`
	ExpiresAt   string `json:"expires_at"`
}

// typingStart is the handler of typing.start (brief M1-66, design note "Ops typing.start and
// typing.stop"): rl.typing keyed by the connection, then typing.start on the thread (people with
// write only, never in a read-only channel; a denial answers the decision's code). Inside
// sync.typing_min_interval since the last fan-out of this connection and thread it does nothing;
// otherwise it publishes typing.started with expires_at = occurred_at + sync.typing_ttl.
func (g *Gateway) typingStart(ctx context.Context, c *Connection, d any) error {
	a, info, err := g.typingCheck(ctx, c, d)
	if err != nil {
		return err
	}
	now := g.d.Clock.Now().UTC().Truncate(time.Millisecond)
	if !c.typingThrottle(g.cfg.TypingMinInterval).Allow(info.ThreadID, now) {
		return nil
	}
	return g.d.Publisher.PublishEphemeral(ctx, eventlog.EphemeralEvent{
		Type: eventTypingStarted, SchemaVersion: typingSchemaVersion,
		Actor:      &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: a.ID},
		Scope:      eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: info.ThreadID},
		Resource:   eventlog.ResourceRef{Kind: resourceKindThread, ID: info.ThreadID},
		OccurredAt: now,
		Payload:    typingStartedBody{ThreadID: info.ThreadID, PrincipalID: a.ID, ExpiresAt: now.Add(g.cfg.TypingTTL).Format(timestampMillisUTC)},
	})
}

// typingStop is the handler of typing.stop: the same rate class and check as typing.start; it
// sends no event (05 and asyncapi.yaml define no "typing stopped"), it only clears the throttle of
// this connection and thread so the next typing.start fans out at once.
func (g *Gateway) typingStop(ctx context.Context, c *Connection, d any) error {
	_, info, err := g.typingCheck(ctx, c, d)
	if err != nil {
		return err
	}
	c.typingThrottle(g.cfg.TypingMinInterval).Reset(info.ThreadID)
	return nil
}

// typingCheck runs the steps both typing ops share: rl.typing keyed by the connection
// (rate_limited), then authorize typing.start on the thread in the request context, then the
// space and channel of the thread from chat. A denial is an OpError with the code of the decision
// (not_found, forbidden, channel_read_only); an agent or app is forbidden by the requirement
// "human and write" of the catalogue.
func (g *Gateway) typingCheck(ctx context.Context, c *Connection, d any) (actor.Actor, domain.FocusEntry, error) {
	in, ok := d.(Typing)
	if !ok {
		return actor.Actor{}, domain.FocusEntry{}, closeWith(domain.CloseDecodeError)
	}
	allowed, err := g.d.OpLimiter.AllowTyping(c.ID())
	switch {
	case err != nil:
		return actor.Actor{}, domain.FocusEntry{}, err
	case !allowed:
		return actor.Actor{}, domain.FocusEntry{}, &OpError{Code: string(errs.CodeRateLimited)}
	}
	_, a := c.current()
	dec, err := g.d.Authorizer.Authorize(ctx, a, authorize.ActionTypingStart, authorize.Resource{Kind: authorize.ResourceThread, ID: in.ThreadID})
	if err != nil {
		return actor.Actor{}, domain.FocusEntry{}, err
	}
	if !dec.Allowed() {
		return actor.Actor{}, domain.FocusEntry{}, &OpError{Code: problemCode(dec.Err())}
	}
	info, err := g.d.Threads.Info(ctx, in.ThreadID)
	switch {
	case errs.KindOf(err) == errs.KindNotFound:
		return actor.Actor{}, domain.FocusEntry{}, &OpError{Code: string(errs.CodeNotFound)}
	case err != nil:
		return actor.Actor{}, domain.FocusEntry{}, err
	}
	return a, domain.FocusEntry{ThreadID: in.ThreadID, ChannelID: info.ChannelID, SpaceID: info.SpaceID}, nil
}

// problemCode returns the problem code of the error of a denial (Decision.Err).
func problemCode(err error) string {
	if e, ok := errs.As(err); ok {
		return string(e.Code)
	}
	return string(errs.CodeInternalError)
}
