// SPDX-License-Identifier: Apache-2.0

//go:build contract_validate

// Package contractcheck is G8 of docs/spec/06-api.md "Contract tests and gate": in the test build
// (build tag contract_validate) it validates the response of every request that the public
// listener answers for a mounted operation against docs/api/openapi.yaml (status,
// headers, media type and body), and ends the process with exit code 3 on the first violation.
// The untagged production build contains neither this package nor a reference to it;
// internal/platform/httpserver installs it from a file with the same build tag.
//
// Ruling (M1-71, docs/DECISIONS.md): a hard exit is the only mechanism that fails every
// integration test regardless of how it asserts, without touching each test; it exists only in
// the test build.
//
// Example:
//
//	// in a file with //go:build contract_validate, with the request validator of the chain
//	handler = contractcheck.Attach(specSum, "/api/v1", v, routes)(handler)
package contractcheck

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	valerrors "github.com/pb33f/libopenapi-validator/errors"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ExitCode is the exit code of a process whose server sent a response that violates the contract.
const ExitCode = 3

// The repository files the checker reads, relative to the repository root.
var (
	specPath       = filepath.Join("docs", "api", "openapi.yaml")
	enableListPath = filepath.Join("services", "server", "api", "operations.yaml")
)

// valueLimit bounds the offending value in a report.
const valueLimit = 200

// Violation is one response that does not match its operation.
type Violation struct {
	// OperationID, Method and Route name the operation (Route is the path of openapi.yaml).
	OperationID, Method, Route string
	// Status is the status the server sent.
	Status int
	// Findings are the single schema or protocol violations.
	Findings []Finding
}

// Finding is one violation inside a response.
type Finding struct {
	// Pointer is the JSON pointer into the body ("" for the status, a header or the media type).
	Pointer string
	// Keyword is the schema keyword that failed, or the kind of the protocol check.
	Keyword string
	// Message is the reason the library gives.
	Message string
	// Value is the offending value, truncated.
	Value string
}

// String formats the violation as the report on standard error: one line per finding with
// operationId, method, route, status, pointer, keyword, reason and value.
func (v Violation) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "contractcheck (G8): response violates docs/api/openapi.yaml\n")
	for _, f := range v.Findings {
		fmt.Fprintf(&b, "  operationId=%s method=%s route=%s status=%d pointer=%s keyword=%s reason=%q value=%q\n",
			v.OperationID, v.Method, v.Route, v.Status, f.Pointer, f.Keyword, f.Message, f.Value)
	}
	return b.String()
}

// Route is one operation the checker knows: its operationId, method, path of openapi.yaml (without
// the server base) and its path item in the parsed contract.
type Route struct {
	ID, Method, Path string
	Item             *v3.PathItem
}

// Checker validates responses of the operations it was built for. It is safe for concurrent use.
type Checker struct {
	mu     *sync.Mutex
	v      validator.Validator
	router *http.ServeMux
	routes map[string]Route // ServeMux pattern -> operation
	base   string
	fail   func(Violation)
}

// New builds a checker over the OpenAPI 3.1 document spec for the operations operationIDs (all
// operations for none). Errors: a document the library cannot load, a document without exactly
// one server, an operation ID that is not in the document.
func New(spec []byte, operationIDs []string) (*Checker, error) {
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("contractcheck: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("contractcheck: %w", err)
	}
	m := &model.Model
	if len(m.Servers) != 1 {
		return nil, errors.New("contractcheck: the contract must declare exactly one server")
	}
	keep := map[string]bool{}
	for _, id := range operationIDs {
		keep[id] = false
	}
	routes, err := prune(m, keep)
	if err != nil {
		return nil, err
	}
	v := validator.NewValidatorFromV3Model(m, config.WithoutSecurityValidation(), config.WithFormatAssertions())
	return FromValidator(strings.TrimSuffix(m.Servers[0].URL, "/"), v, routes), nil
}

// FromValidator returns a checker over an already built validator of the contract: base is the
// server URL of the contract (/api/v1), routes are the operations to check. The request validator
// of the chain passes its own validator here, so the contract is parsed once per process.
func FromValidator(base string, v validator.Validator, routes []Route) *Checker {
	c := &Checker{mu: &sync.Mutex{}, v: v, router: http.NewServeMux(), routes: map[string]Route{}, base: base, fail: exitOn}
	for _, rt := range routes {
		pattern := rt.Method + " " + base + rt.Path
		c.routes[pattern] = rt
		c.router.Handle(pattern, http.NotFoundHandler())
	}
	return c
}

// prune removes the operations keep does not name (none when keep is empty), returns the kept
// ones and reports IDs of keep that the document lacks.
func prune(m *v3.Document, keep map[string]bool) ([]Route, error) {
	var (
		empty  []string
		routes []Route
	)
	for pair := m.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		path, item := pair.Key(), pair.Value()
		left := false
		for method, op := range item.GetOperations().FromOldest() {
			if _, ok := keep[op.OperationId]; len(keep) > 0 && !ok {
				setOperation(item, method, nil)
				continue
			}
			keep[op.OperationId] = true
			left = true
			routes = append(routes, Route{ID: op.OperationId, Method: strings.ToUpper(method), Path: path, Item: item})
		}
		item.AdditionalOperations = nil
		if !left {
			empty = append(empty, path)
		}
	}
	for _, p := range empty {
		m.Paths.PathItems.Delete(p)
	}
	var missing []string
	for id, found := range keep {
		if !found {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("contractcheck: operations not in the contract: %s", strings.Join(missing, ", "))
	}
	return routes, nil
}

// setOperation sets the operation of method on item.
func setOperation(item *v3.PathItem, method string, op *v3.Operation) {
	switch strings.ToLower(method) {
	case "get":
		item.Get = op
	case "put":
		item.Put = op
	case "post":
		item.Post = op
	case "delete":
		item.Delete = op
	case "options":
		item.Options = op
	case "head":
		item.Head = op
	case "patch":
		item.Patch = op
	case "trace":
		item.Trace = op
	case "query":
		item.Query = op
	}
}

// WithFailure returns a copy of c that calls fail instead of exiting; tests record violations
// with it.
func (c *Checker) WithFailure(fail func(Violation)) *Checker {
	cp := *c
	cp.fail = fail
	return &cp
}

// match returns the operation of r, or false for a request no kept operation answers.
func (c *Checker) match(r *http.Request) (Route, bool) {
	_, pattern := c.router.Handler(r)
	rt, ok := c.routes[pattern]
	return rt, ok
}

// Wrap returns next with the response check around it. Responses of requests that match no kept
// operation pass unchecked.
func (c *Checker) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt, ok := c.match(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		rec := &capture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if v, bad := c.check(r, rt, rec.status, w.Header(), rec.body.Bytes()); bad {
			c.fail(v)
		}
	})
}

// check validates one finished response of operation rt and reports whether it violates the
// contract.
func (c *Checker) check(r *http.Request, rt Route, status int, header http.Header, body []byte) (Violation, bool) {
	bodies := [][]byte{body}
	if media, _, err := mime.ParseMediaType(header.Get("Content-Type")); err == nil && media == "application/x-ndjson" {
		bodies = nil
		for _, line := range bytes.Split(body, []byte("\n")) {
			if len(line) > 0 {
				bodies = append(bodies, line)
			}
		}
	}
	v := Violation{OperationID: rt.ID, Method: rt.Method, Route: rt.Path, Status: status}
	for _, b := range bodies {
		resp := &http.Response{StatusCode: status, Header: header.Clone(), Body: io.NopCloser(bytes.NewReader(b)), Request: r}
		c.mu.Lock()
		// The route of the server decides the operation; the library's own path search would
		// prefer a literal path of another method (GET /grants/preview against getGrant).
		ok, errs := c.v.GetResponseBodyValidator().ValidateResponseBodyWithPathItem(r, resp, rt.Item, rt.Path)
		c.mu.Unlock()
		if !ok {
			v.Findings = append(v.Findings, findings(errs, b)...)
		}
	}
	return v, len(v.Findings) > 0
}

// findings turns the library errors into findings with pointer, keyword and value.
func findings(errs []*valerrors.ValidationError, body []byte) []Finding {
	var doc any
	_ = json.Unmarshal(body, &doc)
	var out []Finding
	for _, e := range errs {
		if len(e.SchemaValidationErrors) == 0 {
			out = append(out, Finding{Keyword: e.ValidationType + "/" + e.ValidationSubType, Message: e.Message + ": " + e.Reason})
			continue
		}
		for _, f := range e.SchemaValidationErrors {
			ptr := pointer(f.InstancePath)
			kw := f.KeywordLocation[strings.LastIndex(f.KeywordLocation, "/")+1:]
			if kw == "required" {
				// The library reports a missing member at its parent; the pointer names the member.
				if name := quoted(f.Reason); name != "" {
					ptr += "/" + name
				}
			}
			out = append(out, Finding{Pointer: ptr, Keyword: kw, Message: f.Reason, Value: valueAt(doc, f.InstancePath)})
		}
	}
	return out
}

// pointer builds a JSON pointer (RFC 6901) from path segments.
func pointer(segments []string) string {
	var b strings.Builder
	for _, s := range segments {
		b.WriteString("/" + strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// quoted returns the first name in single quotes of reason ("missing property 'x'").
func quoted(reason string) string {
	_, rest, ok := strings.Cut(reason, "'")
	if !ok {
		return ""
	}
	name, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return name
}

// valueAt returns the JSON text at path in doc, truncated to valueLimit bytes.
func valueAt(doc any, path []string) string {
	for _, seg := range path {
		switch d := doc.(type) {
		case map[string]any:
			doc = d[seg]
		case []any:
			var i int
			if _, err := fmt.Sscanf(seg, "%d", &i); err != nil || i < 0 || i >= len(d) {
				return ""
			}
			doc = d[i]
		default:
			return ""
		}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	if len(b) > valueLimit {
		return string(b[:valueLimit]) + "..."
	}
	return string(b)
}

// exitOn writes the report of v to standard error and ends the process with ExitCode.
func exitOn(v Violation) {
	_, _ = io.WriteString(os.Stderr, v.String())
	os.Exit(ExitCode)
}

// capture passes the response through and keeps a copy of status and body.
type capture struct {
	http.ResponseWriter
	status  int
	written bool
	body    bytes.Buffer
}

// WriteHeader records the first status and passes it on.
func (c *capture) WriteHeader(code int) {
	if !c.written && code >= 200 {
		c.status, c.written = code, true
	}
	c.ResponseWriter.WriteHeader(code)
}

// Write records the bytes and passes them on.
func (c *capture) Write(b []byte) (int, error) {
	c.written = true
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

// Flush passes a flush on when the writer below supports it.
func (c *capture) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the writer below for http.ResponseController.
func (c *capture) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// Load builds the checker of the repository: docs/api/openapi.yaml restricted to the enable list
// of services/server/api/operations.yaml, both found by walking up from the working directory.
// Errors: a missing file, an enable list without entries, those of New.
func Load() (*Checker, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	spec, err := os.ReadFile(filepath.Join(root, specPath)) // #nosec G304 -- a fixed path below the repository root
	if err != nil {
		return nil, fmt.Errorf("contractcheck: %w", err)
	}
	list, err := os.ReadFile(filepath.Join(root, enableListPath)) // #nosec G304 -- a fixed path below the repository root
	if err != nil {
		return nil, fmt.Errorf("contractcheck: %w", err)
	}
	ids := EnabledOperations(list)
	if len(ids) == 0 {
		return nil, errors.New("contractcheck: the enable list names no operation")
	}
	return New(spec, ids)
}

// EnabledOperations returns the operation IDs of the section enabled of an enable list
// (services/server/api/operations.yaml, one "  - <id>" line per entry).
func EnabledOperations(list []byte) []string {
	var ids []string
	in := false
	for _, line := range strings.Split(string(list), "\n") {
		switch {
		case strings.HasPrefix(line, "enabled:"):
			in = true
		case line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#"):
			in = false
		case in && strings.HasPrefix(line, "  - "):
			id, _, _ := strings.Cut(strings.TrimPrefix(line, "  - "), "#")
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	return ids
}

// repoRoot walks up from the working directory to the directory that holds docs/api/openapi.yaml.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("contractcheck: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, specPath)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("contractcheck: %s not found above the working directory", specPath)
		}
		dir = parent
	}
}

// repoSum is the SHA-256 of docs/api/openapi.yaml of the repository, read once.
var repoSum = sync.OnceValues(func() ([sha256.Size]byte, error) {
	root, err := repoRoot()
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	spec, err := os.ReadFile(filepath.Join(root, specPath)) // #nosec G304 -- a fixed path below the repository root
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("contractcheck: %w", err)
	}
	return sha256.Sum256(spec), nil
})

// Attach returns the G8 middleware for a server whose request validator v was built from a
// contract with the SHA-256 specSum: when that contract is docs/api/openapi.yaml of the
// repository, every response of routes is checked and a violation ends the process with
// ExitCode; for any other contract (test fixtures) the middleware passes everything. A missing
// docs/api/openapi.yaml ends the process with ExitCode at once.
func Attach(specSum [sha256.Size]byte, base string, v validator.Validator, routes []Route) func(http.Handler) http.Handler {
	sum, err := repoSum()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "contractcheck (G8): %v\n", err)
		os.Exit(ExitCode)
	}
	if sum != specSum {
		return func(next http.Handler) http.Handler { return next }
	}
	return FromValidator(base, v, routes).Wrap
}
