// SPDX-License-Identifier: Apache-2.0

package routes_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/internal/fixture/rest"
	fixture "github.com/OWNER/bothy/services/server/api/routes/internal/fixture/routes"
	"github.com/OWNER/bothy/services/server/api/routes/routestest"
)

// TestAZP1_FixtureRoutesMatchEnableList (07 AZ-P1, 05 rule 3): the patterns the generated
// std-http server registers equal the Pattern of every enabled operation, and every operation has
// a known action with the catalogue's class.
func TestAZP1_FixtureRoutesMatchEnableList(t *testing.T) {
	tab := fixture.Table()
	rec := &routes.Recorder{}
	rest.HandlerWithOptions(rest.NewStrictHandler(routestest.Stub{}, nil), rest.StdHTTPServerOptions{
		BaseURL: routes.BaseURL, BaseRouter: rec,
	})
	recorded := rec.Patterns()
	slices.Sort(recorded)
	var want []string
	for _, op := range tab.Enabled() {
		want = append(want, op.Pattern)
		class, ok := tab.Catalog().Class(op.Action)
		if !ok || class != op.RateLimitClass {
			t.Errorf("%s: action %q has class %q (known %v), operation says %q", op.ID, op.Action, class, ok, op.RateLimitClass)
		}
	}
	slices.Sort(want)
	if !reflect.DeepEqual(recorded, want) {
		t.Errorf("recorded patterns %q, want the enabled patterns %q", recorded, want)
	}
	if len(want) != 3 {
		t.Errorf("fixture has %d enabled operations, want 3", len(want))
	}

	rs, err := routestest.Build(tab.Catalog())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(rs) != len(want) {
		t.Fatalf("Build returned %d routes, want %d", len(rs), len(want))
	}
	for _, r := range rs {
		if r.Handler == nil {
			t.Errorf("%s has no handler", r.Operation.ID)
		}
	}
}

func TestOperationMetadata_Fixture(t *testing.T) {
	want := map[string]routes.Operation{
		"createThing": {ID: "createThing", Method: "POST", Path: "/api/v1/things", Pattern: "POST /api/v1/things",
			Action: "thing.create", RateLimitClass: "rl.write", IdempotencyKey: true,
			RequestBody: routes.BodyJSON, BodyRequired: true},
		"getThing": {ID: "getThing", Method: "GET", Path: "/api/v1/things/{thing_id}", Pattern: "GET /api/v1/things/{thing_id}",
			Action: "thing.read", RateLimitClass: "rl.public_auth", Public: true},
		"updateThing": {ID: "updateThing", Method: "PATCH", Path: "/api/v1/things/{thing_id}", Pattern: "PATCH /api/v1/things/{thing_id}",
			Action: "thing.update", RateLimitClass: "rl.write", IfMatch: routes.IfMatchRequired,
			RequestBody: routes.BodyJSON, BodyRequired: true, ETagStatuses: []int{200}},
	}
	for id, w := range want {
		got, ok := fixture.Table().ByID(id)
		if !ok {
			t.Errorf("ByID(%s) found nothing", id)
			continue
		}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s =\n%+v\nwant\n%+v", id, got, w)
		}
		if byPattern, _ := fixture.Table().ByPattern(w.Pattern); byPattern.ID != id {
			t.Errorf("ByPattern(%q) = %s, want %s", w.Pattern, byPattern.ID, id)
		}
	}
}

// TestFixtureRoutes_Serve mounts the built routes on a ServeMux and calls each: the generated glue,
// the patterns and the default error handlers work together.
func TestFixtureRoutes_Serve(t *testing.T) {
	rs, err := routestest.Build(routestest.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, r := range rs {
		mux.Handle(r.Operation.Pattern, r.Handler)
	}
	cases := []struct {
		method, path, body string
		header             map[string]string
		status             int
		contains           string
	}{
		{"GET", "/api/v1/things/" + routestest.ThingID, "", nil, 200, `"name":"fixed"`},
		{"POST", "/api/v1/things", `{"name":"new"}`, map[string]string{"Content-Type": "application/json"}, 201, `"name":"new"`},
		{"PATCH", "/api/v1/things/" + routestest.ThingID, `{}`, map[string]string{"Content-Type": "application/json", "If-Match": `"6"`}, 200, `"name":"updated"`},
		{"PATCH", "/api/v1/things/" + routestest.ThingID, `{}`, map[string]string{"Content-Type": "application/json"}, 400, "If-Match"},
		{"GET", "/api/v1/things/not-a-uuid", "", nil, 400, ""},
		{"DELETE", "/api/v1/things/" + routestest.ThingID, "", nil, 405, ""},
		{"GET", "/things/" + routestest.ThingID, "", nil, 404, ""},
	}
	for _, c := range cases {
		req := httptest.NewRequestWithContext(t.Context(), c.method, c.path, strings.NewReader(c.body))
		for k, v := range c.header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.contains) {
			t.Errorf("%s %s: %d %q, want %d containing %q", c.method, c.path, rec.Code, rec.Body.String(), c.status, c.contains)
		}
		if c.method == "PATCH" && c.status == 200 && rec.Header().Get("ETag") != routestest.ETag {
			t.Errorf("PATCH: ETag %q, want %q", rec.Header().Get("ETag"), routestest.ETag)
		}
	}
}

// TestFixtureRoutes_OptionsReachTheGeneratedServer: each handler of routes.Options reaches the
// option of the generated server it belongs to (build.gen.go), so the problem writers of the
// HTTP chain (M1-18) answer the right errors.
func TestFixtureRoutes_OptionsReachTheGeneratedServer(t *testing.T) {
	marker := func(name string) func(http.ResponseWriter, *http.Request, error) {
		return func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(name))
		}
	}
	opts := routes.Options{
		ErrorHandlerFunc:         marker("parameter"),
		RequestErrorHandlerFunc:  marker("request"),
		ResponseErrorHandlerFunc: marker("response"),
	}
	cases := []struct {
		name, method, path, body string
		stub                     routestest.Stub
		want                     string
	}{
		{"bad path parameter", "GET", "/api/v1/things/not-a-uuid", "", routestest.Stub{}, "parameter"},
		{"malformed JSON body", "POST", "/api/v1/things", "{", routestest.Stub{}, "request"},
		{"handler error", "GET", "/api/v1/things/" + routestest.ThingID, "", routestest.Stub{Err: errors.New("boom")}, "response"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs, err := routestest.BuildWith(c.stub, opts, routestest.Catalog())
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			for _, r := range rs {
				mux.Handle(r.Operation.Pattern, r.Handler)
			}
			req := httptest.NewRequestWithContext(t.Context(), c.method, c.path, strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusTeapot || rec.Body.String() != c.want {
				t.Errorf("%d %q, want the %s handler", rec.Code, rec.Body.String(), c.want)
			}
		})
	}
}
