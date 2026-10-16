// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestDirectoryUsers (M3-18): the people among a list in one read, keyed by id, with their time
// zone; agents, apps and unknown ids are absent, and an empty list reads nothing.
func TestDirectoryUsers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := app.NewDirectory(f.repo)
	ctx := context.Background()
	alex, sam := f.add(domain.KindUser, "alex"), f.add(domain.KindUser, "sam")
	agent := f.add(domain.KindAgent, "support-agent")
	got, err := dir.Users(ctx, []ids.ID{alex.ID, sam.ID, agent.ID, ids.New(), alex.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[alex.ID].TimeZone != "Europe/Berlin" || got[sam.ID].Handle != "sam" || got[alex.ID].ID != alex.ID {
		t.Fatalf("Users = %+v", got)
	}
	if m, err := dir.Users(ctx, nil); err != nil || len(m) != 0 {
		t.Fatalf("Users(nil) = %v, %v", m, err)
	}
}
