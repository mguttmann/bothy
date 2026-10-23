// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fakepush"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// maxAge is push.max_age of pushEnv.
const maxAge = time.Hour

// mentionJob lets the Owner mention the person with handle in the main thread of Allgemein, runs
// the consumer and returns the one new job notify.push_send of recipient for that message.
func (w *world) mentionJob(handle string, recipient ids.ID) pushJob {
	w.t.Helper()
	m := w.post("owner", w.mainThread(w.allgemein), "@"+handle+" please have a look")
	w.consume()
	var out []pushJob
	for _, j := range w.pushJobsFor(recipient) {
		if j.Args.MessageID == m.ID {
			out = append(out, j)
		}
	}
	if len(out) != 1 {
		w.t.Fatalf("%d push jobs of the mention, want 1: %+v", len(out), out)
	}
	return out[0]
}

// regState is the delivery state of a push registration row.
type regState struct {
	Failures      int64
	Reason        string
	Disabled      bool
	LastSuccessAt *time.Time
}

// registration reads the delivery columns of the registration id.
func (w *world) registration(id ids.ID) regState {
	w.t.Helper()
	var s regState
	must(w.t, w.db.QueryRow(context.Background(), `SELECT failure_count, coalesce(disabled_reason, ''), disabled_at IS NOT NULL, last_success_at
		FROM push_registrations WHERE id = $1`, id.String()).Scan(&s.Failures, &s.Reason, &s.Disabled, &s.LastSuccessAt))
	return s
}

// resetRegistration sets the delivery columns of the registration id to an active row with
// failures consecutive failures.
func (w *world) resetRegistration(id ids.ID, failures int) {
	w.sql(`UPDATE push_registrations SET failure_count = $2, disabled_at = NULL, disabled_reason = NULL, last_success_at = NULL WHERE id = $1`,
		id.String(), failures)
}

// followUps returns the pending jobs notify.push_send of the hint nid other than the job id.
func (w *world) followUps(nid ids.ID, id int64) []pushJob {
	var out []pushJob
	for _, j := range w.pushJobs() {
		if j.Args.NotificationID == nid && j.ID != id && j.pending() {
			out = append(out, j)
		}
	}
	return out
}

// TestPushSend_ProviderResults (13 7.1 to 7.3, brief M3-19 design note 13): through the job
// notify.push_send with the FakeProvider programmed per result class, the registration columns
// and the follow-up job follow the table of design note 13; a Retry-After of the provider is
// respected; a hint whose next retry is past push.max_age is dropped as expired; the token never
// appears in a captured log.
func TestPushSend_ProviderResults(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	reg := w.alexPhone.Registration
	accepted := fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultAccepted}}
	disable := func(reason string, since time.Time) fakepush.Outcome {
		return fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultDisable, DisableReason: reason, InvalidSince: since}}
	}
	retry := fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultRetry}}
	cases := []struct {
		name     string
		failures int
		out      fakepush.Outcome
		// want is the registration after the job (LastSuccessAt only checked for presence).
		want regState
		// success expects last_success_at at the clock; retry a follow-up job with Attempt 2 at
		// least retryAfter and at most retryMax after the clock.
		success            bool
		retry              bool
		retryMin, retryMax time.Duration
		outcome            string
	}{
		{name: "accepted", failures: 3, out: accepted, want: regState{}, success: true, outcome: domain.OutcomeAccepted},
		{name: "disable_token_invalid", out: disable(domain.DisableTokenInvalid, time.Time{}),
			want: regState{Reason: domain.DisableTokenInvalid, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "disable_topic_mismatch", out: disable(domain.DisableTopicMismatch, time.Time{}),
			want: regState{Reason: domain.DisableTopicMismatch, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "disable_sender_mismatch", out: disable(domain.DisableSenderMismatch, time.Time{}),
			want: regState{Reason: domain.DisableSenderMismatch, Disabled: true}, outcome: domain.OutcomeRejectedToken},
		{name: "disable_invalid_before_created", out: disable(domain.DisableTokenInvalid, start.Add(-time.Hour)),
			want: regState{}, outcome: domain.OutcomeRejectedToken},
		// The jitter is in [0.5, 1] of push.retry_base (2s) for the first retry.
		{name: "retry", out: retry, want: regState{Failures: 1}, retry: true, retryMin: time.Second, retryMax: 2 * time.Second,
			outcome: domain.OutcomeProviderError},
		{name: "retry_go_error", failures: 1, out: fakepush.Outcome{Err: errors.New("connection reset")}, want: regState{Failures: 2},
			retry: true, retryMin: time.Second, retryMax: 2 * time.Second, outcome: domain.OutcomeProviderError},
		{name: "retry_deactivates_at_max_consecutive_failures", failures: 2, out: retry,
			want: regState{Failures: 3, Reason: domain.DisableRepeatedFailures, Disabled: true}, outcome: domain.OutcomeProviderError},
		// Retry-After above push.retry_cap (5m) wins over the backoff.
		{name: "throttled_retry_after", failures: 1, out: fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultThrottled, RetryAfter: 10 * time.Minute}},
			want: regState{Failures: 1}, retry: true, retryMin: 10 * time.Minute, retryMax: 10 * time.Minute, outcome: domain.OutcomeThrottled},
		{name: "config_error", failures: 1, out: fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultConfigError}},
			want: regState{Failures: 1}, outcome: domain.OutcomeRejectedOther},
		{name: "program_error", failures: 1, out: fakepush.Outcome{Result: domain.SendResult{Class: domain.ResultProgramError}},
			want: regState{Failures: 1}, outcome: domain.OutcomeRejectedOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w.t = t
			w.resetRegistration(reg, c.failures)
			j := w.mentionJob("alex", w.alex)
			w.apns.Program(reg, c.out)
			calls, sent := len(w.apns.CallsFor(reg)), w.sent("apns", c.outcome)
			if err := w.send(j); err != nil {
				t.Fatalf("the job failed: %v", err)
			}
			got := w.apns.CallsFor(reg)
			if len(got) != calls+1 {
				t.Fatalf("%d provider calls, want %d", len(got), calls+1)
			}
			if n := got[len(got)-1].Notification; n.ID != j.Args.NotificationID || !n.HasBadge || n.Kind != string(domain.KindMention) {
				t.Fatalf("notification %+v", n)
			}
			s := w.registration(reg)
			if s.Failures != c.want.Failures || s.Reason != c.want.Reason || s.Disabled != c.want.Disabled {
				t.Fatalf("registration %+v, want %+v", s, c.want)
			}
			if c.success != (s.LastSuccessAt != nil) || (c.success && !s.LastSuccessAt.Equal(w.clk.Now().Truncate(time.Microsecond))) {
				t.Fatalf("last_success_at %v, want set at the clock: %v", s.LastSuccessAt, c.success)
			}
			if got := w.sent("apns", c.outcome) - sent; got != 1 {
				t.Fatalf("bothy.push.sent %s rose by %d, want 1", c.outcome, got)
			}
			follow := w.followUps(j.Args.NotificationID, j.ID)
			if !c.retry {
				if len(follow) != 0 {
					t.Fatalf("follow-up jobs %+v, want none", follow)
				}
				return
			}
			if len(follow) != 1 || follow[0].Args.Attempt != 2 || follow[0].Args.RegistrationID != reg || follow[0].Args.DueAtUnixMicro != j.Args.DueAtUnixMicro {
				t.Fatalf("follow-up jobs %+v, want one with Attempt 2", follow)
			}
			delay := follow[0].ScheduledAt.Sub(w.clk.Now())
			if delay < c.retryMin || delay > c.retryMax {
				t.Fatalf("retry after %s, want between %s and %s", delay, c.retryMin, c.retryMax)
			}
			w.complete(follow[0].ID)
		})
	}
	t.Run("retry_past_max_age_is_expired", func(t *testing.T) {
		w.t = t
		w.resetRegistration(reg, 0)
		j := w.mentionJob("alex", w.alex)
		// Half a second before push.max_age ends the job still sends; its next retry lies after it.
		w.clk.Set(time.UnixMicro(j.Args.DueAtUnixMicro).Add(maxAge - 500*time.Millisecond))
		w.apns.Program(reg, retry)
		expired := w.skipped("expired")
		if err := w.send(j); err != nil {
			t.Fatalf("the job failed: %v", err)
		}
		if s := w.registration(reg); s.Failures != 1 || s.Disabled {
			t.Fatalf("registration %+v, want one failure", s)
		}
		if follow := w.followUps(j.Args.NotificationID, j.ID); len(follow) != 0 {
			t.Fatalf("follow-up jobs %+v, want none past push.max_age", follow)
		}
		if got := w.skipped("expired") - expired; got != 1 {
			t.Fatalf("bothy.push.skipped expired rose by %d, want 1", got)
		}
	})
	w.t = t
	for _, tok := range []string{w.alexPhone.Token, w.samPhone.Token, w.ownerPhone.Token} {
		if w.log.Contains(tok) || w.pushLog.Contains(tok) {
			t.Fatal("a device token reached a log")
		}
	}
}

// TestPushSend_NoProvider (D-W-108): only apns is registered; the job for Sam's fcm registration
// ends without error and without a provider call, and counts no_provider.
func TestPushSend_NoProvider(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withProviders("apns"))
	j := w.mentionJob("sam", w.sam)
	if j.Args.RegistrationID != w.samPhone.Registration {
		t.Fatalf("job %+v, want Sam's fcm registration", j.Args)
	}
	before := w.skipped("no_provider")
	if err := w.send(j); err != nil {
		t.Fatalf("the job failed: %v", err)
	}
	if calls := w.apns.Calls(); len(calls) != 0 {
		t.Fatalf("provider calls %v", calls)
	}
	if got := w.skipped("no_provider") - before; got != 1 {
		t.Fatalf("bothy.push.skipped no_provider rose by %d, want 1", got)
	}
	if s := w.registration(w.samPhone.Registration); s != (regState{}) {
		t.Fatalf("the registration changed: %+v", s)
	}
}

// TestPushSend_TooOld (13 7.3): the clock passes DueAt + push.max_age before the job runs: no
// provider call, the job ends without error and counts expired.
func TestPushSend_TooOld(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	j := w.mentionJob("alex", w.alex)
	w.clk.Set(time.UnixMicro(j.Args.DueAtUnixMicro).Add(maxAge + time.Second))
	before := w.skipped("expired")
	if err := w.send(j); err != nil {
		t.Fatalf("the job failed: %v", err)
	}
	if calls := w.apns.Calls(); len(calls) != 0 {
		t.Fatalf("provider calls %v", calls)
	}
	if got := w.skipped("expired") - before; got != 1 {
		t.Fatalf("bothy.push.skipped expired rose by %d, want 1", got)
	}
}
