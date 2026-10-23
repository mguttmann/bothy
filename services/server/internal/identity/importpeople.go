// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReasonImportPrincipalInactive is the reason of 409 conflict when a mapping names a person or
// an agent that is not active (M8-03 design note 2).
const ReasonImportPrincipalInactive = "principal_inactive"

// ImportRef names a person or an agent of the mapping of an import (`user:<handle>`,
// `agent:<handle>`, 22 5.2).
type ImportRef struct {
	// Kind is actor.KindUser or actor.KindAgent.
	Kind actor.Kind
	// Handle is compared without regard to case.
	Handle string
}

// PlaceholderSpec is a placeholder person of an import (22 5.3, M8-03 design note 8). The caller
// (importer) derives both values by the placeholder rules; identity checks them again with the
// rules of every principal.
type PlaceholderSpec struct {
	// Handle is the candidate handle (imported-<name> with an optional counter suffix).
	Handle string
	// DisplayName is the shown name (global_name or username of the source, cleaned and cut to
	// principal.display_name_max_chars).
	DisplayName string
}

// ImportPeople is the incoming port of identity for the people of an import (M8-03 design notes
// 7 and 8). It authorizes nothing: the batch was authorized with import.apply, the mapping with
// import.manage. Reads join the ambient transaction of ctx; Placeholder runs only inside the
// caller's unit of work. Errors are kernel/errs errors without SQL.
type ImportPeople interface {
	// Resolve returns the person (Kind user) or agent (Kind agent) with the handle. Errors:
	// not_found when no principal of that kind has the handle; conflict with reason
	// principal_inactive when it is not active.
	Resolve(ctx context.Context, ref ImportRef) (ids.ID, error)
	// HandleFree reports whether no principal holds handle without regard to case.
	HandleFree(ctx context.Context, handle string) (bool, error)
	// Placeholder creates a person with status disabled and the users row with the instance
	// defaults: no user_identities, no local_credentials, no grant, no group membership, no owner
	// row; it can never log in and never trigger (R-IM-06). It appends user.updated like
	// createUser and writes no audit entry. A taken handle, also one a concurrent unit took after
	// HandleFree, is conflict on /handle and leaves the caller's unit usable, so the caller tries the
	// next candidate. Errors: validation_failed on /handle or /display_name, conflict on /handle,
	// internal outside a unit of work.
	Placeholder(ctx context.Context, spec PlaceholderSpec) (ids.ID, error)
}
