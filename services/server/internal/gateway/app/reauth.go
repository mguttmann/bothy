// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// reauth is the handler of reauth (note 16 of the brief, ruling in docs/DECISIONS.md M1-64):
// the new access token is authenticated like identify. A rejected token or proof answers error
// with its problem code and keeps the connection; a token of another principal, device or auth
// session, or a credential that is no person's access token, closes with 4004; then
// gateway.connect is authorized (a revoked session closes with 4012). Success takes over the new
// expiry, so the next reauth_required and the 4011 follow it. reauth consumes nothing of
// rl.gateway_connect (05 section 3).
func (g *Gateway) reauth(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(Reauth)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	s, _ := c.current()
	a, err := g.authenticate(ctx, in.AccessToken, in.DPoP)
	if err != nil {
		if e, ok := errs.As(err); ok && e.Kind() == errs.KindUnauthenticated {
			return &OpError{Code: string(e.PublicCode())}
		}
		return closeWith(domain.CloseUnknownError)
	}
	var same bool
	g.sessions.update(func() { same = a.ID == s.PrincipalID && a.DeviceID == s.DeviceID && a.SessionID == s.AuthSessionID })
	if a.Kind != actor.KindUser || !a.TokenID.IsZero() || !same {
		return closeWith(domain.CloseAuthenticationFailed)
	}
	if err := g.authorizeConnect(actor.WithActor(ctx, a), a); err != nil {
		return err
	}
	c.renewed(a)
	return nil
}
