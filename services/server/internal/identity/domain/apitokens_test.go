// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
)

func TestCheckTokenName(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"Adapter production", "a", "Lotse \u00e9"} {
		if err := domain.CheckTokenName(s); err != nil {
			t.Errorf("CheckTokenName(%q) = %v", s, err)
		}
	}
	wantFieldError(t, domain.CheckTokenName(""), "/name", "required", "")
	wantFieldError(t, domain.CheckTokenName("  "), "/name", "required", "")
	for _, s := range []string{"a\x00b", "a\u202eb", "a\u200bb\u200b\u2066"} {
		if err := domain.CheckTokenName(s); err == nil {
			t.Errorf("CheckTokenName(%q) accepted a control, format or bidi character", s)
		}
	}
}

func TestCheckTokenScopes(t *testing.T) {
	t.Parallel()
	if err := domain.CheckTokenScopes(false, nil); err != nil {
		t.Fatal(err)
	}
	if err := domain.CheckTokenScopes(true, []string{"messages:read"}); err != nil {
		t.Fatal(err)
	}
	wantFieldError(t, domain.CheckTokenScopes(false, []string{"messages:read"}), "/scopes", "not_allowed", "")
}

func TestCheckTokenExpiry(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	max := 90 * 24 * time.Hour
	for _, e := range []time.Time{now.Add(time.Second), now.Add(max)} {
		if err := domain.CheckTokenExpiry(e, now, max); err != nil {
			t.Errorf("CheckTokenExpiry(%s) = %v", e, err)
		}
	}
	wantFieldError(t, domain.CheckTokenExpiry(time.Time{}, now, max), "/expires_at", "required", "")
	wantFieldError(t, domain.CheckTokenExpiry(now, now, max), "/expires_at", "out_of_range", "")
	wantFieldError(t, domain.CheckTokenExpiry(now.Add(-time.Hour), now, max), "/expires_at", "out_of_range", "")
	wantFieldError(t, domain.CheckTokenExpiry(now.Add(max+time.Millisecond), now, max), "/expires_at", "out_of_range", "")
}

func TestCheckTokenJKT(t *testing.T) {
	t.Parallel()
	good := "0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I"
	for _, c := range []struct {
		jkt      string
		required bool
	}{{good, true}, {good, false}, {"", false}} {
		if err := domain.CheckTokenJKT(c.jkt, c.required); err != nil {
			t.Errorf("CheckTokenJKT(%q, %v) = %v", c.jkt, c.required, err)
		}
	}
	wantFieldError(t, domain.CheckTokenJKT("", true), "/dpop_jkt", "required", "")
	for _, s := range []string{good[:42], good + "A", strings.Replace(good, "-", "+", 1), strings.Replace(good, "0", "=", 1)} {
		wantFieldError(t, domain.CheckTokenJKT(s, false), "/dpop_jkt", "pattern", s)
	}
}
