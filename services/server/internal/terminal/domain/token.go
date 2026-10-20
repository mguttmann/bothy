// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The protected header of a session token (docs/spec/24-terminal.md section 5.3, ADR 0067): the
// algorithm of the server keys and the media type the host service checks.
const (
	// TokenAlg is the JWS algorithm of every session token.
	TokenAlg = "EdDSA"
	// TokenTyp is the header typ of a session token.
	TokenTyp = "terminal-session+jwt"
)

// The values of the claim att.
const (
	AttOpen   = "open"
	AttAttach = "attach"
)

// ProfileKindClaude is the profile kind whose sessions start in a working directory (24 section
// 11.1); only its tokens of att open carry wdi.
const ProfileKindClaude = "claude"

// TokenGrant is what one grant puts into a session token: the instance, the host, the person, the
// session, the device of the grant, the profile with its kind, the hash of the ephemeral key the
// device sent, whether the person is an active owner at issue time, the session user of the
// person's own assignment, the working directory index of an opening of a claude profile, and
// whether the grant opens or attaches.
type TokenGrant struct {
	InstanceID, HostID, UserID, SessionID, DeviceID ids.ID
	Profile, ProfileKind                            string
	CEK                                             string
	Owner                                           bool
	OSUser                                          string
	WorkdirIndex                                    *int
	Attach                                          bool
}

// SessionClaims are the claims of a session token in the order of the table of 24 section 5.3
// (ruling D-M2-44-3): IDs as canonical lower-case UUID strings, iat and exp as NumericDate
// seconds, own always present, wdi present exactly for a claude profile with att open (also with
// the value 0), never a JSON null.
type SessionClaims struct {
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sub string `json:"sub"`
	Sid string `json:"sid"`
	Jti string `json:"jti"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Dev string `json:"dev"`
	Prf string `json:"prf"`
	Cek string `json:"cek"`
	Own bool   `json:"own"`
	Osu string `json:"osu"`
	Wdi *int   `json:"wdi,omitempty"`
	Att string `json:"att"`
}

// errTokenGrant is a grant that cannot become a token: an ID missing, an empty profile, cek or
// session user, a claude opening without an index or a ttl below one second.
var errTokenGrant = errors.New("terminal: incomplete session token grant")

// NewSessionClaims returns the claims of g with the token id jti, issued at now (truncated to
// seconds) and valid for ttl (terminal.session_token_ttl). Errors: an incomplete grant.
//
// Example:
//
//	c, err := domain.NewSessionClaims(g, ids.New(), now, cfg.SessionTokenTTL)
//	payload, err := c.Payload()
func NewSessionClaims(g TokenGrant, jti ids.ID, now time.Time, ttl time.Duration) (SessionClaims, error) {
	for _, id := range []ids.ID{g.InstanceID, g.HostID, g.UserID, g.SessionID, g.DeviceID, jti} {
		if id.IsZero() {
			return SessionClaims{}, errTokenGrant
		}
	}
	if g.Profile == "" || g.CEK == "" || g.OSUser == "" || ttl < time.Second {
		return SessionClaims{}, errTokenGrant
	}
	iat := now.Unix()
	c := SessionClaims{
		Iss: g.InstanceID.String(), Aud: g.HostID.String(), Sub: g.UserID.String(), Sid: g.SessionID.String(), Jti: jti.String(),
		Iat: iat, Exp: iat + int64(ttl/time.Second), Dev: g.DeviceID.String(), Prf: g.Profile, Cek: g.CEK, Own: g.Owner,
		Osu: g.OSUser, Att: AttOpen,
	}
	if g.Attach {
		c.Att = AttAttach
	}
	if g.ProfileKind == ProfileKindClaude && !g.Attach {
		if g.WorkdirIndex == nil {
			return SessionClaims{}, errTokenGrant
		}
		w := *g.WorkdirIndex
		c.Wdi = &w
	}
	return c, nil
}

// Payload returns the JSON payload of c. Errors: none in practice (strings, numbers, booleans).
func (c SessionClaims) Payload() ([]byte, error) { return json.Marshal(c) }
