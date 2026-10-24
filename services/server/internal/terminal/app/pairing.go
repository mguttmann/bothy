// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/hmac"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The audit details of pairing (24 section 10.1, Design note 11 of M2-41). Fingerprints are codes
// in compact lower-case form; never the code, its HMAC, a key, a path or a working directory.
const (
	opPairingCode         = "pairing_code"
	opConfirm             = "confirm"
	attrFingerprint       = "fingerprint"
	attrOldFingerprint    = "old_fingerprint"
	attrKeyGeneration     = "key_generation"
	attrSessionsEnded     = "sessions_ended"
	attrResult            = "result"
	attrAttemptsRemaining = "attempts_remaining"
	attrLLMCredentials    = "llm_credentials"
	attrCredentialHolder  = "credential_holder_id" //nolint:gosec // G101: an audit attribute name, no secret
	attrDeclaredServer    = "declared_server_host"
	attrPlatform          = "platform"
	resultConfirmed       = "confirmed"
	resultMismatch        = "fingerprint_mismatch"
	resultMaxAttempts     = "fingerprint_max_attempts"
	pointerSessionUsers   = "/session_users/"
)

// auditFingerprint is the audit form of a fingerprint: compact and lower case (52 characters of
// [0-9a-z], a code of SEC-AUD-5).
func auditFingerprint(fp string) auditing.Value {
	return auditing.Code(strings.ToLower(domain.CompactFingerprint(fp)))
}

// unauthenticated is the one answer of every pairing code that does not work (TM-T20): the same
// body for a malformed, unknown, used, replaced or expired code.
func unauthenticated() error { return errs.New(errs.CodeUnauthenticated, "") }

// errCodeGone ends the unit of a redemption whose code a parallel request used or replaced.
var errCodeGone = errors.New("terminal: the pairing code no longer works")

// issueCode is the shared issuer of createHost and issueHostPairingCode (Design note 2 of M2-41):
// a new code of kind hpc from identity with its HMAC, the only form stored. It reads no database,
// so createHost calls it between its two units.
func (s *Service) issueCode(ctx context.Context) (identity.PairingCode, error) {
	return s.d.PairingCodes.Issue(ctx)
}

// IssuePairingCode implements terminal.Hosts (re-pairing, 24 section 3.3, Design note 11 of M2-41).
// A first unit authorizes host.manage on the host, checks the reason and refuses a retired host
// with 409 invalid_state_transition, so a refused call never raises the rights version; the code
// comes from identity outside any transaction; a second unit, declared as the rights change
// host.manage, locks the host, decides the effects again (RepairEffects), ends the sessions of an
// active host with host_rekeyed, stores the HMAC of the new code with fingerprint_failures 0,
// drops the stored keys (keeping key_fingerprint), clears a disabled reason, puts the host into
// pairing with the next version, records the allow entry with the old fingerprint and appends
// host.updated with key_changed exactly when keys were dropped. key_generation rises at the next
// host.pair, not here.
func (s *Service) IssuePairingCode(ctx context.Context, a actor.Actor, cmd terminal.IssuePairingCode) (terminal.PairingCodeIssued, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	var (
		reason string
		holder ids.ID
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionHostManage, hostResource(cmd.HostID)); err != nil {
			return err
		}
		var err error
		if reason, err = domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars); err != nil {
			return err
		}
		h, found, err := s.d.Store.Host(ctx, cmd.HostID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		holder = h.AssignedUserID
		_, err = domain.RepairEffects(h.State, h.DisabledReason)
		return err
	})
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	code, err := s.issueCode(ctx)
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	var out terminal.PairingCodeIssued
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		expires, err := s.repairWrite(ctx, cmd.HostID, code.Hash, reason)
		out = terminal.PairingCodeIssued{HostID: cmd.HostID, Code: terminal.NewSecret(code.Secret.Reveal()), ExpiresAt: expires}
		return err
	}, rightsChange(cmd.HostID, holder))
	if err != nil {
		return terminal.PairingCodeIssued{}, err
	}
	// The re-pairing dropped the keys: the old connections end after the commit, on every node (24
	// section 3.3, terminal.host_close of M2-47).
	s.committed(ctx, hostCloseSignal(cmd.HostID))
	return out, nil
}

// repairWrite is the second unit of IssuePairingCode, declared as the rights change. It returns
// the expiry of the code.
func (s *Service) repairWrite(ctx context.Context, hostID ids.ID, hash []byte, reason string) (time.Time, error) {
	h, found, err := s.d.Store.LockHost(ctx, hostID)
	if err != nil {
		return time.Time{}, err
	}
	if !found {
		return time.Time{}, invisible()
	}
	plan, err := domain.RepairEffects(h.State, h.DisabledReason)
	if err != nil {
		return time.Time{}, err
	}
	ended := 0
	if plan.EndSessions {
		if ended, err = s.EndSessions(ctx, SessionSelector{HostID: h.ID}, domain.ServerEnd(domain.CloseHostRekeyed)); err != nil {
			return time.Time{}, err
		}
	}
	t := s.d.Clock.Now()
	expires := t.Add(s.cfg.PairingCodeTTL)
	if err := s.d.Store.SetPairingCode(ctx, h.ID, hash, expires, plan.DropKeys, h.Version+1); err != nil {
		return time.Time{}, err
	}
	if err := s.stage(ctx, hostCloseSignal(h.ID)); err != nil {
		return time.Time{}, err
	}
	attrs := auditing.Attrs{
		attrOperation: auditing.Code(opPairingCode), attrReason: auditing.ReasonText(reason),
		attrFromState: auditing.Code(string(h.State)), attrToState: auditing.Code(string(plan.Result.To)),
		attrKeyGeneration: auditing.Int(h.KeyGeneration), attrSessionsEnded: auditing.Int(int64(ended)),
	}
	if plan.DropKeys && h.KeyFingerprint != "" {
		attrs[attrOldFingerprint] = auditFingerprint(h.KeyFingerprint)
	}
	if err := s.record(ctx, h.ID, attrs); err != nil {
		return time.Time{}, err
	}
	st, found, err := s.load(ctx, h.ID)
	if err != nil {
		return time.Time{}, err
	}
	if !found {
		return time.Time{}, invisible()
	}
	return st.host.PairingExpiresAt, s.emitHostUpdated(ctx, s.view(ctx, st), plan.DropKeys)
}

// requestPrincipal is the principal of a public request: the actor of the chain, or anonymous.
func requestPrincipal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// pairDenial aggregates the refusal of a pairing code (R-AZ-04: written outside the request, per
// IP pseudonym and audit.deny_aggregate_window) and returns the one 401 body.
func (s *Service) pairDenial(ctx context.Context, hostID ids.ID) error {
	e := auditing.Entry{Action: string(authorize.ActionHostPair), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated}
	if !hostID.IsZero() {
		e.ResourceKind, e.ResourceID = resourceHost, hostID
	}
	if err := s.d.Audit.RecordDenial(ctx, e); err != nil {
		return err
	}
	return unauthenticated()
}

// Pair implements terminal.Hosts (host.pair, 24 section 3.2, Design note 3 of M2-41), in this
// order: the HMAC of the code (a malformed code is 401 without any database access); the keys
// and the report (400, the code stays valid); the host of the hash without a lock (none, expired
// or not pairing: 401); one unit, declared as the rights change host.pair (03 counts
// reported_session_users as rights state), locks the host and checks hash, state and expiry
// again (a parallel redemption won: 401), authorizes host.pair (Public: the code was the proof),
// refuses the dropped key pair of this host (409 host_key_reused), stores the pairing (409
// host_key_in_use for a login key of another host), records the allow entry and appends
// host.updated. A retry after a lost answer gets 401, because the code is used (D-T37).
func (s *Service) Pair(ctx context.Context, cmd terminal.PairHost) (terminal.PairResult, error) {
	hash, err := s.d.PairingCodes.Hash(ctx, cmd.Code.Reveal())
	if errors.Is(err, identity.ErrTokenMalformed) {
		return terminal.PairResult{}, s.pairDenial(ctx, ids.ID{})
	}
	if err != nil {
		return terminal.PairResult{}, err
	}
	keys, findings := domain.ParseHostKeys(cmd.NoisePublicKey, cmd.AuthPublicJWK)
	report := pairReport(cmd)
	findings = append(findings, domain.ValidatePairReport(report, domain.ReportLimits{
		OSUserMaxChars: s.cfg.OSUserMaxChars, LabelMaxChars: s.cfg.ReportLabelMaxChars, MaxEntries: s.cfg.ReportMaxEntries,
	})...)
	if len(findings) > 0 {
		return terminal.PairResult{}, errs.New(errs.CodeValidationFailed, "").WithFields(findings...)
	}
	h, found, err := s.d.Store.HostByPairingHash(ctx, hash)
	if err != nil {
		return terminal.PairResult{}, err
	}
	if !found {
		return terminal.PairResult{}, s.pairDenial(ctx, ids.ID{})
	}
	if !s.codeWorks(h, hash) {
		return terminal.PairResult{}, s.pairDenial(ctx, h.ID)
	}
	assigned, err := s.d.Store.ValidSessionUsers(ctx, []ids.ID{h.ID})
	if err != nil {
		return terminal.PairResult{}, err
	}
	var people []ids.ID
	for _, u := range assigned[h.ID] {
		people = append(people, u.UserID)
	}
	ref := txn.Ref{Kind: resourceHost, ID: h.ID}
	p := requestPrincipal(ctx)
	ctx = actor.WithActor(ctx, p)
	var fp string
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		fp, err = s.pairWrite(ctx, p, h.ID, hash, keys, report)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionHostPair), Resource: ref, Principals: people, Targets: []txn.Ref{ref}}))
	if errors.Is(err, errCodeGone) {
		return terminal.PairResult{}, s.pairDenial(ctx, h.ID)
	}
	if err != nil {
		return terminal.PairResult{}, err
	}
	instanceID, err := s.d.Instance.InstanceID(ctx)
	if err != nil {
		return terminal.PairResult{}, err
	}
	return terminal.PairResult{HostID: h.ID, KeyFingerprint: fp, InstanceID: instanceID}, nil
}

// codeWorks reports a host in pairing whose stored hash is hash and whose code has not expired.
func (s *Service) codeWorks(h domain.Host, hash []byte) bool {
	return h.State == domain.StatePairing && hmac.Equal(h.PairingCodeHash, hash) && h.PairingExpiresAt.After(s.d.Clock.Now())
}

// pairReport converts the report of the body.
func pairReport(cmd terminal.PairHost) domain.PairReport {
	r := domain.PairReport{Platform: cmd.Platform, OSVersion: cmd.OSVersion, HostVersion: cmd.HostVersion, Profiles: []domain.Profile{}}
	for _, p := range cmd.Profiles {
		r.Profiles = append(r.Profiles, domain.Profile{Name: p.Name, DisplayName: p.DisplayName, Kind: p.Kind})
	}
	for _, u := range cmd.SessionUsers {
		dirs := u.Workdirs
		if dirs == nil {
			dirs = []string{}
		}
		r.SessionUsers = append(r.SessionUsers, domain.ReportedSessionUser{OSUser: u.OSUser, Workdirs: dirs})
	}
	return r
}

// pairWrite is the unit of Pair, declared as the rights change host.pair. It returns the
// fingerprint.
func (s *Service) pairWrite(ctx context.Context, p actor.Actor, hostID ids.ID, hash []byte, keys domain.HostKeys, report domain.PairReport) (string, error) {
	h, found, err := s.d.Store.LockHost(ctx, hostID)
	if err != nil {
		return "", err
	}
	if !found || !s.codeWorks(h, hash) {
		return "", errCodeGone
	}
	if err := s.check(ctx, p, authorize.ActionHostPair, hostResource(h.ID)); err != nil {
		return "", err
	}
	fp, err := domain.ComputeFingerprint(keys.Noise, keys.AuthRaw)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "terminal: fingerprint of checked keys")
	}
	if h.KeyFingerprint != "" && h.KeyFingerprint == fp {
		return "", errs.New(errs.CodeConflict, "The host sent its dropped keys again.").WithReason(domain.ReasonHostKeyReused)
	}
	if _, err := domain.Transition(h.State, h.DisabledReason, domain.CausePair); err != nil {
		return "", err
	}
	t := s.d.Clock.Now()
	if err := s.d.Store.StorePairing(ctx, h.ID, PairingWrite{Keys: keys, Fingerprint: fp, PairedAt: t, Report: report}, h.Version+1); err != nil {
		return "", err
	}
	attrs := auditing.Attrs{
		attrFingerprint: auditFingerprint(fp), attrKeyGeneration: auditing.Int(h.KeyGeneration + 1), attrPlatform: auditing.Code(report.Platform),
	}
	if h.KeyFingerprint != "" {
		attrs[attrOldFingerprint] = auditFingerprint(h.KeyFingerprint)
	}
	if err := s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionHostPair), Decision: auditing.DecisionAllow, ResourceKind: resourceHost, ResourceID: h.ID, Attrs: attrs,
	}); err != nil {
		return "", err
	}
	st, _, err := s.load(ctx, h.ID)
	if err != nil {
		return "", err
	}
	return fp, s.emitUpdated(ctx, s.view(ctx, st))
}

// confirmBody is the checked body of confirmHost without the database.
type confirmBody struct {
	prefix, reason string
	decl           domain.Declaration
	add            []domain.SessionUserAssign
}

// checkConfirm checks the body of confirmHost without the database (Design note 7c of M2-41): the
// reason, the prefix (a malformed prefix is no attempt), the declaration and the names of the
// session users.
func (s *Service) checkConfirm(cmd terminal.ConfirmHost) (confirmBody, error) {
	var b confirmBody
	var err error
	if b.reason, err = domain.CheckReason(cmd.Reason, s.cfg.ReasonMaxChars); err != nil {
		return confirmBody{}, err
	}
	if b.prefix, err = domain.NormalizeFingerprintPrefix(cmd.FingerprintPrefix, s.cfg.FingerprintConfirmChars); err != nil {
		return confirmBody{}, err
	}
	b.decl = domain.Declaration{Credentials: cmd.LLMCredentials, HolderID: cmd.CredentialHolderID}
	if err := domain.ValidateDeclaration(b.decl); err != nil {
		return confirmBody{}, err
	}
	for i, u := range cmd.SessionUsers {
		name, err := domain.NewOSUser(u.OSUser, s.cfg.OSUserMaxChars, pointerSessionUsers+strconv.Itoa(i)+"/os_user")
		if err != nil {
			return confirmBody{}, err
		}
		b.add = append(b.add, domain.SessionUserAssign{UserID: u.UserID, OSUser: name})
	}
	return b, nil
}

// confirmOutcome is what the unit of Confirm decided: the host after a confirmation, or the
// remaining attempts of a mismatch, whose effect commits before the error is returned.
type confirmOutcome struct {
	view      terminal.HostView
	mismatch  bool
	remaining int
}

// Confirm implements terminal.Hosts (confirmHost, 24 section 3.2, Design note 7 of M2-41). A first
// unit authorizes host.manage (404, owner_only, step-up), checks the body without the database
// (a malformed prefix is 400 and no attempt), If-Match and the state confirming. A second unit,
// declared as the rights change host.manage with the people of the request and the holders, locks
// the host, checks the version and the state again and compares the prefix first, so a wrong host
// never reaches an assignment or a declaration. A mismatch counts (NextAttempt) without a new
// version and commits with its audit entry; the mismatch that reaches
// terminal.fingerprint_max_attempts falls back to pairing (fallback). Then the unit returns
// without an error and Confirm returns 409 fingerprint_mismatch with attempts_remaining; under an
// Idempotency-Key the handler marks the answer with idempotency.KeepEffectOnFinal, so the stage
// commits it too. A match checks the people, assigns the session users (PlanAssignmentsAt, every
// 400 before any 409), declares the LLM login (ApplyLlmDeclaration; I6 runs in phase 3.1),
// records declared_server_host and the confirmation, puts the host into active, records the allow
// entry and appends host.updated; any failure rolls all of it back.
func (s *Service) Confirm(ctx context.Context, a actor.Actor, cmd terminal.ConfirmHost) (terminal.HostView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostView{}, err
	}
	var (
		b      confirmBody
		people []ids.ID
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionHostManage, hostResource(cmd.HostID)); err != nil {
			return err
		}
		var err error
		if b, err = s.checkConfirm(cmd); err != nil {
			return err
		}
		st, err := s.loadFor(ctx, cmd.HostID, cmd.IfMatch)
		if err != nil {
			return err
		}
		if st.host.State != domain.StateConfirming {
			return domain.ErrInvalidTransition
		}
		people = []ids.ID{st.host.AssignedUserID, declarationOf(st.host).HolderID, b.decl.HolderID}
		for _, x := range b.add {
			people = append(people, x.UserID)
		}
		return nil
	})
	if err != nil {
		return terminal.HostView{}, err
	}
	var out confirmOutcome
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.confirmWrite(ctx, a, cmd, b)
		return err
	}, rightsChange(cmd.HostID, people...))
	if err != nil {
		return terminal.HostView{}, err
	}
	return s.afterConfirm(ctx, cmd.HostID, out)
}

// afterConfirm tells the control connection of the host what the committed confirmation did
// (M2-43): the fallback to pairing dropped the keys, so the connection ends with 4110; a
// confirmation makes the host active, so its connection gets ready again with the state and the
// declaration (M2-42 Design note 15). A mismatch answers 409 fingerprint_mismatch.
func (s *Service) afterConfirm(ctx context.Context, hostID ids.ID, out confirmOutcome) (terminal.HostView, error) {
	if !out.mismatch {
		s.sendReady(ctx, hostID)
		return out.view, nil
	}
	if out.remaining == 0 {
		s.committed(ctx, hostCloseSignal(hostID))
	}
	return terminal.HostView{}, errs.FingerprintMismatch(out.remaining)
}

// confirmWrite is the second unit of Confirm, declared as the rights change.
func (s *Service) confirmWrite(ctx context.Context, a actor.Actor, cmd terminal.ConfirmHost, b confirmBody) (confirmOutcome, error) {
	h, err := s.lockFor(ctx, cmd.HostID, cmd.IfMatch)
	if err != nil {
		return confirmOutcome{}, err
	}
	if h.State != domain.StateConfirming {
		return confirmOutcome{}, domain.ErrInvalidTransition
	}
	attempt := domain.NextAttempt(h.FingerprintFailures, s.cfg.FingerprintMaxAttempts, domain.PrefixMatches(h.KeyFingerprint, b.prefix))
	if !attempt.Match {
		return s.mismatch(ctx, h, attempt, b.reason)
	}
	plan, err := s.confirmPlan(ctx, h, b)
	if err != nil {
		return confirmOutcome{}, err
	}
	t := s.d.Clock.Now()
	if err := s.applyPlan(ctx, a, h.ID, plan, b.reason, t); err != nil {
		return confirmOutcome{}, err
	}
	next, _, err := s.ApplyLlmDeclaration(ctx, h, b.decl, a.ID, t)
	if err != nil {
		return confirmOutcome{}, err
	}
	if _, err := domain.Transition(h.State, h.DisabledReason, domain.CauseConfirm); err != nil {
		return confirmOutcome{}, err
	}
	if err := s.d.Store.ConfirmPairing(ctx, h.ID, cmd.DeclaredServerHost, a.ID, t, next.Version); err != nil {
		return confirmOutcome{}, err
	}
	attrs := confirmAttrs(h, resultConfirmed, b.reason)
	attrs[attrLLMCredentials] = auditing.Code(b.decl.Credentials)
	attrs[attrDeclaredServer] = auditing.Bool(cmd.DeclaredServerHost)
	if !b.decl.HolderID.IsZero() {
		attrs[attrCredentialHolder] = auditing.ID(b.decl.HolderID)
	}
	if err := s.record(ctx, h.ID, attrs); err != nil {
		return confirmOutcome{}, err
	}
	st, _, err := s.load(ctx, h.ID)
	if err != nil {
		return confirmOutcome{}, err
	}
	v := s.view(ctx, st)
	return confirmOutcome{view: v}, s.emitUpdated(ctx, v)
}

// confirmPlan checks the people of the body (unknown_user) and plans the assignments against the
// locked session user rows of h (every 400 before any 409); it runs only after the prefix matched.
func (s *Service) confirmPlan(ctx context.Context, h domain.Host, b confirmBody) (domain.Plan, error) {
	for i, x := range b.add {
		if err := s.person(ctx, x.UserID, pointerSessionUsers+strconv.Itoa(i)+"/user_id"); err != nil {
			return domain.Plan{}, err
		}
	}
	if !b.decl.HolderID.IsZero() {
		if err := s.person(ctx, b.decl.HolderID, "/credential_holder_id"); err != nil {
			return domain.Plan{}, err
		}
	}
	rows, err := s.d.Store.LockSessionUsers(ctx, h.ID)
	if err != nil {
		return domain.Plan{}, err
	}
	return domain.PlanAssignmentsAt(pointerSessionUsers, domain.ReportedNames(h.ReportedUsers), rows, nil, b.add)
}

// confirmAttrs are the common details of the allow entry confirm (24 section 10.1).
func confirmAttrs(h domain.Host, result, reason string) auditing.Attrs {
	return auditing.Attrs{
		attrOperation: auditing.Code(opConfirm), attrReason: auditing.ReasonText(reason), attrResult: auditing.Code(result),
		attrFingerprint: auditFingerprint(h.KeyFingerprint), attrKeyGeneration: auditing.Int(h.KeyGeneration),
	}
}

// mismatch counts a failed confirmation (TM-T19): the counter rises without a new version; at
// terminal.fingerprint_max_attempts the host falls back to pairing (fallback). Both commit with
// their audit entry.
func (s *Service) mismatch(ctx context.Context, h domain.Host, attempt domain.AttemptOutcome, reason string) (confirmOutcome, error) {
	if attempt.Fallback {
		return s.fallback(ctx, h, reason)
	}
	if err := s.d.Store.SetFingerprintFailures(ctx, h.ID, attempt.Failures); err != nil {
		return confirmOutcome{}, err
	}
	attrs := confirmAttrs(h, resultMismatch, reason)
	attrs[attrAttemptsRemaining] = auditing.Int(int64(attempt.Remaining))
	if err := s.record(ctx, h.ID, attrs); err != nil {
		return confirmOutcome{}, err
	}
	return confirmOutcome{mismatch: true, remaining: attempt.Remaining}, nil
}

// fallback puts a host whose confirmation failed terminal.fingerprint_max_attempts times back into
// pairing (Design note 8 of M2-41): the keys dropped, key_fingerprint kept, the hash of a fresh
// code that is never shown, logged or returned with pairing_expires_at now (03 demands a hash in
// pairing, and no code may work until the owner issues one), the counter at 0, the next version,
// host.updated with key_changed and attempts_remaining 0.
func (s *Service) fallback(ctx context.Context, h domain.Host, reason string) (confirmOutcome, error) {
	r, err := domain.Transition(h.State, h.DisabledReason, domain.CauseFingerprintAttemptsExhausted)
	if err != nil {
		return confirmOutcome{}, err
	}
	code, err := s.issueCode(ctx)
	if err != nil {
		return confirmOutcome{}, err
	}
	if err := s.d.Store.SetPairingCode(ctx, h.ID, code.Hash, s.d.Clock.Now(), true, h.Version+1); err != nil {
		return confirmOutcome{}, err
	}
	if err := s.stage(ctx, hostCloseSignal(h.ID)); err != nil {
		return confirmOutcome{}, err
	}
	attrs := confirmAttrs(h, resultMaxAttempts, reason)
	attrs[attrAttemptsRemaining] = auditing.Int(0)
	attrs[attrToState] = auditing.Code(string(r.To))
	if err := s.record(ctx, h.ID, attrs); err != nil {
		return confirmOutcome{}, err
	}
	st, _, err := s.load(ctx, h.ID)
	if err != nil {
		return confirmOutcome{}, err
	}
	return confirmOutcome{mismatch: true}, s.emitHostUpdated(ctx, s.view(ctx, st), true)
}
