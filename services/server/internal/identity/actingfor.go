// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ActingForSource is the source port through which the Authenticator resolves the Invocation-Token
// header of an agent's request into its acting-for binding (07 guard rule SR9, brief M2-14 design
// notes 2 and 3). identity declares it because the Authenticator owns the actor (02 section 4.3);
// the module runs implements it in runs/adapters/actingfor and internal/wire binds it (02 section
// 4.1 rule 4, a source port of a lower module). The signature has only kernel types, so no module
// edge arises.
//
// ResolveActingFor checks token for the agent agentID (signature, aud, run) and returns the binding
// with Status bound, run_inactive or invalid; a token it cannot use is a status, never an error.
// The error is reserved for a failure of the source itself (database), which the Authenticator
// turns into 503 unavailable (fail closed). Neither the result nor the error may contain token.
type ActingForSource interface {
	ResolveActingFor(ctx context.Context, agentID ids.ID, token string) (actor.ActingFor, error)
}

// NoActingForSource is the ActingForSource of a process or test without the module runs: every
// header is invalid, so authorize refuses the request (fail closed) instead of letting the agent
// act with its own rights.
type NoActingForSource struct{}

// Compile-time check: NoActingForSource is the port.
var _ ActingForSource = NoActingForSource{}

// ResolveActingFor implements ActingForSource: always ActingFor{Status: invalid}, never an error.
func (NoActingForSource) ResolveActingFor(context.Context, ids.ID, string) (actor.ActingFor, error) {
	return actor.ActingFor{Status: actor.ActingForInvalid}, nil
}
