// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var errLimitReached = errors.New("limit_reached")

// limitedCommands is a register of one owner's live commands that refuses a new name at max,
// like agents.CommandService.UpsertOwned with command.max_per_principal.
type limitedCommands struct {
	max  int
	live map[string]ids.ID
}

func (f *limitedCommands) UpsertOwned(_ context.Context, _ ids.ID, def agents.RegisterCommand) (agents.Command, bool, error) {
	name := strings.ToLower(def.Name)
	if id, ok := f.live[name]; ok {
		return agents.Command{ID: id}, true, nil
	}
	if len(f.live) >= f.max {
		return agents.Command{}, false, errLimitReached
	}
	id := ids.New()
	f.live[name] = id
	return agents.Command{ID: id}, true, nil
}

func (f *limitedCommands) RemoveOwned(_ context.Context, _ ids.ID, names []string) ([]ids.ID, error) {
	var out []ids.ID
	for _, n := range names {
		if id, ok := f.live[n]; ok {
			out = append(out, id)
			delete(f.live, n)
		}
	}
	return out, nil
}

func (f *limitedCommands) RemoveAllOwned(ctx context.Context, owner ids.ID) ([]ids.ID, error) {
	var names []string
	for n := range f.live {
		names = append(names, n)
	}
	return f.RemoveOwned(ctx, owner, names)
}

func manifestOf(names ...string) domain.Manifest {
	var m domain.Manifest
	for _, n := range names {
		m.Commands = append(m.Commands, domain.Command{Name: n, Description: "d"})
	}
	return m
}

// TestUpgradeCommands_RemovesBeforeAdding: an app at command.max_per_principal that replaces one
// command by another stays within the limit, so the upgrade succeeds (removed names free their
// place before new names are added).
func TestUpgradeCommands_RemovesBeforeAdding(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	app := ids.New()
	reg := &limitedCommands{max: 3, live: map[string]ids.ID{}}
	s := &Service{d: Deps{Commands: reg}}
	if err := s.installCommands(ctx, app, manifestOf("a", "b", "c"), auditing.Attrs{}); err != nil {
		t.Fatalf("install: %v", err)
	}
	attrs := auditing.Attrs{}
	if err := s.upgradeCommands(ctx, app, manifestOf("a", "b", "c"), manifestOf("a", "b", "d"), attrs); err != nil {
		t.Fatalf("upgrade within the limit failed: %v", err)
	}
	if _, ok := reg.live["d"]; !ok || len(reg.live) != 3 {
		t.Fatalf("live commands after the upgrade = %v", reg.live)
	}
	if _, ok := reg.live["c"]; ok {
		t.Fatal("c was not removed")
	}
}
