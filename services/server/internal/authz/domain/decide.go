// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Outcome is the decision of 05 section 2.5 as text (kernel/authorize.Outcome converts it).
type Outcome string

// The outcomes of 05 section 2.5.
const (
	OutcomeAllow              Outcome = "allow"
	OutcomeNotFound           Outcome = "not_found"
	OutcomeForbidden          Outcome = "forbidden"
	OutcomeStepUpRequired     Outcome = "step_up_required"
	OutcomeInvariantViolation Outcome = "invariant_violation"
	OutcomeUnauthenticated    Outcome = "unauthenticated"
)

// Decision is an outcome with its reason; the empty Reason belongs to allow. QuickReattach is true
// exactly when the step-up rule of an action with fresh step-up was waived by the quick reattach (M2-40):
// the use case then consumes no step-up.
type Decision struct {
	Outcome       Outcome
	Reason        Reason
	QuickReattach bool
}

// Allow is the allowing decision.
var Allow = Decision{Outcome: OutcomeAllow}

func deny(o Outcome, r Reason) Decision { return Decision{Outcome: o, Reason: r} }

// Liveness are the facts of the liveness checks of 07 section 4.2 step 1 (SR7) for the acting
// principal and its credentials.
type Liveness struct {
	// Status is principals.status; "" for an unknown principal.
	Status string
	// HasSession, HasDevice and HasToken report which credential ids the principal carries.
	HasSession, HasDevice, HasToken bool
	// IsApp reports kind app; HasInstallation an installation id.
	IsApp, HasInstallation bool
	// SessionActive and SessionExpiresAt describe the session.
	SessionActive    bool
	SessionExpiresAt time.Time
	// DeviceState is pending, active, revoked or unknown.
	DeviceState string
	// DeviceKeyRevoked reports that the authenticator found the request's key (cnf.jkt) not to be
	// the device's active dpop key any more (07 section 4.1 device_inactive: "key
	// revoked"; M1-34).
	DeviceKeyRevoked bool
	// TokenActive and TokenExpiresAt describe the API token; a zero expiry means none.
	TokenActive    bool
	TokenExpiresAt time.Time
}

// The device states of CredentialFacts.
const (
	DevicePending = "pending"
	DeviceActive  = "active"
	DeviceRevoked = "revoked"
	DeviceUnknown = "unknown"
)

// CheckLiveness applies the liveness checks in the order of the brief M1-28: status, session,
// device, token, installation. It returns Allow or an unauthenticated decision.
func CheckLiveness(l Liveness, t time.Time) Decision {
	switch l.Status {
	case "":
		return deny(OutcomeUnauthenticated, ReasonUnauthenticated)
	case "disabled", "deleted":
		return deny(OutcomeUnauthenticated, ReasonPrincipalInactive)
	case "active", "pending":
	default:
		return deny(OutcomeUnauthenticated, ReasonPrincipalInactive)
	}
	if l.HasSession && (!l.SessionActive || !t.Before(l.SessionExpiresAt)) {
		return deny(OutcomeUnauthenticated, ReasonSessionRevoked)
	}
	if l.HasDevice && (l.DeviceKeyRevoked || l.DeviceState != DevicePending && l.DeviceState != DeviceActive) {
		return deny(OutcomeUnauthenticated, ReasonDeviceInactive)
	}
	if l.HasToken && (!l.TokenActive || (!l.TokenExpiresAt.IsZero() && !t.Before(l.TokenExpiresAt))) {
		return deny(OutcomeUnauthenticated, ReasonTokenRevoked)
	}
	if l.IsApp && !l.HasInstallation {
		return deny(OutcomeUnauthenticated, ReasonAppNotInstalled)
	}
	return Allow
}

// pendingAllowed are the actions a pending person or a pending device may run (05 section 2.5).
// device.list passes here and is narrowed to the own device after the visibility check (Decide).
//
//nolint:goconst // action ids of the generated catalogue, listed once here
var pendingAllowed = []string{"self.read", actionDeviceList, "auth.token.refresh", "auth.logout"}

// actionDeviceList is the one pending-allowed action that is narrowed to the own device.
const actionDeviceList = "device.list"

// CheckPending applies 07 section 4.2 step 1 for pending principals and devices: a person in
// status pending gets principal_pending, a pending device device_pending, except for the self
// information actions.
func CheckPending(l Liveness, action string) Decision {
	if slices.Contains(pendingAllowed, action) {
		return Allow
	}
	if l.Status == "pending" {
		return deny(OutcomeForbidden, ReasonPrincipalPending)
	}
	if l.HasDevice && l.DeviceState == DevicePending {
		return deny(OutcomeForbidden, ReasonDevicePending)
	}
	return Allow
}

// Principal is the acting principal as Decide sees it.
type Principal struct {
	// ID and Kind identify the principal; Kind is one of the Kind constants.
	ID   ids.ID
	Kind string
	// HasSession reports a session (people only); StepUpAt its last step-up.
	HasSession bool
	StepUpAt   time.Time
	// DeviceID is the device of the session.
	DeviceID ids.ID
	// Pending is the reason of a pending person or device (CheckPending passed the action); empty
	// otherwise.
	Pending Reason
	// GrantedScopes are the scopes of an app installation.
	GrantedScopes []string
	// ViewAs reports "view as"; ActingFor a run of a triggering person.
	ViewAs, ActingFor bool
}

// ResourceFacts are the facts of the resource descriptor.
type ResourceFacts struct {
	// Kind is the resource kind of the catalogue.
	Kind string
	// ID is the resource; OwnerID the principal it belongs to.
	ID, OwnerID ids.ID
	// HasAttachments and RestrictedTags feed HasCapIf.
	HasAttachments, RestrictedTags bool
}

// PreviewFacts are the action that access.preview previews.
type PreviewFacts struct {
	Action string
	Spec   ActionSpec
	Req    Expr
}

// HostFacts are the facts of a host resource (M2-39, 07 section 2.12), computed by the
// application layer from the snapshots of the principal and of "view as": Visible is the
// visibility of the host (owner, holder or terminal), Assignee and Terminal the atoms of the
// requirement after the "view as" intersection. The remaining fields are the stored facts of the
// host that the guards SR10 and SR11 read (M2-40); for a terminal_session they describe its host.
type HostFacts struct {
	Visible, Assignee, Terminal bool
	// ID is the host; AssignedUserID its assignee (zero: none).
	ID, AssignedUserID ids.ID
	// LLMCredentials is none, personal or organization; CredentialHolderID is set with personal.
	LLMCredentials     string
	CredentialHolderID ids.ID
	// DeclaredServerHost and ReportedIsServerHost mark the server host (24 section 2.3).
	DeclaredServerHost, ReportedIsServerHost bool
}

// TargetFacts are the facts of the second target (thread.move).
type TargetFacts struct {
	Visible bool
	Eff     Rights
}

// GrantFacts are the facts of the grant guards.
type GrantFacts struct {
	// Shape is the new grant (grant.create) or the existing grant (grant.revoke).
	Shape GrantShape
}

// TokenFacts are the facts of the token.create guard (brief M1-44): the kind of the principal that
// would hold the new token.
type TokenFacts struct {
	// HolderFound reports a known principal; HolderKind is its kind (user, agent or app).
	HolderFound bool
	HolderKind  string
}

// Input is everything Decide needs; the application layer gathers it.
type Input struct {
	// Action is the action id, Spec its catalogue entry and Req its requirement.
	Action string
	Spec   ActionSpec
	Req    Expr
	// Now is the time of the call; StepUpMaxAge is auth.step_up_max_age.
	Now          time.Time
	StepUpMaxAge time.Duration
	// StepUpWaived reports a conditional step-up ("bedingt") whose condition is false: the person
	// has no sign-in data of the kind the action guards yet (hat_anmeldedaten, 07 section 4.2 step
	// 8; M1-40), so a human session suffices.
	StepUpWaived bool
	Principal    Principal
	Resource     ResourceFacts
	Path         Path
	// Eff are the final effective rights on Path (after "view as" and acting-for intersections);
	// BaseEff the principal's own rights before the acting-for intersection.
	Eff, BaseEff Rights
	// BaseVisible reports whether the resource is visible with the principal's own rights; only
	// read when Principal.ActingFor is set.
	BaseVisible bool
	// GrantTarget is the target kind of a grant resource.
	GrantTarget TargetKind
	// SpaceHasVisibleChannel, ReadOnOperatedChannel and GroupMember are facts of the visibility
	// rules and atoms.
	SpaceHasVisibleChannel, ReadOnOperatedChannel, GroupMember bool
	Preview                                                    *PreviewFacts
	Target                                                     *TargetFacts
	Grant                                                      *GrantFacts
	Invoke                                                     *InvokeFacts
	Token                                                      *TokenFacts
	// Host are the facts of a host resource, and of the host of a terminal_session; nil for every
	// other kind.
	Host *HostFacts
	// Fresh are the step-up facts of the session for an action with fresh step-up (05 section 2.4); FreshMaxAge
	// is terminal.step_up_max_age and ReattachWindow terminal.quick_reattach_window (M2-40).
	Fresh          FreshFacts
	FreshMaxAge    time.Duration
	ReattachWindow time.Duration
	// Reattach are the facts of the quick reattach of terminal.attach; nil for every other action.
	Reattach *ReattachFacts
	// Terminal are the facts of the guards of terminal.open and terminal.attach; nil otherwise.
	Terminal *TerminalGuardFacts
	// quickReattach records that step 8 passed through the quick reattach.
	quickReattach bool
}

// GateHumanSession is the gate of 07 section 4.2 for a requirement gated by HumanSession
// (HumanGated): every principal that is not a person acting in a session is refused with
// forbidden subject_kind_not_allowed. The answer depends on the principal alone, never on the
// resource, so an agent learns nothing about a host (AZ-P4). Any other requirement passes.
func GateHumanSession(req Expr, p Principal) Decision {
	if HumanGated(req) && !humanSession(p) {
		return deny(OutcomeForbidden, ReasonSubjectKindNotAllowed)
	}
	return Allow
}

// humanSession reports a person acting in a session (not with an API token).
func humanSession(p Principal) bool { return p.Kind == KindUser && p.HasSession }

// Decide decides one call from the facts of in, steps 5 to 9 of 07 section 4.2 in the order of the
// brief M1-28: visibility (not_found), requirement (forbidden with the atom's reason), app scope,
// read-only channel, the delegation limits of grant.create and grant.revoke (part of the
// requirement, brief M1-48), "view as" read-only, step-up, guards. Liveness and pending checks run
// before (CheckLiveness, CheckPending). The error reports facts that cannot be evaluated (an
// unknown resource kind, a deferred or system requirement, a missing descriptor); it is never an
// allow.
func Decide(in Input) (Decision, error) {
	if d, done, err := decideVisibility(&in); done || err != nil {
		return d, err
	}
	if d, done, err := decideRequirement(&in); done || err != nil {
		return d, err
	}
	if d, done, err := decideDelegation(&in); done || err != nil {
		return d, err
	}
	if d, done := decidePrincipalRules(&in); done {
		return d, nil
	}
	d, err := guards(&in)
	if err == nil && d.Outcome == OutcomeAllow {
		d.QuickReattach = in.quickReattach
	}
	return d, err
}

// decideVisibility is step 5: no visibility gives not_found; a pending device lists only itself.
func decideVisibility(in *Input) (Decision, bool, error) {
	visible, reason, err := Visible(*in)
	if err != nil {
		return Decision{}, true, err
	}
	if !visible {
		if in.Principal.ActingFor && in.BaseVisible {
			reason = ReasonActingForInsufficient
		}
		return deny(OutcomeNotFound, reason), true, nil
	}
	if in.Principal.Pending != "" && in.Action == actionDeviceList &&
		(in.Resource.Kind != kindDevice || in.Resource.ID != in.Principal.DeviceID) {
		return deny(OutcomeForbidden, in.Principal.Pending), true, nil
	}
	return Decision{}, false, nil
}

// decideRequirement is step 6 with the app scope and the read-only channel (05 rule 1.8: manage
// and owner paths stay open). In an archived channel a requirement that needs write or trigger
// fails with channel_archived before any other reason (07 section 4.5 point 1).
func decideRequirement(in *Input) (Decision, bool, error) {
	if d, done, err := decideArchived(in); done || err != nil {
		return d, done, err
	}
	ok, reason, err := evaluate(in, in.Req, in.Eff, false)
	if err != nil {
		return Decision{}, true, err
	}
	if !ok {
		reason = notSelfForPersonalToken(in, reason)
		if in.Principal.ActingFor {
			if base, _, _ := evaluate(in, in.Req, in.BaseEff, false); base {
				reason = ReasonActingForInsufficient
			}
		}
		return deny(OutcomeForbidden, reason), true, nil
	}
	if in.Principal.Kind == KindApp {
		if r := CheckAppScope(in.Action, in.Principal.GrantedScopes); r != "" {
			return deny(OutcomeForbidden, r), true, nil
		}
	}
	if in.Path.ReadOnly {
		if ok, _, _ := evaluate(in, in.Req, in.Eff, true); !ok {
			return deny(OutcomeForbidden, ReasonChannelReadOnly), true, nil
		}
	}
	return Decision{}, false, nil
}

// decideArchived denies a requirement above read in an archived channel (07 section 4.5 point 1);
// manage and owner paths stay open like in a read-only channel.
func decideArchived(in *Input) (Decision, bool, error) {
	if !in.Path.Archived {
		return Decision{}, false, nil
	}
	ok, _, err := evaluate(in, in.Req, in.Eff, true)
	if err != nil || !ok {
		return deny(OutcomeForbidden, ReasonChannelArchived), true, err
	}
	return Decision{}, false, nil
}

// decidePrincipalRules are steps 7 and 8: "view as" is read-only, step-up needs a fresh
// confirmation in a session of a person; a waived conditional step-up needs only the session.
func decidePrincipalRules(in *Input) (Decision, bool) {
	p := in.Principal
	if p.ViewAs && Writes(in.Action) {
		return deny(OutcomeForbidden, ReasonViewAsReadOnly), true
	}
	if !in.Spec.StepUp {
		return Decision{}, false
	}
	if p.Kind != KindUser || !p.HasSession {
		// Also a first set-up without step-up needs a session (07 section 4.2 step 8).
		return deny(OutcomeForbidden, ReasonStepUpNotPossible), true
	}
	if in.Spec.StepUpFresh {
		return decideFresh(in)
	}
	if in.StepUpWaived {
		// The first setup needs only a human with a session (07 section 4.2 step 8).
		return Decision{}, false
	}
	if p.StepUpAt.IsZero() || in.Now.Sub(p.StepUpAt) > in.StepUpMaxAge {
		return deny(OutcomeStepUpRequired, ReasonStepUpRequired), true
	}
	return Decision{}, false
}

// decideFresh is step 8 for an action with fresh step-up (05 section 2.4, 08 section 6.3; M2-40):
// only this branch, never the one of auth.step_up_max_age. The quick reattach of terminal.attach
// passes without step-up and is recorded for the decision; otherwise the step-up must be fresh and
// unconsumed (FreshStepUp), else step_up_required with terminal_step_up_not_fresh.
func decideFresh(in *Input) (Decision, bool) {
	if in.Reattach != nil && QuickReattach(*in.Reattach, in.Now, in.ReattachWindow) {
		in.quickReattach = true
		return Decision{}, false
	}
	if FreshStepUp(in.Fresh, in.Now, in.FreshMaxAge) {
		return Decision{}, false
	}
	return deny(OutcomeStepUpRequired, ReasonTerminalStepUpNotFresh), true
}

// evaluator evaluates a requirement with the rights eff. noWrite forces Level(write) and
// Level(trigger) false for the read-only rule (05 rule 1.8).
type evaluator struct {
	in      *Input
	eff     Rights
	noWrite bool
}

// evaluate returns whether e holds for in with eff and, if not, the reason of the deciding atom.
func evaluate(in *Input, e Expr, eff Rights, noWrite bool) (bool, Reason, error) {
	return evaluator{in: in, eff: eff, noWrite: noWrite}.eval(e)
}

// atom is the evaluation of one leaf of a requirement: whether it holds and the reason if not.
type atom func(v evaluator, e Expr) (bool, Reason)

// atoms are the leaves of the brief M1-28 table "Requirement table".
var atoms = map[op]atom{
	opPublic: func(evaluator, Expr) (bool, Reason) { return true, "" },
	opAuth: func(v evaluator, _ Expr) (bool, Reason) {
		return v.in.Principal.Kind != KindAnonymous, ReasonUnauthenticated
	},
	opHuman: func(v evaluator, _ Expr) (bool, Reason) {
		return v.in.Principal.Kind == KindUser, ReasonSubjectKindNotAllowed
	},
	opAppKind: func(v evaluator, _ Expr) (bool, Reason) {
		return v.in.Principal.Kind == KindApp, ReasonSubjectKindNotAllowed
	},
	opSelf: func(v evaluator, _ Expr) (bool, Reason) { return v.self(), ReasonNotSelf },
	opAgentSelf: func(v evaluator, _ Expr) (bool, Reason) {
		return v.kindSelf(KindAgent)
	},
	opAppSelf: func(v evaluator, _ Expr) (bool, Reason) { return v.kindSelf(KindApp) },
	opOwner:   func(v evaluator, _ Expr) (bool, Reason) { return v.eff.Owner, ReasonOwnerOnly },
	opParticipant: func(v evaluator, _ Expr) (bool, Reason) {
		return v.in.Path.Participant(v.in.Principal.ID) && v.eff.Level >= LevelRead, ReasonNoGrant
	},
	opLevel: func(v evaluator, e Expr) (bool, Reason) { return v.level(e.level), ReasonLevelInsufficient },
	opCap:   func(v evaluator, e Expr) (bool, Reason) { return v.eff.Caps.Has(e.cap), ReasonCapabilityMissing },
	opCapIf: func(v evaluator, e Expr) (bool, Reason) {
		return !factOf(v.in, e.fact) || v.eff.Caps.Has(e.cap), ReasonCapabilityMissing
	},
	opThreadOperator: func(v evaluator, _ Expr) (bool, Reason) {
		return v.isAgentOnChannel() && v.in.Path.ThreadOperator() == v.in.Principal.ID, ReasonSubjectKindNotAllowed
	},
	opChannelOperator: func(v evaluator, _ Expr) (bool, Reason) {
		return v.isAgentOnChannel() && v.in.Path.ChannelOperatorID == v.in.Principal.ID, ReasonSubjectKindNotAllowed
	},
	opReadOnOperatedChannel: func(v evaluator, _ Expr) (bool, Reason) { return v.in.ReadOnOperatedChannel, ReasonNoGrant },
	opGroupMember:           func(v evaluator, _ Expr) (bool, Reason) { return v.in.GroupMember, ReasonNotSelf },
	opHumanSession: func(v evaluator, _ Expr) (bool, Reason) {
		return humanSession(v.in.Principal), ReasonSubjectKindNotAllowed
	},
	opAssignee: func(v evaluator, _ Expr) (bool, Reason) { return v.in.Host != nil && v.in.Host.Assignee, ReasonNotSelf },
	opTerminal: func(v evaluator, _ Expr) (bool, Reason) {
		return v.in.Host != nil && v.in.Host.Terminal, ReasonCapabilityMissing
	},
}

// self reports that the resource belongs to the principal (isSelf).
func (v evaluator) self() bool { return isSelf(*v.in) }

// kindSelf is AgentSelf or AppSelf: the kind first, then the owner.
func (v evaluator) kindSelf(kind string) (bool, Reason) {
	if v.in.Principal.Kind != kind {
		return false, ReasonSubjectKindNotAllowed
	}
	return v.self(), ReasonNotSelf
}

// isAgentOnChannel reports an agent principal on a channel path.
func (v evaluator) isAgentOnChannel() bool {
	return v.in.Principal.Kind == KindAgent && v.in.Path.Kind == PathChannel
}

// level is Level(l); on a space, read means "space visible" (07 section 4.3): read on one of its
// channels suffices.
func (v evaluator) level(l Level) bool {
	switch {
	case l == LevelNone, v.noWrite && l >= LevelWrite:
		return false
	case l == LevelRead && v.in.Resource.Kind == kindSpace:
		return v.eff.Level >= LevelRead || v.in.SpaceHasVisibleChannel
	}
	return v.eff.Level >= l
}

// combinator is the evaluation of a node that may fail with an error.
type combinator func(v evaluator, e Expr) (bool, Reason, error)

// combinators are All, Any, ByKind and the atoms that need more than the facts of the path.
var combinators map[op]combinator

func init() {
	combinators = map[op]combinator{
		opAll:       func(v evaluator, e Expr) (bool, Reason, error) { return v.all(e.args) },
		opAny:       func(v evaluator, e Expr) (bool, Reason, error) { return v.any(e.args) },
		opByKind:    func(v evaluator, e Expr) (bool, Reason, error) { return v.byKind(e) },
		opTargetCap: func(v evaluator, e Expr) (bool, Reason, error) { return v.targetCap(e) },
		opPreview:   func(v evaluator, _ Expr) (bool, Reason, error) { return v.preview() },
	}
}

// eval evaluates e. An Expr that is neither an atom nor a combinator (invalid, deferred, system)
// is an internal error, never an allow.
func (v evaluator) eval(e Expr) (bool, Reason, error) {
	if a, ok := atoms[e.op]; ok {
		holds, reason := a(v, e)
		if holds {
			return true, "", nil
		}
		return false, reason, nil
	}
	if c, ok := combinators[e.op]; ok {
		holds, reason, err := c(v, e)
		if err != nil || holds {
			return holds && err == nil, "", err
		}
		return false, reason, nil
	}
	return false, "", internalError("requirement cannot be evaluated: " + e.String())
}

// all is the conjunction with the reason of the first failing part.
func (v evaluator) all(parts []Expr) (bool, Reason, error) {
	for _, x := range parts {
		if ok, r, err := v.eval(x); err != nil || !ok {
			return false, r, err
		}
	}
	return true, "", nil
}

// byKind chooses the expression by the resource kind; a kind not listed is an internal error.
func (v evaluator) byKind(e Expr) (bool, Reason, error) {
	x, ok := e.byKind[v.in.Resource.Kind]
	if !ok {
		return false, "", internalError("no requirement for resource kind " + v.in.Resource.Kind)
	}
	return v.eval(x)
}

// targetCap is TargetCap on the destination channel of thread.move.
func (v evaluator) targetCap(e Expr) (bool, Reason, error) {
	if v.in.Target == nil {
		return false, "", internalError("thread.move without its target channel")
	}
	return v.in.Target.Eff.Caps.Has(e.cap), ReasonCapabilityMissing, nil
}

// any is the disjunction with the reason of its first alternative.
func (v evaluator) any(alts []Expr) (bool, Reason, error) {
	var first Reason
	for i, x := range alts {
		ok, r, err := v.eval(x)
		if err != nil || ok {
			return ok, "", err
		}
		if i == 0 {
			first = r
		}
	}
	return false, first, nil
}

// preview evaluates the requirement of the previewed action (access.preview), which must change
// rights.
func (v evaluator) preview() (bool, Reason, error) {
	p := v.in.Preview
	if p == nil {
		return false, "", internalError("access.preview without the previewed action")
	}
	if !p.Spec.Rights || p.Req.op == opPreview {
		return false, "", internalError("access.preview of an action that changes no rights")
	}
	return v.eval(p.Req)
}

// factOf returns the descriptor fact f.
func factOf(in *Input, f Fact) bool {
	switch f {
	case FactHasAttachments:
		return in.Resource.HasAttachments
	case FactRestrictedTags:
		return in.Resource.RestrictedTags
	}
	return true
}
