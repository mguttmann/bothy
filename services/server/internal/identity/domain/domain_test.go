// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantFieldError checks that err is a validation_failed error with exactly one field error on
// pointer whose code is code, and that its texts do not echo the rejected input.
func wantFieldError(t *testing.T, err error, pointer, code, input string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("error %v is no *errs.Error", err)
	}
	if e.Code != errs.CodeValidationFailed || e.Kind() != errs.KindBadRequest {
		t.Fatalf("code %q kind %v, want validation_failed bad_request", e.Code, e.Kind())
	}
	if len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("fields %+v, want one on %s with code %s", e.Fields, pointer, code)
	}
	if strings.TrimSpace(input) != "" {
		for _, s := range []string{e.Error(), e.Fields[0].Detail} {
			if strings.Contains(s, input) {
				t.Errorf("error text %q echoes the input", s)
			}
		}
	}
}

func TestNewHandle(t *testing.T) {
	t.Parallel()
	for _, maxChars := range []int{32, 5} {
		valid := []string{"alex", "a", "robin.s-1", strings.Repeat("a", maxChars)}
		invalid := map[string]string{
			"":                              "required",
			"-alex":                         "pattern",
			".alex":                         "pattern",
			"Alex":                          "pattern",
			"al ex":                         "pattern",
			"alëx":                          "pattern",
			"_alex":                         "pattern",
			"alex\n":                        "pattern",
			strings.Repeat("a", maxChars+1): "too_long",
		}
		for _, s := range valid {
			if len(s) > maxChars {
				continue
			}
			h, err := domain.NewHandle(s, maxChars)
			if err != nil {
				t.Errorf("max %d: NewHandle(%q) = %v", maxChars, s, err)
				continue
			}
			if h.String() != s {
				t.Errorf("max %d: Handle %q, want %q", maxChars, h.String(), s)
			}
		}
		for s, code := range invalid {
			if code == "pattern" && len(s) > maxChars {
				code = "too_long"
			}
			_, err := domain.NewHandle(s, maxChars)
			if err == nil {
				t.Errorf("max %d: NewHandle(%q) passed", maxChars, s)
				continue
			}
			wantFieldError(t, err, "/handle", code, s)
		}
	}
}

func TestSECT07_DisplayNameRejectsControlAndBidi(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"RLO":     "Alex\u202Eevil",
		"LRI":     "\u2066Sam",
		"ALM":     "Ro\u061Cbin",
		"RLM":     "Sam\u200F",
		"NUL":     "Sam\x00",
		"newline": "Robin\nAdmin",
		"tab":     "Alex\tB",
		"C1":      "Alex\u0085",
		"invalid": "Al\xffex",
	} {
		_, err := domain.NewDisplayName(in, 64)
		if err == nil {
			t.Errorf("%s: NewDisplayName passed", name)
			continue
		}
		e, _ := errs.As(err)
		code := ""
		if e != nil && len(e.Fields) == 1 {
			code = e.Fields[0].Code
		}
		if code != "control_character" && code != "bidi_control" && code != "invalid_utf8" {
			t.Errorf("%s: field code %q, want a textcheck rule", name, code)
		}
		wantFieldError(t, err, "/display_name", code, in)
	}
	for _, ok := range []string{"Alex", "Zoë", "Sam 🙂", "Robin 👩\u200D💻", " Alex "} {
		d, err := domain.NewDisplayName(ok, 64)
		if err != nil {
			t.Errorf("NewDisplayName(%q) = %v", ok, err)
			continue
		}
		if d.String() != ok {
			t.Errorf("DisplayName %q, want %q", d.String(), ok)
		}
	}
}

func TestNewDisplayName_RejectsBlank(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", " ", "   ", "\u3000", " \u3000 ", "\u00A0", "\u200B", "\u200D\u2060", " \uFEFF "} {
		_, err := domain.NewDisplayName(in, 64)
		if err == nil {
			t.Errorf("NewDisplayName(%q) passed", in)
			continue
		}
		wantFieldError(t, err, "/display_name", "required", in)
	}
}

func TestNewDisplayName_MaxChars(t *testing.T) {
	t.Parallel()
	exact := strings.Repeat("ë", 5) // 10 bytes, 5 runes
	if _, err := domain.NewDisplayName(exact, 5); err != nil {
		t.Fatalf("5 runes with limit 5: %v", err)
	}
	if _, err := domain.NewDisplayName(strings.Repeat("ë", 6), 5); err == nil {
		t.Fatal("6 runes with limit 5 passed")
	} else {
		wantFieldError(t, err, "/display_name", "too_long", "")
	}
	if _, err := domain.NewDisplayName(strings.Repeat("a", 64), 64); err != nil {
		t.Fatalf("64 runes with limit 64: %v", err)
	}
}

func TestNewOwner_RequiresReason(t *testing.T) {
	t.Parallel()
	user := ids.New()
	for _, reason := range []string{"", "  ", "\u3000"} {
		_, err := domain.NewOwner(ids.New(), user, ids.ID{}, reason)
		if err == nil {
			t.Errorf("NewOwner with reason %q passed", reason)
			continue
		}
		wantFieldError(t, err, "/reason", "required", reason)
	}
	_, err := domain.NewOwner(ids.New(), user, ids.ID{}, "first\u202Eowner")
	if err == nil {
		t.Fatal("NewOwner with a bidi control in the reason passed")
	}
	wantFieldError(t, err, "/reason", "bidi_control", "first\u202Eowner")

	o, err := domain.NewOwner(ids.New(), user, ids.ID{}, "first owner of the instance")
	if err != nil {
		t.Fatal(err)
	}
	if o.UserID != user || !o.GrantedBy.IsZero() || o.Reason != "first owner of the instance" {
		t.Fatalf("owner %+v", o)
	}
}

func TestNewOwner_RequiresIDs(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewOwner(ids.ID{}, ids.New(), ids.ID{}, "r"); err == nil {
		t.Error("zero owner id passed")
	}
	if _, err := domain.NewOwner(ids.New(), ids.ID{}, ids.ID{}, "r"); err == nil {
		t.Error("zero user id passed")
	}
}

func TestNewPrincipal(t *testing.T) {
	t.Parallel()
	h, _ := domain.NewHandle("alex", 32)
	d, _ := domain.NewDisplayName("Alex", 64)
	id := ids.New()
	p, err := domain.NewPrincipal(id, domain.KindUser, h, d)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != id || p.Kind != domain.KindUser || p.Status != domain.StatusActive || p.Handle != h || p.DisplayName != d {
		t.Fatalf("principal %+v", p)
	}
	if _, err := domain.NewPrincipal(id, domain.Kind("bot"), h, d); err == nil {
		t.Error("unknown kind passed")
	}
	if _, err := domain.NewPrincipal(ids.ID{}, domain.KindAgent, h, d); err == nil {
		t.Error("zero id passed")
	}
	if _, err := domain.NewPrincipal(id, domain.KindApp, domain.Handle{}, d); err == nil {
		t.Error("zero handle passed")
	}
	if _, err := domain.NewPrincipal(id, domain.KindApp, h, domain.DisplayName{}); err == nil {
		t.Error("zero display name passed")
	}
}

func TestNewUser(t *testing.T) {
	t.Parallel()
	h, _ := domain.NewHandle("sam", 32)
	d, _ := domain.NewDisplayName("Sam", 64)
	p, _ := domain.NewPrincipal(ids.New(), domain.KindUser, h, d)
	u, err := domain.NewUser(p, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Locale != "de" || u.TimeZone != "UTC" || u.Email != "" {
		t.Fatalf("defaults %+v", u)
	}
	u, err = domain.NewUser(p, "sam@example.com", "en", "Europe/Berlin")
	if err != nil || u.Email != "sam@example.com" || u.Locale != "en" || u.TimeZone != "Europe/Berlin" {
		t.Fatalf("user %+v, %v", u, err)
	}
	agent, _ := domain.NewPrincipal(ids.New(), domain.KindAgent, h, d)
	if _, err := domain.NewUser(agent, "", "", ""); err == nil {
		t.Fatal("user for an agent principal passed")
	}
}

func TestKindAndStatusValues(t *testing.T) {
	t.Parallel()
	for _, k := range []domain.Kind{domain.KindUser, domain.KindAgent, domain.KindApp} {
		if !k.Valid() {
			t.Errorf("%q invalid", k)
		}
	}
	if domain.Kind("anonymous").Valid() {
		t.Error("anonymous is no principal kind")
	}
	for _, s := range []domain.Status{domain.StatusPending, domain.StatusActive, domain.StatusDisabled, domain.StatusDeleted} {
		if !s.Valid() {
			t.Errorf("%q invalid", s)
		}
	}
	if domain.Status("gone").Valid() {
		t.Error("unknown status valid")
	}
}
