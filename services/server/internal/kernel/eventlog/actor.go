// SPDX-License-Identifier: Apache-2.0

package eventlog

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// principalActors are the actor kinds of a principal that acts in an event; their values equal
// the principal kinds of kernel/actor. A host and an anonymous caller have none.
var principalActors = map[ActorKind]bool{ActorUser: true, ActorAgent: true, ActorApp: true}

// ActorFor returns the event actor of the principal id of the kind (the text of an actor.Kind):
// a person, an agent and an app act in events. Any other kind, and a principal without an id,
// give no actor.
func ActorFor(kind string, id ids.ID) (ActorKind, ids.ID) {
	if k := ActorKind(kind); principalActors[k] && !id.IsZero() {
		return k, id
	}
	return "", ids.ID{}
}
