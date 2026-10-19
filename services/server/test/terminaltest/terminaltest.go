// SPDX-License-Identifier: Apache-2.0

// Package terminaltest holds the test fixtures of the terminal module (M2-39): hosts in any state
// with reported profiles, session users and check results and with keys, session user rows and
// terminal_sessions rows, inserted directly in SQL. In production these columns are written by
// pairing (M2-41), the control connection (M2-43) and the sessions (M2-44); the fixtures let the
// tests of M2-39 and of authz build that state without them. Only _test.go files import this
// package.
//
// Example:
//
//	r1 := terminaltest.Host{Name: "Build server", State: "active", AssignedUserID: alex, CreatedBy: robin,
//		ReportedSessionUsers: []string{"bothy-s-alex"}}
//	id, err := terminaltest.InsertHost(ctx, pool, r1)
package terminaltest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// Host describes a host row. Zero values take the defaults named per field.
type Host struct {
	// ID is the host; zero takes a new id.
	ID ids.ID
	// Name is the name; empty takes "Host " and the first characters of the id.
	Name string
	// State is pairing, confirming, active, disabled or retired; empty means active.
	State string
	// DisabledReason is owner or duplicate_connection with state disabled; empty means owner.
	DisabledReason string
	// AssignedUserID is the holder; zero for none.
	AssignedUserID ids.ID
	// CreatedBy is the owner that created the host; required.
	CreatedBy ids.ID
	// ReportedSessionUsers are the names the host reported, each without working directories.
	// A host in pairing never reported any; the paired states report these.
	ReportedSessionUsers []string
	// CreatedAt is the creation time; zero means now() of the database.
	CreatedAt time.Time
}

// The states the fixtures write besides the ones they only pass through.
const (
	stateActive   = "active"
	stateDisabled = "disabled"
)

// KeyFingerprint is the fingerprint the fixture writes for every host with keys.
const KeyFingerprint = "7K2Q-M9XD-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000"

// paired are the states with keys (03 hosts_keys_check).
var paired = map[string]bool{"confirming": true, stateActive: true, stateDisabled: true}

// InsertHost inserts h as the database owner role of db and returns its id. A host in pairing
// gets a random pairing code hash valid for an hour; the paired states get random keys, the
// fingerprint KeyFingerprint, platform linux, one profile shell, one passed check and the
// reported session users; active and disabled are confirmed by CreatedBy; retired carries
// retired_at.
func InsertHost(ctx context.Context, db *pg.Pool, h Host) (ids.ID, error) {
	if h.CreatedBy.IsZero() {
		return ids.ID{}, errors.New("terminaltest: a host needs CreatedBy")
	}
	if h.ID.IsZero() {
		h.ID = ids.New()
	}
	if h.State == "" {
		h.State = stateActive
	}
	if h.Name == "" {
		h.Name = "Host " + h.ID.String()[:8]
	}
	reported := make([]map[string]any, 0, len(h.ReportedSessionUsers))
	for _, n := range h.ReportedSessionUsers {
		reported = append(reported, map[string]any{"os_user": n, "workdirs": []string{}})
	}
	users, err := json.Marshal(reported)
	if err != nil {
		return ids.ID{}, err
	}
	var (
		hash, noise                []byte
		jwk, jkt, fingerprint, pf  any
		pairedAt, confirmedAt, ret any
		confirmedBy, reason        any
		profiles, doctor           = "[]", "[]"
		disabledAt                 any
		expires                    any
	)
	switch {
	case h.State == "pairing":
		hash = random(32)
		expires = time.Now().Add(time.Hour)
		users = []byte("[]")
	case paired[h.State]:
		noise = random(32)
		jwk, jkt, fingerprint, pf = `{"kty":"OKP","crv":"Ed25519","x":"AAAA"}`, "jkt-"+h.ID.String(), KeyFingerprint, "linux"
		pairedAt = time.Now()
		profiles = `[{"name":"shell","display_name":"Shell","kind":"shell"}]`
		doctor = `[{"check":"hd1_user","ok":true}]`
	}
	if h.State == stateActive || h.State == stateDisabled {
		confirmedAt, confirmedBy = time.Now(), h.CreatedBy.String()
	}
	if h.State == stateDisabled {
		reason, disabledAt = h.DisabledReason, time.Now()
		if h.DisabledReason == "" {
			reason = "owner"
		}
	}
	if h.State == "retired" {
		ret = time.Now()
	}
	var holder any
	if !h.AssignedUserID.IsZero() {
		holder = h.AssignedUserID.String()
	}
	created := any(nil)
	if !h.CreatedAt.IsZero() {
		created = h.CreatedAt
	}
	_, err = db.Exec(ctx, `INSERT INTO hosts (id, name, state, platform, assigned_user_id, pairing_code_hash, pairing_expires_at,
		noise_public_key, auth_public_jwk, auth_jkt, key_fingerprint, key_generation, paired_at, confirmed_by, confirmed_at,
		reported_profiles, reported_session_users, reported_doctor, created_by, created_at, disabled_at, disabled_reason, retired_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CASE WHEN $8::bytea IS NULL THEN 0 ELSE 1 END, $12, $13, $14,
		$15, $16, $17, $18, coalesce($19, now()), $20, $21, $22)`,
		h.ID.String(), h.Name, h.State, pf, holder, hash, expires, noise, jwk, jkt, fingerprint, pairedAt, confirmedBy, confirmedAt,
		profiles, string(users), doctor, h.CreatedBy.String(), created, disabledAt, reason, ret)
	return h.ID, err
}

// SessionUser describes a host_session_users row.
type SessionUser struct {
	// ID is the row; zero takes a new id.
	ID ids.ID
	// HostID, UserID and OSUser are required.
	HostID, UserID ids.ID
	OSUser         string
	// CreatedBy is the owner of the assignment; required.
	CreatedBy ids.ID
	// RevokedBy ends the assignment when not zero (revoked_at now()).
	RevokedBy ids.ID
}

// InsertSessionUser inserts u and returns its id.
func InsertSessionUser(ctx context.Context, db *pg.Pool, u SessionUser) (ids.ID, error) {
	if u.ID.IsZero() {
		u.ID = ids.New()
	}
	var revokedBy, revokedAt any
	if !u.RevokedBy.IsZero() {
		revokedBy, revokedAt = u.RevokedBy.String(), time.Now()
	}
	_, err := db.Exec(ctx, `INSERT INTO host_session_users (id, host_id, user_id, os_user, created_by, revoked_by, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, u.ID.String(), u.HostID.String(), u.UserID.String(), u.OSUser, u.CreatedBy.String(), revokedBy, revokedAt)
	return u.ID, err
}

// Session describes a terminal_sessions row of a person on a host with its session user. A
// device and an app session of the person are created for it.
type Session struct {
	// ID is the session; zero takes a new id.
	ID ids.ID
	// HostID, UserID and SessionUserID (the host_session_users row) are required.
	HostID, UserID, SessionUserID ids.ID
	// State is opening, active, detached or closed; empty means active. A closed session gets
	// close_reason client_closed.
	State string
	// BytesIn and BytesOut are the counters of the relay.
	BytesIn, BytesOut int64
	// OpenedAt is the start; zero means now() of the database.
	OpenedAt time.Time
}

// InsertSession inserts s with a new device and app session of its person and returns its id.
func InsertSession(ctx context.Context, db *pg.Pool, s Session) (ids.ID, error) {
	if s.ID.IsZero() {
		s.ID = ids.New()
	}
	if s.State == "" {
		s.State = stateActive
	}
	device, auth := ids.New().String(), ids.New().String()
	if _, err := db.Exec(ctx, `INSERT INTO devices (id, user_id, name, platform) VALUES ($1, $2, 'Phone', 'ios')`, device, s.UserID.String()); err != nil {
		return ids.ID{}, err
	}
	if _, err := db.Exec(ctx, `INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'oidc', $4)`,
		auth, s.UserID.String(), device, time.Now().Add(24*time.Hour)); err != nil {
		return ids.ID{}, err
	}
	var detached, closed, reason any
	switch s.State {
	case "detached":
		detached = time.Now()
	case "closed":
		closed, reason = time.Now(), "client_closed"
	}
	opened := any(nil)
	if !s.OpenedAt.IsZero() {
		opened = s.OpenedAt
	}
	_, err := db.Exec(ctx, `INSERT INTO terminal_sessions (id, host_id, user_id, host_session_user_id, opened_by_device_id, device_id,
		auth_session_id, profile, state, token_jti, key_generation, opened_at, detached_at, closed_at, close_reason, bytes_in, bytes_out)
		VALUES ($1, $2, $3, $4, $5, $5, $6, 'shell', $7, $8, 1, coalesce($9, now()), $10, $11, $12, $13, $14)`,
		s.ID.String(), s.HostID.String(), s.UserID.String(), s.SessionUserID.String(), device, auth, s.State, ids.New().String(),
		opened, detached, closed, reason, s.BytesIn, s.BytesOut)
	return s.ID, err
}

// random returns n random bytes.
func random(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}
