// SPDX-License-Identifier: Apache-2.0

package flagsource_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/flagsource"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakePrincipals returns the known principals of a list.
type fakePrincipals struct {
	known []domain.Principal
	err   error
}

func (f fakePrincipals) Principals(_ context.Context, list []ids.ID) ([]domain.Principal, error) {
	var out []domain.Principal
	for _, p := range f.known {
		if slices.Contains(list, p.ID) {
			out = append(out, p)
		}
	}
	return out, f.err
}

func TestExistingUserIDs_PeopleNotDeleted(t *testing.T) {
	t.Parallel()
	sam, robin, agent, gone, unknown := ids.New(), ids.New(), ids.New(), ids.New(), ids.New()
	src, err := flagsource.New(fakePrincipals{known: []domain.Principal{
		{ID: sam, Kind: domain.KindUser, Status: domain.StatusActive},
		{ID: robin, Kind: domain.KindUser, Status: domain.StatusDisabled},
		{ID: agent, Kind: domain.KindAgent, Status: domain.StatusActive},
		{ID: gone, Kind: domain.KindUser, Status: domain.StatusDeleted},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.ExistingUserIDs(context.Background(), []ids.ID{unknown, sam, agent, gone, robin})
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(a, b ids.ID) int { return slices.Compare(a[:], b[:]) })
	want := []ids.ID{sam, robin}
	slices.SortFunc(want, func(a, b ids.ID) int { return slices.Compare(a[:], b[:]) })
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	src, _ = flagsource.New(fakePrincipals{err: errors.New("down")})
	if _, err := src.ExistingUserIDs(context.Background(), []ids.ID{sam}); err == nil {
		t.Error("error lost")
	}
	if _, err := flagsource.New(nil); err == nil {
		t.Error("nil store accepted")
	}
}
