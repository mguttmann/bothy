// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// repoData is the data set of the repository, seen from this package.
var repoData = filepath.Join("..", "..", "..", "..", "..", "infra", "demo")

// loadRepo loads the data set of the repository.
func loadRepo(t *testing.T) *DataSet {
	t.Helper()
	ds, err := Load(repoData)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// TestDataSetOfTheRepository: the data set validates, names the people, spaces and roles of the
// brief and keeps every size in its values (brief rules 8 and 9).
func TestDataSetOfTheRepository(t *testing.T) {
	t.Parallel()
	ds := loadRepo(t)
	var people, spaces []string
	for _, p := range ds.People {
		people = append(people, p.DisplayName)
	}
	for _, s := range ds.Spaces {
		spaces = append(spaces, s.Name)
	}
	if !reflect.DeepEqual(people, []string{"Alex", "Sam", "Robin"}) || !reflect.DeepEqual(spaces, []string{"Support", "Marketing", "Engineering", "Research"}) {
		t.Fatalf("people %v, spaces %v", people, spaces)
	}
	roles := map[string]bool{}
	for _, a := range ds.Agents {
		roles[a.Role] = true
	}
	if !roles[RoleWorker] || !roles[RoleReviewer] || !roles[RoleWatcher] {
		t.Fatalf("roles %v: every role needs an agent", roles)
	}
	if ds.Owner().Handle != "alex" || ds.Values.WarmupPrompts == 0 || time.Duration(ds.Values.Pace) <= 0 {
		t.Fatalf("owner %s, values %+v", ds.Owner().Handle, ds.Values)
	}
}

// TestPlanDeterministic (16 16.4): the same seed draws the same plan, another seed another one,
// and every planned person may trigger in the space of the prompt.
func TestPlanDeterministic(t *testing.T) {
	t.Parallel()
	ds := loadRepo(t)
	a, b, c := ds.Plan(7), ds.Plan(7), ds.Plan(8)
	if !reflect.DeepEqual(a, b) || reflect.DeepEqual(a, c) || len(a) != ds.Values.WarmupPrompts {
		t.Fatalf("plans: %d, equal %v, other equal %v", len(a), reflect.DeepEqual(a, b), reflect.DeepEqual(a, c))
	}
	for _, p := range a {
		allowed := ds.Space(p.Space).triggerPeople()
		if !strings.Contains(strings.Join(allowed, ","), p.Person) {
			t.Fatalf("%s may not trigger in %s", p.Person, p.Space)
		}
	}
}

// TestDataSetReferences: every rule the schema cannot express is an invalid data set.
func TestDataSetReferences(t *testing.T) {
	t.Parallel()
	for name, mutate := range map[string]func(*DataSet){
		"two owners":          func(ds *DataSet) { ds.People[1].Owner = true },
		"no owner":            func(ds *DataSet) { ds.People[0].Owner = false },
		"person twice":        func(ds *DataSet) { ds.People[2].Handle = ds.People[1].Handle },
		"agent is a person":   func(ds *DataSet) { ds.Agents[0].Handle = "sam" },
		"model without price": func(ds *DataSet) { ds.Agents[0].Model = "model-none" },
		"space twice":         func(ds *DataSet) { ds.Spaces[1].Name = ds.Spaces[0].Name },
		"unknown member":      func(ds *DataSet) { ds.Spaces[0].Members[0].Person = "nobody" },
		"unknown agent":       func(ds *DataSet) { ds.Spaces[0].Members[3].Agent = "nobody" },
		"channel twice":       func(ds *DataSet) { ds.Spaces[0].Channels[1].Name = ds.Spaces[0].Channels[0].Name },
		"foreign operator":    func(ds *DataSet) { ds.Spaces[0].Channels[0].Operator = "paper-scout" },
		"foreign author":      func(ds *DataSet) { ds.Spaces[1].Channels[0].Threads[0].Messages[0].Author = "robin" },
		"unknown host person": func(ds *DataSet) { ds.Host.SessionUsers[0].Person = "nobody" },
		"prompt channel":      func(ds *DataSet) { ds.Prompts[0].Channel = "nowhere" },
		"prompt space":        func(ds *DataSet) { ds.Prompts[0].Space = "Nowhere" },
		"prompt no worker":    func(ds *DataSet) { ds.Prompts[0].Space, ds.Prompts[0].Channel = "Engineering", "builds" },
		"no prompts":          func(ds *DataSet) { ds.Prompts = nil },
		"live channel":        func(ds *DataSet) { ds.Live[0].Channel = "nowhere" },
		"live worker":         func(ds *DataSet) { ds.Live[0].Space, ds.Live[0].Channel = "Support", "tickets" },
		"live no trigger":     func(ds *DataSet) { ds.Live[0].Person = "sam" },
	} {
		ds := loadRepo(t)
		mutate(ds)
		if err := ds.check(); !errors.Is(err, ErrInvalidDataSet) {
			t.Errorf("%s: %v, want an invalid data set", name, err)
		}
	}
}

// TestLoadRefusesSchemaErrors: a member the schema does not know, a missing file and a wrong
// duration are errors before any connection.
func TestLoadRefusesSchemaErrors(t *testing.T) {
	t.Parallel()
	schema, err := os.ReadFile(filepath.Join(repoData, SchemaFile)) //nolint:gosec // G304: the data set of the repository
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoData, DataFile)) //nolint:gosec // G304: the data set of the repository
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"unknown member": strings.Replace(string(data), "version: 1", "version: 1\nextra: true", 1),
		"bad duration":   strings.Replace(string(data), "pace: 20s", "pace: soon", 1),
		"no yaml":        "version: [",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, SchemaFile), schema, 0o600); err != nil { //nolint:gosec // G703: a file of t.TempDir
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, DataFile), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); !errors.Is(err, ErrInvalidDataSet) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := Load(t.TempDir()); err == nil || errors.Is(err, ErrInvalidDataSet) {
		t.Errorf("missing files: %v", err)
	}
}

// TestRetryAfter: only whole seconds up to the bound are waited for, at least one second.
func TestRetryAfter(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]time.Duration{"0": time.Second, "3": 3 * time.Second, "120": 2 * time.Minute, "121": 0, "-1": 0, "soon": 0} {
		got, ok := retryAfter(v)
		if got != want || ok != (want > 0) {
			t.Errorf("retryAfter(%q) = %v %v", v, got, ok)
		}
	}
}

// TestTransportURLs: the API address and the public URL are checked; the htu names the public
// origin and the escaped path without query.
func TestTransportURLs(t *testing.T) {
	t.Parallel()
	for _, bad := range [][2]string{{"ftp://x", "https://chat.example.com"}, {"http://127.0.0.1:1", "https://chat.example.com/#x"},
		{"http://127.0.0.1:1", "https://user@chat.example.com"}, {"http://127.0.0.1:1?x=1", "https://chat.example.com"}} {
		if _, err := newTransport(bad[0], bad[1], nil); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
	tr, err := newTransport("http://127.0.0.1:1/", "https://chat.example.com:8443", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := restHTU(tr.public, "/api/v1/threads/a b?x=1"); got != "https://chat.example.com:8443/api/v1/threads/a%20b" {
		t.Fatalf("htu %s", got)
	}
}

// TestSessionRetries: a 401 use_dpop_nonce is repeated with the new nonce, a 429 after its
// Retry-After, a 401 token_expired after a refresh; a refused call is a StatusError with the
// first field error and never the body.
func TestSessionRetries(t *testing.T) {
	t.Parallel()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path+" "+r.Header.Get("Idempotency-Key"))
		problem := func(status int, code string) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "errors": []any{map[string]any{"code": "pattern", "pointer": "/name"}}})
		}
		switch {
		case r.URL.Path == "/api/v1/auth/token/refresh":
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "a2", "refresh_token": "r2"})
		case len(calls) == 1:
			w.Header().Set("DPoP-Nonce", "n1")
			problem(http.StatusUnauthorized, "use_dpop_nonce")
		case len(calls) == 2:
			w.Header().Set("Retry-After", "0")
			problem(http.StatusTooManyRequests, "rate_limited")
		case len(calls) == 3:
			problem(http.StatusUnauthorized, "token_expired")
		case r.URL.Path == "/api/v1/fail":
			problem(http.StatusBadRequest, "validation_failed")
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"x"}`))
		}
	}))
	t.Cleanup(srv.Close)
	slept := 0
	tr, err := newTransport(srv.URL, "https://chat.example.com", func(context.Context, time.Duration) error { slept++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	s, err := newSession(tr)
	if err != nil {
		t.Fatal(err)
	}
	s.token, s.refresh = "a1", "r1"
	var out created
	if err := s.post(t.Context(), "createSpace", "/spaces", map[string]string{"name": "x"}, &out); err != nil || out.ID != "x" {
		t.Fatalf("post: %v %+v", err, out)
	}
	if slept != 1 || s.nonce != "n1" || s.token != "a2" || s.refresh != "r2" {
		t.Fatalf("slept %d, nonce %q, tokens %q %q", slept, s.nonce, s.token, s.refresh)
	}
	key := strings.Fields(calls[0])[1]
	for _, c := range []string{calls[1], calls[2], calls[4]} {
		if !strings.HasSuffix(c, key) {
			t.Fatalf("calls %v: the repeats must keep the Idempotency-Key", calls)
		}
	}
	err = s.post(t.Context(), "fail", "/fail", map[string]string{"secret": "value"}, nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusBadRequest || se.Field != "pattern /name" || strings.Contains(err.Error(), "value") {
		t.Fatalf("error %v", err)
	}
}

// TestExchangeFiles: the exchange directory must be private, its files are written with mode
// 0600 and the paired answer is read once it exists.
func TestExchangeFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil { //nolint:gosec // G302: the test needs an open directory
		t.Fatal(err)
	}
	if err := checkExchangeDir(dir); err == nil {
		t.Fatal("an open exchange directory was accepted")
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: the mode the exchange directory needs
		t.Fatal(err)
	}
	if err := checkExchangeDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := writePrivate(dir, AgentsFile, AgentsDoc{InstanceID: "i"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, AgentsFile))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("agents.json: %v %v", err, fi)
	}
	if _, ok, err := readPaired(dir); ok || err != nil {
		t.Fatalf("paired before the answer: %v %v", ok, err)
	}
	if err := os.WriteFile(filepath.Join(dir, HostPairedFile), []byte(`{"fingerprint":"ABCD-EFGH-JKMN"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, ok, err := readPaired(dir)
	if !ok || err != nil || doc.Fingerprint != "ABCD-EFGH-JKMN" {
		t.Fatalf("paired: %+v %v %v", doc, ok, err)
	}
	ticks := 0
	err = waitFor(t.Context(), func(context.Context, time.Duration) error { ticks++; return nil }, 0, "nothing", func() (bool, error) { return false, nil })
	if !errors.Is(err, errWaitExpired) {
		t.Fatalf("wait: %v after %d ticks", err, ticks)
	}
}
