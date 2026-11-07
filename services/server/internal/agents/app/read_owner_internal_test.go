// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ownerDirectory answers User from users and err; every other method of identity.Directory is
// not used by isOwner.
type ownerDirectory struct {
	identity.Directory
	users map[ids.ID]identity.UserView
	err   error
}

func (d ownerDirectory) User(_ context.Context, id ids.ID) (identity.UserView, error) {
	if d.err != nil {
		return identity.UserView{}, d.err
	}
	u, ok := d.users[id]
	if !ok {
		return identity.UserView{}, errs.New(errs.CodeNotFound, "user not found")
	}
	return u, nil
}

// TestIsOwner (A1-03b): only a person with an active owners row is an owner; an agent, an unknown
// person and a person without the row are not, and a directory fault propagates.
func TestIsOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner, person := ids.New(), ids.New()
	dir := ownerDirectory{users: map[ids.ID]identity.UserView{owner: {IsOwner: true}, person: {}}}
	s := &Service{d: Deps{Directory: dir}}
	for _, tc := range []struct {
		name string
		a    actor.Actor
		want bool
	}{
		{"owner", actor.Actor{ID: owner, Kind: actor.KindUser}, true},
		{"person", actor.Actor{ID: person, Kind: actor.KindUser}, false},
		{"unknown person", actor.Actor{ID: ids.New(), Kind: actor.KindUser}, false},
		{"agent with an owner's id", actor.Actor{ID: owner, Kind: actor.KindAgent}, false},
	} {
		if got, err := s.isOwner(ctx, tc.a); err != nil || got != tc.want {
			t.Fatalf("%s: isOwner = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	fault := errors.New("directory fault")
	s = &Service{d: Deps{Directory: ownerDirectory{err: fault}}}
	if got, err := s.isOwner(ctx, actor.Actor{ID: owner, Kind: actor.KindUser}); !errors.Is(err, fault) || got {
		t.Fatalf("a directory fault: isOwner = %v, %v", got, err)
	}
}
