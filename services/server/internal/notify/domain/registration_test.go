// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// policy is the provider policy of the tests: both providers enabled, one topic, a small limit.
var policy = domain.RegistrationPolicy{
	Enabled:       map[string]bool{"apns": true, "fcm": true},
	APNsTopics:    []string{"com.example.chat"},
	FieldMaxBytes: 64,
}

// field is one expected field error.
type field struct{ ptr, code string }

// TestValidateRegistration_Table holds the rules of brief M3-22 design note 4, one row per rule.
func TestValidateRegistration_Table(t *testing.T) {
	t.Parallel()
	hex := strings.Repeat("ab", 16)
	long := strings.Repeat("a", policy.FieldMaxBytes+1)
	for _, tc := range []struct {
		name   string
		in     domain.RegistrationInput
		code   errs.Code
		reason string
		fields []field
	}{
		{name: "wns", in: domain.RegistrationInput{Provider: "wns", Token: "x", AppID: "a", Platform: "windows"},
			code: errs.CodeValidationFailed, fields: []field{{"/provider", "not_enabled"}}},
		{name: "unifiedpush", in: domain.RegistrationInput{Provider: "unifiedpush", Token: "x", AppID: "a", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/provider", "not_enabled"}}},
		{name: "apns from android", in: domain.RegistrationInput{Provider: "apns", Token: hex, AppID: "com.example.chat", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/provider", "invalid"}}},
		{name: "fcm from ios", in: domain.RegistrationInput{Provider: "fcm", Token: "EXAMPLEtoken", AppID: "com.example.chat", Platform: "ios"},
			code: errs.CodeValidationFailed, fields: []field{{"/provider", "invalid"}}},
		{name: "fcm sandbox", in: domain.RegistrationInput{Provider: "fcm", Token: "EXAMPLEtoken", Environment: "sandbox", AppID: "com.example.chat", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/environment", "invalid"}}},
		{name: "apns token with g", in: domain.RegistrationInput{Provider: "apns", Token: "abg0", AppID: "com.example.chat", Platform: "ios"},
			code: errs.CodeValidationFailed, fields: []field{{"/token", "pattern_mismatch"}}},
		{name: "apns token odd", in: domain.RegistrationInput{Provider: "apns", Token: "abc", AppID: "com.example.chat", Platform: "ios"},
			code: errs.CodeValidationFailed, fields: []field{{"/token", "pattern_mismatch"}}},
		{name: "fcm token with space", in: domain.RegistrationInput{Provider: "fcm", Token: "EXAMPLE token", AppID: "com.example.chat", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/token", "pattern_mismatch"}}},
		{name: "token too long", in: domain.RegistrationInput{Provider: "fcm", Token: long, AppID: "com.example.chat", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/token", "too_long"}}},
		{name: "app_id too long", in: domain.RegistrationInput{Provider: "fcm", Token: "EXAMPLEtoken", AppID: long, Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/app_id", "too_long"}}},
		{name: "app_id outside topics", in: domain.RegistrationInput{Provider: "apns", Token: hex, AppID: "com.example.other", Platform: "ios"},
			code: errs.CodeValidationFailed, fields: []field{{"/app_id", "invalid"}}},
		{name: "several faults", in: domain.RegistrationInput{Provider: "apns", Token: "xyz", Environment: "sandbox", AppID: "com.example.other", Platform: "android"},
			code: errs.CodeValidationFailed, fields: []field{{"/provider", "invalid"}, {"/token", "pattern_mismatch"}, {"/app_id", "invalid"}}},
		{name: "apns disabled", in: domain.RegistrationInput{Provider: "apns", Token: hex, AppID: "com.example.chat", Platform: "ios"},
			code: errs.CodeFeatureDisabled, reason: "push_provider_disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pol := policy
			if tc.reason != "" {
				pol.Enabled = map[string]bool{"fcm": true}
			}
			_, err := domain.ValidateRegistration(tc.in, pol)
			e, ok := errs.As(err)
			if !ok || e.Code != tc.code || e.Reason != tc.reason {
				t.Fatalf("err = %v, want code %s reason %q", err, tc.code, tc.reason)
			}
			var got []field
			for _, f := range e.Fields {
				got = append(got, field{f.Pointer, f.Code})
			}
			if !slices.Equal(got, tc.fields) {
				t.Fatalf("fields = %v, want %v", got, tc.fields)
			}
		})
	}
}

// TestValidateRegistration_Normalizes: an apns token in upper case is accepted and stored in lower
// case; an absent environment is production; sandbox stays for apns.
func TestValidateRegistration_Normalizes(t *testing.T) {
	t.Parallel()
	out, err := domain.ValidateRegistration(domain.RegistrationInput{Provider: "apns", Token: "ABCDEF01", AppID: "com.example.chat", Platform: "macos"}, policy)
	if err != nil || out.Token != "abcdef01" || out.Environment != "production" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
	out, err = domain.ValidateRegistration(domain.RegistrationInput{Provider: "apns", Token: "ab", Environment: "sandbox", AppID: "com.example.chat", Platform: "ios"}, policy)
	if err != nil || out.Environment != "sandbox" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
	out, err = domain.ValidateRegistration(domain.RegistrationInput{Provider: "fcm", Token: "EXAMPLE:Token-1", AppID: "com.example.chat", Platform: "android"}, policy)
	if err != nil || out.Token != "EXAMPLE:Token-1" {
		t.Fatalf("out = %+v, err = %v", out, err)
	}
}

// TestDisableReasonForSession holds the mapping of brief M3-22 design note 7 a.
func TestDisableReasonForSession(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"device_revoked": "device_revoked", "user_disabled": "device_revoked",
		"logout": "logout", "admin": "logout", "refresh_reuse": "logout", "inactive": "logout", "something_new": "logout",
	} {
		if got := domain.DisableReasonForSession(in); got != want {
			t.Errorf("DisableReasonForSession(%q) = %q, want %q", in, got, want)
		}
	}
}
