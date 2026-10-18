// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// structureEvents are the event types that change the path or the read-only state of a resource
// without a new rights version for every affected principal (04 section 8.1).
var structureEvents = map[string]bool{
	"channel.created": true, "channel.updated": true, "channel.archived": true, "space.archived": true, "thread.moved": true,
}

// credRef keys the delivery state of one credential: the principal with session, device and
// token, like cacheKey without the version.
type credRef struct {
	principal, session, device, token ids.ID
}

// builtAt is the rights version and structure generation a delivery snapshot was built at.
type builtAt struct {
	version int64
	gen     uint64
}

// deliveryState is the part of the rights cache that Observe drives (02 "Permission cache"): the
// highest rights version of the log so far, a process-wide structure generation, and per
// credential the version and generation of the snapshot the last delivery check built. A snapshot
// serves a delivery check while its version is at least the highest rights version the log has
// shown and its generation is current. The key is the process-wide version, not a floor per
// principal, because a rights change without a difference of visible targets writes no
// access.changed (04 section 8.2), for example the revocation of an API token; every later event
// carries the new version (docs/DECISIONS.md M1-65). It is safe for concurrent use.
type deliveryState struct {
	mu    sync.Mutex
	max   int
	n     int
	built map[ids.ID]map[credRef]builtAt
	gen   uint64
	seen  int64
}

func newDeliveryState(maxEntries int) *deliveryState {
	return &deliveryState{max: maxEntries, built: map[ids.ID]map[credRef]builtAt{}}
}

// observe applies one event: its rights version raises the highest version seen, so every older
// delivery snapshot is rebuilt at its next use; access.changed also forgets the delivery
// snapshots of its audience; a structure event starts a new generation.
func (s *deliveryState) observe(ev authorize.LogEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = max(s.seen, ev.PermissionVersion)
	switch {
	case ev.Type == eventAccessChanged && !ev.Audience.IsZero():
		s.n -= len(s.built[ev.Audience])
		delete(s.built, ev.Audience)
	case structureEvents[ev.Type]:
		s.gen++
	}
}

// lookup returns the version of the delivery snapshot of r when it is still valid.
func (s *deliveryState) lookup(r credRef) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.built[r.principal][r]
	if !ok || b.version < s.seen || b.gen != s.gen {
		return 0, false
	}
	return b.version, true
}

// generation returns the current structure generation.
func (s *deliveryState) generation() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gen
}

// record notes that the snapshot of r was built at version v and generation gen. Above the bound
// of authz.snapshot_cache_max_entries the state is forgotten, which only costs rebuilds.
func (s *deliveryState) record(r credRef, v int64, gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.built[r.principal]
	if m == nil {
		m = map[credRef]builtAt{}
		s.built[r.principal] = m
	}
	if _, ok := m[r]; !ok {
		s.n++
	}
	m[r] = builtAt{version: v, gen: gen}
	if s.n > s.max {
		s.built, s.n = map[ids.ID]map[credRef]builtAt{}, 0
	}
}

// observed returns the highest rights version Observe has seen; 0 before the first event.
func (s *deliveryState) observed() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seen
}

// Observe implements authorize.Authorizer (02 "Permission cache"): the log tail calls it for every
// durable event in seq order before the hub delivers it. An event with a newer rights version than
// a delivery snapshot makes the next delivery check rebuild that snapshot (access.changed is one
// such event; a revocation without access.changed is covered by the next event of the log); the
// structure events advance the generation, so every delivery snapshot is rebuilt at
// its next use (docs/DECISIONS.md M1-65: a global generation is correct without knowing every
// affected principal, and these events are rare). Requests are unaffected: they read the rights
// version per request (R-RE-02).
func (a *Authorizer) Observe(_ context.Context, ev authorize.LogEvent) {
	a.delivery.observe(ev)
}

// loadDelivery is load in delivery mode: the snapshot of ref serves while it is valid at t, its
// version is at least the highest rights version of the log and its generation is current; otherwise it is
// rebuilt from the current database state at the current rights version, which is never older
// than the log position (permissions at delivery time, 04 section 8.1).
func (a *Authorizer) loadDelivery(ctx context.Context, ref authz.CredentialRef, t time.Time) (entry, error) {
	r := credRef{principal: ref.PrincipalID, session: ref.SessionID, device: ref.DeviceID, token: ref.TokenID}
	key := cacheKey{principal: ref.PrincipalID, session: ref.SessionID, device: ref.DeviceID, token: ref.TokenID}
	if v, ok := a.delivery.lookup(r); ok {
		key.version = v
		if e, ok := a.cache.get(key, t); ok {
			return e, nil
		}
	}
	// The generation is read before the state, so a structure event that commits meanwhile makes
	// the next check rebuild again.
	gen := a.delivery.generation()
	v, err := a.versions.PermissionVersion(ctx)
	if err != nil {
		return entry{}, errs.Wrap(errs.CodeUnavailable, err, "authz: the rights version cannot be read").
			WithCause(errs.CauseDependencyUnavailable)
	}
	key.version = v
	e, err := a.build(ctx, key, ref, t)
	if err != nil {
		return entry{}, err
	}
	a.cache.put(e)
	a.delivery.record(r, v, gen)
	return e, nil
}
