// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// refreshDeniedError is a refused refresh: the error of the answer and the audit entry written after
// the rollback. It never carries the token or its hash.
type refreshDeniedError struct {
	answer error
	entry  auditing.Entry
}

func (d *refreshDeniedError) Error() string { return "identity: refresh denied" }

// The causes of the refresh denials in audit details (codes, SEC-AUD-5).
const (
	causeRefreshReuse   = "refresh_reuse"
	causeKeyMismatch    = "dpop_key_mismatch"
	causeRefreshExpired = "refresh_expired"
	causeUnknownToken   = "unknown_token"
	causeRevokedState   = "revoked_state"
	causeIDPRecheck     = "idp_recheck"
)

// deny builds the denial of a refresh with the answer code, the audit reason and the cause.
func deny(answer error, reason auditing.Reason, cause string, sessionID ids.ID) *refreshDeniedError {
	e := auditing.Entry{
		Action: string(authorize.ActionAuthTokenRefresh), Decision: auditing.DecisionDeny, Reason: reason,
		ResourceKind: resourceAuthSession, ResourceID: sessionID, Attrs: auditing.Attrs{attrCause: auditing.Code(cause)},
	}
	return &refreshDeniedError{answer: answer, entry: e}
}

// Refresh implements identity.Auth (brief M1-34 "Refresh", 03 refresh_tokens): one unit of work
// with the session rows and the family locked, the checks in the order of the brief, then the
// rotation decision of domain/session. A reuse commits its revocation before the request fails;
// every other denial rolls back and is audited afterwards. In maintenance mode
// (server.maintenance_mode = read_only, M7-08) a due rotation becomes reissue: a new access token
// and the presented refresh token, without a write.
func (s *Sessions) Refresh(ctx context.Context, token string, proof identity.VerifiedProof) (identity.TokenSet, error) {
	// Proof of check (02 section 9.3): the anonymous principal, right public.
	d, err := s.d.Authorizer.Authorize(ctx, actor.Anonymous(), authorize.ActionAuthTokenRefresh, authorize.Resource{Kind: authorize.ResourceAuthSession})
	if err != nil {
		return identity.TokenSet{}, err
	}
	if err := d.Err(); err != nil {
		return identity.TokenSet{}, err
	}
	hash := session.TokenHash(s.pepper, token)
	var (
		out       identity.TokenSet
		committed error
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		set, after, err := s.rotate(ctx, hash, token, proof)
		out, committed = set, after
		return err
	})
	var denial *refreshDeniedError
	if errors.As(err, &denial) {
		_ = s.d.Audit.RecordDenial(ctx, denial.entry)
		return identity.TokenSet{}, denial.answer
	}
	if err != nil {
		return identity.TokenSet{}, err
	}
	if committed != nil {
		return identity.TokenSet{}, committed
	}
	return out, nil
}

// refreshState is a refresh token with its locked session rows, session and family.
type refreshState struct {
	tok  RefreshRecord
	tree []SessionRecord
	sess SessionRecord
	fam  FamilyRecord
}

// rotate is the body of Refresh inside its unit of work. A refusal whose revocation must commit
// (a reuse, an OIDC session due for a new provider login, a trusted header session whose binding
// failed) returns its answer as committed with a nil error, so the unit commits and Refresh
// answers with it.
func (s *Sessions) rotate(ctx context.Context, hash []byte, token string, proof identity.VerifiedProof) (set identity.TokenSet, committed, err error) {
	st, err := s.lockRefresh(ctx, hash)
	if err != nil {
		return identity.TokenSet{}, nil, err
	}
	// A proof of another key never revokes: whoever lacks the device key must not be able to end
	// someone's session (08 section 13, ID-T17).
	if proof.JKT != st.fam.DPoPJKT {
		answer := &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: s.d.DPoP.CurrentNonce()}
		return identity.TokenSet{}, nil, deny(answer, auditing.ReasonUnauthenticated, causeKeyMismatch, st.sess.ID)
	}
	now := s.d.Clock.Now()
	due, err := s.recheckDue(ctx, st.sess, now)
	if err != nil {
		return identity.TokenSet{}, nil, err
	}
	if due {
		return identity.TokenSet{}, errs.New(errs.CodeUnauthenticated, ""), s.expireForRecheck(ctx, st)
	}
	if s.d.Binding != nil {
		ok, err := s.d.Binding.Bound(ctx, st.sess)
		if err != nil {
			return identity.TokenSet{}, nil, err
		}
		if !ok {
			// Like a reuse, the revocation commits and the refresh answers session_revoked
			// (08 section 2.4 rule 5).
			return identity.TokenSet{}, errs.New(errs.CodeSessionRevoked, ""), s.unbound(ctx, st)
		}
	}
	outcome := session.Decide(session.Family{CurrentGeneration: st.fam.CurrentGeneration, RotatedAt: st.fam.RotatedAt},
		session.Token{Generation: st.tok.Generation, UsedAt: st.tok.UsedAt, ExpiresAt: st.tok.ExpiresAt}, now, s.cfg.RefreshReuseGrace)
	return s.settle(ctx, st, token, outcome, now)
}

// settle carries out the rotation decision outcome for st; its results are those of rotate.
func (s *Sessions) settle(ctx context.Context, st refreshState, token string, outcome session.Outcome, now time.Time) (set identity.TokenSet, committed, err error) {
	switch outcome {
	case session.OutcomeRotate, session.OutcomeGraceRotate:
		if outcome == session.OutcomeRotate && s.cfg.readOnly() {
			// Maintenance mode: a new access token, the same refresh token, no write (08
			// section 7.4, ADR 0065 item 4). A grace rotation still rotates (D-M7-08-3).
			set, err := s.reissue(ctx, st, token)
			return set, nil, err
		}
		// Rotate marks the presented token; grace marks the current generation the client never
		// received (03 refresh_tokens item 2).
		if err := s.d.Store.MarkRefreshUsed(ctx, st.fam.ID, st.fam.CurrentGeneration, now); err != nil {
			return identity.TokenSet{}, nil, err
		}
		next := st.fam.CurrentGeneration + 1
		if err := s.d.Store.AdvanceFamily(ctx, st.fam.ID, next, now); err != nil {
			return identity.TokenSet{}, nil, err
		}
		set, err := s.issue(ctx, st.sess, st.fam, next)
		return set, nil, err
	case session.OutcomeReuse:
		return identity.TokenSet{}, errs.New(errs.CodeSessionRevoked, ""), s.reuse(ctx, st, now)
	case session.OutcomeExpired, session.OutcomeFamilyRevoked:
	}
	return identity.TokenSet{}, nil, deny(errs.New(errs.CodeUnauthenticated, ""), auditing.ReasonUnauthenticated, causeRefreshExpired, st.sess.ID)
}

// reissue is the refresh of maintenance mode: it issues an access token with the normal lifetime
// for the session and the family key of st and returns the presented refresh token with its stored
// expiry; nothing is written, so the rows of the session and the family stay as they are and the
// refresh token keeps its sender binding to the device key (RFC 9700 4.14.2).
func (s *Sessions) reissue(ctx context.Context, st refreshState, token string) (identity.TokenSet, error) {
	access, claims, err := s.d.Tokens.Issue(ctx, st.sess, st.fam.DPoPJKT)
	if err != nil {
		return identity.TokenSet{}, err
	}
	dev, found, err := s.d.Devices.Device(ctx, st.sess.DeviceID)
	if err != nil {
		return identity.TokenSet{}, err
	}
	if !found {
		return identity.TokenSet{}, invisible()
	}
	return identity.TokenSet{
		AccessToken: identity.NewToken(access), RefreshToken: identity.NewToken(token), ExpiresIn: claims.ExpiresIn(s.d.Clock.Now()),
		RefreshExpiresAt: st.tok.ExpiresAt, Session: sessionView(st.sess), Device: dev,
	}, nil
}

// recheckDue reports an OIDC session whose provider login is auth.idp_recheck_interval or longer
// ago (08 section 3.5, T-ID-06): created_at of an OIDC session is the time of the provider login,
// and a child session counts from its parent. A zero interval switches the rule off.
func (s *Sessions) recheckDue(ctx context.Context, sess SessionRecord, now time.Time) (bool, error) {
	if s.cfg.IDPRecheckInterval <= 0 || sess.AuthMethod != identity.AuthOIDC {
		return false, nil
	}
	loginAt := sess.CreatedAt
	if !sess.ParentID.IsZero() {
		parent, found, err := s.d.Store.Session(ctx, sess.ParentID)
		if err != nil {
			return false, err
		}
		if found {
			loginAt = parent.CreatedAt
		}
	}
	return !now.Before(loginAt.Add(s.cfg.IDPRecheckInterval)), nil
}

// expireForRecheck revokes the session and its children (expired, families revoked) and records
// the denial inside the unit of work, which commits: the next login goes to the provider.
func (s *Sessions) expireForRecheck(ctx context.Context, st refreshState) error {
	if err := s.revoke(ctx, st.tree, identity.RevokeExpired, nil); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthTokenRefresh), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: resourceAuthSession, ResourceID: st.sess.ID, Attrs: auditing.Attrs{attrCause: auditing.Code(causeIDPRecheck)},
	})
}

// lockRefresh finds the token of hash and locks its session rows in ascending id, then the family
// (lock order of 03 section 6.3). Denials: an unknown token; a revoked family, session, device or
// person and a session past its end.
func (s *Sessions) lockRefresh(ctx context.Context, hash []byte) (refreshState, error) {
	tok, sid, found, err := s.d.Store.RefreshTokenByHash(ctx, hash)
	if err != nil {
		return refreshState{}, err
	}
	if !found {
		return refreshState{}, deny(errs.New(errs.CodeUnauthenticated, ""), auditing.ReasonUnauthenticated, causeUnknownToken, ids.ID{})
	}
	tree, err := s.d.Store.LockSessionTree(ctx, sid)
	if err != nil {
		return refreshState{}, err
	}
	fam, ffound, err := s.d.Store.LockFamily(ctx, tok.FamilyID)
	if err != nil {
		return refreshState{}, err
	}
	sess, sfound := find(tree, sid)
	revoked := deny(errs.New(errs.CodeSessionRevoked, ""), auditing.ReasonSessionRevoked, causeRevokedState, sid)
	if !sfound || !ffound || !sess.Active(s.d.Clock.Now()) {
		return refreshState{}, revoked
	}
	live, err := s.alive(ctx, sess, fam)
	if err != nil {
		return refreshState{}, err
	}
	if !live {
		return refreshState{}, revoked
	}
	return refreshState{tok: tok, tree: tree, sess: sess, fam: fam}, nil
}

// reuse revokes the family (reuse_detected), the session and its children (refresh_reuse, their
// families session_revoked) and records the denial inside the unit of work, which commits.
func (s *Sessions) reuse(ctx context.Context, st refreshState, now time.Time) error {
	if err := s.d.Store.RevokeFamily(ctx, st.fam.ID, FamilyReuseDetected, now); err != nil {
		return err
	}
	if err := s.revoke(ctx, st.tree, identity.RevokeRefreshReuse, nil); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthTokenRefresh), Decision: auditing.DecisionDeny, Reason: auditing.ReasonSessionRevoked,
		ResourceKind: resourceAuthSession, ResourceID: st.sess.ID,
		Attrs: auditing.Attrs{attrCause: auditing.Code(causeRefreshReuse), attrFamilyID: auditing.ID(st.fam.ID)},
	})
}

// unbound revokes the session tree of st whose header binding failed (trusted_header_mismatch,
// families session_revoked) and records the denial inside the unit of work, which commits.
func (s *Sessions) unbound(ctx context.Context, st refreshState) error {
	if err := s.revoke(ctx, st.tree, identity.RevokeTrustedHeaderMismatch, nil); err != nil {
		return err
	}
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthTokenRefresh), Decision: auditing.DecisionDeny, Reason: auditing.ReasonSessionRevoked,
		ResourceKind: resourceAuthSession, ResourceID: st.sess.ID,
		Attrs: auditing.Attrs{attrCause: auditing.Code(string(identity.RevokeTrustedHeaderMismatch))},
	})
}

// alive reports a family that is not revoked, a device that is not revoked whose active dpop key
// is the family's, and a person that is neither disabled nor deleted. A pending person refreshes
// like a pending device (08 R-ID-14).
func (s *Sessions) alive(ctx context.Context, sess SessionRecord, fam FamilyRecord) (bool, error) {
	if !fam.RevokedAt.IsZero() {
		return false, nil
	}
	dev, found, err := s.d.Devices.Device(ctx, sess.DeviceID)
	// A pending device past auth.device_pending_ttl counts as revoked (08 section 5.2, M1-38).
	if err != nil || !found || dev.State == identity.DeviceStateRevoked || pendingExpired(dev, s.d.Clock.Now(), s.cfg.DevicePendingTTL) {
		return false, err
	}
	// The family is bound to the device's dpop key; once that key is no longer active, the access
	// tokens of a refresh would be refused anyway (device_inactive), so the refresh ends here.
	if dev.KeyThumbprint != fam.DPoPJKT {
		return false, nil
	}
	// An active device without contact for auth.device_inactive_revoke_after refreshes no more
	// (08 section 5.4, M1-45), like the Authenticator refuses its access tokens.
	if after := s.cfg.DeviceInactiveRevokeAfter; after > 0 && dev.State == identity.DeviceStateActive &&
		inactiveSince(dev.LastSeenAt, dev.ApprovedAt, time.Time{}, s.d.Clock.Now().Add(-after)) {
		return false, nil
	}
	p, found, err := s.d.People.Principal(ctx, sess.UserID)
	if err != nil || !found {
		return false, err
	}
	st := p.Principal.Status
	return st == domain.StatusActive || st == domain.StatusPending, nil
}
