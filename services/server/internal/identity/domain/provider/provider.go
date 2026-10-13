// SPDX-License-Identifier: Apache-2.0

// Package provider holds the rules of the identity providers of the identity module
// (docs/spec/03-datenmodell.md table identity_providers, docs/spec/08-identitaet.md sections 2.1
// and 2.6): the kinds, the provider name and the strict validation of the configuration schemas
// identity_provider.oidc.v1, identity_provider.trusted_header.v1 and identity_provider.local.v1.
//
// The package is pure: it resolves no secret reference and opens no connection. Secret references
// are checked for their syntax through the port SecretRefs, which the composition root implements
// with the parser of platform/secrets.
//
// Validation errors are kernel/errs validation_failed with one field error per violation; the
// pointers start with /config (or /name) and the details state the rule, never the input, because
// a secret may stand where a reference belongs.
//
// Example:
//
//	cfg, err := provider.Validate(provider.KindOIDC, raw, refs)
//	// cfg is the normalized configuration as it is stored and returned
package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Kind is identity_providers.kind.
type Kind string

// The provider kinds of 08 section 2.1.
const (
	// KindOIDC is an OpenID Connect provider; any number may exist.
	KindOIDC Kind = "oidc"
	// KindTrustedHeader is a reverse proxy that sets the identity in a header; at most one exists.
	KindTrustedHeader Kind = "trusted_header"
	// KindLocal is the local account driver; at most one exists.
	KindLocal Kind = "local"
)

// Valid reports whether k is a value of the check on identity_providers.kind.
func (k Kind) Valid() bool {
	return k == KindOIDC || k == KindTrustedHeader || k == KindLocal
}

// SecretRefs checks the syntax of a secret reference without resolving it (19 section 10).
type SecretRefs interface {
	// Valid returns nil for a reference the secret provider could resolve and an error otherwise.
	// The error must not repeat the reference.
	Valid(ref string) error
}

// Field error codes of this package.
const (
	// CodeRequired marks a missing or empty member.
	CodeRequired = "required"
	// CodeUnknownMember marks a member the schema does not define.
	CodeUnknownMember = "unknown_member"
	// CodeDuplicateKey marks a member that appears twice.
	CodeDuplicateKey = "duplicate_key"
	// CodeNull marks a JSON null.
	CodeNull = "null_not_allowed"
	// CodeInvalidType marks a value of the wrong JSON type.
	CodeInvalidType = "invalid_type"
	// CodeInvalidEnum marks a value outside its enumeration.
	CodeInvalidEnum = "invalid_enum"
	// CodeInvalid marks a value that breaks the rule of its member.
	CodeInvalid = "invalid"
	// CodeNotAllowed marks a member that the other members exclude.
	CodeNotAllowed = "not_allowed"
	// CodeTooLong marks a name above its limit.
	CodeTooLong = "too_long"
)

// Pointer prefixes of the request members.
const (
	pointerConfig = "/config"
	pointerName   = "/name"
)

// errNotObject is the detail of a configuration that is no JSON object.
const detailNotObject = "The configuration must be a JSON object."

// NewName returns the trimmed provider name. Errors: validation_failed at /name when it is empty
// after trimming, longer than maxChars runes or breaks the display-text rules of SEC-IN-4.
func NewName(s string, maxChars int) (string, error) {
	n := strings.TrimSpace(s)
	switch {
	case n == "":
		return "", invalid(field(pointerName, CodeRequired, "The name must not be empty."))
	case utf8.RuneCountInString(n) > maxChars:
		return "", invalid(field(pointerName, CodeTooLong, "The name is longer than "+strconv.Itoa(maxChars)+" characters."))
	}
	if err := textcheck.DisplayText(n); err != nil {
		var te *textcheck.Error
		code := CodeInvalid
		if errors.As(err, &te) {
			code = string(te.Rule)
		}
		return "", invalid(field(pointerName, code, "The name contains a character that is not allowed."))
	}
	return n, nil
}

// Validate validates raw against the schema of kind and returns the normalized configuration.
// Errors: validation_failed for an unknown kind (at /kind) and for every violation of the schema.
func Validate(kind Kind, raw json.RawMessage, refs SecretRefs) (json.RawMessage, error) {
	switch kind {
	case KindOIDC:
		return ValidateOIDC(raw, refs)
	case KindTrustedHeader:
		return ValidateTrustedHeader(raw)
	case KindLocal:
		return ValidateLocal(raw)
	}
	return nil, invalid(field("/kind", CodeInvalidEnum, "The kind is not one of oidc, trusted_header, local."))
}

// ValidateLocal validates identity_provider.local.v1: an empty object. It returns `{}`.
func ValidateLocal(raw json.RawMessage) (json.RawMessage, error) {
	if _, err := members(raw, nil); err != nil {
		return nil, err
	}
	return json.RawMessage(`{}`), nil
}

// field builds a field error.
func field(pointer, code, detail string) errs.FieldError {
	return errs.FieldError{Pointer: pointer, Code: code, Detail: detail}
}

// invalid returns validation_failed with the field errors fs.
func invalid(fs ...errs.FieldError) error {
	return errs.New(errs.CodeValidationFailed, "The identity provider is not valid.").WithFields(fs...)
}

// memberPointer is the pointer of the top-level member name.
func memberPointer(name string) string {
	return pointerConfig + "/" + escapePointer(name)
}

// escapePointer escapes a reference token of a JSON pointer (RFC 6901 section 3).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// members checks raw strictly (one JSON object, no duplicate key and no null at any depth, no
// member outside allowed) and returns its top-level members. A nil allowed admits no member.
func members(raw json.RawMessage, allowed []string) (map[string]json.RawMessage, error) {
	fs, err := scan(raw)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, invalid(field(pointerConfig, CodeInvalidType, detailNotObject))
	}
	for name := range m {
		if !contains(allowed, name) {
			fs = append(fs, field(memberPointer(name), CodeUnknownMember, "The schema has no such member."))
		}
	}
	if len(fs) > 0 {
		sortFields(fs)
		return nil, invalid(fs...)
	}
	return m, nil
}

// contains reports whether list holds s.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sortFields orders field errors by pointer, so one input always gives the same body.
func sortFields(fs []errs.FieldError) {
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fs[j].Pointer < fs[j-1].Pointer; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// frame is one open container while scan walks the tokens.
type frame struct {
	object    bool
	expectKey bool
	pointer   string
	key       string
	index     int
	keys      map[string]bool
}

// child returns the pointer of the next value inside f.
func (f *frame) child() string {
	if f.object {
		return f.pointer + "/" + escapePointer(f.key)
	}
	return f.pointer + "/" + strconv.Itoa(f.index)
}

// scan walks raw with the tokenizer of encoding/json. A value that is not one JSON object is an
// error at /config; duplicate keys and nulls at any depth are field errors at their pointer.
func scan(raw json.RawMessage) ([]errs.FieldError, error) {
	notObject := invalid(field(pointerConfig, CodeInvalidType, detailNotObject))
	if !utf8.Valid(raw) {
		return nil, notObject
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, notObject
	}
	w := walker{stack: []*frame{{object: true, expectKey: true, pointer: pointerConfig, keys: map[string]bool{}}}}
	for len(w.stack) > 0 {
		tok, err := dec.Token()
		if err != nil {
			return nil, notObject
		}
		w.step(tok)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, notObject
	}
	return w.fs, nil
}

// walker is the state of scan: the open containers and the field errors found so far.
type walker struct {
	stack []*frame
	fs    []errs.FieldError
}

// step applies one token.
func (w *walker) step(tok json.Token) {
	top := w.stack[len(w.stack)-1]
	if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
		w.stack = w.stack[:len(w.stack)-1]
		return
	}
	if top.object && top.expectKey {
		k, _ := tok.(string)
		top.key, top.expectKey = k, false
		if top.keys[k] {
			w.fs = append(w.fs, field(top.child(), CodeDuplicateKey, "The member appears more than once."))
		}
		top.keys[k] = true
		return
	}
	ptr := top.child()
	if top.object {
		top.expectKey = true
	} else {
		top.index++
	}
	switch v := tok.(type) {
	case nil:
		w.fs = append(w.fs, field(ptr, CodeNull, "The value must not be null."))
	case json.Delim:
		w.stack = append(w.stack, &frame{object: v == '{', expectKey: v == '{', pointer: ptr, keys: map[string]bool{}})
	}
}

// fieldSet collects the field errors of one configuration.
type fieldSet struct {
	fs []errs.FieldError
}

// add records a field error.
func (s *fieldSet) add(pointer, code, detail string) {
	s.fs = append(s.fs, field(pointer, code, detail))
}

// err returns validation_failed with the collected errors, or nil.
func (s *fieldSet) err() error {
	if len(s.fs) == 0 {
		return nil
	}
	sortFields(s.fs)
	return invalid(s.fs...)
}

// str decodes the string member name of m into dst; present reports a member of the right type.
func (s *fieldSet) str(m map[string]json.RawMessage, name string, dst *string) bool {
	raw, ok := m[name]
	if !ok {
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		s.add(memberPointer(name), CodeInvalidType, "The value must be a string.")
		return false
	}
	return true
}

// strings decodes the array of strings name of m into dst.
func (s *fieldSet) strings(m map[string]json.RawMessage, name string, dst *[]string) bool {
	raw, ok := m[name]
	if !ok {
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		s.add(memberPointer(name), CodeInvalidType, "The value must be an array of strings.")
		return false
	}
	return true
}

// text checks a plain text member: not empty and free of control and bidi characters
// (SEC-IN-4).
func (s *fieldSet) text(pointer, v string) {
	switch {
	case strings.TrimSpace(v) == "":
		s.add(pointer, CodeRequired, "The value must not be empty.")
	case textcheck.DisplayText(v) != nil:
		s.add(pointer, CodeInvalid, "The value contains a character that is not allowed.")
	}
}

// canonical encodes v; the struct order of the schema types fixes the member order.
func canonical(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "identity: provider configuration")
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n")), nil
}
