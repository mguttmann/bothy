// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/trustedheader"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the incoming port of the header login.
var _ identity.TrustedHeaderLogins = (*Service)(nil)

// The audit causes of a refused header login (codes, 19 SEC-AUD-5); never the login.
// causeNoMapping and attrIdentityID are shared with the OIDC login (oidc_complete.go, oidc.go).
const (
	causeHeaderAbsent = "header_absent"
	causeInactive     = "person_not_active"
)

// headerRejectedError is a refused header login after the header was read: the cause for the audit
// entry; the answer is always the neutral 401.
type headerRejectedError struct{ cause string }

// Error implements error without the login.
func (e *headerRejectedError) Error() string {
	return "identity: header login rejected (" + e.cause + ")"
}

// headerLoginInput is the evaluated request of a header login.
type headerLoginInput struct {
	provider identity.Provider
	login    string
	name     string
	code     string
	jkt      string
	in       identity.TrustedHeaderLogin
}

// LoginWithTrustedHeader implements identity.TrustedHeaderLogins (brief M1-43
// "loginWithTrustedHeader"): authorize auth.login.complete for the anonymous principal; 404 while
// the driver is off; the login header from a trusted peer behind a loopback listener (else 401 and
// an audit denial); the DPoP proof; then one unit of work maps the login (identity, enrollment
// code, provisioning) and completes the login with auth_method trusted_header.
func (s *Service) LoginWithTrustedHeader(ctx context.Context, in identity.TrustedHeaderLogin) (identity.RedeemResult, error) {
	d, err := s.d.Authorizer.Authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginComplete, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if err := d.Err(); err != nil {
		return identity.RedeemResult{}, err
	}
	drv := s.d.TrustedHeader
	if !drv.Loopback() {
		return identity.RedeemResult{}, invisible()
	}
	p, cfg, found, err := drv.enabledProvider(ctx)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if !found {
		return identity.RedeemResult{}, invisible()
	}
	login, ok := drv.evaluate(ctx, in.Peer, in.Header, cfg.LoginHeader)
	if !ok {
		_ = s.d.Audit.RecordDenial(ctx, headerDenial(causeHeaderAbsent, p.ID, ids.ID{}))
		return identity.RedeemResult{}, errs.New(errs.CodeUnauthenticated, "")
	}
	if in.Console && in.EnrollmentCode != "" {
		return identity.RedeemResult{}, consoleFormRefused(fieldEnrollmentCode)
	}
	proof, err := s.verifyLoginProof(ctx, in.Console, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	name, ok := drv.evaluate(ctx, in.Peer, in.Header, cfg.NameHeader)
	if !ok {
		name = login
	}
	hl := headerLoginInput{provider: p, login: login, name: name, code: in.EnrollmentCode, jkt: proof.JKT, in: in}
	var (
		out     LoginResult
		matched MatchedEnrollmentCode
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		matched = MatchedEnrollmentCode{}
		var err error
		out, err = s.headerLogin(ctx, hl, &matched)
		return err
	})
	if err != nil {
		return identity.RedeemResult{}, s.headerFailed(ctx, err, p.ID, matched.ID)
	}
	res := out.Result()
	if proof.NonceStale {
		res.Nonce = s.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// headerLogin is the unit of work of a header login (08 section 3.1): the provider share-locked,
// then the active identity of (provider, login); else the enrollment code links the login to the
// person of the code (device active, code used, identity.link); else provisioning creates a
// pending person; else the neutral rejection.
func (s *Service) headerLogin(ctx context.Context, hl headerLoginInput, matched *MatchedEnrollmentCode) (LoginResult, error) {
	if _, found, err := s.d.Providers.EnabledProvider(ctx, hl.provider.ID); err != nil || !found {
		if err == nil {
			err = &headerRejectedError{cause: causeNoMapping}
		}
		return LoginResult{}, err
	}
	c := LoginCompletion{
		AuthMethod: identity.AuthTrustedHeader, Device: hl.in.Device, ProofJKT: hl.jkt, Trust: TrustUnconfirmed,
		ClientIP: hl.in.ClientIP, ProviderID: hl.provider.ID, AllowPending: true, Console: hl.in.Console,
	}
	ident, found, err := s.d.Identities.ActiveIdentityBySubject(ctx, hl.provider.ID, hl.login)
	switch {
	case err != nil:
		return LoginResult{}, err
	case found:
		c.UserID = ident.UserID
		c.IdentityID = &ident.ID
	case hl.code != "":
		m, err := s.d.Enrollment.Match(ctx, hl.code, domain.ConsumablePurposes(domain.ConsumerIdentityLink)...)
		if err != nil {
			return LoginResult{}, err
		}
		*matched = m
		id, err := s.linkHeaderIdentity(ctx, m.UserID, hl, m.ID)
		if err != nil {
			return LoginResult{}, err
		}
		c.UserID, c.IdentityID, c.Trust = m.UserID, &id, TrustConfirmed
	case hl.provider.AllowProvisioning:
		user, err := s.provision(ctx, hl.login, hl.name)
		if err != nil {
			return LoginResult{}, err
		}
		id, err := s.insertHeaderIdentity(ctx, user, hl)
		if err != nil {
			return LoginResult{}, err
		}
		c.UserID, c.IdentityID, c.Provisioned = user, &id, true
	default:
		return LoginResult{}, &headerRejectedError{cause: causeNoMapping}
	}
	out, err := s.completeLogin(ctx, c)
	if err != nil {
		return LoginResult{}, err
	}
	if !matched.ID.IsZero() {
		return out, s.d.Enrollment.MarkUsed(ctx, matched.ID, out.Device.ID)
	}
	return out, nil
}

// insertHeaderIdentity links the login of hl to userID at the header provider (issuer empty).
func (s *Service) insertHeaderIdentity(ctx context.Context, userID ids.ID, hl headerLoginInput) (ids.ID, error) {
	rec, err := s.d.Identities.InsertIdentity(ctx, identity.Identity{
		ID: ids.New(), UserID: userID, ProviderID: hl.provider.ID, ProviderKind: identity.ProviderTrustedHeader,
		Subject: hl.login, LinkedAt: s.d.Clock.Now(),
	})
	return rec.ID, err
}

// linkHeaderIdentity links the login to the person of an enrollment code and records the allow
// entry of identity.link with the person as actor: provider, identity and code ids, never the
// login (08 section 3.1 item 2).
func (s *Service) linkHeaderIdentity(ctx context.Context, userID ids.ID, hl headerLoginInput, codeID ids.ID) (ids.ID, error) {
	id, err := s.insertHeaderIdentity(ctx, userID, hl)
	if err != nil {
		return ids.ID{}, err
	}
	as := actor.WithActor(ctx, actor.Actor{Kind: actor.KindUser, ID: userID})
	return id, s.d.Audit.Record(as, auditing.Entry{
		Action: string(authorize.ActionIdentityLink), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: userID,
		Attrs: auditing.Attrs{attrProviderID: auditing.ID(hl.provider.ID), attrIdentityID: auditing.ID(id), attrEnrollmentCodeID: auditing.ID(codeID)},
	})
}

// suffixAttempts bounds the random suffixes tried for a taken handle.
const suffixAttempts = 5

// provision creates the pending person of an unknown login (08 section 3.1 item 3): handle from the
// part of the login before "@" (a taken one gets a random suffix), display name from the name
// header or the login, no e-mail, status pending; user.updated is appended like createUser does.
func (s *Service) provision(ctx context.Context, login, name string) (ids.ID, error) {
	h, err := s.freeHandle(ctx, trustedheader.HandleFromLogin(login, s.cfg.HandleMaxChars))
	if err != nil {
		return ids.ID{}, err
	}
	dn, err := domain.NewDisplayName(cutRunes(name, s.cfg.DisplayNameMaxChars), s.cfg.DisplayNameMaxChars)
	if err != nil {
		if dn, err = domain.NewDisplayName(h.String(), s.cfg.DisplayNameMaxChars); err != nil {
			return ids.ID{}, err
		}
	}
	p, err := domain.NewPrincipal(ids.New(), domain.KindUser, h, dn)
	if err != nil {
		return ids.ID{}, err
	}
	p.Status = domain.StatusPending
	u, err := domain.NewUser(p, "", "", "")
	if err != nil {
		return ids.ID{}, err
	}
	if err := s.d.People.InsertPrincipal(ctx, &u.Principal); err != nil {
		return ids.ID{}, err
	}
	if err := s.d.People.InsertUser(ctx, u); err != nil {
		return ids.ID{}, err
	}
	if s.d.Events != nil {
		if err := appendUserUpdated(ctx, s.d.Events, userView(UserRecord{User: u})); err != nil {
			return ids.ID{}, err
		}
	}
	return u.Principal.ID, nil
}

// freeHandle returns base as a handle when no principal has it, else base with a random suffix.
// Errors: conflict at /handle when every attempt is taken; the store errors.
func (s *Service) freeHandle(ctx context.Context, base string) (domain.Handle, error) {
	candidate := base
	for i := 0; i <= suffixAttempts; i++ {
		_, taken, err := s.d.People.PrincipalByHandle(ctx, candidate)
		if err != nil {
			return domain.Handle{}, err
		}
		if !taken {
			return domain.NewHandle(candidate, s.cfg.HandleMaxChars)
		}
		suffix, err := randomSuffix(s.d.Random)
		if err != nil {
			return domain.Handle{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
		}
		candidate = trustedheader.HandleWithSuffix(base, suffix, s.cfg.HandleMaxChars)
	}
	return domain.Handle{}, errs.New(errs.CodeConflict, "").WithFields(errs.FieldError{Pointer: "/handle", Code: fieldCodeTaken, Detail: "The handle is taken."})
}

// suffixAlphabet are the characters of a handle suffix (lower case Crockford Base32).
const suffixAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"

// randomSuffix returns four characters of suffixAlphabet from r (crypto/rand when nil).
func randomSuffix(r io.Reader) (string, error) {
	if r == nil {
		r = rand.Reader
	}
	var b [4]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", err
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = suffixAlphabet[int(v)%len(suffixAlphabet)]
	}
	return string(out), nil
}

// cutRunes returns s cut to at most n runes.
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// headerFailed audits a failed header login and returns its answer: an unmapped login, a rejected
// code and a person that is not active give the one neutral 401; a rejected login with a matching
// code counts a failed attempt of the code (as a redemption does); a server failure is returned
// unchanged.
func (s *Service) headerFailed(ctx context.Context, err error, providerID, codeID ids.ID) error {
	var rej *headerRejectedError
	var code *EnrollmentRejectedError
	switch {
	case errors.As(err, &rej):
		_ = s.d.Audit.RecordDenial(ctx, headerDenial(rej.cause, providerID, ids.ID{}))
		return errs.New(errs.CodeUnauthenticated, "")
	case errors.As(err, &code):
		_ = s.d.Audit.RecordDenial(ctx, headerDenial(code.Cause, providerID, code.CodeID))
		return code.Unwrap()
	case codeID.IsZero() && errs.KindOf(err) == errs.KindUnauthenticated && !isDPoPError(err):
		_ = s.d.Audit.RecordDenial(ctx, headerDenial(causeInactive, providerID, ids.ID{}))
		return errs.New(errs.CodeUnauthenticated, "")
	}
	return s.redeemFailed(ctx, err, codeID)
}

// headerDenial is the deny entry of a refused header login: cause, provider and, when a code
// matched, the code id; never the login.
func headerDenial(cause string, providerID, codeID ids.ID) auditing.Entry {
	e := redeemDenial(cause, codeID)
	e.Attrs[attrProviderID] = auditing.ID(providerID)
	return e
}
