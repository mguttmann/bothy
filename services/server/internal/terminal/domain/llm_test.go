// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestValidateDeclaration (Design note 10 of M2-40): personal needs a holder, none and
// organization take none, an unknown value is invalid.
func TestValidateDeclaration(t *testing.T) {
	t.Parallel()
	holder := ids.New()
	cases := map[string]struct {
		d             domain.Declaration
		code, pointer string
	}{
		"personal with holder":    {domain.Declaration{Credentials: "personal", HolderID: holder}, "", ""},
		"organization":            {domain.Declaration{Credentials: "organization"}, "", ""},
		"none":                    {domain.Declaration{Credentials: "none"}, "", ""},
		"personal without holder": {domain.Declaration{Credentials: "personal"}, "required", "/credential_holder_id"},
		"organization with holder": {domain.Declaration{Credentials: "organization", HolderID: holder}, "not_allowed",
			"/credential_holder_id"},
		"none with holder": {domain.Declaration{Credentials: "none", HolderID: holder}, "not_allowed", "/credential_holder_id"},
		"unknown value":    {domain.Declaration{Credentials: "shared"}, "invalid", "/llm_credentials"},
	}
	for name, c := range cases {
		err := domain.ValidateDeclaration(c.d)
		if c.code == "" {
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Code != c.code || e.Fields[0].Pointer != c.pointer {
			t.Errorf("%s: %+v, want %s at %s", name, err, c.code, c.pointer)
		}
	}
}

// TestEndsForeignSessions (24 section 7.3 point 3): to personal and a holder change end the
// sessions of others; back to organization and an unchanged declaration do not.
func TestEndsForeignSessions(t *testing.T) {
	t.Parallel()
	alex, robin := ids.New(), ids.New()
	org := domain.Declaration{Credentials: "organization"}
	pa := domain.Declaration{Credentials: "personal", HolderID: alex}
	pr := domain.Declaration{Credentials: "personal", HolderID: robin}
	cases := map[string]struct {
		before, after domain.Declaration
		want          bool
	}{
		"to personal":            {org, pa, true},
		"none to personal":       {domain.Declaration{Credentials: "none"}, pa, true},
		"holder change":          {pa, pr, true},
		"back to organization":   {pa, org, false},
		"unchanged personal":     {pa, pa, false},
		"unchanged organization": {org, org, false},
	}
	for name, c := range cases {
		if got := domain.EndsForeignSessions(c.before, c.after); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

// rsu is a reported session user.
func rsu(name string, dirs ...string) domain.ReportedSessionUser {
	return domain.ReportedSessionUser{OSUser: name, Workdirs: dirs}
}

// TestSameReport (Design note 8 of M2-40): reordered names are equal, reordered working
// directories are not, and a missing list equals the empty one.
func TestSameReport(t *testing.T) {
	t.Parallel()
	a := []domain.ReportedSessionUser{rsu("bothy-s-alex", "app", "docs"), rsu("bothy-s-sam")}
	cases := map[string]struct {
		b    []domain.ReportedSessionUser
		want bool
	}{
		"same":                 {[]domain.ReportedSessionUser{rsu("bothy-s-alex", "app", "docs"), rsu("bothy-s-sam")}, true},
		"names reordered":      {[]domain.ReportedSessionUser{rsu("bothy-s-sam"), rsu("bothy-s-alex", "app", "docs")}, true},
		"workdirs reordered":   {[]domain.ReportedSessionUser{rsu("bothy-s-alex", "docs", "app"), rsu("bothy-s-sam")}, false},
		"name removed":         {[]domain.ReportedSessionUser{rsu("bothy-s-alex", "app", "docs")}, false},
		"name renamed by case": {[]domain.ReportedSessionUser{rsu("bothy-s-ALEX", "app", "docs"), rsu("bothy-s-sam")}, false},
		"nil workdirs":         {[]domain.ReportedSessionUser{rsu("bothy-s-alex", "app", "docs"), {OSUser: "bothy-s-sam"}}, true},
	}
	for name, c := range cases {
		if got := domain.SameReport(a, c.b); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	if !domain.SameReport(nil, []domain.ReportedSessionUser{}) {
		t.Error("nil and empty differ")
	}
}

// TestVanishedNames: removed and renamed names vanish, sorted; new names and reorderings do not.
func TestVanishedNames(t *testing.T) {
	t.Parallel()
	before := []domain.ReportedSessionUser{rsu("bothy-s-sam"), rsu("bothy-s-alex"), rsu("bothy-s-robin")}
	after := []domain.ReportedSessionUser{rsu("bothy-s-robin"), rsu("BOTHY-S-ALEX"), rsu("bothy-s-new")}
	if got := domain.VanishedNames(before, after); !slices.Equal(got, []string{"bothy-s-alex", "bothy-s-sam"}) {
		t.Fatalf("VanishedNames = %v", got)
	}
	if got := domain.VanishedNames(before, before); len(got) != 0 {
		t.Fatalf("unchanged report: %v", got)
	}
	if !domain.Reports(before, "bothy-s-alex") || domain.Reports(after, "bothy-s-alex") {
		t.Fatal("Reports compares exactly")
	}
}
