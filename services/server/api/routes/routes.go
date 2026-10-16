// SPDX-License-Identifier: Apache-2.0

// Package routes is the route table of the REST API: for every enabled operation of
// docs/api/openapi.yaml it knows method, pattern, authorization action, rate-limit class and the
// request metadata the HTTP chain needs (ADR 0013, plan decision P-04). The table, the action
// catalogue and the glue that registers the generated handlers come from the generator in
// api/routes/gen (files *.gen.go); this file holds the hand-written types and checks.
//
// Build runs the generated std-http and strict server on a Recorder, so the patterns are
// exactly those the generated code registers, and Verify compares them with the table and the
// catalogue before anything is mounted (05 rule 3, test AZ-P1). The server refuses to start
// when Build fails.
//
// Example:
//
//	rs, err := routes.Build(handlers, routes.Options{}, routes.SpecCatalog())
//	if err != nil {
//		return err // route check failed: unknown pattern, action or class
//	}
//	for _, r := range rs {
//		mux.Handle(r.Operation.Pattern, r.Handler)
//	}
package routes

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
)

// BaseURL is the prefix of every REST path (06 "Overview", W6). The generator checks that the
// spec declares exactly this server URL, and the generated registration prefixes every path
// with it.
const BaseURL = "/api/v1"

// Texts written in more than one place (rule go:S1192 of the quality gate): the zero modes
// IfMatchNone and BodyNone print as none, and the catalogue marks actions that no route may
// carry with the right system or the class none (05 rule 10, "Rate-limit classes").
const (
	nameNone    = "none"
	rightSystem = "system"
	classNone   = "none"
)

// IfMatchMode says whether an operation takes an If-Match header (06 "Concurrency").
type IfMatchMode int

// If-Match modes. The zero value is IfMatchNone.
const (
	IfMatchNone     IfMatchMode = iota // no If-Match parameter
	IfMatchOptional                    // #/components/parameters/IfMatchOptional
	IfMatchRequired                    // #/components/parameters/IfMatchRequired; missing gives 428
)

// String returns the mode as written in the generator's input names: none, optional, required.
func (m IfMatchMode) String() string {
	switch m {
	case IfMatchNone:
		return nameNone
	case IfMatchOptional:
		return "optional"
	case IfMatchRequired:
		return "required"
	}
	return fmt.Sprintf("IfMatchMode(%d)", int(m))
}

// BodyKind is the media type of an operation's request body (06 "Basic rules", "Media types").
type BodyKind int

// Request body kinds. The zero value is BodyNone.
const (
	BodyNone        BodyKind = iota // no request body
	BodyJSON                        // application/json
	BodyMultipart                   // multipart/form-data
	BodyOctetStream                 // application/octet-stream
)

// String returns the kind as a short name: none, json, multipart, octet-stream.
func (k BodyKind) String() string {
	switch k {
	case BodyNone:
		return nameNone
	case BodyJSON:
		return "json"
	case BodyMultipart:
		return "multipart"
	case BodyOctetStream:
		return "octet-stream"
	}
	return fmt.Sprintf("BodyKind(%d)", int(k))
}

// Operation describes one enabled REST operation. All values come from docs/api/openapi.yaml.
type Operation struct {
	ID             string      // operationId, for example getDiscovery
	Method         string      // upper case HTTP method
	Path           string      // template including the base URL, for example /api/v1/threads/{thread_id}
	Pattern        string      // net/http pattern exactly as the generated code registers it
	Action         string      // x-authz-action
	RateLimitClass string      // x-rate-limit-class
	Public         bool        // operation-level security: []
	IdempotencyKey bool        // declares the header Idempotency-Key (#/components/parameters/IdempotencyKey)
	IfMatch        IfMatchMode // IfMatchNone, IfMatchOptional, IfMatchRequired (header If-Match)
	RequestBody    BodyKind    // BodyNone, BodyJSON, BodyMultipart, BodyOctetStream
	BodyRequired   bool        // requestBody.required
	ETagStatuses   []int       // 2xx statuses whose response declares the ETag header
	Download       bool        // a 2xx response streams a body that is not JSON (route class of downloads)
}

// clone returns a copy of op that shares no slice with it.
func (op Operation) clone() Operation {
	op.ETagStatuses = slices.Clone(op.ETagStatuses)
	return op
}

// label names the operation in errors: ID, method and path.
func (op Operation) label() string {
	return fmt.Sprintf("%s (%s %s)", op.ID, op.Method, op.Path)
}

// CatalogEntry is one action of docs/spec/actions.yaml as the generated catalogue holds it.
type CatalogEntry struct {
	ID             string // action ID, for example thread.read
	Right          string // column "Permission" of chapter 05, verbatim; "system" marks a system action
	RateLimitClass string // rate-limit class of the action, "none" for system actions
}

// Catalog answers which rate-limit class an action has. ok is false for an action the
// catalogue does not know; a route with such an action must not be served (05 rule 5).
//
// SpecCatalog is the catalogue generated from docs/spec/actions.yaml; the authz module will
// provide the closed catalogue that replaces it at start (M1-25).
type Catalog interface { // NOSONAR: a catalogue is a thing, not an action; see M1-17/33.
	Class(action string) (class string, ok bool)
}

// specCatalog is a Catalog over generated entries.
type specCatalog map[string]CatalogEntry

// Class returns the class of action. Actions with right system and actions without a class
// (none: system actions and internal delivery checks) are unknown here: only the server runs
// them, so no route may carry one (05 rule 10, 06 "Authorization in the API" item 7).
func (c specCatalog) Class(action string) (string, bool) {
	e, ok := c[action]
	if !ok || e.Right == rightSystem || e.RateLimitClass == classNone {
		return "", false
	}
	return e.RateLimitClass, true
}

// Options carries the error handlers of the generated server. A nil handler means the
// oapi-codegen default, which writes the error text with 400 (parameters and request bodies)
// or 500 (responses); the HTTP chain (M1-18) passes its problem writers instead.
type Options struct {
	// ErrorHandlerFunc handles parameter binding errors of the std-http server.
	ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
	// RequestErrorHandlerFunc handles request decoding errors of the strict server.
	RequestErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
	// ResponseErrorHandlerFunc handles errors returned by a handler of the strict server.
	ResponseErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
}

// Route is one operation with the handler the generated server registered for it.
type Route struct {
	Operation Operation
	Handler   http.Handler
}

// Recorder is the ServeMux the generated std-http server registers on during Build. It records
// every pattern with its handler in registration order and serves nothing: the recorded
// handlers are mounted on the real mux only after Verify accepted them.
type Recorder struct {
	patterns []string
	handlers []http.HandlerFunc
}

// HandleFunc records one registration; it implements the ServeMux interface of the generated
// code.
func (r *Recorder) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
	r.handlers = append(r.handlers, handler)
}

// ServeHTTP panics: a Recorder never serves. The generated code requires the method, and a
// request reaching it would mean that unverified routes are mounted.
func (r *Recorder) ServeHTTP(http.ResponseWriter, *http.Request) {
	panic("routes: a Recorder only records registrations; mount the routes returned by Build")
}

// Patterns returns the recorded patterns in registration order.
func (r *Recorder) Patterns() []string {
	return slices.Clone(r.patterns)
}

// handler returns the handler recorded first for pattern, or nil.
func (r *Recorder) handler(pattern string) http.Handler {
	if i := slices.Index(r.patterns, pattern); i >= 0 {
		return r.handlers[i]
	}
	return nil
}

// Table is one generated REST surface: its enabled operations, its action catalogue and the
// glue that registers the generated handlers. S is the type the handlers implement (the
// generated StrictServerInterface, or any while no operation is enabled). The package-level
// functions use the table generated for docs/api/openapi.yaml; the fixture of the generator
// builds its own, so both run through the same checks.
type Table[S any] struct {
	ops       []Operation // sorted by ID
	byID      map[string]int
	byPattern map[string]int
	catalog   specCatalog
	register  func(impl S, opts Options, mux *Recorder)
}

// NewTable builds a table from generated data. ops may come in any order; the table sorts
// them by ID and keeps its own copy. register runs the generated server on mux; nil registers
// nothing.
//
// NewTable panics when two operations share an ID or a pattern: the generator rules both out,
// so a duplicate is a programming error in hand-made test data.
func NewTable[S any](ops []Operation, entries []CatalogEntry, register func(impl S, opts Options, mux *Recorder)) *Table[S] {
	t := &Table[S]{
		byID:      make(map[string]int, len(ops)),
		byPattern: make(map[string]int, len(ops)),
		catalog:   make(specCatalog, len(entries)),
		register:  register,
	}
	for _, op := range ops {
		t.ops = append(t.ops, op.clone())
	}
	slices.SortFunc(t.ops, func(a, b Operation) int { return cmp.Compare(a.ID, b.ID) })
	for i, op := range t.ops {
		if _, dup := t.byID[op.ID]; dup {
			panic("routes: two operations with the ID " + op.ID)
		}
		if _, dup := t.byPattern[op.Pattern]; dup {
			panic("routes: two operations with the pattern " + op.Pattern)
		}
		t.byID[op.ID], t.byPattern[op.Pattern] = i, i
	}
	for _, e := range entries {
		t.catalog[e.ID] = e
	}
	return t
}

// Enabled returns a copy of the enabled operations, sorted by ID.
func (t *Table[S]) Enabled() []Operation {
	out := make([]Operation, len(t.ops))
	for i, op := range t.ops {
		out[i] = op.clone()
	}
	return out
}

// ByPattern returns the operation registered under pattern, for example
// "GET /api/v1/threads/{thread_id}" (http.Request.Pattern of a matched request).
func (t *Table[S]) ByPattern(pattern string) (Operation, bool) {
	i, ok := t.byPattern[pattern]
	if !ok {
		return Operation{}, false
	}
	return t.ops[i].clone(), true
}

// ByID returns the enabled operation with the operationId id.
func (t *Table[S]) ByID(id string) (Operation, bool) {
	i, ok := t.byID[id]
	if !ok {
		return Operation{}, false
	}
	return t.ops[i].clone(), true
}

// Catalog returns the generated action catalogue of the table.
func (t *Table[S]) Catalog() Catalog {
	return t.catalog
}

// Verify compares registered (the patterns the generated code registered) with the enabled
// operations and cat. It returns one joined error that names every problem, or nil:
//   - a registered pattern that no enabled operation has, or one registered more than once,
//   - an enabled operation that was not registered,
//   - an action that cat does not know,
//   - a rate-limit class that differs from the class cat gives the action.
func (t *Table[S]) Verify(registered []string, cat Catalog) error {
	if cat == nil {
		return errors.New("routes: no action catalogue to verify against")
	}
	var problems []string
	count := map[string]int{}
	for _, p := range registered {
		count[p]++
	}
	for _, p := range slices.Sorted(maps.Keys(count)) {
		if _, ok := t.byPattern[p]; !ok {
			problems = append(problems, fmt.Sprintf("pattern %q is registered, but no enabled operation has it", p))
		}
		if n := count[p]; n > 1 {
			problems = append(problems, fmt.Sprintf("pattern %q is registered %d times", p, n))
		}
	}
	for _, op := range t.ops {
		if count[op.Pattern] == 0 {
			problems = append(problems, fmt.Sprintf("operation %s is enabled but not registered", op.label()))
		}
		class, ok := cat.Class(op.Action)
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("operation %s names the action %q, which the catalogue does not know", op.label(), op.Action))
		case class != op.RateLimitClass:
			problems = append(problems, fmt.Sprintf("operation %s has the rate-limit class %q, but the catalogue gives %q for the action %q",
				op.label(), op.RateLimitClass, class, op.Action))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	errs := make([]error, len(problems))
	for i, p := range problems {
		errs[i] = errors.New("routes: " + p)
	}
	return errors.Join(errs...)
}

// Build registers impl through the generated glue on a Recorder, verifies the registrations
// with Verify and returns one Route per enabled operation, sorted by Operation.ID. On error it
// returns no routes, so nothing unverified can be mounted. impl must not be nil while
// operations are enabled.
func (t *Table[S]) Build(impl S, opts Options, cat Catalog) ([]Route, error) {
	if len(t.ops) > 0 && any(impl) == nil {
		return nil, fmt.Errorf("routes: no implementation for %d enabled operations", len(t.ops))
	}
	rec := &Recorder{}
	if t.register != nil {
		t.register(impl, opts, rec)
	}
	if err := t.Verify(rec.Patterns(), cat); err != nil {
		return nil, err
	}
	out := make([]Route, len(t.ops))
	for i, op := range t.ops {
		out[i] = Route{Operation: op.clone(), Handler: rec.handler(op.Pattern)}
	}
	return out, nil
}

// spec is the table generated from docs/api/openapi.yaml, docs/spec/actions.yaml and
// api/operations.yaml (operations, catalogEntries, register and Server are in the *.gen.go
// files of this package).
var spec = NewTable[Server](operations, catalogEntries, register)

// Enabled returns a copy of the operations enabled in api/operations.yaml, sorted by ID.
func Enabled() []Operation { return spec.Enabled() }

// ByPattern returns the enabled operation registered under pattern; see Table.ByPattern.
func ByPattern(pattern string) (Operation, bool) { return spec.ByPattern(pattern) }

// ByID returns the enabled operation with the operationId id.
func ByID(id string) (Operation, bool) { return spec.ByID(id) }

// SpecCatalog returns the action catalogue generated from docs/spec/actions.yaml. System actions
// and actions without a rate-limit class are unknown to it (05 rule 10).
func SpecCatalog() Catalog { return spec.Catalog() }

// Verify compares registered patterns with the enabled operations and cat; see Table.Verify.
func Verify(registered []string, cat Catalog) error { return spec.Verify(registered, cat) }

// Build builds and verifies the routes of the enabled operations; see Table.Build. impl is the
// composite of every module's handlers (02 "Layers within a module", rule 3).
func Build(impl Server, opts Options, cat Catalog) ([]Route, error) {
	return spec.Build(impl, opts, cat)
}
