// SPDX-License-Identifier: Apache-2.0

// Package consolesession holds the pure rules of the web console session (docs/spec/08-identity.md
// section 7.5, brief M6-01): the cookie value and its HMAC, the cookie attributes, the absolute and
// idle end with the write bound of last_seen_at, the gate before the first step-up, the end of a
// stored "View as person" and the web device of a console login. It reads no clock and no
// database; the pepper comes through the port Pepper.
//
// Example:
//
//	v, _ := consolesession.NewCookieValue(rand.Reader)
//	hash := consolesession.CookieHash(pepper, v) // stored in auth_sessions.cookie_hash
//	header := consolesession.SetCookie(v, expiresAt.Sub(now))
package consolesession

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CookieName is the name of the console cookie, equal to kernel/actor.ConsoleCookieName (the
// domain imports no kernel port; a test of the application layer holds them equal). The prefix
// __Host- makes the browser refuse it without Secure, with a Domain or with another Path (RFC
// 6265bis section 4.1.3.2); the name carries no product name (02 section 6).
const CookieName = "__Host-console_session"

// valueBytes is the size of the random cookie value: 256 bits (ID-T27).
const valueBytes = 32

// attributes are the fixed cookie attributes of D-M6-01-2; Max-Age comes before them.
const attributes = "; Secure; HttpOnly; SameSite=Strict"

// WebDeviceName is the stored name of the device of a console login; the console shows its own
// translated text (D-M6-01-1).
const WebDeviceName = "Web console"

// Pepper is the port of the token pepper (auth.token_pepper_ref): the HMAC key of every stored
// token hash (03 principle 9).
type Pepper interface {
	// TokenPepper returns the raw pepper; the caller must not change it.
	TokenPepper() []byte
}

// StaticPepper is a Pepper over a fixed key.
type StaticPepper []byte

// TokenPepper implements Pepper.
func (p StaticPepper) TokenPepper() []byte { return p }

// NewCookieValue returns a new cookie value: 32 bytes from r (crypto/rand in production),
// base64url without padding. Errors: a short or failing random source.
func NewCookieValue(r io.Reader) (string, error) {
	b := make([]byte, valueBytes)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CookieHash returns HMAC-SHA-256 of value with the pepper of p, the only form the cookie is
// stored in (auth_sessions.cookie_hash); the same derivation as the refresh tokens.
func CookieHash(p Pepper, value string) []byte {
	m := hmac.New(sha256.New, p.TokenPepper())
	m.Write([]byte(value))
	return m.Sum(nil)
}

// SetCookie returns the Set-Cookie value of the cookie value with Max-Age = ttl in whole seconds
// (0 for a past end): Path=/, Secure, HttpOnly, SameSite=Strict and no Domain (08 section 7.5).
func SetCookie(value string, ttl time.Duration) string {
	secs := max(int64(ttl/time.Second), 0)
	return CookieName + "=" + value + "; Path=/; Max-Age=" + strconv.FormatInt(secs, 10) + attributes
}

// ClearCookie returns the Set-Cookie value that removes the cookie: an empty value with Max-Age=0
// and the same attributes, so the browser replaces the stored cookie.
func ClearCookie() string { return SetCookie("", 0) }

// Timing are the two durations of a console session: auth.console_session_max_age and
// auth.console_session_idle_timeout.
type Timing struct {
	MaxAge, IdleTimeout time.Duration
}

// writeBoundDivisor makes the write bound one hundredth of the idle timeout (D-M6-01-3).
const writeBoundDivisor = 100

// Validate reports durations that cannot work: a MaxAge that is not positive or an idle timeout
// whose hundredth is zero.
func (t Timing) Validate() error {
	if t.MaxAge <= 0 || t.IdleTimeout/writeBoundDivisor <= 0 {
		return errors.New("consolesession: auth.console_session_max_age and auth.console_session_idle_timeout must be positive")
	}
	return nil
}

// ExpiresAt returns the absolute end of a session created at created.
func (t Timing) ExpiresAt(created time.Time) time.Time { return created.Add(t.MaxAge) }

// Expired reports a session whose absolute end expiresAt is reached at now.
func (t Timing) Expired(expiresAt, now time.Time) bool { return !now.Before(expiresAt) }

// WriteBound returns the age last_seen_at may reach before a request writes it again: one
// hundredth of auth.console_session_idle_timeout, so writes stay bounded (D-M6-01-3).
func (t Timing) WriteBound() time.Duration { return t.IdleTimeout / writeBoundDivisor }

// Idle reports a session without a request for the idle timeout at now. The check allows the lag
// of the write bound, so activity just before the deadline is never lost: idle from lastSeen plus
// the idle timeout plus WriteBound on.
func (t Timing) Idle(lastSeen, now time.Time) bool {
	return !now.Before(lastSeen.Add(t.IdleTimeout + t.WriteBound()))
}

// NeedsTouch reports whether an accepted request at now writes last_seen_at: when it is unset or
// older than WriteBound.
func (t Timing) NeedsTouch(lastSeen, now time.Time) bool {
	return lastSeen.IsZero() || now.Sub(lastSeen) > t.WriteBound()
}

// gateActions are the actions a console session may call before its first step-up (08 section
// 7.5: only the own profile): reading oneself, the step-up, the logout, the discovery and the
// public login operations.
var gateActions = map[string]bool{
	"self.read": true, "auth.step_up": true, "auth.logout": true, "instance.discovery.read": true,
	"auth.login.begin": true, "auth.login.complete": true,
}

// PassesGate reports whether a console session with the step-up time stepUpAt (zero for none) may
// call the action act; every other action is 401 step_up_required with reason
// console_step_up_required (D-M6-01-8).
func PassesGate(act string, stepUpAt time.Time) bool {
	return !stepUpAt.IsZero() || gateActions[act]
}

// ViewActive reports whether a view started at start is still active at now:
// now < start + authz.view_as_max_duration. A view without a start has ended (fail closed).
func ViewActive(start, now time.Time, maxDuration time.Duration) bool {
	return !start.IsZero() && now.Before(ViewEndsAt(start, maxDuration))
}

// ViewEndsAt returns the end of a view started at start.
func ViewEndsAt(start time.Time, maxDuration time.Duration) time.Time { return start.Add(maxDuration) }

// WebDevice returns the device of a console login: platform web, active, approved by the person,
// without device keys and with the fixed name WebDeviceName (08 section 6.1, row "Web console").
func WebDevice(id, userID ids.ID, now time.Time) device.Device {
	return device.Device{
		ID: id, UserID: userID, Name: WebDeviceName, Platform: device.PlatformWeb, State: device.StateActive,
		ApprovedBy: userID, CreatedAt: now,
	}
}
