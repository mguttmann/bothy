// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

func TestCheckEmail(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"alex@example.com", "robin.s+chat@mail.example.com"} {
		if err := domain.CheckEmail(s); err != nil {
			t.Errorf("CheckEmail(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "alex", "Alex <alex@example.com>", "alex@example.com\u202e", "a\x00@example.com", " alex@example.com"} {
		wantFieldError(t, domain.CheckEmail(s), "/email", "invalid", s)
	}
}

func TestCheckTimeZone(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"UTC", "Europe/Berlin", "America/New_York"} {
		if err := domain.CheckTimeZone(s); err != nil {
			t.Errorf("CheckTimeZone(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "Local", "Mars/Olympus", "../../etc/passwd", "Europe//Berlin", "./UTC", "posixrules", "right/Europe/Berlin", "europe/berlin", "Europe/BERLIN", "Utc", "ETC/UTC", "Factory", "Europe/Berlin\u202e"} {
		wantFieldError(t, domain.CheckTimeZone(s), "/time_zone", "invalid", s)
	}
}

func TestInvalidLocale(t *testing.T) {
	t.Parallel()
	wantFieldError(t, domain.InvalidLocale(), "/locale", "invalid", "")
}

func TestCheckReason(t *testing.T) {
	t.Parallel()
	if err := domain.CheckReason("Second owner as deputy"); err != nil {
		t.Fatal(err)
	}
	wantFieldError(t, domain.CheckReason(" "), "/reason", "required", "")
	wantFieldError(t, domain.CheckReason("ok\u202e"), "/reason", "bidi_control", "")
}

func TestEnableTransition(t *testing.T) {
	t.Parallel()
	for _, s := range []domain.Status{domain.StatusPending, domain.StatusDisabled} {
		if change, err := domain.EnableTransition(s); err != nil || !change {
			t.Errorf("%s: change %v, err %v", s, change, err)
		}
	}
	if change, err := domain.EnableTransition(domain.StatusActive); err != nil || change {
		t.Errorf("active: change %v, err %v; want a call without effect", change, err)
	}
	_, err := domain.EnableTransition(domain.StatusDeleted)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvalidStateTransition {
		t.Errorf("deleted: %v, want invalid_state_transition", err)
	}
}

func TestCheckLocaleLength(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"zh-Hant-TW", strings.Repeat("a", domain.LocaleMaxChars)} {
		if err := domain.CheckLocaleLength(s); err != nil {
			t.Fatalf("%q: %v", s, err)
		}
	}
	if domain.LocaleMaxChars != 35 {
		t.Fatalf("LocaleMaxChars = %d, want 35 (RFC 5646 section 4.4.1)", domain.LocaleMaxChars)
	}
	wantFieldError(t, domain.CheckLocaleLength(strings.Repeat("a", domain.LocaleMaxChars+1)), "/locale", "too_long", "")
}

func TestCheckSearchPrefix(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "an", "x%_"} {
		if err := domain.CheckSearchPrefix(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"a\x00b", "\xff", "a\u202e"} {
		wantFieldError(t, domain.CheckSearchPrefix(s), "#query/q", "invalid", "")
	}
}
