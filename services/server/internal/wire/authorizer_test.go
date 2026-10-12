// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

var authzTestConfig = authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: 5 * time.Minute}

// seedUser inserts one active user with plain SQL (the identity tables of M1-26).
func seedUser(t *testing.T, e *auditEnv) ids.ID {
	t.Helper()
	id := ids.New()
	conn := e.db.SuperConn(t)
	ctx := context.Background()
	if _, err := conn.Exec(ctx, "INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', 'alex', 'Alex')", id.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO users (principal_id, email) VALUES ($1, 'alex@example.com')", id.String()); err != nil {
		t.Fatal(err)
	}
	return id
}

func newWiredAuthorizer(t *testing.T, e *auditEnv) *authzapp.Authorizer {
	t.Helper()
	az, err := wire.NewAuthorizer(e.pool, authzTestConfig, authzapp.NewCatalog(slog.New(slog.DiscardHandler)),
		e.audit.Recorder(), clock.System(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return az
}

// TestWireAuthorizerWithEmptySources: the wired authorizer reads the principal through identity
// and knows no structure and no credential until their tickets (D2).
func TestWireAuthorizerWithEmptySources(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Minute)
	user := seedUser(t, e)
	var az authorize.Authorizer = newWiredAuthorizer(t, e)
	ctx := context.Background()
	d, err := az.Authorize(ctx, actor.Actor{ID: user, Kind: actor.KindUser}, authorize.ActionChannelRead,
		authorize.Resource{Kind: authorize.ResourceChannel, ID: ids.New()})
	if err != nil || d.Outcome != authorize.OutcomeNotFound || d.Reason != "no_grant" {
		t.Fatalf("without credentials: %+v %v", d, err)
	}
	d, err = az.Authorize(ctx, actor.Actor{ID: user, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()},
		authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: ids.New()})
	if err != nil || d.Outcome != authorize.OutcomeUnauthenticated || d.Reason != "session_revoked" {
		t.Fatalf("with a session id: %+v %v", d, err)
	}
	sc, err := az.Scope(ctx, actor.Actor{ID: user, Kind: actor.KindUser}, authorize.ActionMessageRead)
	if err != nil || len(sc.ChannelIDs) != 0 || sc.InstanceWide {
		t.Fatalf("scope %+v %v", sc, err)
	}
}

// TestAZP13_DenialThroughTheChainIsAudited: two identical denied requests through the HTTP chain
// leave exactly one aggregated row in audit_events (05 section 2.4, R-AZ-04).
func TestAZP13_DenialThroughTheChainIsAudited(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Minute)
	az := newWiredAuthorizer(t, e)
	op := httpservertest.Op("getThing", http.MethodGet, "/api/v1/things/{thing_id}", "thing.read")
	h := httpservertest.New(t, httpservertest.Options{
		Authenticator: actor.AnonymousOnly{},
		Routes: []httpservertest.Route{{Op: op, Handler: func(w http.ResponseWriter, r *http.Request) {
			a, _ := actor.FromContext(r.Context())
			d, err := az.Authorize(r.Context(), a, authorize.ActionSelfRead, authorize.Resource{Kind: authorize.ResourcePrincipal})
			if err == nil {
				err = d.Err()
			}
			httpserver.WriteProblem(w, r, err)
		}}},
	})
	for range 2 {
		resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+ids.New().String(), "", "X-Mode", "fast"))
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
	}
	if err := e.audit.Recorder().Aggregator().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	var n, count int
	if err := e.pool.QueryRow(context.Background(),
		"SELECT count(*), coalesce(max((details->'aggregate'->>'count')::int), 0) FROM audit_events WHERE action = 'self.read' AND decision = 'deny' AND deny_reason = 'unauthenticated'").
		Scan(&n, &count); err != nil {
		t.Fatal(err)
	}
	if n != 1 || count != 2 {
		t.Fatalf("audit rows %d with count %d, want one row counting 2", n, count)
	}
}
