// SPDX-License-Identifier: Apache-2.0

package app

import (
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Connection is the gateway's state of one WebSocket connection: awaiting identify until
// identify or resume succeed, then ready with its session, the authenticated actor and the
// deadlines of heartbeat and token expiry; closed after the first close. adapters/ws creates it
// through Gateway.Open, feeds it ops through Gateway.Handle and the clock through Gateway.Tick,
// and reports its end through Gateway.Closed. It is safe for concurrent use; the gateway never
// holds its lock while it calls Conn.
type Connection struct {
	id     ids.ID
	conn   Conn
	opened time.Time

	mu         gosync.Mutex
	state      domain.State
	session    *Session
	actor      actor.Actor
	lastBeat   time.Time
	expiresAt  time.Time
	reauthSent bool
	wasReady   bool
	code       domain.CloseCode
	// handling counts the frames Gateway.Handle is handling; Tick closes for no deadline but
	// the expiry while it is above 0 (docs/DECISIONS.md M1-64/15).
	handling int
	// typing is the fan-out throttle of typing.start of this connection, created on first use.
	typing *domain.TypingThrottle
}

// ID returns the id of the connection, for logs; it is not the gateway session id.
func (c *Connection) ID() ids.ID { return c.id }

// Close closes the connection with code and reason (the code name when empty); only the first
// close counts. The adapter calls it for the closes it decides itself (4002 for a frame it cannot
// read, 4014 for a full queue, 4000 for a panic).
func (c *Connection) Close(code domain.CloseCode, reason string) {
	c.mu.Lock()
	if c.state == domain.StateClosed {
		c.mu.Unlock()
		return
	}
	c.state, c.code = domain.StateClosed, code
	c.mu.Unlock()
	if reason == "" {
		reason = code.Name()
	}
	c.conn.Close(code, reason)
}

// Closing reports whether the connection has been closed.
func (c *Connection) Closing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == domain.StateClosed
}

// Result returns the close code (0 when the client went away first), whether the connection
// reached ready or resumed, and the principal kind of its actor ("" before identify), for the
// span gateway.connect.
func (c *Connection) Result() (code domain.CloseCode, ready bool, kind actor.Kind) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.code, c.wasReady, c.actor.Kind
}

// identified reports whether identify or resume succeeded on the connection.
func (c *Connection) identified() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session != nil
}

// become sets the connection ready with s and a from now: the heartbeat deadline starts and the
// expiry of the credential is taken over. It reports false when the connection closed meanwhile.
func (c *Connection) become(s *Session, a actor.Actor, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == domain.StateClosed {
		return false
	}
	c.state, c.session, c.actor, c.lastBeat, c.wasReady = domain.StateReady, s, a, now, true
	c.expiresAt, c.reauthSent = a.CredentialExpiresAt, false
	return true
}

// current returns the session and actor of the connection.
func (c *Connection) current() (*Session, actor.Actor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session, c.actor
}

// beat records a heartbeat at now.
func (c *Connection) beat(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastBeat = now
}

// renewed takes over the actor of a successful reauth with its new expiry.
func (c *Connection) renewed(a actor.Actor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actor, c.expiresAt, c.reauthSent = a, a.CredentialExpiresAt, false
}

// typingThrottle returns the typing throttle of the connection, created with interval
// (sync.typing_min_interval) on first use.
func (c *Connection) typingThrottle(interval time.Duration) *domain.TypingThrottle {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.typing == nil {
		c.typing = domain.NewTypingThrottle(interval)
	}
	return c.typing
}
