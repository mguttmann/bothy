// SPDX-License-Identifier: Apache-2.0

package httpservertest

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ResponseValidator checks finished responses against the response schemas of an OpenAPI 3.1
// contract (status, media type and body against the component schema, JSON Schema 2020-12),
// with the same validator library as the request validator of the chain. Handler tests set
// Options.ValidateResponses, so every response of the harness is checked (M1-31: "every
// response matches its schema").
type ResponseValidator struct {
	// mu serializes Validate: the shared validator serves parallel tests, and the library does not
	// promise concurrent use.
	mu sync.Mutex
	v  validator.Validator
}

// validators caches one validator per contract and operation list: building one over the whole
// docs/api/openapi.yaml takes seconds (more than a minute under -race), and the handler tests build
// a harness per test (red team M1-31 round 2, the class of M1-33/18).
var validators sync.Map // [sha256.Size]byte -> func() (*ResponseValidator, error)

// SharedResponseValidator returns the validator of spec for operationIDs, built once per process,
// contract and list. Errors: those of NewResponseValidator, also on later calls.
func SharedResponseValidator(spec []byte, operationIDs []string) (*ResponseValidator, error) {
	ids := slices.Sorted(slices.Values(operationIDs))
	key := sha256.Sum256(append(append(slices.Clone(spec), 0), []byte(strings.Join(ids, "\x00"))...))
	build, _ := validators.LoadOrStore(key, sync.OnceValues(func() (*ResponseValidator, error) {
		return NewResponseValidator(spec, ids)
	}))
	return build.(func() (*ResponseValidator, error))()
}

// NewResponseValidator builds the validator over spec, keeping only the operations operationIDs
// (all for none), like the request validator of the chain, so the schemas of other operations are
// never compiled. SharedResponseValidator caches it; it is safe for concurrent use (Validate
// serializes). Errors: a spec the library cannot load, an operation id not in the spec.
func NewResponseValidator(spec []byte, operationIDs []string) (*ResponseValidator, error) {
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, err
	}
	m, err := doc.BuildV3Model()
	if err != nil {
		return nil, err
	}
	if len(operationIDs) > 0 {
		if err := keepOperations(&m.Model, operationIDs); err != nil {
			return nil, err
		}
	}
	return &ResponseValidator{v: validator.NewValidatorFromV3Model(&m.Model, config.WithoutSecurityValidation(), config.WithFormatAssertions())}, nil
}

// keepOperations removes every operation of m that ids does not name and the path items left
// empty. Errors: an id that is not in m.
func keepOperations(m *v3.Document, ids []string) error {
	keep := map[string]bool{}
	for _, id := range ids {
		keep[id] = false
	}
	var empty []string
	for pair := m.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		if !keepItem(pair.Value(), keep) {
			empty = append(empty, pair.Key())
		}
	}
	for _, p := range empty {
		m.Paths.PathItems.Delete(p)
	}
	for id, found := range keep {
		if !found {
			return errors.New("httpservertest: operation " + id + " is not in the contract")
		}
	}
	return nil
}

// keepItem removes the operations of item that keep does not name, marks the kept ones found and
// reports whether item still has one.
func keepItem(item *v3.PathItem, keep map[string]bool) bool {
	left := false
	for _, slot := range []**v3.Operation{&item.Get, &item.Put, &item.Post, &item.Delete, &item.Options, &item.Head, &item.Patch, &item.Trace, &item.Query} {
		if *slot == nil {
			continue
		}
		if _, ok := keep[(*slot).OperationId]; !ok {
			*slot = nil
			continue
		}
		keep[(*slot).OperationId], left = true, true
	}
	item.AdditionalOperations = nil
	return left
}

// Validate checks resp, the answer to req, and returns every finding joined, or nil. req must
// carry the path the contract names (with its server base).
func (rv *ResponseValidator) Validate(req *http.Request, resp *Response) error {
	clone := *resp.Response
	clone.Body = io.NopCloser(bytes.NewReader(resp.Body))
	rv.mu.Lock()
	ok, list := rv.v.ValidateHttpResponse(req, &clone)
	rv.mu.Unlock()
	if ok {
		return nil
	}
	var b strings.Builder
	for _, e := range list {
		fmt.Fprintf(&b, "%s: %s", e.Message, e.Reason)
		for _, f := range e.SchemaValidationErrors {
			fmt.Fprintf(&b, " [%s %s]", f.FieldPath, f.Reason)
		}
		b.WriteString("; ")
	}
	return errors.New(b.String())
}
