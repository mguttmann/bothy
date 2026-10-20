// SPDX-License-Identifier: Apache-2.0

// Package leak holds the leak tests behind threat T-AZ-02 (PLAN M1-70): AZ-P4 proves route by
// route that a foreign and an unknown ID give the same answer, AZ-P5 proves on random worlds that
// lists, counters, the snapshot and catch-up count nothing the principal cannot read, and AZ-M52
// is the matrix row of a foreign message. The non-test files are the parts the tests share: the
// route selection (routes.go), the fixtures of foreign IDs (fixtures.go), the normalization of
// responses (normalize.go) and the random world generator (world.go).
//
// The package belongs to the architecture component testsupport (M1-05) and is never imported by
// production code.
package leak

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Route is one enabled GET operation with at least one path parameter of format uuid.
type Route struct {
	// OperationID is the operationId of the contract.
	OperationID string
	// Path is the path template relative to /api/v1, for example /messages/{message_id}.
	Path string
	// Params are the names of the path parameters of format uuid, in template order.
	Params []string
}

// FixedParams are valid values of the path parameters that are no ID, so a route reaches its
// handler: the variant of downloadThumbnail (M3-05). A and B of a pair carry the same value.
var FixedParams = map[string]string{"variant": "small"}

// FixedQueries are the required query parameters of an operation, so a route reaches its handler
// instead of the 400 of the request validation: the range of exportChannel (M3-11). A and B of a
// pair carry the same query.
var FixedQueries = map[string]string{"exportChannel": "from=2026-01-01T00:00:00.000Z&to=2026-01-02T00:00:00.000Z"}

// Build returns the path of r with every parameter of values replaced, every parameter of
// FixedParams, and the query of FixedQueries for the operation of r.
func (r Route) Build(values map[string]string) string {
	p := r.Path
	for name, v := range values {
		p = strings.ReplaceAll(p, "{"+name+"}", v)
	}
	for name, v := range FixedParams {
		p = strings.ReplaceAll(p, "{"+name+"}", v)
	}
	if q, ok := FixedQueries[r.OperationID]; ok {
		p += "?" + q
	}
	return p
}

// operationList is the documented format of services/server/api/operations.yaml
// (services/server/api/README.md "Freischalt-Liste").
type operationList struct {
	Version int      `yaml:"version"`
	Enabled []string `yaml:"enabled"`
}

// openAPI is the part of the contract the selection reads.
type openAPI struct {
	Paths      map[string]map[string]yaml.Node `yaml:"paths"`
	Components struct {
		Parameters map[string]parameter `yaml:"parameters"`
		Schemas    map[string]schema    `yaml:"schemas"`
	} `yaml:"components"`
}

// parameter is a parameter object or a reference to one.
type parameter struct {
	Ref    string `yaml:"$ref"`
	Name   string `yaml:"name"`
	In     string `yaml:"in"`
	Schema schema `yaml:"schema"`
}

// schema is the part of a schema object the selection reads.
type schema struct {
	Ref    string `yaml:"$ref"`
	Format string `yaml:"format"`
}

// operation is the part of an operation object the selection reads.
type operation struct {
	OperationID string      `yaml:"operationId"`
	Parameters  []parameter `yaml:"parameters"`
}

// SelectRoutes reads the enabled operation IDs from services/server/api/operations.yaml and the
// operations of the contract docs/api/openapi.yaml (embedded as rest.Spec, byte-equal and held
// by make codegen-check) and returns every enabled GET operation with at least one path parameter
// of format uuid, sorted by operation ID. Errors: a file that cannot be read or parsed, a
// reference that does not resolve.
func SelectRoutes() ([]Route, error) {
	raw, err := os.ReadFile(filepath.Join(moduleDir(), "api", "operations.yaml"))
	if err != nil {
		return nil, err
	}
	return selectRoutes(raw, rest.Spec)
}

// selectRoutes is SelectRoutes on the bytes of the enable list and the contract.
func selectRoutes(list, contract []byte) ([]Route, error) {
	var ops operationList
	if err := yaml.Unmarshal(list, &ops); err != nil {
		return nil, fmt.Errorf("operations.yaml: %w", err)
	}
	if ops.Version != 1 {
		return nil, fmt.Errorf("operations.yaml: version %d, want 1", ops.Version)
	}
	enabled := map[string]bool{}
	for _, id := range ops.Enabled {
		enabled[id] = true
	}
	var doc openAPI
	if err := yaml.Unmarshal(contract, &doc); err != nil {
		return nil, fmt.Errorf("openapi.yaml: %w", err)
	}
	var out []Route
	for path, item := range doc.Paths {
		var common []parameter
		if n, ok := item["parameters"]; ok {
			if err := n.Decode(&common); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
		n, ok := item[strings.ToLower(http.MethodGet)]
		if !ok {
			continue
		}
		var op operation
		if err := n.Decode(&op); err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}
		if !enabled[op.OperationID] {
			continue
		}
		var uuids []string
		for _, p := range append(slices.Clone(common), op.Parameters...) {
			p, err := doc.resolve(p)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", op.OperationID, err)
			}
			if p.In == "path" && doc.format(p.Schema) == "uuid" && !slices.Contains(uuids, p.Name) {
				uuids = append(uuids, p.Name)
			}
		}
		if len(uuids) == 0 {
			continue
		}
		slices.SortFunc(uuids, func(a, b string) int { return strings.Index(path, a) - strings.Index(path, b) })
		out = append(out, Route{OperationID: op.OperationID, Path: path, Params: uuids})
	}
	slices.SortFunc(out, func(a, b Route) int { return strings.Compare(a.OperationID, b.OperationID) })
	return out, nil
}

// resolve follows a reference to components/parameters.
func (d *openAPI) resolve(p parameter) (parameter, error) {
	if p.Ref == "" {
		return p, nil
	}
	name, ok := strings.CutPrefix(p.Ref, "#/components/parameters/")
	if !ok {
		return p, fmt.Errorf("unsupported reference %s", p.Ref)
	}
	q, ok := d.Components.Parameters[name]
	if !ok {
		return p, fmt.Errorf("unknown parameter %s", p.Ref)
	}
	return q, nil
}

// format returns the format of s, following references to components/schemas.
func (d *openAPI) format(s schema) string {
	for range 8 {
		name, ok := strings.CutPrefix(s.Ref, "#/components/schemas/")
		if !ok {
			return s.Format
		}
		s = d.Components.Schemas[name]
	}
	return ""
}

// moduleDir returns services/server of the source tree of this file.
func moduleDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}
