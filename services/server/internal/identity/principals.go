// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CreatePrincipalCmd is the input of PrincipalWriter.CreatePrincipal.
type CreatePrincipalCmd struct {
	// ID is the id of the new principal; the zero ID makes identity generate a UUIDv7. A caller
	// that declares the new principal in a rights change before the insert chooses it itself.
	ID ids.ID
	// Kind is actor.KindAgent or actor.KindApp; people are created through People.CreateUser.
	Kind actor.Kind
	// Handle follows the rules of principals.handle (^[a-z0-9][a-z0-9._-]*$, at most
	// principal.handle_max_chars characters); it must be free without regard to case, also
	// against people.
	Handle string
	// DisplayName follows the rules of principals.display_name: not blank, at most
	// principal.display_name_max_chars characters, no control or bidi characters (SEC-IN-4).
	DisplayName string
}

// PrincipalWriter is the writing port of identity for the modules that own principals other than
// people: agents (M1-51) and later apps (M2-24). It authorizes nothing, because the calling use
// case did, and it writes nothing outside the ambient unit of work of ctx: every method fails with
// kind internal outside one. People are refused (kind internal), because their lifecycle belongs
// to the use cases of People. Errors are kernel/errs errors without SQL text or values.
type PrincipalWriter interface {
	// CreatePrincipal inserts an active principal of cmd.Kind and returns it. Errors:
	// validation_failed on /handle or /display_name, conflict on /handle when the handle is taken
	// without regard to case (also by a person).
	CreatePrincipal(ctx context.Context, cmd CreatePrincipalCmd) (PrincipalView, error)
	// SetPrincipalStatus moves the principal id to status active or disabled; disabled sets
	// disabled_at, active clears it. changed is false when the principal already has the status.
	// The caller declares the rights change (agent.disable, agent.enable). Errors: not_found for an
	// unknown id; invalid_state_transition for a deleted principal or another target status.
	SetPrincipalStatus(ctx context.Context, id ids.ID, status Status) (p PrincipalView, changed bool, err error)
	// SetDisplayName changes the display name of the principal id; changed is false when the
	// validated name equals the current one. Errors: not_found, validation_failed on
	// /display_name.
	SetDisplayName(ctx context.Context, id ids.ID, displayName string) (p PrincipalView, changed bool, err error)
}
