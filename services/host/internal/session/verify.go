// SPDX-License-Identifier: Apache-2.0

package session

import (
	"slices"
	"time"
)

// Close details of a refused handshake (24 10.3).
const (
	DetailTokenInvalid        = "token_invalid"
	DetailTokenExpired        = "token_expired"
	DetailTokenReplayed       = "token_replayed"
	DetailHandshakeFailed     = "handshake_failed"
	DetailProfileUnknown      = "profile_unknown"
	DetailSessionUserUnknown  = "session_user_unknown"
	DetailSessionUserMismatch = "session_user_mismatch"
	DetailServerHostOwnerOnly = "server_host_owner_only"
)

// Close codes of the data connection (24 13, asyncapi hostData and terminalStream).
const (
	CodeNormal        = 1000
	CodeClosed        = 4100
	CodeTokenStale    = 4101
	CodeHandshakeFail = 4104
)

// CloseCodeOf returns the close code of a refusal with detail (24 5.3 table, R-TM-16): 4101 for
// token_expired and token_replayed, 4104 for every other detail.
func CloseCodeOf(detail string) int {
	if detail == DetailTokenExpired || detail == DetailTokenReplayed {
		return CodeTokenStale
	}
	return CodeHandshakeFail
}

// Live is a session with a running PTY as the check of att sees it.
type Live struct {
	Sub, Osu, Prf string
}

// OfferedProfile is a profile the host offers now (profiles.Offered) with its kind.
type OfferedProfile struct {
	Name, Kind string
}

// Check is the input of Verify for one message 1.
type Check struct {
	// Token is the payload of message 1; Keys the jwks of the hello of this data connection.
	Token string
	Keys  []map[string]any
	// HostID and SessionID are the own host ID and the session ID of the prologue.
	HostID, SessionID string
	// Now and Leeway: the clock and auth.clock_leeway of the hello of this data connection.
	Now    time.Time
	Leeway time.Duration
	// Cek is EphemeralKeyHash of the e of message 1.
	Cek string
	// Live returns the session with a running PTY of a session ID.
	Live func(sid string) (Live, bool)
	// Offered are the profiles offered now; SessionUsers is host_service.session_users;
	// Workdirs returns the length of the reported list of a session user after HD7.
	Offered      []OfferedProfile
	SessionUsers []string
	Workdirs     func(osu string) int
	// ServerHost is HD5 or-ed with PeerIsLocal of the control connection.
	ServerHost bool
}

// Verifier runs the check order of Design note 5 of M2-45 with the jti memory and the bindings.
type Verifier struct {
	JTI      *JTIMemory
	Bindings *Bindings
}

// Verify checks c in the fixed order of 24 5.3 with the steps 8 and 12 of D-M2-45 and stops at the
// first failure; it returns the claims and "" when every step passed, else the close_detail of the
// failing step. Steps (1) signature, (2) typ, (3) aud, (4) sid, (5) exp plus leeway, (6) jti never
// seen (remembered from here on, so a replay of a token that failed later is still token_replayed),
// (7) cek, (8) att, (9) prf offered, (10) osu configured, (11) osu bound to sub (bound now when
// unbound), (12) wdi exactly for claude with att open and inside the list of osu, (13) own on the
// server host. Step 0, message 1 itself, runs before Verify (handshake_failed).
func (v *Verifier) Verify(c Check) (Claims, string) {
	cl, detail := v.verifyToken(c)
	if detail != "" {
		return Claims{}, detail
	}
	if detail := v.verifyAccess(c, cl); detail != "" {
		return Claims{}, detail
	}
	return cl, ""
}

// verifyToken runs steps 1 to 7: the token itself and its binding to this handshake.
func (v *Verifier) verifyToken(c Check) (Claims, string) {
	h, cl, err := ParseToken(c.Token, c.Keys)
	switch {
	case err != nil, h.Typ != TokenType, cl.Aud != c.HostID, cl.Sid != c.SessionID:
		return Claims{}, DetailTokenInvalid
	case c.Now.After(time.Unix(cl.Exp, 0).Add(c.Leeway)):
		return Claims{}, DetailTokenExpired
	case !v.JTI.Remember(cl.Jti, time.Unix(cl.Exp, 0).Add(c.Leeway), c.Now):
		return Claims{}, DetailTokenReplayed
	case cl.Cek != c.Cek:
		return Claims{}, DetailTokenInvalid
	}
	return cl, ""
}

// verifyAccess runs steps 8 to 13: the session, the profile, the session user and the server host.
func (v *Verifier) verifyAccess(c Check, cl Claims) string {
	live, isLive := c.Live(cl.Sid)
	switch {
	case cl.Att == AttOpen && isLive:
		return DetailTokenInvalid
	case cl.Att == AttAttach && (!isLive || live.Sub != cl.Sub || live.Osu != cl.Osu || live.Prf != cl.Prf):
		return DetailTokenInvalid
	}
	i := slices.IndexFunc(c.Offered, func(p OfferedProfile) bool { return p.Name == cl.Prf })
	switch {
	case i < 0:
		return DetailProfileUnknown
	case !slices.Contains(c.SessionUsers, cl.Osu):
		return DetailSessionUserUnknown
	case !v.Bindings.Bind(cl.Osu, cl.Sub):
		return DetailSessionUserMismatch
	}
	wantWdi := c.Offered[i].Kind == kindClaude && cl.Att == AttOpen
	if wantWdi != (cl.Wdi != nil) || (cl.Wdi != nil && *cl.Wdi >= c.Workdirs(cl.Osu)) {
		return DetailProfileUnknown
	}
	if c.ServerHost && !cl.Own {
		return DetailServerHostOwnerOnly
	}
	return ""
}

// kindClaude is the kind of the profile claude (config.KindClaude).
const kindClaude = "claude"
