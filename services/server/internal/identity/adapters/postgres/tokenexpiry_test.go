// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestExpiringBotTokens (M6-07): the source of "Expiring soon" lists the active tokens of agents
// and apps expiring in (at, until], ordered by expiry and id; personal tokens, revoked tokens and
// tokens outside the window never appear, and the result carries no token value.
func TestExpiringBotTokens(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	sam := e.mustAddUser(t, "sam", "sam@example.com").Principal
	agent := principal(t, domain.KindAgent, "texter", "Texter")
	bot := principal(t, domain.KindApp, "tracker", "Tracker")
	if err := e.write(t, func(ctx context.Context) error {
		for _, p := range []*domain.Principal{&agent, &bot} {
			if err := e.repo.InsertPrincipal(ctx, p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	tok := func(holder ids.ID, exp time.Time) app.APITokenRecord {
		r := tokenAt(holder, sam.ID, at.Add(-time.Hour), "")
		r.ExpiresAt = exp
		return r
	}
	late := tok(agent.ID, at.Add(2*time.Hour))
	early := tok(bot.ID, at.Add(time.Hour))
	edge := tok(agent.ID, at.Add(48*time.Hour))
	personal := tok(sam.ID, at.Add(time.Hour))
	revoked := tok(agent.ID, at.Add(time.Hour))
	outside := tok(agent.ID, at.Add(48*time.Hour+time.Second))
	expired := tok(agent.ID, at)
	if err := e.write(t, func(ctx context.Context) error {
		for _, r := range []app.APITokenRecord{late, early, edge, personal, revoked, outside, expired} {
			if err := e.repo.InsertAPIToken(ctx, r); err != nil {
				return err
			}
		}
		_, err := e.repo.RevokeAPIToken(ctx, revoked.ID, sam.ID, at)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	src, err := app.NewTokenExpiry(e.repo)
	if err != nil {
		t.Fatal(err)
	}
	got, err := src.ExpiringTokens(ctx, at, at.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := []app.APITokenRecord{early, late, edge}
	if len(got) != len(want) {
		t.Fatalf("tokens %+v", got)
	}
	for i, w := range want {
		if got[i].ID != w.ID || got[i].PrincipalID != w.PrincipalID || !got[i].ExpiresAt.Equal(w.ExpiresAt) {
			t.Errorf("token %d %+v, want %v", i, got[i], w.ID)
		}
	}
	if _, err := app.NewTokenExpiry(nil); err == nil {
		t.Error("a nil store")
	}
}
