// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/routestest"
)

// recordingMux records every mounted pattern; it stands for the public mux of the server.
type recordingMux struct {
	patterns []string
	handlers []http.Handler
}

func (m *recordingMux) Handle(pattern string, handler http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.handlers = append(m.handlers, handler)
}

// logRecord is one captured log record.
type logRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// captureHandler is a slog.Handler that keeps every record for assertions.
type captureHandler struct {
	mu      sync.Mutex
	records []logRecord
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *captureHandler) WithGroup(string) slog.Handler            { return h }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := logRecord{level: r.Level, msg: r.Message, attrs: map[string]string{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}

// withoutAction is a catalogue that lacks one action of the base catalogue.
type withoutAction struct {
	base    routes.Catalog
	missing string
}

func (c withoutAction) Class(action string) (string, bool) {
	if action == c.missing {
		return "", false
	}
	return c.base.Class(action)
}

// TestAZP1_RoutesMatchEnableList (07 AZ-P1, 05 rule 3 and test 1): the routes mounted from the
// real generated code are exactly the enabled operations, each with a known action and the
// catalogue's class; zero at this ticket. The fixture surface proves the same for a non-empty
// list through the same start step.
func TestAZP1_RoutesMatchEnableList(t *testing.T) {
	for _, role := range []Role{RoleAPI, RoleAll} {
		mux := &recordingMux{}
		if err := MountRoutes(role, mux, Handlers{}, routes.SpecCatalog(), slog.New(&captureHandler{})); err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
		if got, want := len(mux.patterns), len(routes.Enabled()); got != want {
			t.Errorf("role %s: %d routes mounted, %d enabled", role, got, want)
		}
		assertKnownActions(t, mux, routes.ByPattern, routes.SpecCatalog())
	}

	mux := &recordingMux{}
	if err := mountRoutes(RoleAPI, mux, routestest.Build, routestest.Catalog(), slog.New(&captureHandler{})); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	var want []string
	for _, op := range routestest.Operations() {
		want = append(want, op.Pattern)
	}
	if !slices.Equal(mux.patterns, want) {
		t.Errorf("fixture mounted %q, want %q", mux.patterns, want)
	}
	byPattern := func(p string) (routes.Operation, bool) {
		for _, op := range routestest.Operations() {
			if op.Pattern == p {
				return op, true
			}
		}
		return routes.Operation{}, false
	}
	assertKnownActions(t, mux, byPattern, routestest.Catalog())
}

// assertKnownActions requires every mounted pattern to belong to an operation whose action the
// catalogue knows with the operation's class, and every mounted handler to be set.
func assertKnownActions(t *testing.T, mux *recordingMux, lookup func(string) (routes.Operation, bool), cat routes.Catalog) {
	t.Helper()
	for i, p := range mux.patterns {
		op, ok := lookup(p)
		if !ok {
			t.Errorf("mounted pattern %q has no operation", p)
			continue
		}
		if class, known := cat.Class(op.Action); !known || class != op.RateLimitClass {
			t.Errorf("%s: action %q has class %q (known %v), want %q", op.ID, op.Action, class, known, op.RateLimitClass)
		}
		if mux.handlers[i] == nil {
			t.Errorf("%s has no handler", op.ID)
		}
	}
}

// TestServe_AbortsOnUnknownAction: a start whose catalogue lacks an enabled action fails in the
// route step, before anything is mounted (so before the public listener can report ready), and
// logs "route check failed" at level error with the joined error naming operation and action.
func TestServe_AbortsOnUnknownAction(t *testing.T) {
	mux := &recordingMux{}
	logs := &captureHandler{}
	cat := withoutAction{base: routestest.Catalog(), missing: "thing.update"}

	err := mountRoutes(RoleAll, mux, routestest.Build, cat, slog.New(logs))

	if err == nil {
		t.Fatal("the start step accepted a route with an unknown action")
	}
	for _, want := range []string{"updateThing", "thing.update"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if len(mux.patterns) != 0 {
		t.Errorf("mounted %q despite the failed check", mux.patterns)
	}
	if len(logs.records) != 1 {
		t.Fatalf("got %d log records, want exactly one", len(logs.records))
	}
	rec := logs.records[0]
	if rec.level != slog.LevelError || rec.msg != "route check failed" {
		t.Errorf("log record = %s %q, want ERROR \"route check failed\"", rec.level, rec.msg)
	}
	if !strings.Contains(rec.attrs["error"], "updateThing") || !strings.Contains(rec.attrs["error"], "thing.update") {
		t.Errorf("log attribute error = %q, want the joined error", rec.attrs["error"])
	}
}

func TestMountRoutes_WorkerMountsNothing(t *testing.T) {
	mux := &recordingMux{}
	called := false
	build := func(routes.Catalog) ([]routes.Route, error) {
		called = true
		return nil, errors.New("must not run")
	}
	if err := mountRoutes(RoleWorker, mux, build, nil, slog.New(&captureHandler{})); err != nil {
		t.Fatalf("worker: %v", err)
	}
	if called || len(mux.patterns) != 0 {
		t.Errorf("worker built (%v) or mounted %q; it serves no REST API", called, mux.patterns)
	}
}

func TestMountRoutes_UnknownRole(t *testing.T) {
	mux := &recordingMux{}
	err := MountRoutes(Role("both"), mux, Handlers{}, routes.SpecCatalog(), slog.New(&captureHandler{}))
	if err == nil || !strings.Contains(err.Error(), `"both"`) {
		t.Fatalf("error = %v, want one naming the role", err)
	}
	if len(mux.patterns) != 0 {
		t.Errorf("mounted %q for an unknown role", mux.patterns)
	}
}

// TestMountRoutes_ConflictingPatterns: patterns that net/http cannot hold side by side fail the
// step as an error, before the first route is mounted, instead of a panic in the middle of
// mounting.
func TestMountRoutes_ConflictingPatterns(t *testing.T) {
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	build := func(routes.Catalog) ([]routes.Route, error) {
		return []routes.Route{
			{Operation: routes.Operation{ID: "getA", Pattern: "GET /api/v1/x/{a}"}, Handler: ok},
			{Operation: routes.Operation{ID: "getB", Pattern: "GET /api/v1/x/{b}"}, Handler: ok},
		}, nil
	}
	mux := &recordingMux{}
	logs := &captureHandler{}
	err := mountRoutes(RoleAPI, mux, build, routes.SpecCatalog(), slog.New(logs))
	if err == nil || !strings.Contains(err.Error(), "getB") {
		t.Fatalf("error = %v, want one naming the conflicting operation", err)
	}
	if len(mux.patterns) != 0 || len(logs.records) != 1 {
		t.Errorf("mounted %q, %d log records; want nothing mounted and one record", mux.patterns, len(logs.records))
	}
}

func TestMountRoutes_NilLoggerUsesDefault(t *testing.T) {
	if err := MountRoutes(RoleAPI, &recordingMux{}, Handlers{}, routes.SpecCatalog(), nil); err != nil {
		t.Fatal(err)
	}
}

// TestMountRoutes_ForwardsTheCatalogue: the exported start step hands its catalogue to Build;
// without one, Build refuses even an empty table.
func TestMountRoutes_ForwardsTheCatalogue(t *testing.T) {
	mux := &recordingMux{}
	err := MountRoutes(RoleAPI, mux, Handlers{}, nil, slog.New(&captureHandler{}))
	if err == nil || !strings.Contains(err.Error(), "catalogue") {
		t.Fatalf("error = %v, want the missing catalogue reported by Build", err)
	}
}

// TestWire_NoFixtureInProductionGraph keeps the test helper, the fixture packages and the
// generator with its libraries out of the binary: only _test.go files of internal/wire import
// the first two, and nothing in the server imports the generator (02 "Code-Layout"; the
// component api may use the vendor groups yaml and openapi only for api/routes/gen). The YAML
// library is checked on the graph of the api packages the server builds from, because
// platform/config brings it into internal/wire on purpose (docs/DECISIONS.md M1-17/28).
func TestWire_NoFixtureInProductionGraph(t *testing.T) {
	deps := goListDeps(t, exec.CommandContext(t.Context(), "go", "list", "-deps", "."))
	if !slices.Contains(deps, "github.com/OWNER/bothy/services/server/api/routes") {
		t.Fatalf("go list -deps does not list api/routes; the check would prove nothing:\n%s", strings.Join(deps, "\n"))
	}
	for _, d := range deps {
		if generatorOrFixture(d) {
			t.Errorf("internal/wire depends on %s, which belongs to tests or to the generator", d)
		}
	}
	api := exec.CommandContext(t.Context(), "go", "list", "-deps", "../../api/rest", "../../api/routes")
	for _, d := range goListDeps(t, api) {
		if generatorOrFixture(d) || strings.HasPrefix(d, "go.yaml.in/yaml/") {
			t.Errorf("api/rest or api/routes depends on %s, which belongs to tests or to the generator", d)
		}
	}
}

// generatorOrFixture reports whether the import path d is the test helper, a fixture package,
// the generator or the OpenAPI library only the generator uses.
func generatorOrFixture(d string) bool {
	return strings.Contains(d, "/routestest") || strings.Contains(d, "/internal/fixture/") ||
		strings.HasSuffix(d, "/api/routes/gen") || strings.HasPrefix(d, "github.com/getkin/kin-openapi")
}

// goListDeps runs a go list -deps command and returns the import paths it prints: the packages
// and everything they depend on.
func goListDeps(t *testing.T, cmd *exec.Cmd) []string {
	t.Helper()
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v", strings.Join(cmd.Args, " "), err)
	}
	return strings.Fields(out.String())
}

// TestMountRoutes_ConflictWithPatternOnMux: a pattern already on the public mux (a route M1-11
// or a later ticket mounted first) that conflicts with a REST route fails the step with an
// error, not with a panic.
func TestMountRoutes_ConflictWithPatternOnMux(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/things/{id}", http.NotFoundHandler())
	logs := &captureHandler{}
	err := mountRoutes(RoleAPI, mux, routestest.Build, routestest.Catalog(), slog.New(logs))
	if err == nil || !strings.Contains(err.Error(), "getThing") {
		t.Fatalf("error = %v, want one naming getThing", err)
	}
	if len(logs.records) != 1 || logs.records[0].msg != "route check failed" {
		t.Errorf("log records = %+v, want one \"route check failed\"", logs.records)
	}
}
