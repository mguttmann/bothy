// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	stdsync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// otherToken is the credential of the second test principal.
const otherToken = "Bearer other"

// twoUsers accepts the good token of the harness and otherToken as two principals.
type twoUsers struct{ other ids.ID }

func (a twoUsers) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Authorization == otherToken {
		return actor.Actor{ID: a.other, Kind: actor.KindUser}, nil
	}
	return httpservertest.FakeAuthenticator{}.Authenticate(ctx, c)
}

func (twoUsers) Revalidate(context.Context, actor.Actor) error { return nil }

// httpEnv is the full chain of httpserver with the Idempotency-Key stage over the store of sync
// on PostgreSQL 18; handlers write effect rows into test_effects in the ambient transaction.
type httpEnv struct {
	*keyEnv
	h     *httpservertest.Harness
	other ids.ID
}

// keyedHandler is the handler of a test route; it gets the env to write effects.
type keyedHandler func(e *httpEnv, w http.ResponseWriter, r *http.Request)

// httpOptions change newHTTPEnv.
type httpOptions struct {
	lockTimeout time.Duration
	// rateLimit, when set, is installed as StageRateLimit.
	rateLimit func(http.Handler) http.Handler
}

const (
	itemsPath  = "/api/v1/groups/{group_id}/items"
	updatePath = "/api/v1/items/{item_id}"
)

func newHTTPEnv(t *testing.T, o httpOptions, create, update keyedHandler) *httpEnv {
	t.Helper()
	if o.lockTimeout == 0 {
		o.lockTimeout = 5 * time.Second
	}
	e := &httpEnv{keyEnv: newKeyEnv(t, o.lockTimeout, 2)}
	super := e.db.SuperConn(t)
	ctx := context.Background()
	for _, stmt := range []string{
		"CREATE TABLE test_effects (id uuid PRIMARY KEY, n integer NOT NULL DEFAULT 0)",
		"GRANT SELECT, INSERT ON test_effects TO bothy_app",
	} {
		if _, err := super.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := super.Exec(ctx, `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', 'harness', 'Harness')`,
		httpservertest.FakeUserID.String()); err != nil {
		t.Fatal(err)
	}
	e.other = e.principal(t)
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour}, e.store, e.uow, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	stages := map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage}
	if o.rateLimit != nil {
		stages[httpserver.StageRateLimit] = o.rateLimit
	}
	var routes []httpservertest.Route
	wrap := func(action string, h keyedHandler) http.HandlerFunc {
		return httpservertest.Authorized(action, func(w http.ResponseWriter, r *http.Request) { h(e, w, r) })
	}
	if create != nil {
		routes = append(routes, httpservertest.Route{Op: httpservertest.KeyedOp("createItem", http.MethodPost, itemsPath, "item.create", http.StatusCreated), Handler: wrap("item.create", create)})
	}
	if update != nil {
		op := httpservertest.KeyedOp("updateItem", http.MethodPatch, updatePath, "item.update", http.StatusOK)
		op.IfMatch = httpserver.IfMatchRequired
		routes = append(routes, httpservertest.Route{Op: op, Handler: wrap("item.update", update)})
	}
	e.h = httpservertest.New(t, httpservertest.Options{Routes: routes, Stages: stages, Authenticator: twoUsers{other: e.other}})
	return e
}

// effect inserts one effect row in a nested unit of work, as a use case does, and returns its ID.
func (e *httpEnv) effect(r *http.Request) (ids.ID, error) {
	id := ids.New()
	err := e.uow.Do(r.Context(), func(ctx context.Context) error {
		tx, err := pg.Tx(ctx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO test_effects (id) VALUES ($1)", id.String())
		return err
	})
	return id, err
}

// effects returns the number of effect rows.
func (e *httpEnv) effects(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT count(*) FROM test_effects").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// send posts body to path with key and the given token.
func (e *httpEnv) send(method, path, key, body, token string, headers ...string) *httpservertest.Response {
	return e.h.Do(e.h.NewRequest(method, path, body, append([]string{"Authorization", token, "Idempotency-Key", key}, headers...)...))
}

func groupPath() string { return "/api/v1/groups/" + ids.New().String() + "/items" }

// created writes an effect and answers 201 with the effect ID, version 3 and an ETag.
func created(e *httpEnv, w http.ResponseWriter, r *http.Request) {
	id, err := e.effect(r)
	if err != nil {
		httpserver.WriteProblem(w, r, err)
		return
	}
	w.Header().Set("ETag", httpserver.FormatETag(3))
	httpservertest.JSON(w, http.StatusCreated, map[string]any{"id": id.String(), "name": "a", "version": 3})
}

func TestReplay_ReturnsStoredResponseWithHeader(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	path, key := groupPath(), ids.New().String()
	first := e.send(http.MethodPost, path, key, `{"a":1}`, httpservertest.GoodToken)
	if first.StatusCode != http.StatusCreated || first.Header.Get("Idempotent-Replay") != "" {
		t.Fatalf("first: %d %s", first.StatusCode, first.Body)
	}
	second := e.send(http.MethodPost, path, key, `{"a":1}`, httpservertest.GoodToken)
	if second.StatusCode != http.StatusCreated || second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("second: %d %q %s", second.StatusCode, second.Header.Get("Idempotent-Replay"), second.Body)
	}
	var a, b map[string]any
	if json.Unmarshal(first.Body, &a) != nil || json.Unmarshal(second.Body, &b) != nil || a["id"] != b["id"] || len(a) != len(b) {
		t.Errorf("bodies differ: %s vs %s", first.Body, second.Body)
	}
	if got := second.Header.Get("ETag"); got != `"3"` {
		t.Errorf("replay ETag %q, want \"3\" from version", got)
	}
	if second.Header.Get("Content-Type") != "application/json" || second.Header.Get("Correlation-Id") == first.Header.Get("Correlation-Id") {
		t.Errorf("replay Content-Type %q, Correlation-Id %q (first %q)", second.Header.Get("Content-Type"), second.Header.Get("Correlation-Id"), first.Header.Get("Correlation-Id"))
	}
	if n := e.effects(t); n != 1 {
		t.Errorf("%d effects, want 1", n)
	}
	var resource string
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT resource_id::text FROM idempotency_keys").Scan(&resource); err != nil || resource != a["id"] {
		t.Errorf("resource_id %q (%v), want the top-level id %v", resource, err, a["id"])
	}
}

func TestReuse_DifferentBody422(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	path, key := groupPath(), ids.New().String()
	e.send(http.MethodPost, path, key, `{"a":1}`, httpservertest.GoodToken)
	resp := e.send(http.MethodPost, path, key, `{"a":2}`, httpservertest.GoodToken)
	if resp.StatusCode != http.StatusUnprocessableEntity || resp.Problem(t)["code"] != "idempotency_key_reuse" {
		t.Errorf("%d %s, want 422 idempotency_key_reuse", resp.StatusCode, resp.Body)
	}
	if n := e.effects(t); n != 1 {
		t.Errorf("%d effects, want 1", n)
	}
}

func TestReuse_DifferentPath422(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	key := ids.New().String()
	e.send(http.MethodPost, groupPath(), key, `{"a":1}`, httpservertest.GoodToken)
	resp := e.send(http.MethodPost, groupPath(), key, `{"a":1}`, httpservertest.GoodToken)
	if resp.StatusCode != http.StatusUnprocessableEntity || resp.Problem(t)["code"] != "idempotency_key_reuse" {
		t.Errorf("%d %s, want 422 idempotency_key_reuse", resp.StatusCode, resp.Body)
	}
}

func TestCanonical_EquivalentJSONReplays(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	path, key := groupPath(), ids.New().String()
	e.send(http.MethodPost, path, key, `{"name":"Ab","tags":["x"],"n":1}`, httpservertest.GoodToken)
	resp := e.send(http.MethodPost, path, key, "{ \"n\" : 1,\n \"tags\":[ \"\\u0078\" ], \"name\":\"\\u0041b\" }", httpservertest.GoodToken)
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("%d %s, want the replay", resp.StatusCode, resp.Body)
	}
	if n := e.effects(t); n != 1 {
		t.Errorf("%d effects, want 1", n)
	}
}

// TestInProgress_409AfterLockTimeout: while the first request runs, a repeat waits at the key for
// database.lock_timeout and gets 409 request_in_progress without a row; after the first
// completes, a third request replays.
func TestInProgress_409AfterLockTimeout(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	var once stdsync.Once
	e := newHTTPEnv(t, httpOptions{lockTimeout: 300 * time.Millisecond}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-release
		created(e, w, r)
	}, nil)
	path, key := groupPath(), ids.New().String()
	first := make(chan *httpservertest.Response, 1)
	go func() { first <- e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken) }()
	<-started
	resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	if resp.StatusCode != http.StatusConflict || resp.Problem(t)["code"] != "request_in_progress" {
		t.Errorf("second: %d %s, want 409 request_in_progress", resp.StatusCode, resp.Body)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("%d committed rows while the first runs, want 0", n)
	}
	close(release)
	if r := <-first; r.StatusCode != http.StatusCreated {
		t.Fatalf("first: %d %s", r.StatusCode, r.Body)
	}
	third := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	if third.StatusCode != http.StatusCreated || third.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("third: %d %s, want the replay", third.StatusCode, third.Body)
	}
	if n := e.effects(t); n != 1 {
		t.Errorf("%d effects, want 1", n)
	}
}

func TestConcurrent_ExactlyOneEffect(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		created(e, w, r)
	}, nil)
	path, key := groupPath(), ids.New().String()
	const n = 16
	var wg stdsync.WaitGroup
	var originals, replays, inProgress, other atomic.Int32
	for range n {
		wg.Go(func() {
			resp := e.send(http.MethodPost, path, key, `{"x":1}`, httpservertest.GoodToken)
			switch {
			case resp.StatusCode == http.StatusCreated && resp.Header.Get("Idempotent-Replay") == "true":
				replays.Add(1)
			case resp.StatusCode == http.StatusCreated:
				originals.Add(1)
			case resp.StatusCode == http.StatusConflict:
				inProgress.Add(1)
			default:
				other.Add(1)
				t.Errorf("unexpected %d %s", resp.StatusCode, resp.Body)
			}
		})
	}
	wg.Wait()
	if originals.Load() != 1 || other.Load() != 0 || e.effects(t) != 1 {
		t.Errorf("originals %d, replays %d, in progress %d, other %d, effects %d; want exactly one effect",
			originals.Load(), replays.Load(), inProgress.Load(), other.Load(), e.effects(t))
	}
}

func TestNotStored_401StepUpThenRetrySucceeds(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			httpserver.WriteProblem(w, r, errs.New(errs.CodeStepUpRequired, ""))
			return
		}
		created(e, w, r)
	}, nil)
	path, key := groupPath(), ids.New().String()
	if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("first: %d %s", resp.StatusCode, resp.Body)
	}
	if n := e.count(t); n != 0 {
		t.Fatalf("%d rows after 401, want 0", n)
	}
	if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "" {
		t.Fatalf("after step-up: %d %s", resp.StatusCode, resp.Body)
	}
	if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("third: %d, want the stored 2xx", resp.StatusCode)
	}
	if n := e.effects(t); n != 1 || calls.Load() != 2 {
		t.Errorf("effects %d, calls %d; want one effect from the second call", n, calls.Load())
	}
}

func TestNotStored_5xx(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if _, err := e.effect(r); err != nil {
			httpserver.WriteProblem(w, r, err)
			return
		}
		httpserver.WriteProblem(w, r, errs.New(errs.CodeUnavailable, ""))
	}, nil)
	path, key := groupPath(), ids.New().String()
	for range 2 {
		if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%d %s", resp.StatusCode, resp.Body)
		}
	}
	if e.count(t) != 0 || e.effects(t) != 0 || calls.Load() != 2 {
		t.Errorf("rows %d, effects %d, calls %d; want nothing stored and both calls run", e.count(t), e.effects(t), calls.Load())
	}
}

func TestNotStored_429(t *testing.T) {
	t.Parallel()
	var limited atomic.Bool
	limited.Store(true)
	limit := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limited.Swap(false) {
				httpserver.WriteProblem(w, r, errs.New(errs.CodeRateLimited, "").WithRetryAfter(time.Second))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	e := newHTTPEnv(t, httpOptions{rateLimit: limit}, created, nil)
	path, key := groupPath(), ids.New().String()
	if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusTooManyRequests || e.count(t) != 0 {
		t.Fatalf("first: %d, rows %d; want 429 without a row", resp.StatusCode, e.count(t))
	}
	if resp := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "" {
		t.Errorf("after the throttle: %d %s, want the first execution", resp.StatusCode, resp.Body)
	}
}

func TestNotStored_428(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	e := newHTTPEnv(t, httpOptions{}, nil, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		httpservertest.JSON(w, http.StatusOK, map[string]any{"version": 2})
	})
	path, key := "/api/v1/items/"+ids.New().String(), ids.New().String()
	if resp := e.send(http.MethodPatch, path, key, `{}`, httpservertest.GoodToken); resp.StatusCode != http.StatusPreconditionRequired {
		t.Fatalf("without If-Match: %d %s", resp.StatusCode, resp.Body)
	}
	if e.count(t) != 0 {
		t.Fatalf("%d rows after 428, want 0", e.count(t))
	}
	resp := e.send(http.MethodPatch, path, key, `{}`, httpservertest.GoodToken, "If-Match", `"1"`)
	if resp.StatusCode != http.StatusOK || calls.Load() != 1 {
		t.Errorf("with If-Match: %d, calls %d", resp.StatusCode, calls.Load())
	}
	replay := e.send(http.MethodPatch, path, key, `{}`, httpservertest.GoodToken, "If-Match", `"1"`)
	if replay.Header.Get("Idempotent-Replay") != "true" || replay.Header.Get("ETag") != `"2"` {
		t.Errorf("replay %d, ETag %q", replay.StatusCode, replay.Header.Get("ETag"))
	}
}

func TestStoredFinal_404ReplayedWithoutEffect(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
	}, nil)
	path, key := groupPath(), ids.New().String()
	first := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	second := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	if first.StatusCode != http.StatusNotFound || second.StatusCode != http.StatusNotFound || second.Header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("first %d, second %d %q", first.StatusCode, second.StatusCode, second.Header.Get("Idempotent-Replay"))
	}
	if second.Header.Get("Content-Type") != "application/problem+json" || calls.Load() != 1 || e.effects(t) != 0 {
		t.Errorf("Content-Type %q, calls %d, effects %d", second.Header.Get("Content-Type"), calls.Load(), e.effects(t))
	}
}

func TestStoredFinal_409RollsBackEffect(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		if _, err := e.effect(r); err != nil {
			httpserver.WriteProblem(w, r, err)
			return
		}
		httpserver.WriteProblem(w, r, errs.New(errs.CodeConflict, "").WithReason("channel_archived"))
	}, nil)
	path, key := groupPath(), ids.New().String()
	first := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	if first.StatusCode != http.StatusConflict {
		t.Fatalf("first %d %s", first.StatusCode, first.Body)
	}
	if n := e.effects(t); n != 0 {
		t.Errorf("%d effects after the conflict, want the effect rolled back", n)
	}
	second := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	p1, p2 := first.Problem(t), second.Problem(t)
	if second.Header.Get("Idempotent-Replay") != "true" || p2["reason"] != "channel_archived" {
		t.Errorf("replay %s", second.Body)
	}
	if p2["correlation_id"] != p1["correlation_id"] || second.Header.Get("Correlation-Id") == first.Header.Get("Correlation-Id") {
		t.Errorf("correlation: stored %v vs replay %v, headers %q vs %q; want the original in the body and the current in the header",
			p1["correlation_id"], p2["correlation_id"], first.Header.Get("Correlation-Id"), second.Header.Get("Correlation-Id"))
	}
}

func TestPerPrincipal_SameKeyIndependent(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	path, key := groupPath(), ids.New().String()
	a := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	b := e.send(http.MethodPost, path, key, `{"other":true}`, otherToken)
	if a.StatusCode != http.StatusCreated || b.StatusCode != http.StatusCreated || b.Header.Get("Idempotent-Replay") != "" || e.effects(t) != 2 {
		t.Errorf("a %d, b %d %q, effects %d; want two independent executions", a.StatusCode, b.StatusCode, b.Header.Get("Idempotent-Replay"), e.effects(t))
	}
}

func TestOneTimeSecret_NotStoredReplay409(t *testing.T) {
	t.Parallel()
	const secret = "s3cr3t-0f-the-test"
	e := newHTTPEnv(t, httpOptions{}, func(e *httpEnv, w http.ResponseWriter, r *http.Request) {
		id, err := e.effect(r)
		if err != nil {
			httpserver.WriteProblem(w, r, err)
			return
		}
		idempotency.MarkOneTimeSecret(r.Context())
		httpservertest.JSON(w, http.StatusCreated, map[string]any{"id": id.String(), "secret": secret})
	}, nil)
	path, key := groupPath(), ids.New().String()
	first := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	if first.StatusCode != http.StatusCreated || !strings.Contains(string(first.Body), secret) {
		t.Fatalf("first %d %s", first.StatusCode, first.Body)
	}
	var stored string
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT response_status::text || ' ' || response_body::text FROM idempotency_keys").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, secret) || stored != `201 {"one_time_secret": true}` {
		t.Errorf("stored %q, want the placeholder without the secret", stored)
	}
	second := e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	p := second.Problem(t)
	if second.StatusCode != http.StatusConflict || p["reason"] != "one_time_secret" || p["hint"] != "create a new one" ||
		second.Header.Get("Idempotent-Replay") != "true" || strings.Contains(string(second.Body), secret) || e.effects(t) != 1 {
		t.Errorf("replay %d %s %q, effects %d", second.StatusCode, second.Body, second.Header.Get("Idempotent-Replay"), e.effects(t))
	}
}

func TestExpired_TreatedAsAbsent(t *testing.T) {
	t.Parallel()
	e := newHTTPEnv(t, httpOptions{}, created, nil)
	path, key := groupPath(), ids.New().String()
	e.send(http.MethodPost, path, key, `{}`, httpservertest.GoodToken)
	e.clock.Advance(time.Hour)
	resp := e.send(http.MethodPost, path, key, `{"changed":true}`, httpservertest.GoodToken)
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Idempotent-Replay") != "" || e.effects(t) != 2 {
		t.Errorf("%d %q, effects %d; want a new execution after sync.idempotency_key_ttl", resp.StatusCode, resp.Header.Get("Idempotent-Replay"), e.effects(t))
	}
}
