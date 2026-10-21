// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/schema_validation"
	"github.com/pb33f/libopenapi/datamodel/high/base"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// SchemaValidator checks a JSON value against one named schema of components.schemas of the
// contract, with the engine and the field codes of the request Validator (SEC-IN-1, W7): the
// gateway validates the body d of every client frame with it (M1-64), where no HTTP operation
// exists. It knows only the schemas it was built for and is safe for concurrent use: libopenapi
// renders a schema while it validates and writes into the shared schema model, so one check runs
// at a time (a race under -race with several connections, M1-67).
type SchemaValidator struct {
	mu      sync.Mutex // serializes ValidateSchemaBytes over the shared schema model
	v       schema_validation.SchemaValidator
	schemas map[string]*base.Schema
}

// NewSchemaValidator loads the OpenAPI 3.1 document spec and keeps the schemas of
// components.schemas with the given names. The error names a document that does not load and
// every unknown or repeated name.
//
// Example:
//
//	v, err := httpserver.NewSchemaValidator(rest.Spec, []string{"GatewayIdentify"})
func NewSchemaValidator(spec []byte, names []string) (*SchemaValidator, error) {
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("httpserver: contract: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("httpserver: contract: %w", err)
	}
	keep, err := idSet(names)
	if err != nil {
		return nil, err
	}
	sv := &SchemaValidator{
		v:       schema_validation.NewSchemaValidator(config.WithFormatAssertions()),
		schemas: map[string]*base.Schema{},
	}
	var missing []string
	for name := range keep {
		var proxy *base.SchemaProxy
		if model.Model.Components != nil && model.Model.Components.Schemas != nil {
			proxy = model.Model.Components.Schemas.GetOrZero(name)
		}
		if proxy == nil {
			missing = append(missing, name)
			continue
		}
		s, err := proxy.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("httpserver: schema %s: %w", name, err)
		}
		sv.schemas[name] = s
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("httpserver: schemas not in the contract: %s", strings.Join(missing, ", "))
	}
	return sv, nil
}

// Validate checks the JSON value body against the schema name. It returns nil for a valid value
// and 400 validation_failed with one field error per violation (JSON Pointer relative to the
// value, the codes of the request Validator) otherwise. body must already have passed CheckJSON:
// duplicate members and invalid UTF-8 are not this check. A name the validator was not built for
// is an internal error, never a client error.
func (v *SchemaValidator) Validate(name string, body []byte) error {
	s, ok := v.schemas[name]
	if !ok {
		return errs.Wrap(errs.CodeInternalError, errors.New("httpserver: unknown schema "+name), "httpserver: schema validator")
	}
	v.mu.Lock()
	valid, found := v.v.ValidateSchemaBytes(s, body)
	v.mu.Unlock()
	if !valid {
		if len(found) == 0 {
			return errs.New(errs.CodeValidationFailed, "").WithFields(field("", fieldCodeInvalid))
		}
		return problemOf(found)
	}
	return nil
}
