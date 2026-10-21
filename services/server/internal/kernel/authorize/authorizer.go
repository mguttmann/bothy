// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Principal is the checked principal context of a request as Authorize reads it
// (docs/spec/07-permissions.md section 3). kernel/actor.Actor implements it, so a use case passes its
// actor directly:
//
//	a, _ := actor.FromContext(ctx)
//	d, err := az.Authorize(ctx, a, authorize.ActionThreadRead, r)
//
// It is an interface of plain values because kernel ports never import each other (02 section 7.2,
// go-arch-lint); docs/DECISIONS.md M1-28 records this. Implementations return the zero value for a
// field that does not apply to the kind.
type Principal interface {
	// PrincipalID is the principal; zero for anonymous.
	PrincipalID() ids.ID
	// PrincipalKind is user, agent, app, host or anonymous.
	PrincipalKind() string
	// SessionRef is auth_sessions.id (people only).
	SessionRef() ids.ID
	// DeviceRef is the device of the session (people only).
	DeviceRef() ids.ID
	// TokenRef is the API token of an agent, app or personal token.
	TokenRef() ids.ID
	// InstallationRef is the app installation (apps only).
	InstallationRef() ids.ID
	// StepUpTime is the last fresh confirmation of the session; zero when there was none.
	StepUpTime() time.Time
	// StepUpMethodName is auth_sessions.step_up_method of StepUpTime; "" for none (M2-40).
	StepUpMethodName() string
	// StepUpConsumedTime is auth_sessions.step_up_consumed_at; zero when never consumed (M2-40).
	StepUpConsumedTime() time.Time
	// StepUpUserVerifiedFact reports that the step-up of StepUpTime was a passkey whose stored
	// WebAuthn session required user verification (08 section 6.3 point 3, M6-18).
	StepUpUserVerifiedFact() bool
	// ClientComponentName is auth_sessions.client_component (app, share_ext, console); "" without
	// a session (M2-40).
	ClientComponentName() string
	// AppGrantedScopes are app_installations.granted_scopes (apps only). The result may be shared;
	// callers do not change it.
	AppGrantedScopes() []string
	// AppTokenScopes are api_tokens.scopes of the API token of the request (apps only, 07 section
	// 3 token_scopes). An app request needs the scope of its action in AppGrantedScopes and, when
	// TokenRef is set, in AppTokenScopes too (M2-24). The result may be shared; callers do not
	// change it.
	AppTokenScopes() []string
	// ViewAsPerson is the person whose view an owner looks at (07 section 9.2); zero for none.
	ViewAsPerson() ids.ID
	// ViewAsStart is the start of "view as"; zero for none. From it plus
	// authz.view_as_max_duration on, every decision is step_up_required (M1-49).
	ViewAsStart() time.Time
	// ActingForPerson is the triggering person of the run an agent acts in (SR9); zero for none.
	ActingForPerson() ids.ID
}

// RecheckMarker is implemented by a principal that can carry the marker of the check without a
// request (05 section 5, R-AZ-29; kernel/actor.Actor). IsRecheckPrincipal reports the marker; authz
// treats a principal without this method as an ordinary request principal. It is a different thing
// from the context marker Recheck of the terminal checks after a commit.
type RecheckMarker interface {
	IsRecheckPrincipal() bool
}

// IsRecheckPrincipal reports whether p is the principal of a check without a request (R-AZ-29);
// false for nil and for a principal that does not implement RecheckMarker.
func IsRecheckPrincipal(p Principal) bool {
	r, ok := p.(RecheckMarker)
	return ok && r.IsRecheckPrincipal()
}

// ActingForBinder is implemented by a principal that can carry the acting-for binding of its
// request (07 guard rule SR9, brief M2-14; kernel/actor.Actor). ActingForBinding returns the
// outcome of the check of the Invocation-Token header and the triggering person, the run and its
// thread: status is "" without the header, else "bound", "run_inactive" or "invalid" (then the IDs
// are zero). authz treats a principal without this method whose ActingForPerson is set as bound.
type ActingForBinder interface {
	ActingForBinding() (status string, userID, runID, threadID ids.ID)
}

// The statuses of ActingForBinder (kernel/actor.ActingForStatus).
const (
	ActingForBound       = "bound"
	ActingForRunInactive = "run_inactive"
	ActingForInvalid     = "invalid"
)

// ActingForOf returns the acting-for binding of p: the binding of an ActingForBinder, else bound
// to ActingForPerson when it is set, else none (status ""). nil has none.
func ActingForOf(p Principal) (status string, userID, runID, threadID ids.ID) {
	if p == nil {
		return "", ids.ID{}, ids.ID{}, ids.ID{}
	}
	if b, ok := p.(ActingForBinder); ok {
		return b.ActingForBinding()
	}
	if id := p.ActingForPerson(); !id.IsZero() {
		return ActingForBound, id, ids.ID{}, ids.ID{}
	}
	return "", ids.ID{}, ids.ID{}, ids.ID{}
}

// CredentialObserver is implemented by a principal whose authenticator read the state of its
// session, device and device key for the request (kernel/actor.Actor, M1-34). observed is false
// when it read none. authz uses these facts in check_active instead of its cached ones, so a
// revoked session or device takes effect at the next request without a new rights version (07
// section 4.2, AZ-P9).
type CredentialObserver interface {
	CredentialState() (sessionActive bool, deviceState string, deviceKeyActive, observed bool)
}

// Scope is the result of Authorizer.Scope: the sets list queries filter with (07 section 7.4).
// The lists are built per request from the rights snapshot and the current structure, so a channel
// or space created without a new rights version appears at once.
type Scope struct {
	// ChannelIDs are the visible channels, ascending, for "channel_id = ANY($1)".
	ChannelIDs []ids.ID
	// SpaceIDs are the visible spaces, ascending: a visible channel or read on the space.
	SpaceIDs []ids.ID
	// FullyVisibleSpaceIDs are the spaces with read on the space itself, ascending (07 7.2).
	FullyVisibleSpaceIDs []ids.ID
	// InstanceWide reports owner or read on the instance (07 7.2).
	InstanceWide bool
	// HostIDs are the hosts of host.read, ascending, for "id = ANY($1)" (M2-39): the hosts with
	// the effective right terminal and the own hosts of the holder; empty when AllHosts is set.
	HostIDs []ids.ID
	// AllHosts reports an effective owner, who reads every host (07 section 2.12 point 5).
	AllHosts bool
	// PermissionVersion is the rights version the scope was built at.
	PermissionVersion int64
}

// Authorizer is the one authorization function of B7 (ADR 0017). The authz module implements it;
// every module calls it. Every implementation must call RecordCheck once per Authorize and once per
// Scope call (02 section 9.3) and starts with the catalogue lookup, so an unknown action fails
// closed (AZ-P3).
type Authorizer interface {
	// Authorize decides whether p may run a on r (07 section 4.2). Policy results are always a
	// Decision; the error is only for failures: an unknown or deferred action, an unknown resource
	// kind, contradictory path hints or a source error. Callers treat an error like a denial and
	// return it (it renders as 500 or 503).
	Authorize(ctx context.Context, p Principal, a Action, r Resource) (Decision, error)
	// Scope returns the visible sets for the list action a (an action whose requirement is read on
	// a channel path, space.read, or host.read for the hosts). A principal that fails the liveness checks gets the error of
	// its decision (Decision.Err), not an empty scope.
	Scope(ctx context.Context, p Principal, a Action) (Scope, error)
	// Observe applies one durable event of the log to the process-local rights cache (02
	// "Permission cache"). The log tail calls it in seq order, before the hub sees the event:
	// an event with a newer rights version than a cached snapshot makes that snapshot be rebuilt in
	// delivery mode, and so do the structure events (channel.created, channel.updated,
	// channel.archived, space.archived, thread.moved).
	Observe(ctx context.Context, ev LogEvent)
}
