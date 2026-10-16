// SPDX-License-Identifier: Apache-2.0

package httpservertest

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Actors is a test actor.Authenticator over named bearer tokens: "Authorization: Bearer <name>"
// authenticates as the actor registered under name, a console cookie registered with SetCookie as
// its actor, a request without credentials is anonymous, anything else 401 unauthenticated. Handler tests register the principals they need
// with the builders below (M1-31). It is safe for concurrent use.
type Actors struct {
	mu sync.Mutex
	m  map[string]actor.Actor
}

// NewActors returns an empty set.
func NewActors() *Actors { return &Actors{m: map[string]actor.Actor{}} }

// Set registers a under the token name.
func (s *Actors) Set(name string, a actor.Actor) {
	s.mu.Lock()
	s.m[name] = a
	s.mu.Unlock()
}

// Get returns the actor registered under name.
func (s *Actors) Get(name string) actor.Actor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name]
}

// SetCookie registers a under the console cookie value (M6-01): a request with that cookie and
// without Authorization authenticates as a.
func (s *Actors) SetCookie(value string, a actor.Actor) {
	s.Set(cookiePrefix+value, a)
}

// cookiePrefix keeps the cookie values apart from the token names.
const cookiePrefix = "cookie\x00"

// Authenticate implements actor.Authenticator.
func (s *Actors) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Authorization == "" && !c.ConsoleCookie.IsZero() {
		s.mu.Lock()
		a, ok := s.m[cookiePrefix+c.ConsoleCookie.Reveal()]
		s.mu.Unlock()
		if ok {
			return a, nil
		}
		return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
	}
	if c.Authorization == "" {
		return actor.Anonymous(), nil
	}
	s.mu.Lock()
	a, ok := s.m[strings.TrimPrefix(c.Authorization, "Bearer ")]
	s.mu.Unlock()
	if ok {
		return a, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", "Bearer")
}

// Revalidate implements actor.Authenticator.
func (*Actors) Revalidate(context.Context, actor.Actor) error { return nil }

// InSession returns a person in the session sessionID on deviceID; stepUpAt is the last step-up
// (zero for none). Whether the person is an owner, active or pending, and the device state are
// data of the test world, not of the actor.
func InSession(id, sessionID, deviceID ids.ID, stepUpAt time.Time) actor.Actor {
	return actor.Actor{ID: id, Kind: actor.KindUser, SessionID: sessionID, DeviceID: deviceID, StepUpAt: stepUpAt}
}

// WithToken returns a principal of kind authenticated with the API token tokenID: a personal
// token of a person (kind user, no session, SR5), or the token of an agent or app.
func WithToken(id ids.ID, kind actor.Kind, tokenID ids.ID) actor.Actor {
	return actor.Actor{ID: id, Kind: kind, TokenID: tokenID}
}
