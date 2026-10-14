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
// every other denial rolls back and is audited afterwards.
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
	var out identity.TokenSet
	reused := false
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		reused = false
		set, reuse, err := s.rotate(ctx, hash, proof)
		out, reused = set, reuse
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
	if reused {
		return identity.TokenSet{}, errs.New(errs.CodeSessionRevoked, "")
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

// rotate is the body of Refresh inside its unit of work; reused reports a committed reuse.
func (s *Sessions) rotate(ctx context.Context, hash []byte, proof identity.VerifiedProof) (identity.TokenSet, bool, error) {
	st, err := s.lockRefresh(ctx, hash)
	if err != nil {
		return identity.TokenSet{}, false, err
	}
	// A proof of another key never revokes: whoever lacks the device key must not be able to end
	// someone's session (08 section 13, ID-T17).
	if proof.JKT != st.fam.DPoPJKT {
		answer := &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: s.d.DPoP.CurrentNonce()}
		return identity.TokenSet{}, false, deny(answer, auditing.ReasonUnauthenticated, causeKeyMismatch, st.sess.ID)
	}
	now := s.d.Clock.Now()
	outcome := session.Decide(session.Family{CurrentGeneration: st.fam.CurrentGeneration, RotatedAt: st.fam.RotatedAt},
		session.Token{Generation: st.tok.Generation, UsedAt: st.tok.UsedAt, ExpiresAt: st.tok.ExpiresAt}, now, s.cfg.RefreshReuseGrace)
	switch outcome {
	case session.OutcomeRotate, session.OutcomeGraceRotate:
		// Rotate marks the presented token; grace marks the current generation the client never
		// received (03 refresh_tokens item 2).
		if err := s.d.Store.MarkRefreshUsed(ctx, st.fam.ID, st.fam.CurrentGeneration, now); err != nil {
			return identity.TokenSet{}, false, err
		}
		next := st.fam.CurrentGeneration + 1
		if err := s.d.Store.AdvanceFamily(ctx, st.fam.ID, next, now); err != nil {
			return identity.TokenSet{}, false, err
		}
		set, err := s.issue(ctx, st.sess, st.fam, next)
		return set, false, err
	case session.OutcomeReuse:
		return identity.TokenSet{}, true, s.reuse(ctx, st, now)
	case session.OutcomeExpired, session.OutcomeFamilyRevoked:
	}
	return identity.TokenSet{}, false, deny(errs.New(errs.CodeUnauthenticated, ""), auditing.ReasonUnauthenticated, causeRefreshExpired, st.sess.ID)
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
	p, found, err := s.d.People.Principal(ctx, sess.UserID)
	if err != nil || !found {
		return false, err
	}
	st := p.Principal.Status
	return st == domain.StatusActive || st == domain.StatusPending, nil
}
