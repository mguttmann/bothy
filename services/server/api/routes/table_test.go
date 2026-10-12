// SPDX-License-Identifier: Apache-2.0

package routes_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
)

// Hand-made operations and catalogue: the tests of Table do not depend on generated code.
var (
	createThing = routes.Operation{
		ID: "createThing", Method: http.MethodPost, Path: "/api/v1/things", Pattern: "POST /api/v1/things",
		Action: "thing.create", RateLimitClass: "rl.write", IdempotencyKey: true,
		RequestBody: routes.BodyJSON, BodyRequired: true,
	}
	getThing = routes.Operation{
		ID: "getThing", Method: http.MethodGet, Path: "/api/v1/things/{thing_id}", Pattern: "GET /api/v1/things/{thing_id}",
		Action: "thing.read", RateLimitClass: "rl.public_auth", Public: true, ETagStatuses: []int{200},
	}
	testEntries = []routes.CatalogEntry{
		{ID: "thing.create", Right: "write", RateLimitClass: "rl.write"},
		{ID: "thing.deliver", Right: "read", RateLimitClass: "none"},
		{ID: "thing.purge", Right: "system", RateLimitClass: "none"},
		{ID: "thing.read", Right: "public", RateLimitClass: "rl.public_auth"},
	}
)

// impl is the implementation type of the test table: register writes the operation ID of each
// pattern as the body, so a test can see which handler a route got.
type impl struct{ prefix string }

// registerAll registers every pattern in patterns.
func registerAll(patterns ...string) func(impl, routes.Options, *routes.Recorder) {
	return func(s impl, _ routes.Options, mux *routes.Recorder) {
		for _, p := range patterns {
			mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(s.prefix + p))
			})
		}
	}
}

func newTable(register func(impl, routes.Options, *routes.Recorder)) *routes.Table[impl] {
	return routes.NewTable(
		[]routes.Operation{getThing, createThing}, // unsorted on purpose
		testEntries, register)
}

// catalog is a test catalogue: action to class.
type catalog map[string]string

func (c catalog) Class(action string) (string, bool) {
	class, ok := c[action]
	return class, ok
}

var fullCatalog = catalog{"thing.create": "rl.write", "thing.read": "rl.public_auth"}

// wantError requires a non-nil error that contains every string of wants.
func wantError(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one naming %q", wants)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
}

func TestVerify_Clean(t *testing.T) {
	tab := newTable(nil)
	if err := tab.Verify([]string{createThing.Pattern, getThing.Pattern}, fullCatalog); err != nil {
		t.Fatalf("Verify = %v, want nil", err)
	}
}

func TestVerify_UnknownPattern(t *testing.T) {
	err := newTable(nil).Verify([]string{createThing.Pattern, getThing.Pattern, "DELETE /api/v1/things/{thing_id}"}, fullCatalog)
	wantError(t, err, `"DELETE /api/v1/things/{thing_id}"`, "no enabled operation")
}

func TestVerify_MissingRegistration(t *testing.T) {
	err := newTable(nil).Verify([]string{createThing.Pattern}, fullCatalog)
	wantError(t, err, "getThing", "not registered")
}

func TestVerify_UnknownAction(t *testing.T) {
	err := newTable(nil).Verify([]string{createThing.Pattern, getThing.Pattern}, catalog{"thing.create": "rl.write"})
	wantError(t, err, "getThing", `"thing.read"`, "does not know")
}

func TestVerify_ClassMismatch(t *testing.T) {
	err := newTable(nil).Verify([]string{createThing.Pattern, getThing.Pattern},
		catalog{"thing.create": "rl.read", "thing.read": "rl.public_auth"})
	wantError(t, err, "createThing", `"rl.write"`, `"rl.read"`)
}

func TestVerify_DuplicateRegistration(t *testing.T) {
	err := newTable(nil).Verify([]string{createThing.Pattern, getThing.Pattern, getThing.Pattern}, fullCatalog)
	wantError(t, err, getThing.Pattern, "2 times")
}

func TestVerify_NamesEveryProblem(t *testing.T) {
	err := newTable(nil).Verify([]string{"GET /api/v1/other"}, catalog{})
	wantError(t, err, "GET /api/v1/other", "createThing", "getThing", `"thing.create"`, `"thing.read"`)
}

func TestVerify_NilCatalog(t *testing.T) {
	wantError(t, newTable(nil).Verify(nil, nil), "catalogue")
}

func TestBuild_RoutesSortedWithTheirHandlers(t *testing.T) {
	got, err := newTable(registerAll(getThing.Pattern, createThing.Pattern)).Build(impl{prefix: "h:"}, routes.Options{}, fullCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Operation.ID != "createThing" || got[1].Operation.ID != "getThing" {
		t.Fatalf("routes = %+v, want createThing then getThing", got)
	}
	for _, r := range got {
		rec := httptest.NewRecorder()
		r.Handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), r.Operation.Method, "/", nil))
		if want := "h:" + r.Operation.Pattern; rec.Body.String() != want {
			t.Errorf("handler of %s wrote %q, want %q", r.Operation.ID, rec.Body.String(), want)
		}
	}
}

func TestBuild_FailsWhenVerifyFails(t *testing.T) {
	got, err := newTable(registerAll(createThing.Pattern)).Build(impl{}, routes.Options{}, fullCatalog)
	wantError(t, err, "getThing")
	if got != nil {
		t.Errorf("routes = %v, want none on error", got)
	}
}

func TestBuild_PassesOptionsToTheGlue(t *testing.T) {
	var seen routes.Options
	called := false
	tab := newTable(func(_ impl, opts routes.Options, mux *routes.Recorder) {
		seen, called = opts, true
		registerAll(createThing.Pattern, getThing.Pattern)(impl{}, opts, mux)
	})
	marker := func(http.ResponseWriter, *http.Request, error) {}
	if _, err := tab.Build(impl{}, routes.Options{RequestErrorHandlerFunc: marker}, fullCatalog); err != nil {
		t.Fatal(err)
	}
	if !called || seen.RequestErrorHandlerFunc == nil || seen.ErrorHandlerFunc != nil || seen.ResponseErrorHandlerFunc != nil {
		t.Errorf("glue saw %+v (called %v), want exactly the given request error handler", seen, called)
	}
}

func TestBuild_EmptyTable(t *testing.T) {
	tab := routes.NewTable[any](nil, testEntries, nil)
	got, err := tab.Build(nil, routes.Options{}, fullCatalog)
	if err != nil || len(got) != 0 {
		t.Fatalf("Build = %v, %v; want no routes and no error", got, err)
	}
}

func TestBuild_NilImplementationWithOperations(t *testing.T) {
	tab := routes.NewTable[any]([]routes.Operation{getThing}, testEntries, nil)
	_, err := tab.Build(nil, routes.Options{}, fullCatalog)
	wantError(t, err, "no implementation")
}

func TestRecorder_RecordsInOrder(t *testing.T) {
	var rec routes.Recorder
	rec.HandleFunc("GET /b", func(http.ResponseWriter, *http.Request) {})
	rec.HandleFunc("GET /a", func(http.ResponseWriter, *http.Request) {})
	if got, want := rec.Patterns(), []string{"GET /b", "GET /a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Patterns = %v, want %v", got, want)
	}
}

func TestRecorder_ServeHTTPPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ServeHTTP did not panic")
		}
	}()
	var rec routes.Recorder
	rec.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
}

func TestTable_Lookups(t *testing.T) {
	tab := newTable(nil)
	if op, ok := tab.ByID("getThing"); !ok || op.Pattern != getThing.Pattern {
		t.Errorf("ByID(getThing) = %+v, %v", op, ok)
	}
	if op, ok := tab.ByPattern(createThing.Pattern); !ok || op.ID != "createThing" {
		t.Errorf("ByPattern(%q) = %+v, %v", createThing.Pattern, op, ok)
	}
	if _, ok := tab.ByID("deleteThing"); ok {
		t.Error("ByID(deleteThing) found an operation")
	}
	if _, ok := tab.ByPattern("GET /api/v1/things"); ok {
		t.Error("ByPattern of an unknown pattern found an operation")
	}
	ids := []string{}
	for _, op := range tab.Enabled() {
		ids = append(ids, op.ID)
	}
	if want := []string{"createThing", "getThing"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("Enabled IDs = %v, want %v", ids, want)
	}
}

// TestTable_ReturnsCopies proves that a caller cannot change the table through a returned value.
func TestTable_ReturnsCopies(t *testing.T) {
	tab := newTable(nil)
	ops := tab.Enabled()
	ops[1].Action = "changed"
	ops[1].ETagStatuses[0] = 999
	op, _ := tab.ByID("getThing")
	op.ETagStatuses[0] = 998
	again, _ := tab.ByPattern(getThing.Pattern)
	if again.Action != "thing.read" || again.ETagStatuses[0] != 200 {
		t.Errorf("table changed through a returned copy: %+v", again)
	}
}

// TestNewTable_DoesNotAliasInput proves that the table keeps its own copy of the generated data.
func TestNewTable_DoesNotAliasInput(t *testing.T) {
	ops := []routes.Operation{getThing}
	ops[0].ETagStatuses = []int{200}
	tab := routes.NewTable[any](ops, testEntries, nil)
	ops[0].ID = "renamed"
	ops[0].ETagStatuses[0] = 1
	if op, ok := tab.ByID("getThing"); !ok || op.ETagStatuses[0] != 200 {
		t.Errorf("table follows changes of its input: %+v, %v", op, ok)
	}
}

func TestNewTable_PanicsOnDuplicates(t *testing.T) {
	cases := map[string][]routes.Operation{
		"id":      {getThing, func() routes.Operation { o := createThing; o.ID = "getThing"; return o }()},
		"pattern": {getThing, func() routes.Operation { o := createThing; o.Pattern = getThing.Pattern; return o }()},
	}
	for name, ops := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NewTable did not panic")
				}
			}()
			routes.NewTable[any](ops, testEntries, nil)
		})
	}
}

func TestTableCatalog_SystemActionsAreUnknown(t *testing.T) {
	cat := newTable(nil).Catalog()
	if class, ok := cat.Class("thing.read"); !ok || class != "rl.public_auth" {
		t.Errorf("Class(thing.read) = %q, %v; want rl.public_auth", class, ok)
	}
	if _, ok := cat.Class("thing.purge"); ok {
		t.Error("Class(thing.purge) is known; a system action must be unknown to routes (05 rule 10)")
	}
	if _, ok := cat.Class("thing.deliver"); ok {
		t.Error("Class(thing.deliver) is known; an action without rate-limit class has no REST route")
	}
	if _, ok := cat.Class("thing.delete"); ok {
		t.Error("Class(thing.delete) is known")
	}
}

func TestModes_String(t *testing.T) {
	cases := []struct{ got, want string }{
		{routes.IfMatchNone.String(), "none"},
		{routes.IfMatchOptional.String(), "optional"},
		{routes.IfMatchRequired.String(), "required"},
		{routes.IfMatchMode(9).String(), "IfMatchMode(9)"},
		{routes.BodyNone.String(), "none"},
		{routes.BodyJSON.String(), "json"},
		{routes.BodyMultipart.String(), "multipart"},
		{routes.BodyOctetStream.String(), "octet-stream"},
		{routes.BodyKind(-1).String(), "BodyKind(-1)"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("String() = %q, want %q", c.got, c.want)
		}
	}
}
