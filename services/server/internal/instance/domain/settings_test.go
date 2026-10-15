// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// locales are the supported locales of 01 section 9 (instance.SupportedLocales in production).
var locales = []string{"de", "en"}

// registry returns the registry with the default product name limit.
func registry(t *testing.T) *domain.Registry {
	t.Helper()
	r, err := domain.NewRegistry(64, locales)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// fieldOf returns the only field error of err, failing when err is no validation_failed with
// exactly one field.
func fieldOf(t *testing.T, err error) errs.FieldError {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 {
		t.Fatalf("error %v, want validation_failed with one field", err)
	}
	return e.Fields[0]
}

// TestSettingsRegistryMatchesConfigurationDoc parses the table of
// docs/operations/configuration.md section 6 and requires the registry to hold exactly its keys
// with its defaults ("empty" means no default).
func TestSettingsRegistryMatchesConfigurationDoc(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../../../../../docs/operations/configuration.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(b), "\n## 6. Instance settings\n")
	if !ok {
		t.Fatal("configuration.md has no section 6")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	want := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 5 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		key := strings.Trim(strings.TrimSpace(cells[1]), "`")
		def := strings.TrimSpace(cells[3])
		if def == "empty" {
			def = ""
		}
		want[key] = strings.Trim(def, "`")
	}
	if len(want) == 0 {
		t.Fatal("no rows parsed")
	}
	r := registry(t)
	got := map[string]string{}
	for _, key := range r.Keys() {
		d, _ := r.Lookup(key)
		def := ""
		if d.Default != nil {
			if err := json.Unmarshal(d.Default, &def); err != nil {
				t.Fatalf("%s: default %s is no JSON string", key, d.Default)
			}
		}
		got[key] = def
	}
	if len(got) != len(want) {
		t.Errorf("registry has %d keys, configuration.md %d", len(got), len(want))
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("%s: registry default %q (present %v), configuration.md %q", k, g, ok, v)
		}
	}
	if !slices.IsSorted(r.Keys()) {
		t.Error("keys are not sorted")
	}
}

func TestRegistry_Routes(t *testing.T) {
	t.Parallel()
	r := registry(t)
	for key, route := range map[string]domain.Route{
		domain.KeyProductName: domain.RouteBranding, domain.KeyAccentColor: domain.RouteBranding,
		domain.KeyLogoFileID: domain.RouteBranding, domain.KeyLocaleDefault: domain.RouteSettings,
	} {
		d, ok := r.Lookup(key)
		if !ok || d.Route != route {
			t.Errorf("%s: %+v, %v; want route %s", key, d, ok, route)
		}
	}
	if _, ok := r.Lookup("branding.unknown"); ok {
		t.Error("an unknown key was found")
	}
	if domain.DefaultProductName != "Bothy" {
		t.Errorf("DefaultProductName = %q", domain.DefaultProductName)
	}
}

func TestNewRegistry_Invalid(t *testing.T) {
	t.Parallel()
	if _, err := domain.NewRegistry(0, locales); err == nil {
		t.Error("limit 0 accepted")
	}
	if _, err := domain.NewRegistry(64, nil); err == nil {
		t.Error("no locales accepted")
	}
	if _, err := domain.NewRegistry(64, []string{"en"}); err == nil {
		t.Error("locales without the default de accepted")
	}
}

func TestValidateProductName(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in, want, code string
	}{
		{"", "", "required"},
		{"   ", "", "required"},
		{"　\t", "", "required"},
		{"Team\u202eChat", "", "invalid_text"},
		{"Team\u0007Chat", "", "invalid_text"},
		{"\xff", "", "invalid_text"},
		{strings.Repeat("x", 65), "", "too_long"},
		{"  Team Chat ", "Team Chat", ""},
		{strings.Repeat("ä", 64), strings.Repeat("ä", 64), ""},
	} {
		got, err := domain.ValidateProductName(c.in, 64)
		if c.code == "" {
			if err != nil || got != c.want {
				t.Errorf("%q: %q, %v; want %q", c.in, got, err, c.want)
			}
			continue
		}
		f := fieldOf(t, err)
		if f.Pointer != "/product_name" || f.Code != c.code {
			t.Errorf("%q: field %+v, want /product_name %s", c.in, f, c.code)
		}
		if strings.Contains(f.Detail, c.in) && strings.TrimSpace(c.in) != "" {
			t.Errorf("%q: detail repeats the value", c.in)
		}
	}
}

func TestValidateAccentColor(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"spruce", "loch", "heather", "rowan", "amber", "gorse", "moss"} {
		if err := domain.ValidateAccentColor(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"#5865f2", "rgb(0,0,0)", "Spruce", "granite", "\U0001F535", ""} {
		f := fieldOf(t, domain.ValidateAccentColor(bad))
		if f.Pointer != "/accent_color" || f.Code != "unknown_color_key" {
			t.Errorf("%q: field %+v", bad, f)
		}
	}
}

func TestValidateLocale(t *testing.T) {
	t.Parallel()
	r := registry(t)
	for _, ok := range []string{`"de"`, `"en"`} {
		v, err := r.Validate(domain.KeyLocaleDefault, json.RawMessage(ok))
		if err != nil || string(v) != ok {
			t.Errorf("%s: %s, %v", ok, v, err)
		}
	}
	for raw, code := range map[string]string{
		`"fr"`: "invalid_enum", `"DE"`: "invalid_enum", `"de-DE"`: "invalid_enum", `""`: "invalid_enum",
		`1`: "invalid_type", `true`: "invalid_type", `null`: "invalid_type", `["de"]`: "invalid_type", `{"a":"de"}`: "invalid_type",
	} {
		_, err := r.Validate(domain.KeyLocaleDefault, json.RawMessage(raw))
		f := fieldOf(t, err)
		if f.Pointer != "/value" || f.Code != code {
			t.Errorf("%s: field %+v, want /value %s", raw, f, code)
		}
	}
}

func TestRegistry_ValidateBrandingKeysPointAtValue(t *testing.T) {
	t.Parallel()
	r := registry(t)
	v, err := r.Validate(domain.KeyProductName, json.RawMessage(`" Team Chat "`))
	if err != nil || string(v) != `"Team Chat"` {
		t.Errorf("product name: %s, %v", v, err)
	}
	if f := fieldOf(t, func() error { _, err := r.Validate(domain.KeyAccentColor, json.RawMessage(`"#fff"`)); return err }()); f.Pointer != "/value" || f.Code != "unknown_color_key" {
		t.Errorf("accent: %+v", f)
	}
	id := `"0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e"`
	if v, err := r.Validate(domain.KeyLogoFileID, json.RawMessage(id)); err != nil || string(v) != id {
		t.Errorf("logo: %s, %v", v, err)
	}
	if f := fieldOf(t, func() error {
		_, err := r.Validate(domain.KeyLogoFileID, json.RawMessage(`"0192f4a0-7b1c-4db7-89b9-2fe4c48dcb3e"`))
		return err
	}()); f.Pointer != "/value" || f.Code != "invalid_format" {
		t.Errorf("logo v4: %+v", f)
	}
	if _, err := r.Validate("unknown.key", json.RawMessage(`"x"`)); err == nil {
		t.Error("unknown key validated")
	}
}

func TestRegistry_LimitAndBrandingEndpointError(t *testing.T) {
	t.Parallel()
	reg, err := domain.NewRegistry(12, []string{"de", "en"})
	if err != nil {
		t.Fatal(err)
	}
	if reg.ProductNameMax() != 12 {
		t.Errorf("ProductNameMax = %d", reg.ProductNameMax())
	}
	e, ok := errs.As(domain.UseBrandingEndpoint())
	if !ok || e.PublicCode() != errs.CodeValidationFailed || len(e.Fields) != 1 ||
		e.Fields[0].Pointer != "#path/key" || e.Fields[0].Code != domain.CodeUseBrandingEndpoint {
		t.Errorf("error %+v", e)
	}
}
