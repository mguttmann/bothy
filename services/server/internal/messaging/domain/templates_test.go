// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Limits of the template tests.
const (
	nameMax    = 12
	contentMax = 32
)

// TestTemplateValidation (brief M1-60 note 8): the name is required after trimming, at most
// template.name_max_chars code points and free of control and bidi characters; the content is
// required after trimming and at most message.max_content_bytes bytes.
func TestTemplateValidation(t *testing.T) {
	t.Parallel()
	rules := domain.TemplateRules{NameMaxChars: nameMax, ContentMaxBytes: contentMax}
	ok := []struct{ name, content string }{
		{strings.Repeat("ä", nameMax), "hi"},
		{" Escalation ", strings.Repeat("a", contentMax)},
		{"Ok", "line one\nline two"},
	}
	for _, c := range ok {
		if err := rules.Check(c.name, c.content); err != nil {
			t.Errorf("%q/%q: %v", c.name, c.content, err)
		}
	}
	bad := []struct{ name, content, pointer, code string }{
		{"", "hi", "/name", domain.FieldRequired},
		{" \t ", "hi", "/name", domain.FieldRequired},
		{strings.Repeat("ä", nameMax+1), "hi", "/name", domain.FieldTooLong},
		{"a\u202eb", "hi", "/name", domain.FieldInvalidCharacters},
		{"a\u0007b", "hi", "/name", domain.FieldInvalidCharacters},
		{"Ok", "", "/content_md", domain.FieldRequired},
		{"Ok", " \n ", "/content_md", domain.FieldRequired},
		{"Ok", strings.Repeat("a", contentMax+1), "/content_md", domain.FieldTooLong},
		{"Ok", "a\x00b", "/content_md", domain.FieldInvalid},
	}
	for _, c := range bad {
		err := rules.Check(c.name, c.content)
		e, isErr := errs.As(err)
		if !isErr || e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 || e.Fields[0].Pointer != c.pointer || e.Fields[0].Code != c.code {
			t.Errorf("%q/%q: %v, want %s %s", c.name, c.content, err, c.pointer, c.code)
		}
	}
	// Both fields wrong: both are reported.
	e, _ := errs.As(rules.Check("", ""))
	if e == nil || len(e.Fields) != 2 {
		t.Fatalf("two wrong fields: %v", e)
	}
}

// TestTemplateNoop (brief M1-60 note 10): a change with the stored values or without any member
// changes nothing.
func TestTemplateNoop(t *testing.T) {
	t.Parallel()
	cur := domain.Template{Name: "Escalation", ContentMD: "Please escalate."}
	name, content, other := "Escalation", "Please escalate.", "Other"
	cases := []struct {
		name, content *string
		noop          bool
	}{
		{nil, nil, true},
		{&name, nil, true},
		{nil, &content, true},
		{&name, &content, true},
		{&other, nil, false},
		{nil, &other, false},
		{&name, &other, false},
	}
	for i, c := range cases {
		next, changed := cur.Apply(c.name, c.content)
		if changed == c.noop {
			t.Errorf("case %d: changed %v, want noop %v", i, changed, c.noop)
		}
		if c.noop && next != cur {
			t.Errorf("case %d: a no-op changed the template: %+v", i, next)
		}
	}
	next, _ := cur.Apply(&other, nil)
	if next.Name != other || next.ContentMD != cur.ContentMD {
		t.Fatalf("apply %+v", next)
	}
}

// TestTemplateScope: the scope of a row and the key of its count lock.
func TestTemplateScope(t *testing.T) {
	t.Parallel()
	p := domain.Template{OwnerUserID: [16]byte{1}}
	c := domain.Template{ChannelID: [16]byte{2}}
	if p.Scope() != domain.ScopePersonal || c.Scope() != domain.ScopeChannel {
		t.Fatalf("scopes %s %s", p.Scope(), c.Scope())
	}
	if domain.ScopeLockKey(domain.ScopePersonal, p.OwnerUserID) == domain.ScopeLockKey(domain.ScopeChannel, p.OwnerUserID) {
		t.Fatal("personal and channel lock keys of the same id are equal")
	}
	first, second := domain.ScopeLockKey(domain.ScopeChannel, c.ChannelID), domain.ScopeLockKey(domain.ScopeChannel, c.ChannelID)
	if first != second {
		t.Fatal("the lock key is not stable")
	}
}

// TestTemplateCheckChangeAndLimit: a change checks only the given members; the bound error has
// an empty pointer and names the key.
func TestTemplateCheckChangeAndLimit(t *testing.T) {
	t.Parallel()
	rules := domain.TemplateRules{NameMaxChars: nameMax, ContentMaxBytes: contentMax}
	empty, ok := "", "Ok"
	if err := rules.CheckChange(nil, nil); err != nil {
		t.Fatalf("empty change: %v", err)
	}
	if err := rules.CheckChange(&ok, nil); err != nil {
		t.Fatalf("name only: %v", err)
	}
	e, _ := errs.As(rules.CheckChange(nil, &empty))
	if e == nil || len(e.Fields) != 1 || e.Fields[0].Pointer != "/content_md" {
		t.Fatalf("content: %v", e)
	}
	e, _ = errs.As(domain.LimitReached())
	if e == nil || e.Code != errs.CodeValidationFailed || e.Fields[0].Pointer != "" || e.Fields[0].Code != domain.FieldLimitReached ||
		!strings.Contains(e.Detail, "template.max_per_scope") {
		t.Fatalf("limit: %v", e)
	}
}
