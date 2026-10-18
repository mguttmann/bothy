// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The ops of M1-66.
const (
	opFocus       = "focus"
	opTypingStart = "typing.start"
	opTypingStop  = "typing.stop"
)

// detailMaxFocused is the detail of the error of a focus over sync.max_focused_threads.
const detailMaxFocused = "sync.max_focused_threads"

// focus is the handler of focus (brief M1-66, design note "Op focus"): rl.read keyed by the
// principal, then the rules of the set (repeated ids count once; more than
// sync.max_focused_threads answers validation_failed and keeps the previous set), then
// gateway.focus per thread in the normal request context, so denials are audited by authz.
// Unknown and unreadable threads take the same path (authorize answers not_found for both) and
// are dropped without an error frame. The readable ones replace the focus of the connection and
// its session atomically, with idle. It is accepted while the connection catches up.
func (g *Gateway) focus(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(Focus)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	_, a := c.current()
	if err := g.allowRead(a.ID); err != nil {
		return err
	}
	set, err := domain.NewFocusSet(in.ThreadIDs, in.Idle, g.cfg.MaxFocusedThreads)
	if errors.Is(err, domain.ErrTooManyFocused) {
		return &OpError{Code: string(errs.CodeValidationFailed), Detail: detailMaxFocused}
	}
	if len(set.Threads) == 0 {
		noResource(ctx)
	}
	f := domain.Focus{Idle: set.Idle}
	for _, id := range set.Threads {
		e, ok, err := g.readableThread(ctx, a, id)
		if err != nil {
			return err
		}
		if ok {
			f.Entries = append(f.Entries, e)
		}
	}
	g.d.Delivery.Focus(c, f)
	return nil
}

// allowRead takes a token of rl.read for the principal; an empty bucket is rate_limited.
func (g *Gateway) allowRead(principal ids.ID) error {
	ok, err := g.d.OpLimiter.AllowRead(principal)
	switch {
	case err != nil:
		return err
	case !ok:
		return &OpError{Code: string(errs.CodeRateLimited)}
	}
	return nil
}

// readableThread authorizes gateway.focus on thread for a and returns its focus entry with the
// channel and space chat reports; false for a denial (not_found for an unknown thread too). An
// error of authorize or of chat ends the op (4000); a thread that disappears between the check
// and the read is dropped like an unknown one.
func (g *Gateway) readableThread(ctx context.Context, a actor.Actor, thread ids.ID) (domain.FocusEntry, bool, error) {
	dec, err := g.d.Authorizer.Authorize(ctx, a, authorize.ActionGatewayFocus, authorize.Resource{Kind: authorize.ResourceThread, ID: thread})
	if err != nil {
		return domain.FocusEntry{}, false, err
	}
	if !dec.Allowed() {
		return domain.FocusEntry{}, false, nil
	}
	info, err := g.d.Threads.Info(ctx, thread)
	switch {
	case errs.KindOf(err) == errs.KindNotFound:
		return domain.FocusEntry{}, false, nil
	case err != nil:
		return domain.FocusEntry{}, false, err
	}
	return domain.FocusEntry{ThreadID: thread, ChannelID: info.ChannelID, SpaceID: info.SpaceID}, true, nil
}
