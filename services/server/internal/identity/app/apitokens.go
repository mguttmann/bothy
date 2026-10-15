// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PersonalTokenSwitch is the consumer port of the switch tokens.personal_enabled (16 section 6.2):
// whether principalID may create personal tokens. internal/wire binds it over the feature flags
// of instance (adapters/instanceflags, M1-47).
type PersonalTokenSwitch interface {
	// PersonalTokensEnabled reports whether the switch is on for principalID. Errors: those of
	// the flag source; the caller fails closed.
	PersonalTokensEnabled(ctx context.Context, principalID ids.ID) (bool, error)
}

// The values of auth.dpop_required_for (08 section 18).
const (
	DPoPForAgent    = "agent"
	DPoPForApp      = "app"
	DPoPForPersonal = "personal"
)

// DPoPRequiredForValues are the allowed values of auth.dpop_required_for.
var DPoPRequiredForValues = []string{DPoPForAgent, DPoPForApp, DPoPForPersonal}

// TokensDeps are the collaborators of the API token use cases (M1-44).
type TokensDeps struct {
	// Store reads and writes api_tokens.
	Store APITokenStore
	// Principals reads the holder of a token.
	Principals Store
	// UoW runs every write; its registry holds the rights, sync and audit hooks.
	UoW txn.UnitOfWork
	// Authorizer decides token.list, token.create and token.revoke.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries of token.create and token.revoke.
	Audit auditing.Recorder
	// Cursors encodes and checks the cursors of listTokens.
	Cursors Cursors
	// Switch is tokens.personal_enabled.
	Switch PersonalTokenSwitch
	// Clock is the time of creation, expiry checks and revocation; nil means clock.System().
	Clock clock.Clock
	// Random is the source of the random part; nil means crypto/rand (ID-T27). Only tests set it.
	Random io.Reader
}

// Tokens implements identity.Tokens and identity.TokenAdmin (08 section 12, brief M1-44). It is
// safe for concurrent use.
type Tokens struct {
	cfg    Config
	pepper []byte
	d      TokensDeps
}

// Compile-time checks: the two incoming ports.
var (
	_ identity.Tokens     = (*Tokens)(nil)
	_ identity.TokenAdmin = (*Tokens)(nil)
)

// NewTokens returns the token use cases with auth.api_token_max_ttl and
// auth.personal_token_max_ttl above 0, auth.dpop_required_for out of agent, app and personal, the
// page sizes and the raw token pepper (the HMAC key). Errors: a missing collaborator, an empty
// pepper or a value outside its range.
func NewTokens(cfg Config, pepper []byte, d TokensDeps) (*Tokens, error) {
	if d.Store == nil || d.Principals == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Cursors == nil || d.Switch == nil {
		return nil, errors.New("identity: NewTokens needs every collaborator")
	}
	if len(pepper) == 0 || cfg.APITokenMaxTTL <= 0 || cfg.PersonalTokenMaxTTL <= 0 || cfg.PageSizeDefault < 1 || cfg.PageSizeMax < cfg.PageSizeDefault {
		return nil, errors.New("identity: API tokens need the token pepper, auth.api_token_max_ttl and auth.personal_token_max_ttl above 0 and valid page sizes")
	}
	if err := CheckDPoPRequiredFor(cfg.DPoPRequiredFor); err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Tokens{cfg: cfg, pepper: append([]byte(nil), pepper...), d: d}, nil
}

// CheckDPoPRequiredFor reports a value of auth.dpop_required_for outside agent, app and personal;
// the error names the key, never the value.
func CheckDPoPRequiredFor(values []string) error {
	for _, v := range values {
		if !slices.Contains(DPoPRequiredForValues, v) {
			return errors.New("identity: auth.dpop_required_for allows only agent, app and personal")
		}
	}
	return nil
}

// tokenKind is what the kind of a holder decides: the token kind, the value of
// auth.dpop_required_for and whether the kind carries scopes.
type tokenKind struct {
	kind   identity.TokenKind
	dpop   string
	scopes bool
}

// kindOf returns the token kind of a principal kind (08 section 12: agt only for agents, app only
// for apps, pat only for people).
func kindOf(k domain.Kind) (tokenKind, bool) {
	switch k {
	case domain.KindAgent:
		return tokenKind{kind: identity.TokenKindAgent, dpop: DPoPForAgent}, true
	case domain.KindApp:
		return tokenKind{kind: identity.TokenKindApp, dpop: DPoPForApp, scopes: true}, true
	case domain.KindUser:
		return tokenKind{kind: identity.TokenKindPersonal, dpop: DPoPForPersonal}, true
	}
	return tokenKind{}, false
}

// actorKindOf returns the principal kind an API token kind authenticates; whi and hpc
// authenticate none, so the authenticator refuses them before any database call.
func actorKindOf(k apitoken.Kind) (domain.Kind, actor.Kind, bool) {
	switch k {
	case apitoken.KindAgent:
		return domain.KindAgent, actor.KindAgent, true
	case apitoken.KindApp:
		return domain.KindApp, actor.KindApp, true
	case apitoken.KindPersonal:
		return domain.KindUser, actor.KindUser, true
	case apitoken.KindIncomingWebhook, apitoken.KindHostPairingCode:
	}
	return "", "", false
}

// NewHostPairingCode implements identity.Tokens: a code of kind hpc from the random source of
// the use cases (crypto/rand unless a test sets one) and its HMAC with the pepper.
func (s *Tokens) NewHostPairingCode(context.Context) (identity.PairingCode, error) {
	secret, err := apitoken.Generate(s.d.Random, apitoken.KindHostPairingCode)
	if err != nil {
		return identity.PairingCode{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	return identity.PairingCode{Secret: identity.NewToken(secret), Hash: apitoken.Hash(s.pepper, secret)}, nil
}

// HashHostPairingCode implements identity.Tokens: apitoken.Parse checks form, checksum and range,
// then the kind must be hpc; only then is the HMAC computed (M2-41, Design note 1). Every other
// value is identity.ErrTokenMalformed.
func (s *Tokens) HashHostPairingCode(_ context.Context, raw string) ([]byte, error) {
	p, err := apitoken.Parse(raw)
	if err != nil || p.Kind != apitoken.KindHostPairingCode {
		return nil, identity.ErrTokenMalformed
	}
	return apitoken.Hash(s.pepper, raw), nil
}

// now is the time of the clock at the precision of PostgreSQL, so answers equal stored values.
func (s *Tokens) now() time.Time { return s.d.Clock.Now().Truncate(time.Microsecond) }

// Issue implements identity.Tokens.
func (s *Tokens) Issue(ctx context.Context, cmd identity.IssueTokenCmd) (identity.IssuedToken, error) {
	var out identity.IssuedToken
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, found, err := s.d.Principals.Principal(ctx, cmd.PrincipalID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		k, ok := kindOf(rec.Principal.Kind)
		if !ok {
			return errs.New(errs.CodeInternalError, "identity: principal of an unknown kind")
		}
		out, err = s.issue(ctx, k, cmd)
		return err
	})
	return out, err
}

// issue validates cmd for the kind k and inserts the token inside the unit of work.
func (s *Tokens) issue(ctx context.Context, k tokenKind, cmd identity.IssueTokenCmd) (identity.IssuedToken, error) {
	now := s.now()
	maxTTL := s.cfg.APITokenMaxTTL
	if k.kind == identity.TokenKindPersonal {
		maxTTL = s.cfg.PersonalTokenMaxTTL
	}
	if err := domain.CheckTokenName(cmd.Name); err != nil {
		return identity.IssuedToken{}, err
	}
	if err := domain.CheckTokenScopes(k.scopes, cmd.Scopes); err != nil {
		return identity.IssuedToken{}, err
	}
	// The expiry is checked at the precision it is stored with, so a token never leaves already
	// expired.
	expires := cmd.ExpiresAt.Truncate(time.Microsecond)
	if err := domain.CheckTokenExpiry(expires, now, maxTTL); err != nil {
		return identity.IssuedToken{}, err
	}
	if err := domain.CheckTokenJKT(cmd.DPoPJKT, slices.Contains(s.cfg.DPoPRequiredFor, k.dpop)); err != nil {
		return identity.IssuedToken{}, err
	}
	secret, err := apitoken.Generate(s.d.Random, apitoken.Kind(k.kind))
	if err != nil {
		return identity.IssuedToken{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	p, err := apitoken.Parse(secret)
	if err != nil {
		return identity.IssuedToken{}, errs.Wrap(errs.CodeInternalError, err, "identity: generated token")
	}
	r := APITokenRecord{
		ID: ids.New(), PrincipalID: cmd.PrincipalID, Name: cmd.Name, Prefix: p.Prefix(), Hash: apitoken.Hash(s.pepper, secret),
		Scopes: slices.Clone(cmd.Scopes), DPoPJKT: cmd.DPoPJKT, ExpiresAt: expires,
		CreatedBy: cmd.CreatedBy, CreatedAt: now,
	}
	if err := s.d.Store.InsertAPIToken(ctx, r); err != nil {
		return identity.IssuedToken{}, err
	}
	return identity.IssuedToken{Token: tokenView(r), Secret: identity.NewToken(secret)}, nil
}

// tokenView converts a record into the port DTO; the kind is the one of its prefix.
func tokenView(r APITokenRecord) identity.APIToken {
	v := identity.APIToken{
		ID: r.ID, PrincipalID: r.PrincipalID, Name: r.Name, TokenPrefix: r.Prefix, Scopes: slices.Clone(r.Scopes), DPoPJKT: r.DPoPJKT,
		ExpiresAt: r.ExpiresAt, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt,
	}
	if n := len(apitoken.Prefix); len(r.Prefix) > n+3 {
		v.Kind = identity.TokenKind(r.Prefix[n : n+3])
	}
	return v
}

// tokensList is the list name of the token cursors.
const tokensList = "api_tokens"

// tokenCursor is the keyset position of a token page: created_at and id of its last row.
type tokenCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        ids.ID    `json:"id"`
}

// List implements identity.Tokens; the cursor is bound to the listed principal.
func (s *Tokens) List(ctx context.Context, principalID ids.ID, p identity.Page) (identity.TokenPage, error) {
	return s.list(ctx, []byte("principal="+principalID.String()+"\x00sort=created_at_desc"), principalID, p)
}

// list reads one page of the active tokens of principalID with cursors bound to binding.
func (s *Tokens) list(ctx context.Context, binding []byte, principalID ids.ID, p identity.Page) (identity.TokenPage, error) {
	var after tokenCursor
	if p.Cursor != "" {
		if err := s.d.Cursors.Decode(tokensList, binding, p.Cursor, &after); err != nil {
			return identity.TokenPage{}, err
		}
		if after.ID.IsZero() || after.CreatedAt.IsZero() {
			return identity.TokenPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.d.Store.ActiveAPITokens(ctx, principalID, after.CreatedAt, after.ID, limit+1)
	if err != nil {
		return identity.TokenPage{}, err
	}
	var out identity.TokenPage
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1]
		next, err := s.d.Cursors.Encode(tokensList, binding, tokenCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return identity.TokenPage{}, errs.Wrap(errs.CodeInternalError, err, "identity: token cursor")
		}
		out.NextCursor = next
	}
	for _, r := range rows {
		out.Items = append(out.Items, tokenView(r))
	}
	return out, nil
}

// tokenChange declares the rights change of token.revoke for the token's principal (05: token.revoke
// changes rights; the M1-29 path raises permission_version and appends access.changed).
func tokenChange(tokenID, principalID ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(authorize.ActionTokenRevoke), Resource: txn.Ref{Kind: resourceAPIToken, ID: tokenID}, Principals: []ids.ID{principalID},
	})
}

// errNoRightsChange is the misuse of a revocation inside a unit of work that does not change
// rights: authz would keep the token in its snapshot cache until the next rights version.
func errNoRightsChange() error {
	return errs.New(errs.CodeInternalError, "identity: revoking API tokens needs a unit of work declared as a rights change")
}

// Revoke implements identity.Tokens.
func (s *Tokens) Revoke(ctx context.Context, tokenID, by ids.ID) error {
	if txn.InUnit(ctx) {
		if !txn.InRightsChange(ctx) {
			return errNoRightsChange()
		}
		_, err := s.revoke(ctx, tokenID, by)
		return err
	}
	r, found, err := s.d.Store.APIToken(ctx, tokenID)
	if err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		_, err := s.revoke(ctx, tokenID, by)
		return err
	}, tokenChange(tokenID, r.PrincipalID))
}

// revoke locks the token and revokes it when it is active; changed reports a revocation.
func (s *Tokens) revoke(ctx context.Context, tokenID, by ids.ID) (APITokenRecord, error) {
	r, found, err := s.d.Store.LockAPIToken(ctx, tokenID)
	if err != nil {
		return APITokenRecord{}, err
	}
	if !found {
		return APITokenRecord{}, invisible()
	}
	if !r.RevokedAt.IsZero() {
		return APITokenRecord{}, nil
	}
	now := s.now()
	changed, err := s.d.Store.RevokeAPIToken(ctx, tokenID, by, now)
	if err != nil || !changed {
		return APITokenRecord{}, err
	}
	r.RevokedAt, r.RevokedBy = now, by
	return r, nil
}

// RevokeAllForPrincipal implements identity.Tokens.
func (s *Tokens) RevokeAllForPrincipal(ctx context.Context, principalID, by ids.ID) (int, error) {
	if !txn.InRightsChange(ctx) {
		return 0, errNoRightsChange()
	}
	return s.d.Store.RevokeAPITokensOfPrincipal(ctx, principalID, by, s.now())
}

// Audit values of token.create and token.revoke (05 section 6, SEC-AUD-5): ids, codes and times,
// never the secret or the hash.
const (
	resourceAPIToken = "api_token"
	attrTokenID      = "token_id"
	attrKind         = "kind"
	attrPrincipalID  = "principal_id"
	attrDPoPBound    = "dpop_bound"
	attrExpiresAt    = "expires_at"
)

// reasonAppNotInstalled is the reason of createToken for an app until M2-24 (08 section 12).
const reasonAppNotInstalled = "app_not_installed"

// principalResource describes the principal id: kind principal, owned by itself.
func principalResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourcePrincipal, ID: id, OwnerID: id}
}

// authorizeOn authorizes act on r for a and returns the decision as an error.
func (s *Tokens) authorizeOn(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// CreateToken implements identity.TokenAdmin (brief M1-44 "createToken"): authorize token.create
// on the principal (with step-up and the not_self guard), then 404 for an unknown principal,
// 409 app_not_installed for an app until M2-24, 403 feature_disabled for a personal token while
// the switch is off, the body rules, the insert and the allow entry in one unit of work. The idempotency
// stage stores no part of the answer (the handler marks the one-time secret).
func (s *Tokens) CreateToken(ctx context.Context, a actor.Actor, principalID ids.ID, cmd identity.CreateTokenCmd) (identity.IssuedToken, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.IssuedToken{}, err
	}
	var out identity.IssuedToken
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		rec, found, err := s.d.Principals.Principal(ctx, principalID)
		if err != nil {
			return err
		}
		if err := s.authorizeOn(ctx, a, authorize.ActionTokenCreate, principalResource(principalID)); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		k, ok := kindOf(rec.Principal.Kind)
		if !ok {
			return errs.New(errs.CodeInternalError, "identity: principal of an unknown kind")
		}
		if err := s.admitKind(ctx, a, k); err != nil {
			return err
		}
		out, err = s.issue(ctx, k, identity.IssueTokenCmd{
			PrincipalID: principalID, Name: cmd.Name, Scopes: cmd.Scopes, DPoPJKT: cmd.DPoPJKT, ExpiresAt: cmd.ExpiresAt, CreatedBy: a.ID,
		})
		if err != nil {
			return err
		}
		t := out.Token
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionTokenCreate), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourcePrincipal),
			ResourceID: principalID, Attrs: auditing.Attrs{
				attrTokenID: auditing.ID(t.ID), attrKind: auditing.Code(string(t.Kind)), attrPrincipalID: auditing.ID(principalID),
				attrDPoPBound: auditing.Bool(t.DPoPJKT != ""), attrExpiresAt: auditing.Time(t.ExpiresAt),
			},
		})
	})
	if err != nil {
		return identity.IssuedToken{}, err
	}
	return out, nil
}

// admitKind applies the rules of the token kind before the body: apps need an installation
// (M2-24), personal tokens the switch tokens.personal_enabled of the caller.
func (s *Tokens) admitKind(ctx context.Context, a actor.Actor, k tokenKind) error {
	switch k.kind {
	case identity.TokenKindApp:
		return errs.New(errs.CodeConflict, "App tokens are issued with an installation.").WithReason(reasonAppNotInstalled)
	case identity.TokenKindPersonal:
		on, err := s.d.Switch.PersonalTokensEnabled(ctx, a.ID)
		if err != nil {
			return err
		}
		if !on {
			return errs.New(errs.CodeFeatureDisabled, "Personal API tokens are switched off.")
		}
	case identity.TokenKindAgent:
	}
	return nil
}

// ListTokens implements identity.TokenAdmin: token.list on the principal (owner, or the principal
// itself); not_self of another principal is the not_found of 05 rule 2, then an unknown principal
// is not_found. The cursor is bound to the caller and the principal.
func (s *Tokens) ListTokens(ctx context.Context, a actor.Actor, principalID ids.ID, p identity.Page) (identity.TokenPage, error) {
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionTokenList, principalResource(principalID))
	if err != nil {
		return identity.TokenPage{}, err
	}
	if d.Outcome == authorize.OutcomeForbidden && d.Reason == reasonNotSelf {
		return identity.TokenPage{}, invisible()
	}
	if err := d.Err(); err != nil {
		return identity.TokenPage{}, err
	}
	if _, found, err := s.d.Principals.Principal(ctx, principalID); err != nil || !found {
		return identity.TokenPage{}, orInvisible(err)
	}
	binding := []byte(a.ID.String() + "\x00principal=" + principalID.String() + "\x00sort=created_at_desc")
	return s.list(ctx, binding, principalID, p)
}

// RevokeToken implements identity.TokenAdmin (brief M1-44 "revokeToken"): a first unit loads the
// token and authorizes token.revoke on it (kind api_token, owner its principal), so a refused call
// never raises the rights version; an already revoked token ends here without a new entry. A
// second unit declared as the rights change locks and revokes the token and writes the allow
// entry; the rights hook raises permission_version and appends access.changed.
func (s *Tokens) RevokeToken(ctx context.Context, a actor.Actor, tokenID ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	var rec APITokenRecord
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		r, found, err := s.d.Store.APIToken(ctx, tokenID)
		if err != nil {
			return err
		}
		res := authorize.Resource{Kind: authorize.ResourceApiToken, ID: tokenID, OwnerID: r.PrincipalID}
		if err := s.authorizeOn(ctx, a, authorize.ActionTokenRevoke, res); err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		rec = r
		return nil
	})
	if err != nil || !rec.RevokedAt.IsZero() {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		r, err := s.revoke(ctx, tokenID, a.ID)
		if err != nil || r.ID.IsZero() {
			return err
		}
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionTokenRevoke), Decision: auditing.DecisionAllow, ResourceKind: resourceAPIToken, ResourceID: tokenID,
			Attrs: auditing.Attrs{attrPrincipalID: auditing.ID(r.PrincipalID), attrKind: auditing.Code(string(tokenView(r).Kind))},
		})
	}, tokenChange(tokenID, rec.PrincipalID))
}
