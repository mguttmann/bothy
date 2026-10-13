// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// sqlError is a database error with a SQLSTATE, as pgconn.PgError reports it.
type sqlError struct{ code string }

func (e *sqlError) Error() string    { return "database error " + e.code }
func (e *sqlError) SQLState() string { return e.code }

// memKey is the primary key of the in-memory store.
type memKey struct{ principal, key ids.ID }

// memTx holds the writes of one attempt of memUoW.
type memTx struct{ writes map[memKey]idempotency.Entry }

type memTxKey struct{}

// memStore is an idempotency.Store in memory; Complete writes into the attempt of memUoW, which
// applies it at the commit.
type memStore struct {
	mu     stdsync.Mutex
	rows   map[memKey]idempotency.Entry
	begins int
	reqs   []idempotency.Request
}

func newMemStore() *memStore { return &memStore{rows: map[memKey]idempotency.Entry{}} }

func (s *memStore) Claim(_ context.Context, req idempotency.Request) (idempotency.Entry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.begins++
	s.reqs = append(s.reqs, req)
	e, ok := s.rows[memKey{req.PrincipalID, req.Key}]
	return e, ok, nil
}

func (s *memStore) Complete(ctx context.Context, req idempotency.Request, resp idempotency.Response) error {
	tx, ok := ctx.Value(memTxKey{}).(*memTx)
	if !ok {
		return errors.New("Complete outside the unit")
	}
	tx.writes[memKey{req.PrincipalID, req.Key}] = idempotency.Entry{Response: resp, SHA256: req.SHA256, Completed: true}
	return nil
}

func (s *memStore) SaveFinal(ctx context.Context, req idempotency.Request, resp idempotency.Response) error {
	if _, in := ctx.Value(memTxKey{}).(*memTx); in {
		return errors.New("SaveFinal inside the unit")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := memKey{req.PrincipalID, req.Key}
	if _, ok := s.rows[k]; !ok {
		s.rows[k] = idempotency.Entry{Response: resp, SHA256: req.SHA256, Completed: true}
	}
	return nil
}

func (s *memStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}

// memUoW runs fn like platform/pg: an error with SQLSTATE 40P01 retries the attempt, at most
// three attempts; failCommits makes that many commits fail with 40P01 after fn succeeded.
type memUoW struct {
	store       *memStore
	failCommits int
	attempts    int
	// late records whether the last Do allowed a late rights change.
	late bool
}

func (u *memUoW) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...txn.Option) error {
	u.late = txn.ApplyOptions(opts...).LateRightsChange
	for {
		u.attempts++
		tx := &memTx{writes: map[memKey]idempotency.Entry{}}
		err := fn(context.WithValue(ctx, memTxKey{}, tx))
		if err == nil && u.failCommits > 0 {
			u.failCommits--
			err = &sqlError{code: "40P01"}
		}
		var se interface{ SQLState() string }
		if errors.As(err, &se) && se.SQLState() == "40P01" && u.attempts < 3 {
			continue
		}
		if err != nil {
			return err
		}
		u.store.mu.Lock()
		for k, v := range tx.writes {
			u.store.rows[k] = v
		}
		u.store.mu.Unlock()
		return nil
	}
}

// stageEnv is a harness with the Idempotency-Key stage over the in-memory store.
type stageEnv struct {
	h     *httpservertest.Harness
	store *memStore
	uow   *memUoW
	clock *clock.Fake
}

const itemsPath = "/api/v1/groups/{group_id}/items"

func createItem() httpserver.Operation {
	return httpservertest.KeyedOp("createItem", http.MethodPost, itemsPath, "item.create", http.StatusCreated)
}

func newStageEnv(t *testing.T, cfg *httpserver.ChainConfig, routes ...httpservertest.Route) *stageEnv {
	t.Helper()
	store := newMemStore()
	e := &stageEnv{store: store, uow: &memUoW{store: store}, clock: clock.NewFake(time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC))}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour}, store, e.uow, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	e.h = httpservertest.New(t, httpservertest.Options{Config: cfg, Routes: routes,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage}})
	return e
}

// post sends a keyed request with the good token.
func (e *stageEnv) post(path, key, body string) *httpservertest.Response {
	return e.h.Do(e.h.NewRequest(http.MethodPost, path, body, "Authorization", httpservertest.GoodToken, "Idempotency-Key", key))
}

func TestNewIdempotencyStage_RefusesMissingParts(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	if _, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: 0}, store, &memUoW{store: store}, clock.System()); err == nil {
		t.Error("a TTL of 0 was accepted")
	}
	if _, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour}, nil, nil, nil); err == nil {
		t.Error("a stage without store and unit of work was built")
	}
}

func TestFingerprint_Definition(t *testing.T) {
	t.Parallel()
	sum := func(s string) [32]byte { return sha256.Sum256([]byte(s)) }
	cases := []struct {
		name, method, path, body, want string
	}{
		{"canonical JSON", "post", "/api/v1/groups/a%2Fb/items", "{ \"b\": 1, \"a\": \"\\u0041\" }", "POST\n/api/v1/groups/a%2Fb/items\n{\"a\":\"A\",\"b\":1}"},
		{"no body", "POST", "/api/v1/items/x/archive", "", "POST\n/api/v1/items/x/archive\n"},
		{"not JSON stays raw", "PATCH", "/api/v1/items/x", "{\"a\":", "PATCH\n/api/v1/items/x\n{\"a\":"},
		{"duplicate member stays raw", "POST", "/p", "{\"a\":1,\"a\":2}", "POST\n/p\n{\"a\":1,\"a\":2}"},
	}
	for _, c := range cases {
		if got := httpserver.Fingerprint(c.method, c.path, []byte(c.body)); got != sum(c.want) {
			t.Errorf("%s: fingerprint differs from SHA-256 of %q", c.name, c.want)
		}
	}
	if httpserver.Fingerprint("POST", "/p", []byte(`{"a":1}`)) == httpserver.Fingerprint("POST", "/q", []byte(`{"a":1}`)) {
		t.Error("the path does not count")
	}
}

func TestETag_FromVersion(t *testing.T) {
	t.Parallel()
	for body, want := range map[string]string{
		`{"id":"x","version":7}`: `"7"`,
		`{"version":0}`:          `"0"`,
		`{"version":-1}`:         "",
		`{"version":1.5}`:        "",
		`{"version":"7"}`:        "",
		`{"id":"x"}`:             "",
		`[1]`:                    "",
		``:                       "",
	} {
		if got := httpserver.ReplayETag([]byte(body)); got != want {
			t.Errorf("ReplayETag(%s) = %q, want %q", body, got, want)
		}
	}
}

func TestInvalidKey_400(t *testing.T) {
	t.Parallel()
	runs := 0
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		runs++
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
	})})
	path := "/api/v1/groups/" + ids.New().String() + "/items"
	for _, key := range []string{"abc", "0192f4a0-7b1c-4577-8bf0-c4f4ee4f887e"} {
		resp := e.post(path, key, `{}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("key %q: status %d, want 400 (%s)", key, resp.StatusCode, resp.Body)
		}
		p := resp.Problem(t)
		fields, _ := p["errors"].([]any)
		f, _ := fields[0].(map[string]any)
		if p["code"] != "validation_failed" || f["pointer"] != "#header/Idempotency-Key" || f["code"] != "invalid_format" {
			t.Errorf("key %q: problem %v", key, p)
		}
	}
	if runs != 0 || e.store.begins != 0 || e.store.count() != 0 {
		t.Errorf("runs %d, begins %d, rows %d; want nothing for an invalid key", runs, e.store.begins, e.store.count())
	}
}

func TestIgnored_OperationWithoutKeyParameter(t *testing.T) {
	t.Parallel()
	runs := 0
	op := createItem()
	op.IdempotencyKey = false
	e := newStageEnv(t, nil, httpservertest.Route{Op: op, Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		runs++
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
	})})
	path, key := "/api/v1/groups/"+ids.New().String()+"/items", ids.New().String()
	for range 2 {
		if resp := e.post(path, key, `{}`); resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "" {
			t.Fatalf("status %d, replay %q", resp.StatusCode, resp.Header.Get("Idempotent-Replay"))
		}
	}
	if runs != 2 || e.store.begins != 0 {
		t.Errorf("runs %d, begins %d; want the header ignored", runs, e.store.begins)
	}
}

func TestIgnored_AnonymousActor(t *testing.T) {
	t.Parallel()
	runs := 0
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		runs++
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
	})})
	path, key := "/api/v1/groups/"+ids.New().String()+"/items", ids.New().String()
	for range 2 {
		resp := e.h.Do(e.h.NewRequest(http.MethodPost, path, `{}`, "Idempotency-Key", key))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
		}
	}
	if runs != 2 || e.store.begins != 0 {
		t.Errorf("runs %d, begins %d; want the header ignored for the anonymous actor", runs, e.store.begins)
	}
}

// TestRetry_ReinvokesWithFreshBuffer: the unit of work retries after a deadlock at the commit;
// the second attempt reads the whole body again and only its response is sent and stored.
func TestRetry_ReinvokesWithFreshBuffer(t *testing.T) {
	t.Parallel()
	var bodies []string
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("X-Attempt", strings.Repeat("a", len(bodies)))
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"attempt": len(bodies)})
	})})
	e.uow.failCommits = 1
	path, key := "/api/v1/groups/"+ids.New().String()+"/items", ids.New().String()
	resp := e.post(path, key, `{"n":1}`)
	if resp.StatusCode != http.StatusCreated || strings.TrimSpace(string(resp.Body)) != `{"attempt":2}` || resp.Header.Get("X-Attempt") != "aa" {
		t.Fatalf("status %d, body %s, X-Attempt %q; want only the second attempt", resp.StatusCode, resp.Body, resp.Header.Get("X-Attempt"))
	}
	if len(bodies) != 2 || bodies[0] != `{"n":1}` || bodies[1] != `{"n":1}` {
		t.Errorf("the handler read %q, want the whole body in both attempts", bodies)
	}
	replay := e.post(path, key, `{"n":1}`)
	var got map[string]any
	if err := json.Unmarshal(replay.Body, &got); err != nil || got["attempt"] != float64(2) || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("replay %d %s, want the stored second attempt", replay.StatusCode, replay.Body)
	}
}

// TestRetry_HandlerDeadlockReachesTheUnitOfWork: a deadlock that the handler reports through
// WriteProblem (its nested unit of work joined the stage's one) makes the stage's unit retry,
// as without the stage (02 "Lock order").
func TestRetry_HandlerDeadlockReachesTheUnitOfWork(t *testing.T) {
	t.Parallel()
	runs := 0
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, r *http.Request) {
		runs++
		if runs == 1 {
			httpserver.WriteProblem(w, r, &sqlError{code: "40P01"})
			return
		}
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
	})})
	resp := e.post("/api/v1/groups/"+ids.New().String()+"/items", ids.New().String(), `{}`)
	if resp.StatusCode != http.StatusCreated || runs != 2 || e.store.count() != 1 {
		t.Errorf("status %d, runs %d, rows %d; want the retried success stored", resp.StatusCode, runs, e.store.count())
	}
}

// TestNotStored_413BeforeTheFingerprint: a body over http.max_json_body_bytes is refused before
// the store is asked, because a truncated body has no fingerprint.
func TestNotStored_413BeforeTheFingerprint(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONBodyBytes = 16
	runs := 0
	e := newStageEnv(t, &cfg, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		runs++
		w.WriteHeader(http.StatusCreated)
	})})
	req := e.h.NewRequest(http.MethodPost, "/api/v1/groups/"+ids.New().String()+"/items", `{"a":"`+strings.Repeat("x", 64)+`"}`,
		"Authorization", httpservertest.GoodToken, "Idempotency-Key", ids.New().String())
	req.ContentLength = -1 // chunked: the limit applies while reading
	resp := e.h.Do(req)
	if resp.StatusCode != http.StatusRequestEntityTooLarge || runs != 0 || e.store.begins != 0 {
		t.Errorf("status %d, runs %d, begins %d; want 413 without store", resp.StatusCode, runs, e.store.begins)
	}
}

// TestRefusedSuccess_RollsBack: a success body the response writer would refuse (a null array
// element, ADR 0053) rolls the unit back, so the client never gets 500 for a committed effect.
func TestRefusedSuccess_RollsBack(t *testing.T) {
	t.Parallel()
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"a":[null]}`))
	})})
	resp := e.post("/api/v1/groups/"+ids.New().String()+"/items", ids.New().String(), `{}`)
	if resp.StatusCode != http.StatusInternalServerError || e.store.count() != 0 {
		t.Errorf("status %d, rows %d; want 500 and nothing stored", resp.StatusCode, e.store.count())
	}
}

// TestUnprovenSuccess_RollsBack: a keyed success without a check of the route's action (02 9.3)
// is refused by the response writer; the stage rolls the unit back and stores nothing, so a
// retry never gets the held-back body as a replay.
func TestUnprovenSuccess_RollsBack(t *testing.T) {
	t.Parallel()
	runs := 0
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: func(w http.ResponseWriter, _ *http.Request) {
		runs++
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"secret": "held back"})
	}})
	path, key := "/api/v1/groups/"+ids.New().String()+"/items", ids.New().String()
	for range 2 {
		resp := e.post(path, key, `{}`)
		if resp.StatusCode != http.StatusInternalServerError || strings.Contains(string(resp.Body), "held back") {
			t.Fatalf("status %d %s; want 500 without the body", resp.StatusCode, resp.Body)
		}
	}
	if runs != 2 || e.store.count() != 0 {
		t.Errorf("runs %d, rows %d; want nothing stored and the handler run again", runs, e.store.count())
	}
}

// TestStage_AllowsLateRightsChange: the stage opens its unit with txn.AllowLateRightsChange, so a
// keyed use case may declare its rights change in its nested unit (docs/DECISIONS.md M1-31).
func TestStage_AllowsLateRightsChange(t *testing.T) {
	t.Parallel()
	e := newStageEnv(t, nil, httpservertest.Route{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
	})})
	if r := e.post("/api/v1/groups/"+ids.New().String()+"/items", ids.New().String(), `{}`); r.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", r.StatusCode)
	}
	if !e.uow.late {
		t.Fatal("the stage's unit of work does not allow a late rights change")
	}
}

// inactiveAuth authenticates every request with credentials as a principal whose API token the
// authenticator found revoked or expired.
type inactiveAuth struct{ id ids.ID }

func (a inactiveAuth) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	if !c.Present() {
		return actor.Anonymous(), nil
	}
	return actor.Actor{ID: a.id, Kind: actor.KindAgent, TokenID: ids.New(), CredentialInactive: true}, nil
}

func (inactiveAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// TestIgnored_InactiveCredential (M1-44, red team round 1): for a principal whose credential the
// authenticator found revoked or expired the stage neither claims nor replays; the request goes on
// to authorize, which refuses it.
func TestIgnored_InactiveCredential(t *testing.T) {
	t.Parallel()
	runs := 0
	store := newMemStore()
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour}, store, &memUoW{store: store}, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	h := httpservertest.New(t, httpservertest.Options{Authenticator: inactiveAuth{id: ids.New()},
		Routes: []httpservertest.Route{{Op: createItem(), Handler: httpservertest.Authorized("item.create", func(w http.ResponseWriter, _ *http.Request) {
			runs++
			httpservertest.JSON(w, http.StatusCreated, map[string]any{"ok": true})
		})}},
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage}})
	path, key := "/api/v1/groups/"+ids.New().String()+"/items", ids.New().String()
	for range 2 {
		resp := h.Do(h.NewRequest(http.MethodPost, path, `{}`, "Authorization", "Bearer dead", "Idempotency-Key", key))
		if resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "" {
			t.Fatalf("status %d, replay %q: %s", resp.StatusCode, resp.Header.Get("Idempotent-Replay"), resp.Body)
		}
	}
	if runs != 2 || store.begins != 0 {
		t.Errorf("runs %d, begins %d; want the header ignored for an inactive credential", runs, store.begins)
	}
}
