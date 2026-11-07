// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

var wakeLim = domain.WakeLimits{LabelMaxChars: 10, TextMaxBytes: 64, MinDelay: time.Minute, MaxWait: 7 * 24 * time.Hour}

// pointers returns the field pointers of a validation error.
func pointers(t *testing.T, err error) []string {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("want validation_failed, got %v", err)
	}
	var out []string
	for _, f := range e.Fields {
		out = append(out, f.Pointer+" "+f.Code)
	}
	return out
}

// TestCheckWakeRequest: every field rule of run.wake.create is its own 400 with its pointer (06
// R-API-30); a valid request of each kind passes.
func TestCheckWakeRequest(t *testing.T) {
	t.Parallel()
	ok := []domain.WakeRequest{
		{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Label: "CI of #12"},
		{Kind: domain.WakePRChecks, Reason: domain.WakeReasonAgent, Condition: domain.WakeCondition{Repository: "example/app", Number: 12, HeadCommit: "abc1234"}},
		{Kind: domain.WakeCommand, Reason: domain.WakeReasonAgent, Condition: domain.WakeCondition{CommandLine: "test -f done\n\techo ok"}},
		{Kind: domain.WakeAt, Reason: domain.WakeReasonUsageLimit, Outage: 1},
	}
	for _, r := range ok {
		if err := domain.CheckWakeRequest(r, wakeLim); err != nil {
			t.Errorf("%+v: %v", r, err)
		}
	}
	bad := []struct {
		r    domain.WakeRequest
		want string
	}{
		{domain.WakeRequest{Kind: "timer", Reason: domain.WakeReasonAgent}, "/kind invalid"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonRecovery}, "/reason not_allowed"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: "boredom"}, "/reason invalid"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Outage: 2}, "/outage not_allowed"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonUsageLimit}, "/outage required"},
		{domain.WakeRequest{Kind: domain.WakeCommand, Reason: domain.WakeReasonUsageLimit, Outage: 1,
			Condition: domain.WakeCondition{CommandLine: "true"}}, "/kind not_allowed"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Label: "eleven char"}, "/label too_long"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Label: "a‮b"}, "/label invalid"},
		{domain.WakeRequest{Kind: domain.WakePRChecks, Reason: domain.WakeReasonAgent, Condition: domain.WakeCondition{Repository: "app", Number: 1}},
			"/condition/repository invalid"},
		{domain.WakeRequest{Kind: domain.WakePRChecks, Reason: domain.WakeReasonAgent, Condition: domain.WakeCondition{Repository: "a/b"}},
			"/condition/number invalid"},
		{domain.WakeRequest{Kind: domain.WakePRChecks, Reason: domain.WakeReasonAgent,
			Condition: domain.WakeCondition{Repository: "a/b", Number: 1, HeadCommit: "XYZ"}}, "/condition/head_commit invalid"},
		{domain.WakeRequest{Kind: domain.WakeCommand, Reason: domain.WakeReasonAgent}, "/condition/command_line required"},
		{domain.WakeRequest{Kind: domain.WakeCommand, Reason: domain.WakeReasonAgent,
			Condition: domain.WakeCondition{CommandLine: strings.Repeat("x", 65)}}, "/condition/command_line too_long"},
		{domain.WakeRequest{Kind: domain.WakeCommand, Reason: domain.WakeReasonAgent,
			Condition: domain.WakeCondition{CommandLine: "echo \x07"}}, "/condition/command_line invalid"},
		{domain.WakeRequest{Kind: domain.WakeAt, Reason: domain.WakeReasonAgent, Condition: domain.WakeCondition{CommandLine: "true"}},
			"/condition not_allowed"},
		{domain.WakeRequest{Kind: domain.WakeCommand, Reason: domain.WakeReasonAgent, NotBefore: time.Now(),
			Condition: domain.WakeCondition{CommandLine: "true"}}, "/not_before not_allowed"},
	}
	for _, b := range bad {
		got := pointers(t, domain.CheckWakeRequest(b.r, wakeLim))
		if len(got) == 0 || got[0] != b.want {
			t.Errorf("%+v: got %v, want %s first", b.r, got, b.want)
		}
	}
}

// TestWakeWindow: the minimum delay holds also for a time in the past and for none; the deadline
// is capped at run.wake_max_wait after creation, for usage_limit after not_before (D-AR-01-12); a
// deadline before not_before is a field error.
func TestWakeWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	nb, dl, err := domain.WakeWindow(now, now.Add(-time.Hour), time.Time{}, domain.WakeReasonAgent, wakeLim)
	if err != nil || !nb.Equal(now.Add(time.Minute)) || !dl.Equal(now.Add(wakeLim.MaxWait)) {
		t.Fatalf("past time: %v %v %v", nb, dl, err)
	}
	reset := now.Add(6 * 24 * time.Hour)
	nb, dl, err = domain.WakeWindow(now, reset, time.Time{}, domain.WakeReasonUsageLimit, wakeLim)
	if err != nil || !nb.Equal(reset) || !dl.Equal(reset.Add(wakeLim.MaxWait)) {
		t.Fatalf("usage limit: %v %v %v", nb, dl, err)
	}
	nb, dl, err = domain.WakeWindow(now, reset, time.Time{}, domain.WakeReasonAgent, wakeLim)
	if err != nil || !nb.Equal(reset) || !dl.Equal(now.Add(wakeLim.MaxWait)) {
		t.Fatalf("agent: %v %v %v", nb, dl, err)
	}
	_, dl, err = domain.WakeWindow(now, time.Time{}, now.Add(30*24*time.Hour), domain.WakeReasonAgent, wakeLim)
	if err != nil || !dl.Equal(now.Add(wakeLim.MaxWait)) {
		t.Fatalf("capped deadline: %v %v", dl, err)
	}
	if _, _, err := domain.WakeWindow(now, now.Add(time.Hour), now.Add(10*time.Minute), domain.WakeReasonAgent, wakeLim); pointers(t, err)[0] != "/deadline invalid" {
		t.Fatalf("deadline before not_before: %v", err)
	}
}

// TestAutonomyCaps: the chain cap refuses at the cap and 0 switches it off; the idle cap refuses
// at its cap whatever the chain cap is; an ended chain keeps its reason; an idle report raises the
// idle count, one with work resets it, and reaching the cap ends the chain once.
func TestAutonomyCaps(t *testing.T) {
	t.Parallel()
	lim := domain.AutonomyLimits{ChainMax: 3, IdleMax: 2}
	if r := domain.AutonomyRefusal(domain.Autonomy{Chain: 2}, lim); r != "" {
		t.Fatalf("below the cap: %q", r)
	}
	if r := domain.AutonomyRefusal(domain.AfterInput(domain.Autonomy{Chain: 2}), lim); r != domain.WakeCodeChainMax {
		t.Fatalf("at the chain cap: %q", r)
	}
	if r := domain.AutonomyRefusal(domain.Autonomy{Chain: 300}, domain.AutonomyLimits{IdleMax: 2}); r != "" {
		t.Fatalf("no chain cap: %q", r)
	}
	if r := domain.AutonomyRefusal(domain.Autonomy{Idle: 2}, domain.AutonomyLimits{IdleMax: 2}); r != domain.WakeCodeIdleMax {
		t.Fatalf("idle cap without chain cap: %q", r)
	}
	if r := domain.AutonomyRefusal(domain.Autonomy{EndedReason: domain.WakeCodeIdleMax}, lim); r != domain.WakeCodeIdleMax {
		t.Fatalf("ended chain: %q", r)
	}
	a, reached := domain.AfterIdleReport(domain.Autonomy{Idle: 1}, false, lim)
	if a.Idle != 0 || reached {
		t.Fatalf("work resets: %+v %v", a, reached)
	}
	a, reached = domain.AfterIdleReport(domain.Autonomy{Idle: 1}, true, lim)
	if a.Idle != 2 || !reached || a.EndedReason != domain.WakeCodeIdleMax {
		t.Fatalf("idle cap reached: %+v %v", a, reached)
	}
	if _, again := domain.AfterIdleReport(a, true, lim); again {
		t.Fatal("the cap is reached once")
	}
	if _, reached := domain.AfterIdleReport(domain.Autonomy{}, true, domain.AutonomyLimits{}); !reached {
		t.Fatal("an idle cap below 1 counts as 1")
	}
}

// TestWakeStateAndNotes: refused, expired and failed notify, fired and cancelled do not; every
// note names the label and the reason.
func TestWakeStateAndNotes(t *testing.T) {
	t.Parallel()
	for s, want := range map[domain.WakeState]bool{domain.WakeRefused: true, domain.WakeExpired: true, domain.WakeFailed: true,
		domain.WakeFired: false, domain.WakeCancelled: false, domain.WakePending: false} {
		if s.Notifies() != want {
			t.Errorf("%s notifies %v", s, !want)
		}
	}
	for _, s := range []domain.WakeState{domain.WakeRefused, domain.WakeExpired, domain.WakeFailed, domain.WakeCancelled} {
		n := domain.WakeNoteText("CI of #12", s, "agent_not_reachable")
		if !strings.Contains(n, "CI of #12") || (s != domain.WakeExpired && !strings.Contains(n, "agent_not_reachable")) {
			t.Errorf("%s: %q", s, n)
		}
	}
	if n := domain.WakeNoteText("", domain.WakeRefused, domain.WakeCodeChainMax); !strings.Contains(n, "reply") {
		t.Errorf("chain cap note: %q", n)
	}
	if !domain.FireInputSkipped.Valid() || domain.FireOutcome("done").Valid() {
		t.Error("fire outcomes")
	}
	c := domain.DecodeCondition(domain.EncodeCondition(domain.WakeCondition{Repository: "a/b", Number: 3}))
	if c.Repository != "a/b" || c.Number != 3 || string(domain.EncodeCondition(domain.WakeCondition{})) != "{}" {
		t.Errorf("condition round trip %+v", c)
	}
	if last, ok := domain.AutonomyLastOf([]byte(`{"autonomy_last":{"input":"w1","idle":true}}`)); !ok || last.Input != "w1" || !last.Idle {
		t.Errorf("autonomy_last %+v %v", last, ok)
	}
	if _, ok := domain.AutonomyLastOf([]byte(`{}`)); ok {
		t.Error("absent autonomy_last")
	}
	if err := domain.CheckResultText("line 1\nline 2", 64); err != nil {
		t.Errorf("multi-line result: %v", err)
	}
	if got := pointers(t, domain.CheckResultText(strings.Repeat("x", 65), 64)); got[0] != "/result_text too_long" {
		t.Errorf("long result %v", got)
	}
}
