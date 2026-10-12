// SPDX-License-Identifier: Apache-2.0

// Package routestest gives tests outside api/routes the REST surface of the route generator's
// fixture: three generated operations (public GET, POST with Idempotency-Key, PATCH with a
// required If-Match) and a stub that implements them. Start checks and the HTTP chain can so be
// tested against generated code while the real enable list is still empty. Test helpers live in
// the tree of the component whose types they build (02 "Code layout"); only _test.go files
// import this package.
package routestest

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/api/routes/internal/fixture/rest"
	fixture "github.com/OWNER/bothy/services/server/api/routes/internal/fixture/routes"
)

// ThingID is the ID every stub response carries.
const ThingID = "0190d6a8-0000-7000-8000-000000000001"

// ETag is the entity tag the stub sends with updateThing.
const ETag = `"7"`

// Operations returns the enabled operations of the fixture, sorted by ID.
func Operations() []routes.Operation { return fixture.Table().Enabled() }

// Catalog returns the action catalogue generated from the fixture's actions.yaml.
func Catalog() routes.Catalog { return fixture.Table().Catalog() }

// Build builds the fixture routes with Stub and the oapi-codegen default error handlers and
// verifies them against cat, exactly as routes.Build does for the real surface.
func Build(cat routes.Catalog) ([]routes.Route, error) {
	return BuildWith(Stub{}, routes.Options{}, cat)
}

// BuildWith builds the fixture routes with stub and the error handlers of opts and verifies them
// against cat; tests of the error paths use it.
func BuildWith(stub Stub, opts routes.Options, cat routes.Catalog) ([]routes.Route, error) {
	return fixture.Table().Build(stub, opts, cat)
}

// Stub implements the strict server interface of the fixture: every operation succeeds with a
// fixed thing, or returns Err when it is set (the error path of the strict server).
type Stub struct {
	Err error
}

// thing is the fixed response body.
func thing(name string) rest.Thing {
	var t rest.Thing
	_ = t.Id.UnmarshalText([]byte(ThingID))
	t.Name, t.Version = name, 7
	return t
}

// CreateThing answers 201 with a thing named like the request.
func (s Stub) CreateThing(_ context.Context, req rest.CreateThingRequestObject) (rest.CreateThingResponseObject, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	name := ""
	if req.Body != nil {
		name = req.Body.Name
	}
	return rest.CreateThing201JSONResponse(thing(name)), nil
}

// GetThing answers 200 with the fixed thing.
func (s Stub) GetThing(context.Context, rest.GetThingRequestObject) (rest.GetThingResponseObject, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	return rest.GetThing200JSONResponse(thing("fixed")), nil
}

// UpdateThing answers 200 with the fixed thing and the ETag of its version.
func (s Stub) UpdateThing(context.Context, rest.UpdateThingRequestObject) (rest.UpdateThingResponseObject, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	tag := ETag
	return rest.UpdateThing200JSONResponse{Body: thing("updated"), Headers: rest.UpdateThing200ResponseHeaders{ETag: &tag}}, nil
}
