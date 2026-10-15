// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// RightsHookDeps are the collaborators of the RightsHook that exist before the unit of work:
// internal/wire builds the hook with them before it seals the txn registry.
type RightsHookDeps struct {
	// Counters locks and raises permission_version in phase 1 (03 section 6.3).
	Counters instance.Counters
	// Store reads grants, groups and memberships inside the transaction.
	Store Store
	// Principals, Structure and Policies are the source ports.
	Principals authz.PrincipalSource
	Structure  authz.StructureSource
	Policies   authz.AgentPolicySource
	// Expiry selects and marks the due expiries every rights change folds in (M1-30).
	Expiry ExpiryStore
	// Clock is read once per phase.
	Clock clock.Clock
	// Log receives failures of the deny audit; nil means slog.Default().
	Log *slog.Logger
}

// RightsBindings are the collaborators that need the unit of work themselves (the recorder of
// audit writes its rows through it, the appender of sync is built after it); internal/wire binds
// them once the unit of work exists (Bind).
type RightsBindings struct {
	// Events appends access.changed.
	Events eventlog.Appender
	// Audit receives the deny entry of a violation (RecordDenial, written in an own transaction
	// after the rollback, 07 section 4.4 step 5).
	Audit auditing.Recorder
}

// RightsHook is the one transactional path of every rights change (02 section 8.1, brief M1-29):
// Begin (phase 1) locks and raises permission_version and records the visible targets of the
// affected principals; BeforeCommit (phase 3.1) verifies I1 and SR8, computes the differences,
// appends access.changed per principal with a non-empty difference and publishes
// authorize.AccessChanged. Use cases of other modules only declare their change with
// txn.DeclareRightsChange and inherit these guarantees. It implements authz.RightsChanges. It
// is safe for concurrent use; the state of one change lives in its unit of work.
type RightsHook struct {
	d     RightsHookDeps
	log   *slog.Logger
	bound atomic.Pointer[RightsBindings]
}

// Compile-time check: other modules name further affected principals through the port.
var _ authz.RightsChanges = (*RightsHook)(nil)

// NewRightsHook returns the hook; it opens no connection, so the golden test of internal/wire
// builds it without a database. Errors: a nil dependency other than Log.
func NewRightsHook(d RightsHookDeps) (*RightsHook, error) {
	if d.Counters == nil || d.Store == nil || d.Expiry == nil || d.Principals == nil || d.Structure == nil || d.Policies == nil || d.Clock == nil {
		return nil, errors.New("authz: NewRightsHook needs every dependency")
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &RightsHook{d: d, log: log}, nil
}

// Bind sets the appender and the recorder. Until Bind every rights change fails with kind
// internal (fail closed: no change without its event and its audit). Errors: a nil field.
func (h *RightsHook) Bind(b RightsBindings) error {
	if b.Events == nil || b.Audit == nil {
		return errors.New("authz: RightsHook.Bind needs the appender and the recorder")
	}
	h.bound.Store(&b)
	return nil
}

// rightsState is the state of one rights change in its unit of work.
type rightsState struct {
	mu      sync.Mutex
	spec    txn.RightsChangeSpec
	action  authorize.Action
	version int64
	// t is the time of Begin; every view of the change is computed at it, so a grant that
	// expires during the unit is not reported as a change of this action (the expiry sweep
	// reports it, 07 section 8.3).
	t time.Time
	// before holds the view of each affected principal before the change.
	before map[ids.ID]domain.AccessView
	// assume is what every view before assumes valid: the unreported rows (reportedView) and the
	// expiries the change folded in (expiryfold.go).
	assume domain.AssumeValid
	// pending are the grant.changed of the folded expiries and expiredMembers the folded
	// memberships whose group.changed is built in phase 3.1; both are appended there before
	// access.changed.
	pending        []eventlog.Event
	expiredMembers []domain.MembershipWithGroup
	// folded counts the folded grants and memberships.
	folded int
}

// reportedView is what every view of a rights change assumes valid: each grant and membership
// whose expiry is not reported yet (domain.AssumeValid.Unreported). Because every rights change
// first folds in the due expiries (expiryfold.go), an unreported row is one that expires after
// the time of the change or, in a sweep batch, one left to a later batch; a view is what the
// client was told, and an expiry reaches the client only once it is reported. authorize itself
// never assumes anything: an expired row is invalid from its expiry on (07 section 2.5).
var reportedView = domain.AssumeValid{Unreported: true}

// stateKey keys the rightsState in the unit of work; a retry starts without it.
var stateKey = txn.NewKey[*rightsState]("authz rights change")

// Begin is the hook of phase 1 (txn.PhaseRightsBegin), the first statement of the transaction:
// it locks and raises permission_version through instance.Counters, folds in the due expiries
// that are not reported yet (expiryfold.go, M1-30), then records the views of the declared
// principals and of the principals authz derives from the declared targets. It reads
// only inside the ambient transaction and never through the snapshot cache, whose keys are
// committed versions. Errors: kind internal for a unit without a declared catalogue action that
// changes rights, or before Bind; the errors of the counters and the sources.
func (h *RightsHook) Begin(ctx context.Context) error {
	if h.bound.Load() == nil {
		return internal("the rights hook is not bound to an appender and a recorder")
	}
	spec, ok := txn.RightsChangeOf(ctx)
	if !ok {
		return internal("the rights begin hook runs only in a unit of work declared as a rights change")
	}
	act, ok := authorize.ParseAction(spec.Action)
	if !ok {
		return internal("a rights change needs a catalogue action")
	}
	if as, ok := domain.LookupAction(spec.Action); !ok || !as.Rights {
		return internal("the action " + spec.Action + " does not change rights (05 column Permissions)")
	}
	v, err := h.d.Counters.Advance(ctx, instance.CounterPermissionVersion, 1)
	if err != nil {
		return err
	}
	t := h.d.Clock.Now()
	st := &rightsState{spec: spec, action: act, version: v, t: t, before: map[ids.ID]domain.AccessView{}, assume: reportedView}
	if err := h.fold(ctx, st, h.bound.Load(), t); err != nil {
		return err
	}
	affected := slices.Clone(spec.Principals)
	derived, err := h.affectedByTargets(ctx, spec.Targets, t)
	if err != nil {
		return err
	}
	affected = append(affected, derived...)
	if err := h.record(ctx, st, affected, t, st.assume); err != nil {
		return err
	}
	return txn.Store(ctx, stateKey, st)
}

// AddAffected implements authz.RightsChanges: in the work phase of a declared rights change it
// records the views of principals not yet affected, before the use case writes rows that concern
// them.
func (h *RightsHook) AddAffected(ctx context.Context, principals ...ids.ID) error {
	if txn.CurrentPhase(ctx) != txn.PhaseWork {
		return internal("AddAffected runs only in the work phase of a rights change")
	}
	st, ok := txn.Load(ctx, stateKey)
	if !ok {
		return internal("AddAffected outside a declared rights change")
	}
	return h.record(ctx, st, principals, st.t, st.assume)
}

// record computes and keeps the view of each principal of list that has none yet, with the rows
// of assume treated as not expired.
func (h *RightsHook) record(ctx context.Context, st *rightsState, list []ids.ID, t time.Time, assume domain.AssumeValid) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	var missing []ids.ID
	for _, p := range list {
		if _, ok := st.before[p]; !ok && !p.IsZero() && !slices.Contains(missing, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	w, err := h.structureNow(ctx)
	if err != nil {
		return err
	}
	for _, p := range missing {
		v, err := h.viewOf(ctx, w, p, st.version, t, assume)
		if err != nil {
			return err
		}
		st.before[p] = v
	}
	return nil
}

// BeforeCommit is the hook of phase 3.1 (txn.PhaseRightsCheck): VerifyInvariants with the time
// read once; on a violation it hands the deny entry to the recorder (written after the rollback
// in an own transaction) and returns the InvariantError, so the unit rolls back. Otherwise it
// computes the view after the change of every affected principal, appends one access.changed per
// principal with a non-empty difference (ascending by principal) and publishes
// authorize.AccessChanged once with all differences; a subscriber veto rolls the unit back.
func (h *RightsHook) BeforeCommit(ctx context.Context) error {
	b := h.bound.Load()
	st, ok := txn.Load(ctx, stateKey)
	if b == nil || !ok {
		return internal("the rights check hook runs without the state of its begin hook")
	}
	// A claim sync (M1-42) is declared before its person is known: the person is the actor of its
	// events and of its deny entry, which names the person, provider, groups and agents.
	cs := claimSyncOf(ctx)
	if cs != nil {
		ctx = actor.WithActor(ctx, cs.actor)
	}
	t := h.d.Clock.Now()
	viol, err := h.VerifyInvariants(ctx, t)
	if err != nil {
		return err
	}
	if !viol.Empty() {
		h.deny(ctx, b.Audit, st.spec, viol, cs)
		if cs != nil && !viol.LastOwner {
			return authz.NewI1Violation(viol.I1).WithHint(authz.HintI1ClaimSync)
		}
		return viol.Err()
	}
	folded, err := h.pendingEvents(ctx, st)
	if err != nil {
		return err
	}
	if err := b.Events.Append(ctx, folded...); err != nil {
		return err
	}
	changes, err := h.differences(ctx, st)
	if err != nil || len(changes) == 0 {
		return err
	}
	if err := h.appendEvents(ctx, b.Events, st, changes); err != nil {
		return err
	}
	return txn.Publish(ctx, authorize.AccessChanged{PermissionVersion: st.version, Action: st.action, Changes: changes})
}

// differences returns the non-empty differences of the affected principals, ascending.
func (h *RightsHook) differences(ctx context.Context, st *rightsState) ([]authorize.PrincipalAccessChange, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.before) == 0 {
		return nil, nil
	}
	w, err := h.structureNow(ctx)
	if err != nil {
		return nil, err
	}
	principals := domain.SortIDs(slices.Collect(mapsKeys(st.before)))
	var out []authorize.PrincipalAccessChange
	for _, p := range principals {
		after, err := h.viewOf(ctx, w, p, st.version, st.t, reportedView)
		if err != nil {
			return nil, err
		}
		d := domain.DiffVisible(st.before[p], after)
		if d.Empty() {
			continue
		}
		out = append(out, authorize.PrincipalAccessChange{
			PrincipalID: p, Added: accessTargets(d.Added), Removed: accessTargets(d.Removed), Changed: accessTargets(d.Changed),
		})
	}
	return out, nil
}

// mapsKeys returns the keys of m.
func mapsKeys(m map[ids.ID]domain.AccessView) func(func(ids.ID) bool) {
	return func(yield func(ids.ID) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// accessTargets converts domain entries; channels in added and changed carry their
// capabilities as a non-nil list.
func accessTargets(list []domain.AccessEntry) []authorize.AccessTarget {
	out := make([]authorize.AccessTarget, 0, len(list))
	for _, e := range list {
		at := authorize.AccessTarget{
			Kind: authorize.AccessTargetKind(e.Kind), ID: e.ID, Level: e.Level.String(), PreviousLevel: e.PreviousLevel.String(),
		}
		if e.Kind == domain.AccessKindChannel && e.Level != domain.LevelNone {
			at.Capabilities = append([]string{}, e.Caps.Names()...)
		}
		out = append(out, at)
	}
	return out
}

// accessTargetJSON is one entry of AccessChangedPayload; a nil Capabilities is left out.
type accessTargetJSON struct {
	Kind          string    `json:"kind"`
	ID            string    `json:"id"`
	Level         string    `json:"level,omitempty"`
	PreviousLevel string    `json:"previous_level,omitempty"`
	Capabilities  *[]string `json:"capabilities,omitempty"`
}

// accessChangedJSON is AccessChangedPayload: the three lists are always present.
type accessChangedJSON struct {
	PermissionVersion int64              `json:"permission_version"`
	Added             []accessTargetJSON `json:"added"`
	Removed           []accessTargetJSON `json:"removed"`
	Changed           []accessTargetJSON `json:"changed"`
}

// payloadTargets converts entries for the payload, never nil.
func payloadTargets(list []authorize.AccessTarget) []accessTargetJSON {
	out := make([]accessTargetJSON, 0, len(list))
	for _, e := range list {
		j := accessTargetJSON{Kind: string(e.Kind), ID: e.ID.String(), Level: e.Level, PreviousLevel: e.PreviousLevel}
		if e.Capabilities != nil {
			caps := slices.Clone(e.Capabilities)
			j.Capabilities = &caps
		}
		out = append(out, j)
	}
	return out
}

// Envelope values of access.changed (04 section 2.2, 05 section 6).
const (
	eventAccessChanged = "access.changed"
	receiveSelf        = "self"
	resourcePrincipal  = "principal"
)

// appendEvents appends one access.changed per change (04 section 2.2): audience and resource the
// principal, scope empty, the actor of the request (system for a system action without one), the
// payload AccessChangedPayload with the version of phase 1.
func (h *RightsHook) appendEvents(ctx context.Context, events eventlog.Appender, st *rightsState, changes []authorize.PrincipalAccessChange) error {
	kind, principal := eventActor(ctx, st.spec.Action)
	evs := make([]eventlog.Event, 0, len(changes))
	for _, c := range changes {
		payload, err := json.Marshal(accessChangedJSON{
			PermissionVersion: st.version, Added: payloadTargets(c.Added), Removed: payloadTargets(c.Removed), Changed: payloadTargets(c.Changed),
		})
		if err != nil {
			return errs.Wrap(errs.CodeInternalError, err, "authz: access.changed payload")
		}
		evs = append(evs, eventlog.Event{
			Type: eventAccessChanged, SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: principal,
			ResourceKind: resourcePrincipal, ResourceID: c.PrincipalID, AudiencePrincipalID: c.PrincipalID,
			ReceiveAction: receiveSelf, Payload: payload,
		})
	}
	return events.Append(ctx, evs...)
}

// eventActor returns the actor of the request in ctx; without one, system for a system action
// and no actor otherwise (05 rule 10).
func eventActor(ctx context.Context, action string) (eventlog.ActorKind, ids.ID) {
	if a, ok := actor.FromContext(ctx); ok && !a.ID.IsZero() {
		switch a.Kind {
		case actor.KindUser:
			return eventlog.ActorUser, a.ID
		case actor.KindAgent:
			return eventlog.ActorAgent, a.ID
		case actor.KindApp:
			return eventlog.ActorApp, a.ID
		case actor.KindHost, actor.KindAnonymous:
		}
	}
	if as, ok := domain.LookupAction(action); ok && as.System {
		return eventlog.ActorSystem, ids.ID{}
	}
	return "", ids.ID{}
}

// deny hands the deny entry of a violation to the recorder (07 section 4.4 step 5, brief M1-29
// "Deny audit entry"): action, reason, resource and the violations; actor, device, session and
// correlation come from ctx. A claim sync cs (M1-42) names the person as resource and adds
// provider, groups and agents. RecordDenial aggregates and writes in an own transaction; its error
// is logged and never changes the answer.
func (h *RightsHook) deny(ctx context.Context, rec auditing.Recorder, spec txn.RightsChangeSpec, v Violations, cs *claimSyncState) {
	e := auditing.Entry{
		Action: spec.Action, Decision: auditing.DecisionDeny,
		ResourceKind: spec.Resource.Kind, ResourceID: spec.Resource.ID,
	}
	if v.LastOwner {
		e.Reason = auditing.ReasonLastOwner
		e.Attrs = auditing.Attrs{"code": auditing.Code(string(auditing.ReasonLastOwner))}
	} else {
		e.Reason = auditing.ReasonI1Violation
		list := make([]auditing.Violation, 0, len(v.I1))
		for _, c := range v.I1 {
			list = append(list, auditing.Violation{
				Code: string(auditing.ReasonI1Violation), AgentID: c.AgentID, CredentialHolderID: c.CredentialHolderID, PrincipalIDs: c.PrincipalIDs,
			})
		}
		e.Attrs = auditing.Attrs{"violations": auditing.Violations(list...)}
		if cs != nil {
			e = claimSyncDenial(e, cs, v)
		}
	}
	if err := rec.RecordDenial(ctx, e); err != nil {
		h.log.LogAttrs(ctx, slog.LevelError, msgAuditFailed, slog.String(keyAction, spec.Action), slog.String(keyErrorType, "audit"))
	}
}

// Violations is the result of VerifyInvariants: SR8 and the I1 conflicts.
type Violations struct {
	// LastOwner reports that no active owner remains (SR8).
	LastOwner bool
	// I1 are the personal agents whose trigger set is not exactly their credential holder,
	// ascending by agent.
	I1 []authz.I1Conflict
}

// Empty reports that both invariants hold.
func (v Violations) Empty() bool { return !v.LastOwner && len(v.I1) == 0 }

// Err returns nil for no violation, else the InvariantError: last_owner before i1_violation,
// because without an owner no personal agent can be in order either.
func (v Violations) Err() error {
	switch {
	case v.LastOwner:
		return authz.NewLastOwnerViolation()
	case len(v.I1) > 0:
		return authz.NewI1Violation(v.I1)
	}
	return nil
}

// VerifyInvariants checks SR8 and I1 at t inside the ambient transaction (07 section 4.4, 03
// section 6.4): at least one active owner remains, and for every active agent with billing_mode
// personal the trigger set is exactly its credential holder (a missing holder or an empty set is
// a violation too). I1 is checked for all personal agents on every rights change (docs/DECISIONS.md
// M1-29). It is not part of the cross-module port; M1-49 (preview) calls it from inside authz.
// The error is a source or store failure, never a violation.
func (h *RightsHook) VerifyInvariants(ctx context.Context, t time.Time) (Violations, error) {
	var v Violations
	owners, err := h.d.Principals.ActiveOwnerIDs(ctx)
	if err != nil {
		return Violations{}, sourceError("principal source", err)
	}
	v.LastOwner = len(owners) == 0
	agents, err := h.d.Policies.PersonalAgents(ctx)
	if err != nil {
		return Violations{}, sourceError("agent policy source", err)
	}
	if len(agents) == 0 {
		return v, nil
	}
	w, err := h.i1World(ctx, owners)
	if err != nil {
		return Violations{}, err
	}
	for _, a := range agents {
		served, err := h.d.Structure.ServedTargets(ctx, a.AgentID)
		if err != nil {
			return Violations{}, sourceError("structure source", err)
		}
		for _, s := range served {
			w.Served[a.AgentID] = append(w.Served[a.AgentID], domain.ServedTarget{ChannelID: s.ChannelID, SpaceID: s.SpaceID})
		}
		set := domain.TriggerSet(w, domain.PersonalAgent{AgentID: a.AgentID, CredentialHolderID: a.CredentialHolderID}, t)
		if !a.CredentialHolderID.IsZero() && len(set) == 1 && set[0] == a.CredentialHolderID {
			continue
		}
		extra := slices.DeleteFunc(slices.Clone(set), func(p ids.ID) bool { return p == a.CredentialHolderID })
		v.I1 = append(v.I1, authz.I1Conflict{AgentID: a.AgentID, CredentialHolderID: a.CredentialHolderID, PrincipalIDs: extra})
	}
	return v, nil
}

// i1World reads the trigger grants, the memberships of their groups and the states of the people
// they name.
func (h *RightsHook) i1World(ctx context.Context, owners []ids.ID) (domain.World, error) {
	w := domain.World{Principals: map[ids.ID]domain.PrincipalState{}, Owners: owners, Served: map[ids.ID][]domain.ServedTarget{}}
	grants, err := h.d.Store.ActiveTriggerGrants(ctx)
	if err != nil {
		return domain.World{}, sourceError("grant store", err)
	}
	w.Grants = grants
	var groups, people []ids.ID
	for _, g := range grants {
		switch g.Subject.Kind {
		case domain.SubjectGroup:
			groups = append(groups, g.Subject.GroupID)
		case domain.SubjectUser:
			people = append(people, g.Subject.PrincipalID)
		case domain.SubjectAgent, domain.SubjectApp:
		}
	}
	if len(groups) > 0 {
		if w.Memberships, err = h.d.Store.ActiveMembershipsOfGroups(ctx, groups); err != nil {
			return domain.World{}, sourceError("grant store", err)
		}
		for _, m := range w.Memberships {
			people = append(people, m.Membership.UserID)
		}
	}
	for _, p := range people {
		if _, done := w.Principals[p]; done {
			continue
		}
		f, found, err := h.d.Principals.Principal(ctx, p)
		if err != nil {
			return domain.World{}, sourceError("principal source", err)
		}
		w.Principals[p] = domain.PrincipalState{Kind: string(f.Kind), Active: found && f.Status == authz.PrincipalActive}
	}
	return w, nil
}

// structureView is the structure read once per phase: the channel paths and the spaces.
type structureView struct {
	channels []domain.Path
	spaces   []ids.ID
}

// structureNow reads the channels and spaces from the structure source.
func (h *RightsHook) structureNow(ctx context.Context) (structureView, error) {
	chans, err := h.d.Structure.Channels(ctx)
	if err != nil {
		return structureView{}, sourceError("structure source", err)
	}
	spaces, err := h.d.Structure.Spaces(ctx)
	if err != nil {
		return structureView{}, sourceError("structure source", err)
	}
	w := structureView{spaces: spaces}
	for _, c := range chans {
		w.channels = append(w.channels, channelPath(c))
	}
	return w, nil
}

// viewOf builds the view of principal at version v and time t from the sources, with the same
// snapshot and Effective function as authorize but without the cache. A person counts as in a
// session: the owner role takes effect for an active owner (the views are what the person sees
// in the app). A principal that is unknown or not active sees nothing (SR7). The rows of assume
// count as not expired.
func (h *RightsHook) viewOf(ctx context.Context, w structureView, principal ids.ID, v int64, t time.Time, assume domain.AssumeValid) (domain.AccessView, error) {
	facts, found, err := h.d.Principals.Principal(ctx, principal)
	if err != nil {
		return domain.AccessView{}, sourceError("principal source", err)
	}
	if !found || facts.Status != authz.PrincipalActive {
		return domain.AccessView{Spaces: map[ids.ID]bool{}, Channels: map[ids.ID]domain.Rights{}}, nil
	}
	kind := string(facts.Kind)
	in := domain.SnapshotInput{PrincipalID: principal, Version: v, At: t, Active: true, Owner: facts.IsOwner && kind == domain.KindUser, Assume: assume}
	if err := readGrants(ctx, h.d.Store, principal, kind == domain.KindUser, &in, t); err != nil {
		return domain.AccessView{}, err
	}
	return domain.AccessViewOf(domain.BuildSnapshot(in), kind, principal, w.channels, w.spaces), nil
}

// affectedByTargets derives the affected principals of target refs (02 section 8.1, 04 section
// 8.2): the subjects of grants in effect at t on the targets, their ancestors and, for a space,
// its channels, with the members of subject groups; the active owners; and the effective
// operators of the channels concerned. The difference step drops everyone without a change.
func (h *RightsHook) affectedByTargets(ctx context.Context, targets []txn.Ref, t time.Time) ([]ids.ID, error) {
	if len(targets) == 0 {
		return nil, nil
	}
	spaces, channels, out, err := h.targetScope(ctx, targets)
	if err != nil {
		return nil, err
	}
	grants, err := h.d.Store.ActiveGrantsOnTargets(ctx, spaces, channels)
	if err != nil {
		return nil, sourceError("grant store", err)
	}
	var groups []ids.ID
	for _, g := range grants {
		if !reportedView.Grant(g).InEffect(t) {
			continue
		}
		if g.Subject.Kind == domain.SubjectGroup {
			groups = append(groups, g.Subject.GroupID)
			continue
		}
		out = append(out, g.Subject.PrincipalID)
	}
	members, err := activeMembers(ctx, h.d.Store, groups, t)
	if err != nil {
		return nil, err
	}
	owners, err := h.d.Principals.ActiveOwnerIDs(ctx)
	if err != nil {
		return nil, sourceError("principal source", err)
	}
	out = append(append(out, members...), owners...)
	return slices.DeleteFunc(out, func(id ids.ID) bool { return id.IsZero() }), nil
}

// targetScope returns the spaces and channels whose grants concern targets (a space with its
// channels, a channel with its space) and the effective operators of those channels.
func (h *RightsHook) targetScope(ctx context.Context, targets []txn.Ref) (spaces, channels, operators []ids.ID, err error) {
	chans, err := h.d.Structure.Channels(ctx)
	if err != nil {
		return nil, nil, nil, sourceError("structure source", err)
	}
	for _, r := range targets {
		switch r.Kind {
		case string(authorize.ResourceSpace):
			spaces = append(spaces, r.ID)
			ch, ops := channelsOfSpace(chans, r.ID)
			channels, operators = append(channels, ch...), append(operators, ops...)
		case string(authorize.ResourceChannel):
			channels = append(channels, r.ID)
			c, ok, err := h.d.Structure.Channel(ctx, r.ID)
			if err != nil {
				return nil, nil, nil, sourceError("structure source", err)
			}
			if ok {
				spaces, operators = append(spaces, c.SpaceID), append(operators, c.EffectiveOperatorID)
			}
		default:
			return nil, nil, nil, internal("a rights change target must be a space or a channel, not " + r.Kind)
		}
	}
	return spaces, channels, operators, nil
}

// channelsOfSpace returns the channels of space among chans and their effective operators.
func channelsOfSpace(chans []authz.ChannelFacts, space ids.ID) (channels, operators []ids.ID) {
	for _, c := range chans {
		if c.SpaceID == space {
			channels, operators = append(channels, c.ID), append(operators, c.EffectiveOperatorID)
		}
	}
	return channels, operators
}

// activeMembers returns the people with a membership in effect at t in one of groups, counting
// unreported expiries as not yet effective (reportedView).
func activeMembers(ctx context.Context, store Store, groups []ids.ID, t time.Time) ([]ids.ID, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	ms, err := store.ActiveMembershipsOfGroups(ctx, groups)
	if err != nil {
		return nil, sourceError("grant store", err)
	}
	var out []ids.ID
	for _, m := range ms {
		if reportedView.Membership(m.Membership).InEffect(m.Group, t) {
			out = append(out, m.Membership.UserID)
		}
	}
	return out, nil
}

// readGrants reads the grants of principal and, for a person, its memberships and the grants of
// the groups in effect at t into in (memberships named in in.Assume count as not expired). The
// Authorizer and the rights hook share it, so a view and a decision always rest on the same facts.
func readGrants(ctx context.Context, store Store, principal ids.ID, person bool, in *domain.SnapshotInput, t time.Time) error {
	var err error
	if in.Grants, err = store.ActiveGrantsOfPrincipal(ctx, principal); err != nil {
		return sourceError("grant store", err)
	}
	if !person {
		return nil
	}
	if in.Memberships, err = store.ActiveMembershipsOfUser(ctx, principal); err != nil {
		return sourceError("grant store", err)
	}
	var groups []ids.ID
	for _, m := range in.Memberships {
		if in.Assume.Membership(m.Membership).InEffect(m.Group, t) {
			groups = append(groups, m.Group.ID)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	if in.GroupGrants, err = store.ActiveGrantsOfGroups(ctx, groups); err != nil {
		return sourceError("grant store", err)
	}
	return nil
}
