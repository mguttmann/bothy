// SPDX-License-Identifier: Apache-2.0

//go:build contract_validate

package contractcheck_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/contractcheck"
)

// meBody is a getMe response that matches the schema Me.
const meBody = `{"principal":{"id":"0192f4a0-7b1c-7f89-8d6b-6960453241bc","kind":"user","handle":"alex","display_name":"Alex"},"is_owner":false,"permission_version":1}`

// recorder collects the violations a checker reports instead of exiting.
type recorder struct {
	mu   sync.Mutex
	list []contractcheck.Violation
}

func (r *recorder) fail(v contractcheck.Violation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, v)
}

func (r *recorder) violations() []contractcheck.Violation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]contractcheck.Violation(nil), r.list...)
}

// repoChecker loads the checker of the repository contract with a recording failure handler.
func repoChecker(t *testing.T) (*contractcheck.Checker, *recorder) {
	t.Helper()
	c, err := contractcheck.Load()
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	return c.WithFailure(rec.fail), rec
}

// serve sends one request through c wrapped around h and returns the status the client saw.
func serve(t *testing.T, c *contractcheck.Checker, h http.HandlerFunc, method, path string) int {
	t.Helper()
	srv := httptest.NewServer(c.Wrap(h))
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// meHandler answers getMe with body.
func meHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Correlation-Id", "4bf92f35-77b3-4da6-a3ce-929d0e0e4736")
		_, _ = io.WriteString(w, body)
	}
}

// TestG8_ValidatorAcceptsConformingResponse: a getMe answer that matches the contract passes and
// reaches the client unchanged.
func TestG8_ValidatorAcceptsConformingResponse(t *testing.T) {
	t.Parallel()
	c, rec := repoChecker(t)
	if got := serve(t, c, meHandler(meBody), http.MethodGet, "/api/v1/me"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
	if v := rec.violations(); len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
}

// childEnv switches the test binary into the child of TestG8_ValidatorRejectsWrongResponseField.
const childEnv = "BOTHY_CONTRACTCHECK_CHILD"

// TestG8_ValidatorRejectsWrongResponseField: a getMe handler that omits the required member
// permission_version makes the process exit with code 3 and a report that names the operation,
// the route and the JSON pointer. The child runs the real Middleware, which exits.
func TestG8_ValidatorRejectsWrongResponseField(t *testing.T) {
	if os.Getenv(childEnv) == "1" { //nolint:forbidigo // switch of the own child test process
		c, err := contractcheck.Load()
		if err != nil {
			t.Fatal(err)
		}
		h := c.Wrap(meHandler(strings.Replace(meBody, `,"permission_version":1`, "", 1)))
		srv := httptest.NewServer(h)
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/v1/me", nil)
		if resp, err := srv.Client().Do(req); err == nil {
			_ = resp.Body.Close()
		}
		// The handler goroutine exits the process; this line is reached only when it did not.
		select {}
	}
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestG8_ValidatorRejectsWrongResponseField$", "-test.count=1") //nolint:gosec // G204: the own test binary
	cmd.Env = append(os.Environ(), childEnv+"=1")                                                                                          //nolint:forbidigo // the child test binary needs the environment of go test
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child: %v, want exit code 3; stderr:\n%s", err, stderr.String())
	}
	report := stderr.String()
	for _, want := range []string{"operationId=getMe", "method=GET", "route=/me", "status=200", "pointer=/permission_version", "keyword=required"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}

// TestG8_AttachSkipsForeignContracts: a server whose validator was built from another contract
// than docs/api/openapi.yaml (a test fixture) is not checked.
func TestG8_AttachSkipsForeignContracts(t *testing.T) {
	t.Parallel()
	mw := contractcheck.Attach(sha256.Sum256([]byte(discriminatorSpec)), "/api/v1", nil, nil)
	junk := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"kind":"dog","x":null}`)
	})
	srv := httptest.NewServer(mw(junk))
	t.Cleanup(srv.Close)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/api/v1/pets/1", nil)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// TestG8_SkipsUnmatchedRoutes: requests whose method and route match no operation of the enable
// list (internal listener, unknown paths, a known path with another method) are not checked.
func TestG8_SkipsUnmatchedRoutes(t *testing.T) {
	t.Parallel()
	c, rec := repoChecker(t)
	junk := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"not":"in the contract","x":null}`)
	}
	for _, rt := range [][2]string{{http.MethodGet, "/readyz"}, {http.MethodGet, "/api/v1/no-such-route"}, {http.MethodDelete, "/api/v1/me"}} {
		serve(t, c, junk, rt[0], rt[1])
	}
	if v := rec.violations(); len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
}

// TestG8_RouteDecidesTheOperation: a request that the router gives to a templated operation is
// checked against that operation, also when a literal path of another method exists (GET
// /grants/preview is getGrant; previewGrant is POST).
func TestG8_RouteDecidesTheOperation(t *testing.T) {
	t.Parallel()
	c, rec := repoChecker(t)
	serve(t, c, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"type":"/api/v1/problems/not_found","title":"Not found","status":404,"code":"not_found","correlation_id":"4bf92f35-77b3-4da6-a3ce-929d0e0e4736"}`)
	}, http.MethodGet, "/api/v1/grants/preview")
	if v := rec.violations(); len(v) != 0 {
		t.Fatalf("violations: %v", v)
	}
}

// TestG8_ChecksStatusAndNull: an undeclared status and a JSON null are violations too.
func TestG8_ChecksStatusAndNull(t *testing.T) {
	t.Parallel()
	c, rec := repoChecker(t)
	serve(t, c, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, meBody)
	}, http.MethodGet, "/api/v1/me")
	serve(t, c, meHandler(strings.Replace(meBody, `"handle":"alex"`, `"handle":null`, 1)), http.MethodGet, "/api/v1/me")
	if v := rec.violations(); len(v) != 2 {
		t.Fatalf("violations: %v", v)
	}
}

// discriminatorSpec is a contract with a oneOf over a discriminator and a uuid format.
const discriminatorSpec = `openapi: 3.1.0
info: {title: t, version: "1"}
servers: [{url: /api/v1}]
paths:
  /pets/{pet_id}:
    get:
      operationId: getPet
      parameters: [{name: pet_id, in: path, required: true, schema: {type: string}}]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                oneOf: [{$ref: "#/components/schemas/Cat"}, {$ref: "#/components/schemas/Dog"}]
                discriminator: {propertyName: kind, mapping: {cat: "#/components/schemas/Cat", dog: "#/components/schemas/Dog"}}
components:
  schemas:
    Cat:
      type: object
      required: [kind, id, lives]
      properties: {kind: {type: string, const: cat}, id: {type: string, format: uuid}, lives: {type: integer}}
    Dog:
      type: object
      required: [kind, id, bark]
      properties: {kind: {type: string, const: dog}, id: {type: string, format: uuid}, bark: {type: string}}
`

// TestG8_ValidatesDiscriminatorAndUUID: the pinned library checks a oneOf with discriminator and
// format uuid (brief design note 1).
func TestG8_ValidatesDiscriminatorAndUUID(t *testing.T) {
	t.Parallel()
	c, err := contractcheck.New([]byte(discriminatorSpec), []string{"getPet"})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	c = c.WithFailure(rec.fail)
	pet := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		}
	}
	const id = "0192f4a0-7b1c-7f89-8d6b-6960453241bc"
	serve(t, c, pet(`{"kind":"cat","id":"`+id+`","lives":9}`), http.MethodGet, "/api/v1/pets/1")
	if v := rec.violations(); len(v) != 0 {
		t.Fatalf("valid cat: %v", v)
	}
	serve(t, c, pet(`{"kind":"dog","id":"`+id+`","lives":9}`), http.MethodGet, "/api/v1/pets/1")
	serve(t, c, pet(`{"kind":"cat","id":"not-a-uuid","lives":9}`), http.MethodGet, "/api/v1/pets/1")
	if v := rec.violations(); len(v) != 2 {
		t.Fatalf("want 2 violations (dog without bark, bad uuid), got %v", v)
	}
}

// TestG8_ProductionBuildExcludesValidator: the untagged server binary does not contain the
// package; the tagged build does, which proves the probe sees it.
func TestG8_ProductionBuildExcludesValidator(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	const pkg = "internal/platform/httpserver/contractcheck"
	deps := func(tags ...string) string {
		args := append([]string{"list", "-deps"}, tags...)
		cmd := exec.CommandContext(context.Background(), "go", append(args, "./cmd/bothy")...) //nolint:gosec // G204: fixed go list call
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list %v: %v", tags, err)
		}
		return string(out)
	}
	if strings.Contains(deps(), pkg) {
		t.Errorf("the production build depends on %s", pkg)
	}
	if !strings.Contains(deps("-tags", "contract_validate"), pkg) {
		t.Errorf("the contract_validate build does not install %s", pkg)
	}
}

// moduleRoot returns the directory of the server go.mod above the working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory")
		}
		dir = parent
	}
}
