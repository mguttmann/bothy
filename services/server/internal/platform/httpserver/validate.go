// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	verrors "github.com/pb33f/libopenapi-validator/errors"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// The vocabulary of FieldError.code (06 "Fehler" item 3, docs/DECISIONS.md M1-18/3).
const (
	fieldCodeRequired      = "required"
	fieldCodeUnknownMember = "unknown_member"
	fieldCodeInvalidType   = "invalid_type"
	fieldCodeInvalidFormat = "invalid_format"
	fieldCodeInvalidEnum   = "invalid_enum"
	fieldCodeTooLong       = "too_long"
	fieldCodeTooShort      = "too_short"
	fieldCodeTooLarge      = "too_large"
	fieldCodeTooSmall      = "too_small"
	fieldCodeTooManyItems  = "too_many_items"
	fieldCodeTooFewItems   = "too_few_items"
	fieldCodePattern       = "pattern_mismatch"
	fieldCodeInvalid       = "invalid"
)

// keywordCodes maps the last segment of a JSON Schema keyword location to the field code.
var keywordCodes = map[string]string{
	"required":              fieldCodeRequired,
	"additionalProperties":  fieldCodeUnknownMember,
	"unevaluatedProperties": fieldCodeUnknownMember,
	"type":                  fieldCodeInvalidType,
	"format":                fieldCodeInvalidFormat,
	"enum":                  fieldCodeInvalidEnum,
	"const":                 fieldCodeInvalidEnum,
	"maxLength":             fieldCodeTooLong,
	"minLength":             fieldCodeTooShort,
	"maximum":               fieldCodeTooLarge,
	"exclusiveMaximum":      fieldCodeTooLarge,
	"minimum":               fieldCodeTooSmall,
	"exclusiveMinimum":      fieldCodeTooSmall,
	"maxItems":              fieldCodeTooManyItems,
	"minItems":              fieldCodeTooFewItems,
	"pattern":               fieldCodePattern,
}

// fieldDetails are the English details per code; they never repeat the submitted value.
var fieldDetails = map[string]string{
	fieldCodeRequired:      "The member is required.",
	fieldCodeUnknownMember: "The member is not part of the schema.",
	fieldCodeInvalidType:   "The value has the wrong type.",
	fieldCodeInvalidFormat: "The value does not have the required format.",
	fieldCodeInvalidEnum:   "The value is not one of the allowed values.",
	fieldCodeTooLong:       "The value is too long.",
	fieldCodeTooShort:      "The value is too short.",
	fieldCodeTooLarge:      "The value is too large.",
	fieldCodeTooSmall:      "The value is too small.",
	fieldCodeTooManyItems:  "The array has too many items.",
	fieldCodeTooFewItems:   "The array has too few items.",
	fieldCodePattern:       "The value does not match the required pattern.",
	fieldCodeInvalid:       "The value is invalid.",
}

// quotedName finds the member names the validator quotes in "additional properties 'x' not
// allowed" and "missing property 'x'".
var quotedName = regexp.MustCompile(`'([^']*)'`)

// opEntry is one operation of the pruned contract.
type opEntry struct {
	item *v3.PathItem
	path string // the path key of the contract, without the server base URL
}

// Validator checks requests against the contract (SEC-IN-1) with
// github.com/pb33f/libopenapi-validator (MIT, OpenAPI 3.1, JSON Schema 2020-12; docs/DECISIONS.md
// M1-18/1). It knows only the operations it was built for: compiling the schemas of every
// operation of docs/api/openapi.yaml would add seconds to the start for routes that are not
// mounted. It is safe for concurrent use.
type Validator struct {
	v   validator.Validator
	ops map[string]opEntry
	// base is the server URL of the contract (/api/v1), cut from Operation.Path.
	base string
}

// NewValidator loads the OpenAPI 3.1 document spec, keeps the operations with the given IDs and
// compiles their schemas. The error names a document that does not load, an unknown or repeated
// operation ID, or a contract whose servers do not give one base path.
//
// Example:
//
//	v, err := httpserver.NewValidator(rest.Spec, []string{"getDiscovery"})
func NewValidator(spec []byte, operationIDs []string) (*Validator, error) {
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		return nil, fmt.Errorf("httpserver: contract: %w", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("httpserver: contract: %w", err)
	}
	m := &model.Model
	if len(m.Servers) != 1 {
		return nil, errors.New("httpserver: contract must declare exactly one server")
	}
	v := &Validator{ops: map[string]opEntry{}, base: strings.TrimSuffix(m.Servers[0].URL, "/")}
	keep, err := idSet(operationIDs)
	if err != nil {
		return nil, err
	}
	v.pruneAll(m, keep)
	var missing []string
	for id := range keep {
		if _, ok := v.ops[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("httpserver: operations not in the contract: %s", strings.Join(missing, ", "))
	}
	v.v = validator.NewValidatorFromV3Model(m, config.WithoutSecurityValidation(), config.WithFormatAssertions())
	return v, nil
}

// idSet returns the operation IDs as a set; a repeated ID is an error.
func idSet(ids []string) (map[string]bool, error) {
	keep := map[string]bool{}
	for _, id := range ids {
		if keep[id] {
			return nil, fmt.Errorf("httpserver: operation %q named twice", id)
		}
		keep[id] = true
	}
	return keep, nil
}

// pruneAll keeps only the operations in keep and drops path items left without one.
func (v *Validator) pruneAll(m *v3.Document, keep map[string]bool) {
	if m.Paths == nil || m.Paths.PathItems == nil {
		return
	}
	var empty []string
	for pair := m.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		if !v.prune(pair.Key(), pair.Value(), keep) {
			empty = append(empty, pair.Key())
		}
	}
	for _, p := range empty {
		m.Paths.PathItems.Delete(p)
	}
}

// prune removes the operations of item that keep does not name and records the others; it
// reports whether item still has an operation.
func (v *Validator) prune(path string, item *v3.PathItem, keep map[string]bool) bool {
	slots := []**v3.Operation{&item.Get, &item.Put, &item.Post, &item.Delete, &item.Options,
		&item.Head, &item.Patch, &item.Trace, &item.Query}
	left := false
	for _, slot := range slots {
		op := *slot
		if op == nil {
			continue
		}
		if !keep[op.OperationId] {
			*slot = nil
			continue
		}
		v.ops[op.OperationId] = opEntry{item: item, path: path}
		left = true
	}
	item.AdditionalOperations = nil
	return left
}

// Validate checks the parameters of r and, for a JSON operation, its body against the contract of
// op. It returns nil, or 400 validation_failed whose errors[] names each violation with a JSON
// Pointer into the body or #path/<name>, #query/<name>, #header/<name> and a code of the
// vocabulary above; a missing required DPoP header is 401 invalid_dpop_proof instead (M1-34). An
// operation the validator was not built for is 500 internal_error (fail closed: an unvalidated
// request must not reach a handler). The body is read and restored.
func (v *Validator) Validate(r *http.Request, op Operation) error {
	entry, ok := v.ops[op.ID]
	if !ok {
		return fmt.Errorf("httpserver: operation %s is not in the validator", op.ID)
	}
	found := v.parameterErrors(r, entry)
	if op.RequestBody == BodyJSON && r.ContentLength != 0 && r.Body != nil && r.Body != http.NoBody {
		list, err := v.bodyErrors(r, entry)
		if err != nil {
			return err
		}
		found = append(found, list...)
	}
	if len(found) == 0 {
		return nil
	}
	return problemOf(found)
}

// problemOf is the 400 validation_failed of the violations found, each field error once; a
// missing DPoP header is the 401 of missingProof instead.
func problemOf(found []*verrors.ValidationError) error {
	e := errs.New(errs.CodeValidationFailed, "")
	seen := map[errs.FieldError]bool{}
	for _, ve := range found {
		for _, f := range fieldErrors(ve) {
			if f.Pointer == pointerDPoP && f.Code == fieldCodeRequired {
				return missingProof()
			}
			if !seen[f] {
				seen[f] = true
				e = e.WithFields(f)
			}
		}
	}
	return e
}

// parameterErrors validates path, query, header and cookie parameters.
func (v *Validator) parameterErrors(r *http.Request, entry opEntry) []*verrors.ValidationError {
	pv := v.v.GetParameterValidator()
	checks := []func(*http.Request, *v3.PathItem, string) (bool, []*verrors.ValidationError){
		pv.ValidatePathParamsWithPathItem, pv.ValidateQueryParamsWithPathItem,
		pv.ValidateHeaderParamsWithPathItem, pv.ValidateCookieParamsWithPathItem,
	}
	var found []*verrors.ValidationError
	for _, check := range checks {
		if ok, list := check(r, entry.item, entry.path); !ok {
			found = append(found, list...)
		}
	}
	return found
}

// bodyErrors validates the JSON body and restores it for the next stages.
func (v *Validator) bodyErrors(r *http.Request, entry opEntry) ([]*verrors.ValidationError, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer func() { r.Body = io.NopCloser(bytes.NewReader(body)) }()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if ok, list := v.v.GetRequestBodyValidator().ValidateRequestBodyWithPathItem(r, entry.item, entry.path); !ok {
		return list, nil
	}
	return nil, nil
}

// pointerDPoP is the field pointer of the DPoP proof header.
const pointerDPoP = "#header/" + headerDPoP

// missingProof is the answer to a missing required DPoP header: 401 invalid_dpop_proof with the
// DPoP challenge, the code of every other failed proof (06 section 3.1, RFC 9449 section 7.1),
// not 400; the client must sign a proof, not fix a parameter (M1-34). The nonce comes with the
// next answer of the verifier.
func missingProof() error {
	return errs.New(errs.CodeInvalidDPoPProof, "").
		WithHeader("WWW-Authenticate", `DPoP error="invalid_dpop_proof", algs="ES256 EdDSA"`)
}

// fieldErrors turns one error of the validator into field errors.
func fieldErrors(ve *verrors.ValidationError) []errs.FieldError {
	if ve.ValidationType == "parameter" {
		return []errs.FieldError{parameterField(ve)}
	}
	if len(ve.SchemaValidationErrors) == 0 {
		return []errs.FieldError{field("", fieldCodeInvalid)}
	}
	var out []errs.FieldError
	for _, s := range ve.SchemaValidationErrors {
		out = append(out, schemaFields(s)...)
	}
	return out
}

// parameterField is the field error of a parameter: #path/<name>, #query/<name>, #header/<name>.
func parameterField(ve *verrors.ValidationError) errs.FieldError {
	code := fieldCodeInvalid
	for _, s := range ve.SchemaValidationErrors {
		code = keywordCode(s.KeywordLocation)
	}
	if strings.Contains(strings.ToLower(ve.Message), "missing") {
		code = fieldCodeRequired
	}
	return field("#"+ve.ValidationSubType+"/"+ve.ParameterName, code)
}

// schemaFields are the field errors of one body violation. required and additionalProperties
// report at the object that holds the member, so the member names quoted in the reason are added
// to the pointer; unevaluatedProperties reports at the member itself.
func schemaFields(s *verrors.SchemaValidationFailure) []errs.FieldError {
	code := keywordCode(s.KeywordLocation)
	base := ""
	if len(s.InstancePath) > 0 {
		base = "/" + strings.Join(s.InstancePath, "/")
	}
	names := quotedName.FindAllStringSubmatch(s.Reason, -1)
	if code != fieldCodeRequired && code != fieldCodeUnknownMember || len(names) == 0 {
		return []errs.FieldError{field(base, code)}
	}
	out := make([]errs.FieldError, 0, len(names))
	for _, n := range names {
		out = append(out, field(base+"/"+escapePointer(n[1]), code))
	}
	return out
}

// keywordCode maps a keyword location such as /properties/name/minLength to its field code.
func keywordCode(location string) string {
	kw := location[strings.LastIndex(location, "/")+1:]
	if code, ok := keywordCodes[kw]; ok {
		return code
	}
	return fieldCodeInvalid
}

// field builds one field error with the detail of its code.
func field(pointer, code string) errs.FieldError {
	return errs.FieldError{Pointer: pointer, Code: code, Detail: fieldDetails[code]}
}

// escapePointer escapes a member name as a JSON Pointer reference token (RFC 6901).
func escapePointer(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}
