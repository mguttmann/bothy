// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// nopRecorder drops audit entries; the denials of these tests are not the subject.
type nopRecorder struct{}

func (nopRecorder) Record(context.Context, auditing.Entry) error         { return nil }
func (nopRecorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (nopRecorder) RecordDetached(context.Context, auditing.Entry) error { return nil }

// liveSessions is the credential source of these tests: the device part of identity (M1-33) with
// every session alive, because sessions come with M1-34.
type liveSessions struct{ devices *app.CredentialSource }

func (s liveSessions) Credentials(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	f, err := s.devices.Credentials(ctx, ref)
	if !ref.SessionID.IsZero() {
		f.SessionActive, f.SessionExpiresAt = true, time.Now().Add(time.Hour)
	}
	return f, err
}

// tokenAuth authenticates "Bearer <name>" as the actor registered under name.
type tokenAuth map[string]actor.Actor

func (a tokenAuth) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Authorization == "" {
		return actor.Anonymous(), nil
	}
	if p, ok := a[strings.TrimPrefix(c.Authorization, "Bearer ")]; ok {
		return p, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

func (tokenAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// env is a database with people, their devices and the three operations behind the full chain.
type env struct {
	t     *testing.T
	owner *pg.Pool
	devs  *app.Devices
	auth  tokenAuth
	h     *httpservertest.Harness
}

func newEnv(t *testing.T, cfg app.Config) *env {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	repo := postgres.NewRepository(pool)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	versions := &fake.Versions{}
	versions.Set(1)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(slog.New(slog.DiscardHandler)), Store: fake.NewStore(),
		Principals: app.NewPrincipalSource(repo), Credentials: liveSessions{devices: app.NewCredentialSource(repo, repo, repo)},
		Structure: fake.NewStructure(), Policies: fake.NewPolicies(), Versions: versions, Recorder: nopRecorder{},
		Clock: clock.System(), Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	devs, err := app.NewDevices(cfg, app.DevicesDeps{Store: repo, People: repo, UoW: uow, Authorizer: az, Cursors: httpapi.Cursors{},
		Events: nopEvents{}})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, owner: db.Pool(t, pg.RoleOwner), devs: devs, auth: tokenAuth{}}
	built, err := routes.Build(wire.Handlers{Devices: httpapi.NewDevices(devs)}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	e.h = httpservertest.New(t, httpservertest.Options{Routes: rs, Contract: rest.Spec, Authenticator: e.auth})
	return e
}

func (e *env) sql(stmt string, args ...any) {
	e.t.Helper()
	if _, err := e.owner.Exec(context.Background(), stmt, args...); err != nil {
		e.t.Fatalf("%s: %v", stmt, err)
	}
}

// person inserts a person; owner true adds an active owners row.
func (e *env) person(handle string, owner bool) ids.ID {
	e.t.Helper()
	id := ids.New()
	e.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), handle)
	e.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		e.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	return id
}

// device registers a device of user in state and returns it.
func (e *env) device(user ids.ID, state identity.DeviceState) identity.Device {
	e.t.Helper()
	_, pub := identitytest.NewDeviceKey(jose.AlgES256)
	r := identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Phone", Platform: identity.PlatformAndroid, DPoPKey: pub, State: state}
	if state == identity.DeviceStateActive {
		r.ApprovedBy = &user
	}
	d, err := e.devs.Register(context.Background(), r)
	if err != nil {
		e.t.Fatal(err)
	}
	return d
}

// login makes token name authenticate as user on device in a session.
func (e *env) login(name string, user, device ids.ID) {
	e.auth[name] = actor.Actor{ID: user, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: device}
}

func (e *env) get(token, path string) *httpservertest.Response {
	e.t.Helper()
	return e.h.Do(e.h.NewRequest(http.MethodGet, path, "", "Authorization", "Bearer "+token))
}

func page(t *testing.T, r *httpservertest.Response) rest.DevicePage {
	t.Helper()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", r.StatusCode, r.Body)
	}
	var p rest.DevicePage
	if err := json.Unmarshal(r.Body, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func wantProblem(t *testing.T, r *httpservertest.Response, status int, code string) {
	t.Helper()
	if r.StatusCode != status {
		t.Fatalf("status %d, want %d: %s", r.StatusCode, status, r.Body)
	}
	if got := r.Problem(t)["code"]; got != code {
		t.Fatalf("code %v, want %s", got, code)
	}
}

var defaultCfg = app.Config{PageSizeDefault: 50, PageSizeMax: 200}

func TestListMyDevices(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	sam := e.person("sam", false)
	mine := e.device(alex, identity.DeviceStateActive)
	second := e.device(alex, identity.DeviceStatePending)
	revoked := e.device(alex, identity.DeviceStateActive)
	e.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_owner' WHERE id = $1`, revoked.ID.String())
	e.device(sam, identity.DeviceStateActive)
	e.login("alex", alex, mine.ID)
	p := page(t, e.get("alex", "/api/v1/me/devices"))
	if len(p.Items) != 3 || p.NextCursor != nil {
		t.Fatalf("items %d, next %v", len(p.Items), p.NextCursor)
	}
	// Newest first, all states.
	if ids.ID(p.Items[0].Id) != revoked.ID || ids.ID(p.Items[1].Id) != second.ID || ids.ID(p.Items[2].Id) != mine.ID {
		t.Fatalf("order %v", p.Items)
	}
	if p.Items[0].State != "revoked" || p.Items[0].RevokeReason == nil || *p.Items[0].RevokeReason != "revoked_by_owner" || p.Items[0].RevokedAt == nil {
		t.Errorf("revoked item %+v", p.Items[0])
	}
	if p.Items[2].KeyThumbprint == nil || *p.Items[2].KeyThumbprint != mine.KeyThumbprint || p.Items[2].ApprovedAt == nil {
		t.Errorf("active item %+v", p.Items[2])
	}
	// Anonymous requests are unauthenticated.
	r := e.h.Do(e.h.NewRequest(http.MethodGet, "/api/v1/me/devices", ""))
	wantProblem(t, r, http.StatusUnauthorized, "unauthenticated")
}

func TestListMyDevices_PendingForbidden(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	pending := e.device(alex, identity.DeviceStatePending)
	e.login("pending", alex, pending.ID)
	wantProblem(t, e.get("pending", "/api/v1/me/devices"), http.StatusForbidden, "device_pending")
	wantProblem(t, e.get("pending", "/api/v1/users/"+alex.String()+"/devices"), http.StatusForbidden, "device_pending")
	// A revoked device is no longer authenticated (check_active).
	revoked := e.device(alex, identity.DeviceStateActive)
	e.sql(`UPDATE devices SET state = 'revoked', revoked_at = now() WHERE id = $1`, revoked.ID.String())
	e.login("revoked", alex, revoked.ID)
	wantProblem(t, e.get("revoked", "/api/v1/me/devices"), http.StatusUnauthorized, "session_revoked")
}

func TestGetDevice_PendingOwnOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	sam := e.person("sam", false)
	pending := e.device(alex, identity.DeviceStatePending)
	other := e.device(alex, identity.DeviceStateActive)
	samDev := e.device(sam, identity.DeviceStateActive)
	e.login("pending", alex, pending.ID)
	r := e.get("pending", "/api/v1/devices/"+pending.ID.String())
	if r.StatusCode != http.StatusOK {
		t.Fatalf("own device: %d %s", r.StatusCode, r.Body)
	}
	var d rest.Device
	if err := json.Unmarshal(r.Body, &d); err != nil || ids.ID(d.Id) != pending.ID || d.State != "pending" || d.ApprovedAt != nil {
		t.Fatalf("device %+v, %v", d, err)
	}
	wantProblem(t, e.get("pending", "/api/v1/devices/"+other.ID.String()), http.StatusForbidden, "device_pending")
	missing := e.get("pending", "/api/v1/devices/"+ids.New().String())
	foreign := e.get("pending", "/api/v1/devices/"+samDev.ID.String())
	wantProblem(t, missing, http.StatusNotFound, "not_found")
	wantProblem(t, foreign, http.StatusNotFound, "not_found")
	if strip(missing.Problem(t)) != strip(foreign.Problem(t)) {
		t.Errorf("missing and invisible differ:\n%s\n%s", missing.Body, foreign.Body)
	}
	// From an active device: own devices yes, another person's no.
	e.login("alex", alex, other.ID)
	if r := e.get("alex", "/api/v1/devices/"+pending.ID.String()); r.StatusCode != http.StatusOK {
		t.Fatalf("other own device from an active device: %d", r.StatusCode)
	}
	wantProblem(t, e.get("alex", "/api/v1/devices/"+samDev.ID.String()), http.StatusNotFound, "not_found")
}

// strip removes the per-request members of a problem.
func strip(p map[string]any) string {
	delete(p, "correlation_id")
	delete(p, "instance")
	b, _ := json.Marshal(p)
	return string(b)
}

func TestListUserDevices_SelfOwnerOther(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	sam := e.person("sam", false)
	robin := e.person("robin", true)
	a := e.device(alex, identity.DeviceStateActive)
	s := e.device(sam, identity.DeviceStateActive)
	rd := e.device(robin, identity.DeviceStateActive)
	e.login("alex", alex, a.ID)
	e.login("sam", sam, s.ID)
	e.login("robin", robin, rd.ID)
	if p := page(t, e.get("alex", "/api/v1/users/"+alex.String()+"/devices")); len(p.Items) != 1 || ids.ID(p.Items[0].Id) != a.ID {
		t.Fatalf("self: %+v", p)
	}
	other := e.get("sam", "/api/v1/users/"+alex.String()+"/devices")
	wantProblem(t, other, http.StatusNotFound, "not_found")
	unknown := e.get("sam", "/api/v1/users/"+ids.New().String()+"/devices")
	wantProblem(t, unknown, http.StatusNotFound, "not_found")
	if strip(other.Problem(t)) != strip(unknown.Problem(t)) {
		t.Errorf("other person and unknown person differ:\n%s\n%s", other.Body, unknown.Body)
	}
	if p := page(t, e.get("robin", "/api/v1/users/"+alex.String()+"/devices")); len(p.Items) != 1 {
		t.Fatalf("owner: %+v", p)
	}
	wantProblem(t, e.get("robin", "/api/v1/users/"+ids.New().String()+"/devices"), http.StatusNotFound, "not_found")
	// An owner without a session is not an owner (SR5): the token of an owner sees no one else.
	e.auth["robin-token"] = actor.Actor{ID: robin, Kind: actor.KindUser}
	wantProblem(t, e.get("robin-token", "/api/v1/users/"+alex.String()+"/devices"), http.StatusNotFound, "not_found")
}

func TestDevices_Pagination(t *testing.T) {
	t.Parallel()
	e := newEnv(t, app.Config{PageSizeDefault: 2, PageSizeMax: 3})
	alex := e.person("alex", false)
	sam := e.person("sam", true)
	var all []ids.ID
	for range 5 {
		all = append(all, e.device(alex, identity.DeviceStateActive).ID)
	}
	e.login("alex", alex, all[0])
	sd := e.device(sam, identity.DeviceStateActive)
	e.login("sam", sam, sd.ID)
	first := page(t, e.get("alex", "/api/v1/me/devices"))
	if len(first.Items) != 2 || first.NextCursor == nil || ids.ID(first.Items[0].Id) != all[4] {
		t.Fatalf("first page %+v", first)
	}
	// A new device does not shift the next page (keyset on id).
	e.device(alex, identity.DeviceStateActive)
	second := page(t, e.get("alex", "/api/v1/me/devices?cursor="+*first.NextCursor))
	if len(second.Items) != 2 || ids.ID(second.Items[0].Id) != all[2] || ids.ID(second.Items[1].Id) != all[1] {
		t.Fatalf("second page %+v", second)
	}
	last := page(t, e.get("alex", "/api/v1/me/devices?cursor="+*second.NextCursor))
	if len(last.Items) != 1 || last.NextCursor != nil || ids.ID(last.Items[0].Id) != all[0] {
		t.Fatalf("last page %+v", last)
	}
	// limit above api.page_size_max is capped.
	if p := page(t, e.get("alex", "/api/v1/me/devices?limit=1000")); len(p.Items) != 3 {
		t.Fatalf("limit capped: %d items", len(p.Items))
	}
	// A cursor of another list, another caller or a changed one is 400 cursor_invalid.
	wantProblem(t, e.get("sam", "/api/v1/users/"+alex.String()+"/devices?cursor="+*first.NextCursor), http.StatusBadRequest, "cursor_invalid")
	// The same list of the same caller through the other path keeps its position.
	if p := page(t, e.get("alex", "/api/v1/users/"+alex.String()+"/devices?cursor="+*first.NextCursor)); len(p.Items) != 2 {
		t.Fatalf("same list via users/{id}: %+v", p)
	}
	wantProblem(t, e.get("alex", "/api/v1/me/devices?cursor=x"+*first.NextCursor), http.StatusBadRequest, "cursor_invalid")
	foreign, err := httpserver.EncodeCursor("threads", []byte(alex.String()), map[string]string{"id": all[2].String()})
	if err != nil {
		t.Fatal(err)
	}
	wantProblem(t, e.get("alex", "/api/v1/me/devices?cursor="+foreign), http.StatusBadRequest, "cursor_invalid")
}

func TestDevice_NoNullMembers(t *testing.T) {
	t.Parallel()
	e := newEnv(t, defaultCfg)
	alex := e.person("alex", false)
	d := e.device(alex, identity.DeviceStatePending)
	e.login("alex", alex, d.ID)
	for _, path := range []string{"/api/v1/me/devices", "/api/v1/devices/" + d.ID.String(), "/api/v1/users/" + alex.String() + "/devices"} {
		r := e.get("alex", path)
		if path != "/api/v1/devices/"+d.ID.String() && r.StatusCode == http.StatusForbidden {
			continue // a pending device lists nothing; the device read is the subject here
		}
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", path, r.StatusCode, r.Body)
		}
		body := string(r.Body)
		for _, absent := range []string{"approved_at", "revoked_at", "revoke_reason", "last_seen_at", "step_up_key_thumbprint", "app_version", "push_registrations", "verification_code"} {
			if strings.Contains(body, absent) {
				t.Errorf("%s: body has %q: %s", path, absent, body)
			}
		}
		testkit.NoJSONNull(t, path, r.Body)
	}
	// The harness fails the test at the end if a handler sent JSON null (ADR 0053).
}

// nopEvents accepts device.pending without storing it: this setup has no sync step (M3-19).
type nopEvents struct{}

func (nopEvents) Append(context.Context, ...eventlog.Event) error { return nil }
