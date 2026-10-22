// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// repoRoot returns the first directory above the working directory that holds docs/spec.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no docs/spec above the working directory")
		}
		dir = parent
	}
}

// TestReservedNamesMatchChapter09 reads the first column of the table in 09 section 10.1 and
// compares it with the reserved names, so a new built-in fails until the list follows.
func TestReservedNamesMatchChapter09(t *testing.T) {
	f, err := os.Open(filepath.Join(repoRoot(t), "docs", "spec", "09-agents.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var names []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "### 10.1"):
			in = true
		case strings.HasPrefix(line, "### 10.2"):
			in = false
		case in && strings.HasPrefix(line, "| `/"):
			cell := strings.TrimSpace(strings.Split(line, "|")[1])
			names = append(names, strings.Trim(cell, "`/"))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 || !slices.Equal(names, domain.ReservedCommandNames()) {
		t.Fatalf("chapter 09 10.1 names %v, constant %v", names, domain.ReservedCommandNames())
	}
	for _, n := range names {
		if !domain.IsReservedCommandName(n) {
			t.Fatalf("%s not reserved", n)
		}
	}
	if domain.IsReservedCommandName("escalate") || domain.IsReservedCommandName("Stop") {
		t.Fatal("unreserved name refused")
	}
}

// limits are the limits of the test definitions.
var limits = domain.CommandLimits{DescriptionMaxChars: 20, MaxChoices: 3}

// base returns a valid definition input.
func base() domain.CommandInput {
	return domain.CommandInput{Name: "escalate", Description: "Escalate a ticket"}
}

// opt returns a valid string option named name.
func opt(name string) domain.CommandOption {
	return domain.CommandOption{Name: name, Description: "An option", Type: "string"}
}

// TestValidateDefinition covers every rule of design note 3 with its pointer and code.
func TestValidateDefinition(t *testing.T) {
	secret := "SECRETVALUE"
	choice := func(name, value string) domain.CommandChoice { return domain.CommandChoice{Name: name, Value: value} }
	cases := []struct {
		name    string
		mutate  func(*domain.CommandInput)
		pointer string
		code    string
	}{
		{"name pattern", func(c *domain.CommandInput) { c.Name = "Bad" + secret }, "/name", "pattern_mismatch"},
		{"description blank", func(c *domain.CommandInput) { c.Description = "   " }, "/description", "too_short"},
		{"description bidi", func(c *domain.CommandInput) { c.Description = secret + "\u202e" }, "/description", "invalid"},
		{"description control", func(c *domain.CommandInput) { c.Description = secret + "\n" }, "/description", "invalid"},
		{"description too long", func(c *domain.CommandInput) { c.Description = strings.Repeat("ä", 21) }, "/description", "too_long"},
		{"scope host", func(c *domain.CommandInput) { c.ScopeKind = "host" }, "/scope_kind", "invalid_enum"},
		{"scope id on instance", func(c *domain.CommandInput) { c.ScopeKind, c.ScopeID = "instance", ids.New() }, "/scope_id", "invalid"},
		{"scope id missing", func(c *domain.CommandInput) { c.ScopeKind = "channel" }, "/scope_id", "required"},
		{"required level", func(c *domain.CommandInput) { c.RequiredLevel = "read" }, "/required_level", "invalid_enum"},
		{"option name", func(c *domain.CommandInput) { c.Options = []domain.CommandOption{opt("Bad" + secret)} }, "/options/0/name", "pattern_mismatch"},
		{"option twice", func(c *domain.CommandInput) { c.Options = []domain.CommandOption{opt("a"), opt("a")} }, "/options/1/name", "invalid"},
		{"option description", func(c *domain.CommandInput) {
			o := opt("a")
			o.Description = ""
			c.Options = []domain.CommandOption{o}
		}, "/options/0/description", "too_short"},
		{"option type", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type = "date"
			c.Options = []domain.CommandOption{o}
		}, "/options/0/type", "invalid_enum"},
		{"choices on boolean", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type, o.Choices = "boolean", []domain.CommandChoice{choice("Yes", "true")}
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices", "invalid"},
		{"choice without choices", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type = "choice"
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices", "required"},
		{"autocomplete on user", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type, o.Autocomplete = "user", true
			c.Options = []domain.CommandOption{o}
		}, "/options/0/autocomplete", "invalid"},
		{"too many choices", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type = "choice"
			o.Choices = []domain.CommandChoice{choice("A", "a"), choice("B", "b"), choice("C", "c"), choice("D", "d")}
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices", "too_many_items"},
		{"choice name", func(c *domain.CommandInput) {
			o := opt("a")
			o.Choices = []domain.CommandChoice{choice(secret+"\u2066", "x")}
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices/0/name", "invalid"},
		{"integer choice value", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type, o.Choices = "integer", []domain.CommandChoice{choice("One", "01")}
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices/0/value", "invalid_format"},
		{"choice value twice", func(c *domain.CommandInput) {
			o := opt("a")
			o.Type, o.Choices = "choice", []domain.CommandChoice{choice("A", "a"), choice("B", "a")}
			c.Options = []domain.CommandOption{o}
		}, "/options/0/choices/1/value", "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mutate(&in)
			_, err := domain.ValidateCommand(in, limits)
			e, ok := errs.As(err)
			if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != tc.pointer ||
				e.Fields[0].Code != tc.code {
				t.Fatalf("got %v %+v, want %s %s", err, e, tc.pointer, tc.code)
			}
			if strings.Contains(e.Detail+e.Fields[0].Detail, secret) {
				t.Fatalf("detail repeats the value: %q", e.Fields[0].Detail)
			}
		})
	}
}

// TestValidateDefinitionDefaults: absent members take the defaults of 03 (design note 3 c).
func TestValidateDefinitionDefaults(t *testing.T) {
	d, err := domain.ValidateCommand(base(), limits)
	if err != nil {
		t.Fatal(err)
	}
	if d.ScopeKind != "instance" || d.RequiredLevel != "trigger" || d.Control || d.Options == nil || len(d.Options) != 0 {
		t.Fatalf("defaults %+v", d)
	}
	in := base()
	sp := ids.New()
	in.ScopeKind, in.ScopeID, in.RequiredLevel, in.Control = "space", sp, "write", true
	o := opt("prio")
	o.Type, o.Choices, o.Autocomplete = "choice", []domain.CommandChoice{{Name: "High", Value: "high"}}, true
	in.Options = []domain.CommandOption{opt("ticket"), o}
	d, err = domain.ValidateCommand(in, limits)
	if err != nil {
		t.Fatal(err)
	}
	if d.ScopeSpaceID != sp || !d.ScopeChannelID.IsZero() || d.RequiredLevel != "write" || !d.Control || len(d.Options) != 2 {
		t.Fatalf("definition %+v", d)
	}
	// A choice option with autocomplete needs no static choices.
	o.Choices = nil
	in.Options = []domain.CommandOption{o}
	if _, err := domain.ValidateCommand(in, limits); err != nil {
		t.Fatal(err)
	}
}

// TestScopeCoversAndSpecificity: coverage per scope kind; channel before space before instance
// per owner and name (design note 9 c).
func TestScopeCoversAndSpecificity(t *testing.T) {
	space, chA, chB := ids.New(), ids.New(), ids.New()
	inst := domain.CommandDefinition{ScopeKind: "instance"}
	sp := domain.CommandDefinition{ScopeKind: "space", ScopeSpaceID: space}
	ch := domain.CommandDefinition{ScopeKind: "channel", ScopeChannelID: chA}
	at := domain.ChannelPlace{ChannelID: chA, SpaceID: space}
	for _, d := range []domain.CommandDefinition{inst, sp, ch} {
		if !d.Covers(at) {
			t.Fatalf("%s does not cover", d.ScopeKind)
		}
		if d.Covers(domain.ChannelPlace{ChannelID: chA, SpaceID: space, DM: true}) {
			t.Fatalf("%s covers a dm", d.ScopeKind)
		}
	}
	if ch.Covers(domain.ChannelPlace{ChannelID: chB, SpaceID: space}) || sp.Covers(domain.ChannelPlace{ChannelID: chB, SpaceID: ids.New()}) {
		t.Fatal("covers a foreign channel")
	}
	owner, other := ids.New(), ids.New()
	mk := func(o ids.ID, name string, d domain.CommandDefinition) domain.Command {
		d.Name = name
		return domain.Command{ID: ids.New(), OwnerID: o, Def: d}
	}
	list := []domain.Command{
		mk(owner, "deploy", inst), mk(owner, "deploy", ch), mk(owner, "deploy", sp), mk(other, "deploy", inst), mk(owner, "status2", sp),
	}
	kept := domain.MostSpecific(list)
	if len(kept) != 3 {
		t.Fatalf("kept %d", len(kept))
	}
	for _, c := range kept {
		if c.OwnerID == owner && c.Def.Name == "deploy" && c.Def.ScopeKind != "channel" {
			t.Fatalf("kept %s", c.Def.ScopeKind)
		}
	}
}
