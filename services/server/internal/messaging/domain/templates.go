// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Field error codes of the template rules (brief M1-60 notes 8 and 9).
const (
	// FieldRequired marks a name or content that is empty after trimming.
	FieldRequired = "required"
	// FieldTooLong marks a name above template.name_max_chars or a content above
	// message.max_content_bytes.
	FieldTooLong = "too_long"
	// FieldInvalidCharacters marks a name with a control or bidi control character (SEC-IN-4).
	FieldInvalidCharacters = "invalid_characters"
	// FieldLimitReached marks a create beyond template.max_per_scope (pointer empty).
	FieldLimitReached = "limit_reached"
)

// Scope is the kind of a template: personal or of a channel (schema Template.scope).
type Scope string

// The scopes of 03 message_templates.
const (
	ScopePersonal Scope = "personal"
	ScopeChannel  Scope = "channel"
)

// Template is one row of message_templates. Exactly one of OwnerUserID and ChannelID is set
// (check message_templates_scope_check); DeletedAt is zero while the template exists.
type Template struct {
	ID          ids.ID
	OwnerUserID ids.ID
	ChannelID   ids.ID
	Name        string
	ContentMD   string
	CreatedBy   ids.ID
	CreatedAt   time.Time
	DeletedAt   time.Time
	Version     int64
}

// Scope returns personal for a template with an owner and channel otherwise.
func (t Template) Scope() Scope {
	if !t.OwnerUserID.IsZero() {
		return ScopePersonal
	}
	return ScopeChannel
}

// Apply returns t with the given members (nil keeps the stored value) and whether anything
// changed. A change without members or with the stored values is a no-op (brief note 10).
//
// Example:
//
//	next, changed := cur.Apply(&name, nil)
func (t Template) Apply(name, content *string) (Template, bool) {
	next := t
	if name != nil {
		next.Name = *name
	}
	if content != nil {
		next.ContentMD = *content
	}
	return next, next.Name != t.Name || next.ContentMD != t.ContentMD
}

// TemplateRules are the limits of a template: NameMaxChars is template.name_max_chars,
// ContentMaxBytes is message.max_content_bytes (a longer template could never be sent).
type TemplateRules struct {
	NameMaxChars    int
	ContentMaxBytes int
}

// Check validates a whole template (create). Errors: validation_failed with one field error per
// wrong member, the name first; the details never repeat the input.
func (r TemplateRules) Check(name, content string) error {
	return fieldErrors(r.nameError(name), r.contentError(content))
}

// CheckChange validates the given members of a change (nil members are not checked).
func (r TemplateRules) CheckChange(name, content *string) error {
	var fs []errs.FieldError
	if name != nil {
		fs = append(fs, r.nameError(*name)...)
	}
	if content != nil {
		fs = append(fs, r.contentError(*content)...)
	}
	return fieldErrors(fs)
}

// nameError checks a name: not blank, at most NameMaxChars code points, no control or bidi
// control character (kernel/textcheck).
func (r TemplateRules) nameError(name string) []errs.FieldError {
	switch {
	case strings.TrimSpace(name) == "":
		return field("/name", FieldRequired, "the name is required")
	case textcheck.DisplayText(name) != nil:
		return field("/name", FieldInvalidCharacters, "the name contains a control or bidi control character")
	case utf8.RuneCountInString(name) > r.NameMaxChars:
		return field("/name", FieldTooLong, "the name is longer than template.name_max_chars")
	}
	return nil
}

// contentError checks a content: not blank, at most ContentMaxBytes bytes and without U+0000,
// which PostgreSQL text cannot store.
func (r TemplateRules) contentError(content string) []errs.FieldError {
	switch {
	case strings.TrimSpace(content) == "":
		return field("/content_md", FieldRequired, "the content is required")
	case len(content) > r.ContentMaxBytes:
		return field("/content_md", FieldTooLong, "the content is longer than message.max_content_bytes")
	case strings.ContainsRune(content, 0):
		return field("/content_md", FieldInvalid, "the content contains U+0000")
	}
	return nil
}

// field returns one field error.
func field(pointer, code, detail string) []errs.FieldError {
	return []errs.FieldError{{Pointer: pointer, Code: code, Detail: detail}}
}

// fieldErrors returns validation_failed with the field errors of all lists, or nil for none.
func fieldErrors(lists ...[]errs.FieldError) error {
	var fs []errs.FieldError
	for _, l := range lists {
		fs = append(fs, l...)
	}
	if len(fs) == 0 {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "the template is not valid").WithFields(fs...)
}

// LimitReached is the 400 of a create beyond template.max_per_scope (brief note 9): pointer
// empty, code limit_reached, the detail names the key.
func LimitReached() error {
	const detail = "the scope already holds template.max_per_scope templates"
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: "", Code: FieldLimitReached, Detail: detail})
}

// scopeLockDomain separates the template lock keys from any other use of the hash.
const scopeLockDomain = "bothy/message_templates/scope\x00"

// ScopeLockKey returns the key of the transaction advisory lock of one scope (brief note 9):
// the first eight bytes of SHA-256 over "personal:<id>" or "channel:<id>". Two scopes that share
// a key only serialize their creates; the bound stays exact.
func ScopeLockKey(scope Scope, id ids.ID) int64 {
	sum := sha256.Sum256([]byte(scopeLockDomain + string(scope) + ":" + id.String()))
	return int64(binary.BigEndian.Uint64(sum[:8])) //nolint:gosec // G115: the bits are a lock key, the sign is irrelevant
}

// MentionKey is the keyset position of the mention list (brief note 2): the creation time of the
// mention and the message id, both descending.
type MentionKey struct {
	At        time.Time `json:"at"`
	MessageID ids.ID    `json:"id"`
}
