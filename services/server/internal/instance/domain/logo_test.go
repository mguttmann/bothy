// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestLogoSettingEmptyMeansAbsent: the empty string is the stored form of "no logo" (brief M3-10
// design note 9); it validates, gives no logo file id for Branding and no logo_url for discovery.
func TestLogoSettingEmptyMeansAbsent(t *testing.T) {
	t.Parallel()
	r := registry(t)
	if v, err := r.Validate(domain.KeyLogoFileID, json.RawMessage(`""`)); err != nil || string(v) != `""` {
		t.Errorf("empty value: %s, %v", v, err)
	}
	id, err := domain.LogoFileID("")
	if err != nil || !id.IsZero() {
		t.Errorf("LogoFileID(\"\") = %v, %v; want zero id", id, err)
	}
	if u := domain.DiscoveryLogoURL(id); u != "" {
		t.Errorf("discovery logo_url without logo = %q", u)
	}
	set := ids.New()
	if got, err := domain.LogoFileID(set.String()); err != nil || got != set {
		t.Errorf("LogoFileID(%s) = %v, %v", set, got, err)
	}
	if u := domain.DiscoveryLogoURL(set); u != domain.LogoURL {
		t.Errorf("discovery logo_url with logo = %q", u)
	}
}

// TestLogoSettingRejectsOtherValues: anything but a UUIDv7 or the empty string is refused, both by
// the registry and when a stored value is read back.
func TestLogoSettingRejectsOtherValues(t *testing.T) {
	t.Parallel()
	r := registry(t)
	for _, raw := range []string{`" "`, `"no-uuid"`, `"0192f4a0-7b1c-4db7-89b9-2fe4c48dcb3e"`, `"00000000-0000-0000-0000-000000000000"`} {
		f := fieldOf(t, func() error { _, err := r.Validate(domain.KeyLogoFileID, json.RawMessage(raw)); return err }())
		if f.Pointer != "/value" || f.Code != domain.CodeInvalidFormat {
			t.Errorf("%s: field %+v", raw, f)
		}
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatal(err)
		}
		if _, err := domain.LogoFileID(s); err == nil {
			t.Errorf("LogoFileID(%q) accepted", s)
		}
	}
	for _, raw := range []string{`null`, `5`, `{}`} {
		f := fieldOf(t, func() error { _, err := r.Validate(domain.KeyLogoFileID, json.RawMessage(raw)); return err }())
		if f.Code != domain.CodeInvalidType {
			t.Errorf("%s: field %+v", raw, f)
		}
	}
}
