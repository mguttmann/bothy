// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// SessionsDeps are the collaborators of the session use cases (M1-34).
type SessionsDeps struct {
	// Store reads and writes sessions, families and refresh tokens.
	Store SessionStore
	// Devices reads the device of a session and its active dpop key.
	Devices DeviceStore
	// People reads the person of a session and its status.
	People Store
	// Tokens issues the access tokens.
	Tokens *AccessTokens
	// DPoP gives the current nonce of a rejected refresh proof.
	DPoP identity.DPoP
	// UoW runs every write; its registry holds the sync and audit hooks.
	UoW txn.UnitOfWork
	// Authorizer decides auth.token.refresh, auth.logout, session.list and
	// session.extension.create.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries and the refresh denials.
	Audit auditing.Recorder
	// Events appends session.revoked.
	Events eventlog.Appender
	// Cursors encodes and checks the page cursors of the session lists.
	Cursors Cursors
	// Clock is the time of every session, token and revocation; nil means clock.System().
	Clock clock.Clock
	// Binding checks a refresh beyond token and proof (M1-43: the header login of trusted_header
	// sessions); nil checks nothing.
	Binding SessionBinding
}

// Sessions implements identity.Auth. It is safe for concurrent use.
type Sessions struct {
	cfg    Config
	pepper []byte
	d      SessionsDeps
}

// Compile-time check: Sessions is the session port of identity.
var _ identity.Auth = (*Sessions)(nil)

// NewSessions returns the session use cases with the durations of cfg (auth.refresh_token_ttl,
// auth.session_max_age above 0, auth.refresh_reuse_grace not negative), the page sizes and the
// raw token pepper (the HMAC key of refresh tokens). Errors: a missing collaborator, an empty
// pepper or a value outside its range.
func NewSessions(cfg Config, pepper []byte, d SessionsDeps) (*Sessions, error) {
	if d.Store == nil || d.Devices == nil || d.People == nil || d.Tokens == nil || d.DPoP == nil || d.UoW == nil ||
		d.Authorizer == nil || d.Audit == nil || d.Events == nil || d.Cursors == nil {
		return nil, errors.New("identity: NewSessions needs every collaborator")
	}
	if len(pepper) == 0 || cfg.RefreshTokenTTL <= 0 || cfg.SessionMaxAge <= 0 || cfg.RefreshReuseGrace < 0 || cfg.DevicePendingTTL <= 0 {
		return nil, errors.New("identity: sessions need the token pepper, auth.refresh_token_ttl, auth.session_max_age and auth.device_pending_ttl above 0 and auth.refresh_reuse_grace not negative")
	}
	if cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("identity: api.page_size_default must be at least 1 and not above api.page_size_max")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Sessions{cfg: cfg, pepper: append([]byte(nil), pepper...), d: d}, nil
}

// Values of session.revoked and the audit entries (04 section 2.2, 05 section 6).
const (
	eventSessionRevoked = "session.revoked"
	resourceAuthSession = "auth_session"
	receiveSelf         = "self"
	attrCause           = "cause"
	attrFamilyID        = "family_id"
)

// fieldClientComponent is the field pointer of ExtensionSessionCreate.client_component.
const fieldClientComponent = "/client_component"

// invalidEnum is the validation_failed of a value outside its enumeration at pointer.
func invalidEnum(pointer string) error {
	return errs.New(errs.CodeValidationFailed, "").
		WithFields(errs.FieldError{Pointer: pointer, Code: "invalid_enum", Detail: "The value is not one of the allowed values."})
}

// issue writes refresh token generation gen of family f for session s at now and returns the token
// set with a new access token bound to the family key and the device of s.
func (s *Sessions) issue(ctx context.Context, sess SessionRecord, f FamilyRecord, gen int) (identity.TokenSet, error) {
	now := s.d.Clock.Now()
	raw, err := session.NewRefreshToken()
	if err != nil {
		return identity.TokenSet{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	exp := session.RefreshExpiry(now, s.cfg.RefreshTokenTTL, sess.ExpiresAt)
	err = s.d.Store.InsertRefreshToken(ctx, RefreshRecord{
		ID: ids.New(), FamilyID: f.ID, Generation: gen, Hash: session.TokenHash(s.pepper, raw), IssuedAt: now, ExpiresAt: exp,
	})
	if err != nil {
		return identity.TokenSet{}, err
	}
	access, claims, err := s.d.Tokens.Issue(ctx, sess, f.DPoPJKT)
	if err != nil {
		return identity.TokenSet{}, err
	}
	dev, found, err := s.d.Devices.Device(ctx, sess.DeviceID)
	if err != nil {
		return identity.TokenSet{}, err
	}
	if !found {
		return identity.TokenSet{}, invisible()
	}
	return identity.TokenSet{
		AccessToken: identity.NewToken(access), RefreshToken: identity.NewToken(raw), ExpiresIn: claims.ExpiresIn(now),
		RefreshExpiresAt: exp, Session: sessionView(sess), Device: dev,
	}, nil
}

// StartSession implements identity.Auth (brief M1-34 "Session start").
func (s *Sessions) StartSession(ctx context.Context, in identity.StartSession) (identity.TokenSet, error) {
	if !in.AuthMethod.Valid() {
		return identity.TokenSet{}, invalidEnum("/auth_method")
	}
	var out identity.TokenSet
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		jkt, err := s.sessionKey(ctx, in.UserID, in.DeviceID)
		if err != nil {
			return err
		}
		now := s.d.Clock.Now()
		sess := SessionRecord{
			ID: ids.New(), UserID: in.UserID, DeviceID: in.DeviceID, Component: identity.ComponentApp, AuthMethod: in.AuthMethod,
			CreatedAt: now, ExpiresAt: now.Add(s.cfg.SessionMaxAge), LastIP: in.ClientIP,
		}
		if in.IdentityID != nil {
			sess.IdentityID = *in.IdentityID
		}
		out, err = s.create(ctx, sess, key{jkt: jkt})
		return err
	})
	return out, err
}

// key is the DPoP key a new family binds to.
type key struct{ jkt string }

// sessionKey returns the thumbprint of the active dpop key of deviceID when the device belongs to
// the person userID and is not revoked (pending is allowed, 08 section 5.2); not_found otherwise.
func (s *Sessions) sessionKey(ctx context.Context, userID, deviceID ids.ID) (string, error) {
	if _, found, err := s.d.People.User(ctx, userID); err != nil || !found {
		return "", orInvisible(err)
	}
	dev, found, err := s.d.Devices.Device(ctx, deviceID)
	if err != nil || !found || dev.UserID != userID || dev.State == identity.DeviceStateRevoked {
		return "", orInvisible(err)
	}
	k, found, err := s.d.Devices.ActiveKey(ctx, deviceID, identity.KeyPurposeDPoP)
	if err != nil || !found {
		return "", orInvisible(err)
	}
	return k.JKT, nil
}

// create inserts sess with a new family on k and issues generation 0.
func (s *Sessions) create(ctx context.Context, sess SessionRecord, k key) (identity.TokenSet, error) {
	if err := s.d.Store.InsertSession(ctx, sess); err != nil {
		return identity.TokenSet{}, err
	}
	f := FamilyRecord{ID: ids.New(), SessionID: sess.ID, DPoPJKT: k.jkt, CreatedAt: sess.CreatedAt}
	if err := s.d.Store.InsertFamily(ctx, f); err != nil {
		return identity.TokenSet{}, err
	}
	return s.issue(ctx, sess, f, 0)
}

// orInvisible returns err, or the not_found of a missing resource when err is nil.
func orInvisible(err error) error {
	if err != nil {
		return err
	}
	return invisible()
}

// StartChildSession implements identity.Auth (brief M1-34 "Child session").
func (s *Sessions) StartChildSession(ctx context.Context, parentID ids.ID, component identity.ClientComponent) (identity.TokenSet, error) {
	if component != identity.ComponentShareExt {
		return identity.TokenSet{}, invalidEnum(fieldClientComponent)
	}
	var out identity.TokenSet
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		tree, parent, pf, err := s.lockParent(ctx, parentID)
		if err != nil {
			return err
		}
		// At most one active child per parent and component (08 section 7.4): a new one replaces it.
		if err := s.revoke(ctx, activeChildren(tree, parent.ID, component), identity.RevokeLogout, nil); err != nil {
			return err
		}
		child := SessionRecord{
			ID: ids.New(), UserID: parent.UserID, DeviceID: parent.DeviceID, Component: component, ParentID: parent.ID,
			IdentityID: parent.IdentityID, AuthMethod: parent.AuthMethod, CreatedAt: s.d.Clock.Now(), ExpiresAt: parent.ExpiresAt,
		}
		out, err = s.create(ctx, child, key{jkt: pf.DPoPJKT})
		return err
	})
	return out, err
}

// lockParent locks the session parentID with its children and returns them with the parent and
// its active family; not_found unless the parent is an active app session that is no child.
func (s *Sessions) lockParent(ctx context.Context, parentID ids.ID) ([]SessionRecord, SessionRecord, FamilyRecord, error) {
	tree, err := s.d.Store.LockSessionTree(ctx, parentID)
	if err != nil {
		return nil, SessionRecord{}, FamilyRecord{}, err
	}
	parent, ok := find(tree, parentID)
	if !ok || !parent.Active(s.d.Clock.Now()) || parent.Component != identity.ComponentApp || !parent.ParentID.IsZero() {
		return nil, SessionRecord{}, FamilyRecord{}, invisible()
	}
	pf, found, err := s.d.Store.ActiveFamily(ctx, parent.ID)
	if err != nil || !found {
		return nil, SessionRecord{}, FamilyRecord{}, orInvisible(err)
	}
	return tree, parent, pf, nil
}

// activeChildren returns the children of parent in tree with component that are not revoked.
func activeChildren(tree []SessionRecord, parent ids.ID, component identity.ClientComponent) []SessionRecord {
	var out []SessionRecord
	for _, c := range tree {
		if c.ParentID == parent && c.Component == component && c.RevokedAt.IsZero() {
			out = append(out, c)
		}
	}
	return out
}

// find returns the session id of list.
func find(list []SessionRecord, id ids.ID) (SessionRecord, bool) {
	for _, r := range list {
		if r.ID == id {
			return r, true
		}
	}
	return SessionRecord{}, false
}

// ownSession returns the resource auth_session of the request's session for a: its id and the
// person it belongs to. Without a session, or for a session that does not exist, the resource
// has no owner, so authorize answers as for a missing resource (not_found; 401 for anonymous).
func (s *Sessions) ownSession(ctx context.Context, a actor.Actor) (authorize.Resource, error) {
	r := authorize.Resource{Kind: authorize.ResourceAuthSession, ID: a.SessionID}
	if a.SessionID.IsZero() {
		return r, nil
	}
	sess, found, err := s.d.Store.Session(ctx, a.SessionID)
	if err != nil {
		return authorize.Resource{}, err
	}
	if found {
		r.OwnerID = sess.UserID
	}
	return r, nil
}

// check authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *Sessions) check(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// CreateExtensionSession implements identity.Auth: session.extension.create on the caller's
// session (right self, audited), then StartChildSession in the same unit of work.
func (s *Sessions) CreateExtensionSession(ctx context.Context, a actor.Actor, component identity.ClientComponent) (identity.TokenSet, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.TokenSet{}, err
	}
	r, err := s.ownSession(ctx, a)
	if err != nil {
		return identity.TokenSet{}, err
	}
	if err := s.check(ctx, a, authorize.ActionSessionExtensionCreate, r); err != nil {
		return identity.TokenSet{}, err
	}
	var out identity.TokenSet
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		set, err := s.StartChildSession(ctx, a.SessionID, component)
		if err != nil {
			return err
		}
		out = set
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionSessionExtensionCreate), Decision: auditing.DecisionAllow,
			ResourceKind: resourceAuthSession, ResourceID: set.Session.ID,
			Attrs: auditing.Attrs{"parent_session_id": auditing.ID(a.SessionID)},
		})
	})
	return out, err
}

// Logout implements identity.Auth (auth.logout, right self, audited): the session of a with its
// children and their families end with reason logout.
func (s *Sessions) Logout(ctx context.Context, a actor.Actor) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	r, err := s.ownSession(ctx, a)
	if err != nil {
		return err
	}
	if err := s.check(ctx, a, authorize.ActionAuthLogout, r); err != nil {
		return err
	}
	by := a.ID
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.RevokeSession(ctx, a.SessionID, identity.RevokeLogout, &by); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionAuthLogout), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: a.SessionID,
		})
	})
}

// RevokeSession implements identity.Auth.
func (s *Sessions) RevokeSession(ctx context.Context, sessionID ids.ID, reason identity.SessionRevokeReason, by *ids.ID) error {
	if !reason.Valid() {
		return invalidEnum("/reason")
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		tree, err := s.d.Store.LockSessionTree(ctx, sessionID)
		if err != nil {
			return err
		}
		if _, ok := find(tree, sessionID); !ok {
			return invisible()
		}
		return s.revoke(ctx, tree, reason, by)
	})
}

// familyReason is the reason of the families of sessions revoked with reason: logout and expiry
// carry over, every other reason is session_revoked (03 refresh_token_families).
func familyReason(reason identity.SessionRevokeReason) FamilyRevokeReason {
	switch reason {
	case identity.RevokeLogout:
		return FamilyLogout
	case identity.RevokeExpired:
		return FamilyExpired
	case identity.RevokeAdmin, identity.RevokeDeviceRevoked, identity.RevokeRefreshReuse, identity.RevokeUserDisabled,
		identity.RevokeInactive, identity.RevokeStepUpFailed, identity.RevokeTrustedHeaderMismatch:
	}
	return FamilySessionRevoked
}

// revoke revokes the sessions of list (already locked, in ascending id) that are not revoked yet,
// with reason, by the principal by (nil for none): revoked_at and revoke_reason, their active
// families, one session.revoked per session and SessionEnded. Revoked sessions stay unchanged.
func (s *Sessions) revoke(ctx context.Context, list []SessionRecord, reason identity.SessionRevokeReason, by *ids.ID) error {
	var open []ids.ID
	for _, r := range list {
		if r.RevokedAt.IsZero() {
			open = append(open, r.ID)
		}
	}
	if len(open) == 0 {
		return nil
	}
	now := s.d.Clock.Now()
	revoked, err := s.d.Store.RevokeSessions(ctx, open, reason, now)
	if err != nil || len(revoked) == 0 {
		return err
	}
	if err := s.d.Store.RevokeActiveFamilies(ctx, revoked, familyReason(reason), now); err != nil {
		return err
	}
	first, _ := find(list, revoked[0])
	kind, actorID := revokeActor(ctx, by)
	for _, id := range revoked {
		payload, err := json.Marshal(struct {
			SessionID string `json:"session_id"`
			Reason    string `json:"reason"`
		}{id.String(), string(reason)})
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, err, "identity: session.revoked payload")
		}
		err = s.d.Events.Append(ctx, eventlog.Event{
			Type: eventSessionRevoked, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
			ResourceKind: resourceAuthSession, ResourceID: id, AudiencePrincipalID: first.UserID, ReceiveAction: receiveSelf, Payload: payload,
		})
		if err != nil {
			return err
		}
	}
	return txn.Publish(ctx, identity.SessionEnded{SessionIDs: revoked, UserID: first.UserID, DeviceID: first.DeviceID, Reason: reason})
}

// revokeActor is the actor of session.revoked: by with the kind of the request's principal when
// that is by, a person otherwise; none without by.
func revokeActor(ctx context.Context, by *ids.ID) (eventlog.ActorKind, ids.ID) {
	if by == nil || by.IsZero() {
		return "", ids.ID{}
	}
	if kind, id := eventActor(ctx); id == *by {
		return kind, id
	}
	return eventlog.ActorUser, *by
}

// sessionsList is the list name of the session cursors; one name for both lists, because the
// binding names the listed person.
const sessionsList = "sessions"

// ListSessions implements identity.Auth: session.list on the person userID (self or owner). The
// catalogue names the resource user, which every person may see in the directory, so authz
// answers another person with forbidden (not_self); the sessions of another person are not
// readable for the caller, so the answer is the not_found of 05 rule 2 (docs/DECISIONS.md M1-34).
func (s *Sessions) ListSessions(ctx context.Context, a actor.Actor, userID ids.ID, p identity.Page) (identity.SessionPage, error) {
	if err := s.checkList(ctx, a, userID); err != nil {
		return identity.SessionPage{}, err
	}
	binding := []byte(a.ID.String() + "\x00user=" + userID.String() + "\x00sort=id_desc")
	var before ids.ID
	if p.Cursor != "" {
		var k cursorKey
		if err := s.d.Cursors.Decode(sessionsList, binding, p.Cursor, &k); err != nil {
			return identity.SessionPage{}, err
		}
		if k.ID.IsZero() {
			return identity.SessionPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		before = k.ID
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.d.Store.ActiveSessionsOfUser(ctx, userID, before, s.d.Clock.Now(), limit+1)
	if err != nil {
		return identity.SessionPage{}, err
	}
	var out identity.SessionPage
	if len(rows) > limit {
		rows = rows[:limit]
		next, err := s.d.Cursors.Encode(sessionsList, binding, cursorKey{ID: rows[limit-1].ID})
		if err != nil {
			return identity.SessionPage{}, errs.Wrap(errs.CodeInternalError, err, "identity: session cursor")
		}
		out.NextCursor = next
	}
	for _, r := range rows {
		v := sessionView(r)
		v.Current = r.ID == a.SessionID
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// checkList authorizes session.list on the person userID and then answers an unknown person with
// not_found; not_self of another person is not_found too (see ListSessions).
func (s *Sessions) checkList(ctx context.Context, a actor.Actor, userID ids.ID) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionSessionList, userResource(userID))
	if err != nil {
		return err
	}
	if d.Outcome == authorize.OutcomeForbidden && d.Reason == reasonNotSelf {
		return invisible()
	}
	if err := d.Err(); err != nil {
		return err
	}
	if _, found, err := s.d.People.User(ctx, userID); err != nil || !found {
		return orInvisible(err)
	}
	return nil
}

// reasonNotSelf is the reason of authz for a resource of another principal.
const reasonNotSelf authorize.Reason = "not_self"

// sessionView converts a record into the port DTO.
func sessionView(r SessionRecord) identity.Session {
	return identity.Session{
		ID: r.ID, UserID: r.UserID, DeviceID: r.DeviceID, AuthMethod: r.AuthMethod, ClientComponent: r.Component,
		ParentSessionID: r.ParentID, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, StepUpAt: r.StepUpAt,
		StepUpMethod: r.StepUpMethod, LastSeenAt: r.LastSeenAt,
	}
}
