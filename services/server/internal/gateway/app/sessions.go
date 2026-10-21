// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Session is a gateway session (00 "Gateway session"): the server-side state of an identified
// connection that a resume within sync.resume_window picks up again. The exported fields are
// set when the session is created and change only on resume and reauth under the store's lock;
// Delivery reads them.
type Session struct {
	// ID is gateway_session_id, a UUIDv7.
	ID ids.ID
	// PrincipalID and PrincipalKind are the principal of the session.
	PrincipalID   ids.ID
	PrincipalKind actor.Kind
	// DeviceID is the device of a person; zero for agents and apps.
	DeviceID ids.ID
	// AuthSessionID is the auth session of a person's access token; zero otherwise.
	AuthSessionID ids.ID
	// TokenID is the API token of an agent or app; zero for people.
	TokenID ids.ID
	// Consumer is the consumer name of an agent or app (default "default"); "" for people.
	Consumer string
	// Client is the client platform and version of identify, when sent.
	Client ClientInfo

	// state, detachedAt and conn belong to Sessions and are guarded by its mutex.
	state      domain.State
	detachedAt time.Time
	conn       *Connection
}

// Sessions is the store of the gateway sessions of this process (R-AR-11): a map by id, an index
// device id to the open connection of the device (one connection per device, 04 section 6.1)
// and a sweeper that removes sessions detached longer than sync.resume_window. A resume that
// reaches another process finds nothing and gets 4006. It is safe for concurrent use.
type Sessions struct {
	mu       gosync.Mutex
	byID     map[ids.ID]*Session
	byDevice map[ids.ID]*Connection
	window   time.Duration
	clk      clock.Clock
}

// newSessions returns an empty store with the resume window of sync.resume_window.
func newSessions(window time.Duration, clk clock.Clock) *Sessions {
	return &Sessions{byID: map[ids.ID]*Session{}, byDevice: map[ids.ID]*Connection{}, window: window, clk: clk}
}

// bind makes s ready on c and returns the connections c replaces, which the caller closes with
// 4017 after the lock is released: the old connection of s (resume) and the open connection of
// the same device (identify and resume of people). A new session is added to the store.
func (st *Sessions) bind(s *Session, c *Connection) []*Connection {
	st.mu.Lock()
	defer st.mu.Unlock()
	var replaced []*Connection
	if s.conn != nil && s.conn != c {
		replaced = append(replaced, s.conn)
	}
	if !s.DeviceID.IsZero() {
		if old := st.byDevice[s.DeviceID]; old != nil && old != c && old != s.conn {
			replaced = append(replaced, old)
		}
		st.byDevice[s.DeviceID] = c
	}
	s.state, s.conn, s.detachedAt = domain.StateReady, c, time.Time{}
	st.byID[s.ID] = s
	return replaced
}

// release ends the binding of s to c when c is still its connection: a resumable end leaves s
// detached from now on, any other end closes and removes it.
func (st *Sessions) release(s *Session, c *Connection, resumable bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !s.DeviceID.IsZero() && st.byDevice[s.DeviceID] == c {
		delete(st.byDevice, s.DeviceID)
	}
	if s.conn != c {
		return
	}
	s.conn = nil
	if resumable && s.state.CanBecome(domain.StateDetached) {
		s.state, s.detachedAt = domain.StateDetached, st.clk.Now()
		return
	}
	s.state = domain.StateClosed
	delete(st.byID, s.ID)
}

// resumable returns the session id may resume for principal and device at now: known, of the
// same principal and device, and not detached for sync.resume_window or longer. An attached
// session qualifies too; its old connection is replaced (note 17). Every other case is false,
// and the caller answers all of them with the same 4006.
func (st *Sessions) resumable(id, principal, device ids.ID, now time.Time) (*Session, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	s, ok := st.byID[id]
	if !ok || s.PrincipalID != principal || s.DeviceID != device || s.state == domain.StateClosed {
		return nil, false
	}
	if s.state == domain.StateDetached && !now.Before(s.detachedAt.Add(st.window)) {
		return nil, false
	}
	return s, true
}

// update applies f to s under the store's lock (resume and reauth change the token fields).
func (st *Sessions) update(f func()) {
	st.mu.Lock()
	defer st.mu.Unlock()
	f()
}

// Sweep removes every session detached for sync.resume_window or longer at now and returns how
// many it removed.
func (st *Sessions) Sweep(now time.Time) int {
	st.mu.Lock()
	defer st.mu.Unlock()
	n := 0
	for id, s := range st.byID {
		if s.state == domain.StateDetached && !now.Before(s.detachedAt.Add(st.window)) {
			s.state = domain.StateClosed
			delete(st.byID, id)
			n++
		}
	}
	return n
}

// Len returns the number of sessions in the store, ready and detached.
func (st *Sessions) Len() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return len(st.byID)
}

// Run is the sweeper: it calls Sweep every sync.resume_window until ctx ends. A detached session
// is thus removed at the latest two windows after its connection ended; a resume checks the
// window itself, so the sweep only frees memory. The lifecycle of internal/wire starts and stops
// it.
func (st *Sessions) Run(ctx context.Context) {
	t := time.NewTicker(st.window)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st.Sweep(st.clk.Now())
		}
	}
}
