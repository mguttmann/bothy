// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestKindCodesStable: the eight kinds in the order of NotificationKind have the codes 1 to 8 and
// the badge sync 9; KindFromCode inverts them; 0 and 10 are errors (D-M3-19-10).
func TestKindCodesStable(t *testing.T) {
	t.Parallel()
	kinds := append(domain.Kinds(), domain.KindSync)
	for i, k := range kinds {
		code, err := domain.KindCode(k)
		if err != nil || code != int64(i+1) {
			t.Fatalf("KindCode(%s) = %d, %v; want %d", k, code, err, i+1)
		}
		back, err := domain.KindFromCode(code)
		if err != nil || back != k {
			t.Fatalf("KindFromCode(%d) = %s, %v; want %s", code, back, err, k)
		}
	}
	for _, c := range []int64{0, 10, -1} {
		if _, err := domain.KindFromCode(c); err == nil {
			t.Fatalf("KindFromCode(%d) accepted", c)
		}
	}
	if _, err := domain.KindCode("unknown"); err == nil {
		t.Fatal("KindCode(unknown) accepted")
	}
	if domain.KindSync.Valid() {
		t.Fatal("the badge sync is no NotificationKind")
	}
}

// TestApplyResult: one row per line of the table of 13 7.3 (brief M3-19 design note 13).
func TestApplyResult(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	created := now.Add(-48 * time.Hour)
	const maxFailures = 5
	base := domain.RegistrationState{CreatedAt: created, FailureCount: 3}
	cases := []struct {
		name    string
		state   domain.RegistrationState
		res     domain.SendResult
		want    domain.RegistrationState
		changed bool
		retry   bool
		outcome string
	}{
		{"accepted resets", base, domain.SendResult{Class: domain.ResultAccepted},
			domain.RegistrationState{CreatedAt: created, LastSuccessAt: now}, true, false, domain.OutcomeAccepted},
		{"disable token_invalid", base, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid},
			domain.RegistrationState{CreatedAt: created, FailureCount: 3, DisabledAt: now, DisabledReason: domain.DisableTokenInvalid}, true, false, domain.OutcomeRejectedToken},
		{"disable topic_mismatch", base, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTopicMismatch},
			domain.RegistrationState{CreatedAt: created, FailureCount: 3, DisabledAt: now, DisabledReason: domain.DisableTopicMismatch}, true, false, domain.OutcomeRejectedToken},
		{"disable sender_mismatch", base, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableSenderMismatch},
			domain.RegistrationState{CreatedAt: created, FailureCount: 3, DisabledAt: now, DisabledReason: domain.DisableSenderMismatch}, true, false, domain.OutcomeRejectedToken},
		{"disable invalid since before creation keeps the row", base,
			domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid, InvalidSince: created.Add(-time.Hour)},
			base, false, false, domain.OutcomeRejectedToken},
		{"disable invalid since after creation", base,
			domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableTokenInvalid, InvalidSince: created.Add(time.Hour)},
			domain.RegistrationState{CreatedAt: created, FailureCount: 3, DisabledAt: now, DisabledReason: domain.DisableTokenInvalid}, true, false, domain.OutcomeRejectedToken},
		{"retry counts", base, domain.SendResult{Class: domain.ResultRetry},
			domain.RegistrationState{CreatedAt: created, FailureCount: 4}, true, true, domain.OutcomeProviderError},
		{"retry at the limit deactivates", domain.RegistrationState{CreatedAt: created, FailureCount: maxFailures - 1}, domain.SendResult{Class: domain.ResultRetry},
			domain.RegistrationState{CreatedAt: created, FailureCount: maxFailures, DisabledAt: now, DisabledReason: domain.DisableRepeatedFailures}, true, false, domain.OutcomeProviderError},
		{"throttled keeps the count", base, domain.SendResult{Class: domain.ResultThrottled}, base, false, true, domain.OutcomeThrottled},
		{"config_error", base, domain.SendResult{Class: domain.ResultConfigError}, base, false, false, domain.OutcomeRejectedOther},
		{"program_error", base, domain.SendResult{Class: domain.ResultProgramError}, base, false, false, domain.OutcomeRejectedOther},
		{"unknown class", base, domain.SendResult{Class: "boom"}, base, false, false, domain.OutcomeRejectedOther},
	}
	for _, c := range cases {
		got := domain.ApplyResult(c.state, c.res, now, maxFailures)
		if got.State != c.want || got.Changed != c.changed || got.Retry != c.retry || got.Outcome != c.outcome {
			t.Errorf("%s: got %+v; want state %+v changed %v retry %v outcome %s", c.name, got, c.want, c.changed, c.retry, c.outcome)
		}
	}
	// A disable with a reason outside the three provider reasons is a program error of the
	// adapter: nothing changes and the hint is dropped.
	bad := domain.ApplyResult(base, domain.SendResult{Class: domain.ResultDisable, DisableReason: domain.DisableLogout}, now, maxFailures)
	if bad.Changed || bad.Retry || bad.Outcome != domain.OutcomeRejectedOther {
		t.Errorf("disable with logout: %+v", bad)
	}
}

// TestRetryDelay: doubling from push.retry_base, capped at push.retry_cap, never below a
// provider's Retry-After, scaled by the jitter in [0.5, 1].
func TestRetryDelay(t *testing.T) {
	t.Parallel()
	base, maxDelay := 10*time.Second, 5*time.Minute
	want := []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second, 160 * time.Second,
		5 * time.Minute, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for i, w := range want {
		attempt := int64(i + 1)
		if got := domain.RetryDelay(attempt, base, maxDelay, 0, 1); got != w {
			t.Errorf("attempt %d: %s, want %s", attempt, got, w)
		}
		if got := domain.RetryDelay(attempt, base, maxDelay, 0, 0.5); got != w/2 {
			t.Errorf("attempt %d jitter 0.5: %s, want %s", attempt, got, w/2)
		}
		// Jitter outside the bounds is clamped.
		if got := domain.RetryDelay(attempt, base, maxDelay, 0, 7); got != w {
			t.Errorf("attempt %d jitter 7: %s, want %s", attempt, got, w)
		}
		if got := domain.RetryDelay(attempt, base, maxDelay, 0, 0.1); got != w/2 {
			t.Errorf("attempt %d jitter 0.1: %s, want %s", attempt, got, w/2)
		}
		if got := domain.RetryDelay(attempt, base, maxDelay, time.Hour, 0.5); got != time.Hour {
			t.Errorf("attempt %d retry-after: %s, want 1h", attempt, got)
		}
	}
	if got := domain.RetryDelay(0, base, maxDelay, 0, 1); got != base {
		t.Errorf("attempt 0: %s, want %s", got, base)
	}
	if got := domain.RetryDelay(200, base, maxDelay, 0, 1); got != maxDelay {
		t.Errorf("attempt 200: %s, want the cap", got)
	}
}

// TestRunActive: queued, running and waiting_input are active; the end states are not.
func TestRunActive(t *testing.T) {
	t.Parallel()
	for state, want := range map[string]bool{
		"queued": true, "running": true, "waiting_input": true,
		"completed": false, "failed": false, "cancelled": false, "": false, "unknown": false,
	} {
		if got := domain.RunActive(state); got != want {
			t.Errorf("RunActive(%q) = %v, want %v", state, got, want)
		}
	}
}
