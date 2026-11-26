// SPDX-License-Identifier: Apache-2.0

package computer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/host/internal/session"
)

// TokenType is the typ of an attach token of a computer session (docs/spec/27-computer-use.md
// 5.2); it keeps the token apart from a terminal session token.
const TokenType = "computer-session+jwt"

// Claims are the claims of an attach token (docs/spec/27-computer-use.md 5.2).
type Claims struct {
	Iss, Aud, Sub, Sid, Jti, Run, Agt, Cek string
	Iat, Exp, Gen                          int64
	Own                                    bool
}

// errClaims is a payload without the claims of 5.2.
var errClaims = errors.New("computer: claims of the attach token unusable")

// OpenBinding is what computer.open named for the session: step 7 compares the token with it.
type OpenBinding struct {
	SessionID, RunID, AgentID, UserID string
}

// Check is the input of Verify for one message 1.
type Check struct {
	// Token is the payload of message 1; Keys the jwks of the hello of this data connection
	// (purpose terminal).
	Token string
	Keys  []map[string]any
	// HostID and SessionID are the own host ID and the session ID of the prologue.
	HostID, SessionID string
	// Now and Leeway: the clock and auth.clock_leeway of the hello of the data connection.
	Now    time.Time
	Leeway time.Duration
	// Cek is the hash of the e of message 1.
	Cek string
	// Open is the computer.open of this session; Live reports that it has a live session already.
	Open OpenBinding
	Live bool
}

// Verifier runs the token steps 1 to 8 of docs/spec/27-computer-use.md 5.3 with its own jti
// memory. It is safe for concurrent use.
type Verifier struct {
	jti *session.JTIMemory
}

// NewVerifier returns a verifier with an empty jti memory.
func NewVerifier() *Verifier { return &Verifier{jti: session.NewJTIMemory()} }

// Verify checks c in the order of 27 5.3 and stops at the first failure: (1) the compact JWS with
// the claims of 5.2, (2) typ, (3) aud and sid, (4) exp plus the leeway, (5) jti never seen (then
// remembered, so a replay of a token that failed later is still token_replayed), (6) cek, (7) sid,
// run, agt and sub equal the computer.open of the session, which has no live session yet, (8) own.
// It returns the claims and "" when every step passed, else the close_detail of the failing step.
func (v *Verifier) Verify(c Check) (Claims, string) {
	h, payload, err := session.ParseJWS(c.Token, c.Keys)
	if err != nil {
		return Claims{}, DetailTokenInvalid
	}
	cl, err := parseClaims(payload)
	switch {
	case err != nil, h.Typ != TokenType, cl.Aud != c.HostID, cl.Sid != c.SessionID:
		return Claims{}, DetailTokenInvalid
	case c.Now.After(time.Unix(cl.Exp, 0).Add(c.Leeway)):
		return Claims{}, DetailTokenExpired
	case !v.jti.Remember(cl.Jti, time.Unix(cl.Exp, 0).Add(c.Leeway), c.Now):
		return Claims{}, DetailTokenReplayed
	case cl.Cek != c.Cek:
		return Claims{}, DetailTokenInvalid
	case c.Live || cl.Sid != c.Open.SessionID || cl.Run != c.Open.RunID || cl.Agt != c.Open.AgentID || cl.Sub != c.Open.UserID:
		return Claims{}, DetailTokenInvalid
	case !cl.Own:
		return Claims{}, DetailTokenInvalid
	}
	return cl, ""
}

// parseClaims decodes the claims of 5.2: every claim present with its JSON type, no null, integers
// without fraction or exponent; unknown claims are ignored like in a terminal session token.
func parseClaims(b []byte) (Claims, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	var m map[string]json.RawMessage
	if err := dec.Decode(&m); err != nil || m == nil {
		return Claims{}, errClaims
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Claims{}, errClaims
	}
	var c Claims
	ok := str(m, "iss", &c.Iss) && str(m, "aud", &c.Aud) && str(m, "sub", &c.Sub) && str(m, "sid", &c.Sid) &&
		str(m, "jti", &c.Jti) && str(m, "run", &c.Run) && str(m, "agt", &c.Agt) && str(m, "cek", &c.Cek) &&
		integer(m, "iat", &c.Iat) && integer(m, "exp", &c.Exp) && integer(m, "gen", &c.Gen) && boolean(m, "own", &c.Own)
	if !ok {
		return Claims{}, errClaims
	}
	return c, nil
}

func str(m map[string]json.RawMessage, k string, dst *string) bool {
	raw, ok := m[k]
	return ok && len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, dst) == nil
}

func integer(m map[string]json.RawMessage, k string, dst *int64) bool {
	raw, ok := m[k]
	s := strings.TrimSpace(string(raw))
	return ok && s != "" && !strings.ContainsAny(s, ".eE") && s != "null" && json.Unmarshal([]byte(s), dst) == nil
}

func boolean(m map[string]json.RawMessage, k string, dst *bool) bool {
	raw, ok := m[k]
	s := strings.TrimSpace(string(raw))
	return ok && (s == "true" || s == "false") && json.Unmarshal([]byte(s), dst) == nil
}
