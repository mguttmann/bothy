// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Principal is the checked principal context of a request as Authorize reads it
// (docs/spec/07-rechte.md section 3). kernel/actor.Actor implements it, so a use case passes its
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
	// AppGrantedScopes are app_installations.granted_scopes (apps only). The result may be shared;
	// callers do not change it.
	AppGrantedScopes() []string
	// ViewAsPerson is the person whose view an owner looks at (07 section 9.2); zero for none.
	ViewAsPerson() ids.ID
	// ActingForPerson is the triggering person of the run an agent acts in (SR9); zero for none.
	ActingForPerson() ids.ID
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
	// a channel path, or space.read). A principal that fails the liveness checks gets the error of
	// its decision (Decision.Err), not an empty scope.
	Scope(ctx context.Context, p Principal, a Action) (Scope, error)
}
