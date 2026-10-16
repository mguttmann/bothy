// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// AuthorizerConfig holds the configuration keys of the Authorizer.
type AuthorizerConfig struct {
	// SnapshotCacheMaxEntries bounds the snapshot LRU (authz.snapshot_cache_max_entries).
	SnapshotCacheMaxEntries int `config:"authz.snapshot_cache_max_entries"`
	// StepUpMaxAge is how long a step-up is fresh (auth.step_up_max_age).
	StepUpMaxAge time.Duration `config:"auth.step_up_max_age"`
	// ViewAsMaxDuration is how long "view as" lasts (authz.view_as_max_duration, M1-49): from
	// actor.ViewAs.StartedAt plus this duration on, every decision is step_up_required.
	ViewAsMaxDuration time.Duration `config:"authz.view_as_max_duration"`
	// TerminalStepUpMaxAge is how young the step-up "frisch" of terminal.open and terminal.attach
	// must be (terminal.step_up_max_age, M2-40).
	TerminalStepUpMaxAge time.Duration `config:"terminal.step_up_max_age"`
	// QuickReattachWindow is how long after the detachment terminal.attach from the same device
	// and login session needs no new step-up (terminal.quick_reattach_window, M2-40).
	QuickReattachWindow time.Duration `config:"terminal.quick_reattach_window"`
}

// AuthorizerDeps are the collaborators of the Authorizer. Facts owned by other modules come only
// through the source ports (02 section 4.1 rule 4).
type AuthorizerDeps struct {
	// Catalog is the closed action catalogue; every call starts with its lookup (AZ-P3).
	Catalog *Catalog
	// Store reads grants, groups and memberships.
	Store Store
	// Principals, Credentials, Structure and Policies are the source ports.
	Principals  authz.PrincipalSource
	Credentials authz.CredentialSource
	Structure   authz.StructureSource
	Policies    authz.AgentPolicySource
	// Hosts is the host source of the terminal module (M2-39); nil means a world without hosts,
	// in which every host is not found.
	Hosts authz.HostSource
	// Versions reads the rights version.
	Versions instance.Versions
	// Recorder receives every denial and error of a request or op (R-AZ-04).
	Recorder auditing.Recorder
	// Clock is read once per call.
	Clock clock.Clock
	// Log receives programming errors and recorder failures; nil means slog.Default().
	Log *slog.Logger
}

// Authorizer implements kernel/authorize.Authorizer: Authorize decides as docs/spec/07-rechte.md
// section 4.2 describes from a rights snapshot per principal, cached per rights version and valid
// until the earliest expiry; Scope gives list queries the visible sets. It is safe for concurrent
// use.
type Authorizer struct {
	cfg       AuthorizerConfig
	catalog   *Catalog
	store     Store
	principal authz.PrincipalSource
	creds     authz.CredentialSource
	structure authz.StructureSource
	policies  authz.AgentPolicySource
	hosts     authz.HostSource
	versions  instance.Versions
	recorder  auditing.Recorder
	clock     clock.Clock
	log       *slog.Logger
	cache     *snapshotCache
}

// Compile-time check: the Authorizer is the kernel port.
var _ authorize.Authorizer = (*Authorizer)(nil)

// errMissingDep reports a nil collaborator.
var errMissingDep = errors.New("authz: NewAuthorizer needs every dependency")

// NewAuthorizer returns the Authorizer. Errors: a nil dependency other than Log, a cache bound
// below 1 or a negative step-up age.
//
// Example:
//
//	az, err := app.NewAuthorizer(cfg, app.AuthorizerDeps{Catalog: cat, Store: store, ...})
//	d, err := az.Authorize(ctx, actor, authorize.ActionMessageRead, res)
func NewAuthorizer(cfg AuthorizerConfig, d AuthorizerDeps) (*Authorizer, error) {
	if d.Catalog == nil || d.Store == nil || d.Principals == nil || d.Credentials == nil || d.Structure == nil ||
		d.Policies == nil || d.Versions == nil || d.Recorder == nil || d.Clock == nil {
		return nil, errMissingDep
	}
	if cfg.SnapshotCacheMaxEntries < 1 {
		return nil, errors.New("authz: authz.snapshot_cache_max_entries must be at least 1")
	}
	if cfg.StepUpMaxAge < 0 {
		return nil, errors.New("authz: auth.step_up_max_age must not be negative")
	}
	if cfg.ViewAsMaxDuration < 0 {
		return nil, errors.New("authz: authz.view_as_max_duration must not be negative")
	}
	if cfg.TerminalStepUpMaxAge < 0 || cfg.QuickReattachWindow < 0 {
		return nil, errors.New("authz: terminal.step_up_max_age and terminal.quick_reattach_window must not be negative")
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Authorizer{
		cfg: cfg, catalog: d.Catalog, store: d.Store, principal: d.Principals, creds: d.Credentials,
		structure: d.Structure, policies: d.Policies, hosts: hostsOrNone(d.Hosts), versions: d.Versions, recorder: d.Recorder,
		clock: d.Clock, log: log, cache: newSnapshotCache(cfg.SnapshotCacheMaxEntries),
	}, nil
}

// Log message of a failed audit call; the decision is never changed by it.
const msgAuditFailed = "audit of an authorization decision failed"

// Authorize implements authorize.Authorizer (07 section 4.2). After deciding it records the check
// in the request check log (02 section 9.3) and, outside filter mode, hands every non-allow
// decision to the recorder as a denial outside the business transaction, aggregated per
// audit.deny_aggregate_window, and every error as decision error (R-AZ-04, AZ-P13).
func (a *Authorizer) Authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	d, err := a.authorize(ctx, p, act, r)
	if err != nil {
		d = authorize.Decision{}
	}
	authorize.RecordCheck(ctx, authorize.Check{Action: string(act), Kind: authorize.CheckAuthorize, Allowed: err == nil && d.Allowed()})
	a.audit(ctx, act, string(r.Kind), r.ID, d, err)
	return d, err
}

// audit hands a denial or an error to the recorder; allows and filter mode write nothing.
func (a *Authorizer) audit(ctx context.Context, act authorize.Action, kind string, id ids.ID, d authorize.Decision, failure error) {
	if authorize.IsFilter(ctx) || (failure == nil && d.Allowed()) {
		return
	}
	e := auditing.Entry{Action: string(act), ResourceKind: kind, ResourceID: id}
	var err error
	if failure != nil {
		e.Decision = auditing.DecisionError
		if fe, ok := errs.As(failure); ok && fe.Cause == errs.CauseUnknownAction {
			e.Reason = auditing.ReasonUnknownAction
		}
		err = a.recorder.RecordDetached(ctx, e)
	} else {
		e.Decision, e.Reason = auditing.DecisionDeny, auditing.Reason(d.Reason)
		err = a.recorder.RecordDenial(ctx, e)
	}
	if err != nil {
		a.log.LogAttrs(ctx, slog.LevelError, msgAuditFailed, slog.String(keyAction, string(act)),
			slog.String(keyErrorType, "audit"))
	}
}

// outcome converts a domain decision.
func outcome(d domain.Decision) authorize.Decision {
	return authorize.Decision{Outcome: authorize.Outcome(d.Outcome), Reason: authorize.Reason(d.Reason), QuickReattach: d.QuickReattach}
}

// internal returns an internal error for a programming error of the caller.
func internal(detail string) error {
	return errs.New(errs.CodeInternalError, "authz: "+detail)
}

// sourceError wraps an error of a source port; it is never an allow.
func sourceError(what string, err error) error {
	if _, ok := errs.As(err); ok {
		return fmt.Errorf("authz: %s: %w", what, err)
	}
	return errs.Wrap(errs.CodeUnavailable, err, "authz: "+what).WithCause(errs.CauseDependencyUnavailable)
}

// requirement returns spec and requirement of act, or the error of an unknown, deferred or system
// action.
func (a *Authorizer) requirement(act authorize.Action) (domain.ActionSpec, domain.Expr, error) {
	spec, err := a.catalog.Spec(act)
	if err != nil {
		return domain.ActionSpec{}, domain.Expr{}, err
	}
	req, ok := domain.RequirementOf(spec.ID)
	if !ok {
		return domain.ActionSpec{}, domain.Expr{}, a.catalog.Reject(act)
	}
	if _, deferred := req.Expr.DeferredTo(); deferred {
		return domain.ActionSpec{}, domain.Expr{}, a.catalog.Reject(act)
	}
	if req.Expr.IsSystem() {
		return domain.ActionSpec{}, domain.Expr{}, internal("system action " + spec.ID + " is not reachable through Authorize")
	}
	return spec, req.Expr, nil
}

// actingPrincipal is the request principal with its snapshot entry and the snapshots of "view as"
// and acting for.
type actingPrincipal struct {
	id        ids.ID
	kind      string
	ref       authz.CredentialRef
	p         authorize.Principal
	entry     entry
	viewAs    *entry
	actingFor *entry
}

// rights returns the final rights on path (after "view as" and acting for) and the base rights
// before the acting-for intersection.
func (s *actingPrincipal) rights(path domain.Path) (final, base domain.Rights) {
	base = domain.Effective(s.entry.snap, s.kind, s.id, path)
	if s.viewAs != nil {
		base = base.Intersect(domain.Effective(s.viewAs.snap, domain.KindUser, s.viewAs.key.principal, path))
	}
	final = base
	if s.actingFor != nil {
		final = final.Intersect(domain.Effective(s.actingFor.snap, domain.KindUser, s.actingFor.key.principal, path))
	}
	return final, base
}

// domainPrincipal is the acting principal as the gate of HumanSession sees it: kind and session.
func (s *actingPrincipal) domainPrincipal() domain.Principal {
	return domain.Principal{ID: s.id, Kind: s.kind, HasSession: !s.ref.SessionID.IsZero()}
}

// liveness returns the liveness facts of s.
func (s *actingPrincipal) liveness() domain.Liveness {
	l := domain.Liveness{
		HasSession: !s.ref.SessionID.IsZero(), HasDevice: !s.ref.DeviceID.IsZero(), HasToken: !s.ref.TokenID.IsZero(),
		IsApp: s.kind == domain.KindApp, HasInstallation: !s.p.InstallationRef().IsZero(),
		SessionActive: s.entry.creds.SessionActive, SessionExpiresAt: s.entry.creds.SessionExpiresAt,
		DeviceState: string(s.entry.creds.DeviceState), TokenActive: s.entry.creds.TokenActive,
		TokenExpiresAt: s.entry.creds.TokenExpiresAt,
	}
	if s.entry.found && string(s.entry.facts.Kind) == s.kind {
		l.Status = string(s.entry.facts.Status)
	}
	// The authenticator read session, device and key for this very request (M1-34): those facts
	// decide, because the cached ones belong to the rights version, which a logout or a refresh
	// reuse does not raise. A session never comes back, so the cached flag can only be stale in
	// the allowing direction; the device state may also have moved on (pending to active).
	if o, ok := s.p.(authorize.CredentialObserver); ok {
		if active, state, keyActive, observed := o.CredentialState(); observed {
			l.SessionActive = l.SessionActive && active
			l.DeviceState = state
			l.DeviceKeyRevoked = !keyActive
		}
	}
	return l
}

// principalOf loads the acting principal at version v and time t and checks liveness and the
// pending rule for act. A non-allow decision ends the call before any resource is read (SR7).
func (a *Authorizer) principalOf(ctx context.Context, p authorize.Principal, act string, v int64, t time.Time) (*actingPrincipal, domain.Decision, error) {
	s := &actingPrincipal{id: p.PrincipalID(), kind: p.PrincipalKind(), p: p}
	s.ref = authz.CredentialRef{PrincipalID: s.id, SessionID: p.SessionRef(), DeviceID: p.DeviceRef(), TokenID: p.TokenRef()}
	switch s.kind {
	case domain.KindUser, domain.KindAgent, domain.KindApp:
	default:
		return nil, domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}, nil
	}
	if s.id.IsZero() {
		return nil, domain.Decision{Outcome: domain.OutcomeUnauthenticated, Reason: domain.ReasonUnauthenticated}, nil
	}
	e, err := a.load(ctx, s.ref, v, t)
	if err != nil {
		return nil, domain.Decision{}, err
	}
	s.entry = e
	live := s.liveness()
	if d := domain.CheckLiveness(live, t); d.Outcome != domain.OutcomeAllow {
		return nil, d, nil
	}
	if d := domain.CheckPending(live, act); d.Outcome != domain.OutcomeAllow {
		return nil, d, nil
	}
	if id := p.ViewAsPerson(); !id.IsZero() {
		if a.viewAsEnded(p.ViewAsStart(), t) {
			// The view ended; a new one needs a fresh step-up (07 section 9.2, no own reason code).
			return nil, domain.Decision{Outcome: domain.OutcomeStepUpRequired, Reason: domain.ReasonStepUpRequired}, nil
		}
		ve, err := a.load(ctx, authz.CredentialRef{PrincipalID: id}, v, t)
		if err != nil {
			return nil, domain.Decision{}, err
		}
		s.viewAs = &ve
	}
	if id := p.ActingForPerson(); !id.IsZero() {
		ae, err := a.load(ctx, authz.CredentialRef{PrincipalID: id}, v, t)
		if err != nil {
			return nil, domain.Decision{}, err
		}
		s.actingFor = &ae
	}
	return s, domain.Allow, nil
}

// viewAsEnded reports whether a view started at start has ended at t: from start plus
// authz.view_as_max_duration on (07 section 9.2). A view without a start has ended (fail closed).
func (a *Authorizer) viewAsEnded(start, t time.Time) bool {
	return start.IsZero() || !t.Before(start.Add(a.cfg.ViewAsMaxDuration))
}

// load returns the snapshot entry of ref at version v, from the cache when it is valid at t.
// Inside a unit of work declared as a rights change the cache is neither read nor written: the
// unit sees its own uncommitted rights version and state, and a rollback would leave an entry for
// a version another change commits later (07 section 7.3, M1-29).
func (a *Authorizer) load(ctx context.Context, ref authz.CredentialRef, v int64, t time.Time) (entry, error) {
	key := cacheKey{principal: ref.PrincipalID, session: ref.SessionID, device: ref.DeviceID, token: ref.TokenID, version: v}
	if txn.InRightsChange(ctx) {
		return a.build(ctx, key, ref, t)
	}
	if e, ok := a.cache.get(key, t); ok {
		return e, nil
	}
	e, err := a.build(ctx, key, ref, t)
	if err != nil {
		return entry{}, err
	}
	a.cache.put(e)
	return e, nil
}

// build reads the facts of ref and builds its snapshot (07 section 7.2). Grants of a principal
// that is unknown or not active, and of an app without an active installation, are not read (SR7,
// 07 section 2.10).
func (a *Authorizer) build(ctx context.Context, key cacheKey, ref authz.CredentialRef, t time.Time) (entry, error) {
	e := entry{key: key}
	facts, found, err := a.principal.Principal(ctx, ref.PrincipalID)
	if err != nil {
		return entry{}, sourceError("principal source", err)
	}
	e.found, e.facts = found, facts
	if !ref.SessionID.IsZero() || !ref.DeviceID.IsZero() || !ref.TokenID.IsZero() {
		if e.creds, err = a.creds.Credentials(ctx, ref); err != nil {
			return entry{}, sourceError("credential source", err)
		}
	}
	in := domain.SnapshotInput{PrincipalID: ref.PrincipalID, Version: key.version, At: t}
	if !ref.SessionID.IsZero() {
		in.Expiries = append(in.Expiries, e.creds.SessionExpiresAt)
	}
	if !ref.TokenID.IsZero() {
		in.Expiries = append(in.Expiries, e.creds.TokenExpiresAt)
	}
	if !found || !facts.GrantsInEffect() {
		e.snap = domain.BuildSnapshot(in)
		return e, nil
	}
	if e.snap, err = a.buildLive(ctx, ref, facts, in, t); err != nil {
		return entry{}, err
	}
	return e, nil
}

// buildLive builds the snapshot of an active or pending principal: its grants and, for an active
// human, the terminal state (session users and personal hosts, M2-40).
func (a *Authorizer) buildLive(ctx context.Context, ref authz.CredentialRef, facts authz.PrincipalFacts, in domain.SnapshotInput, t time.Time) (domain.Snapshot, error) {
	human := string(facts.Kind) == domain.KindUser
	in.Active = facts.Status == authz.PrincipalActive
	in.Owner = facts.IsOwner && human && !ref.SessionID.IsZero() && in.Active
	if err := readGrants(ctx, a.store, ref.PrincipalID, human, &in, t); err != nil {
		return domain.Snapshot{}, err
	}
	s := domain.BuildSnapshot(in)
	if human && in.Active {
		if err := terminalState(ctx, a.hosts, &s); err != nil {
			return domain.Snapshot{}, err
		}
	}
	return s, nil
}

// authorize is Authorize without the check log and the audit.
func (a *Authorizer) authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if act == authorize.ActionWebhookOutDeliver {
		return a.authorizeDelivery(ctx, p, r)
	}
	r, found, err := a.eventTarget(ctx, act, r)
	if err != nil || !found {
		return outcome(domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}), err
	}
	spec, req, preview, err := a.actionFor(act, r)
	if err != nil {
		return authorize.Decision{}, err
	}
	t := a.clock.Now()
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		if isPublic(req) {
			return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
		}
		return authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: authorize.Reason(domain.ReasonUnauthenticated)}, nil
	}
	s, d, err := a.acting(ctx, p, spec.ID, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return outcome(d), err
	}
	return a.decideFor(ctx, s, act, r, spec, req, preview, t)
}

// acting loads the acting principal at the current rights version (principalOf).
func (a *Authorizer) acting(ctx context.Context, p authorize.Principal, action string, t time.Time) (*actingPrincipal, domain.Decision, error) {
	v, err := a.version(ctx)
	if err != nil {
		return nil, domain.Decision{}, err
	}
	return a.principalOf(ctx, p, action, v, t)
}

// decideFor is steps 5 to 9 of 07 section 4.2 for the acting principal s: the human gate,
// resolving the resource, gathering the facts and Decide.
func (a *Authorizer) decideFor(ctx context.Context, s *actingPrincipal, act authorize.Action, r authorize.Resource, spec domain.ActionSpec,
	req domain.Expr, preview *domain.PreviewFacts, t time.Time) (authorize.Decision, error) {
	if d := domain.GateHumanSession(req, s.domainPrincipal()); d.Outcome != domain.OutcomeAllow {
		return outcome(d), nil
	}
	res, found, err := a.resolve(ctx, act, r)
	if err != nil || !found {
		return outcome(domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}), err
	}
	in, err := a.gather(ctx, s, spec, req, r, res, t)
	if err != nil {
		return authorize.Decision{}, err
	}
	in.Preview = preview
	if in.StepUpWaived, err = a.stepUpWaived(ctx, spec, act, in.Principal); err != nil {
		return authorize.Decision{}, err
	}
	dd, err := domain.Decide(in)
	if err != nil {
		return authorize.Decision{}, err
	}
	return outcome(dd), nil
}

// authorizeDelivery decides webhook_out.deliver (M2-26 design note 8): the liveness check of the
// receiving principal, the delivery gate of domain.DeliveryGate (app kind, events:subscribe, the
// audience of a personal event), then the full decision of the receive action r.DeliveryOf on the
// same resource, whose outcome is the result. Without an audience an empty DeliveryOf or
// webhook_out.deliver itself is an internal error.
func (a *Authorizer) authorizeDelivery(ctx context.Context, p authorize.Principal, r authorize.Resource) (authorize.Decision, error) {
	spec, req, err := a.requirement(authorize.ActionWebhookOutDeliver)
	if err != nil {
		return authorize.Decision{}, err
	}
	if !req.IsDelivery() {
		return authorize.Decision{}, internal("webhook_out.deliver without the requirement Delivery")
	}
	of := r.DeliveryOf
	if r.Audience.IsZero() && (of == "" || of == authorize.ActionWebhookOutDeliver) {
		return authorize.Decision{}, internal("webhook_out.deliver needs the receive action of the event")
	}
	if p == nil || p.PrincipalKind() == domain.KindAnonymous {
		return authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: authorize.Reason(domain.ReasonUnauthenticated)}, nil
	}
	t := a.clock.Now()
	s, d, err := a.acting(ctx, p, spec.ID, t)
	if err != nil || d.Outcome != domain.OutcomeAllow {
		return outcome(d), err
	}
	gate := domain.Principal{ID: s.id, Kind: s.kind, GrantedScopes: p.AppGrantedScopes()}
	if d, done := domain.DeliveryGate(gate, r.Audience); done {
		return outcome(d), nil
	}
	inner := r
	inner.DeliveryOf, inner.Audience = "", ids.ID{}
	inner, found, err := a.eventTarget(ctx, of, inner)
	if err != nil || !found {
		return outcome(domain.Decision{Outcome: domain.OutcomeNotFound, Reason: domain.ReasonNoGrant}), err
	}
	ospec, oreq, preview, err := a.actionFor(of, inner)
	if err != nil {
		return authorize.Decision{}, err
	}
	return a.decideFor(ctx, s, of, inner, ospec, oreq, preview, t)
}

// actionFor returns spec and requirement of act, the previewed action of access.preview, and
// checks that r has a resource kind of the action (an unknown kind fails closed).
func (a *Authorizer) actionFor(act authorize.Action, r authorize.Resource) (domain.ActionSpec, domain.Expr, *domain.PreviewFacts, error) {
	spec, req, err := a.requirement(act)
	if err != nil {
		return domain.ActionSpec{}, domain.Expr{}, nil, err
	}
	var preview *domain.PreviewFacts
	kinds := spec.Resources
	if act == authorize.ActionAccessPreview {
		ps, preq, err := a.requirement(r.PreviewOf)
		if err != nil {
			return domain.ActionSpec{}, domain.Expr{}, nil, err
		}
		preview = &domain.PreviewFacts{Action: ps.ID, Spec: ps, Req: preq}
		kinds = ps.Resources
	}
	if !slices.Contains(kinds, string(r.Kind)) {
		return domain.ActionSpec{}, domain.Expr{}, nil, internal(fmt.Sprintf("resource kind %q is not a resource of %s", r.Kind, spec.ID))
	}
	return spec, req, preview, nil
}

// isPublic reports a requirement that is exactly Public.
func isPublic(e domain.Expr) bool { return e.String() == domain.Public.String() }
