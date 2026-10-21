// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var sessionStart = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// current is a current, started row of agent at version 3.
func current(agent ids.ID) domain.Session {
	return domain.Session{
		ID: ids.New(), ThreadID: ids.New(), AgentID: agent, ExternalSessionRef: "sess-7f3a", IsCurrent: true, Started: true,
		Model: "model-large", Currency: "USD", CostIsEstimate: true, CreatedAt: sessionStart, Version: 3,
	}
}

func TestEndSession(t *testing.T) {
	at := sessionStart.Add(time.Hour)
	for _, reason := range []string{domain.EndReset, domain.EndRebind, domain.EndImport, domain.EndAgentChanged} {
		t.Run(reason, func(t *testing.T) {
			s := current(ids.New())
			ended, err := domain.EndSession(s, reason, at)
			if err != nil {
				t.Fatalf("EndSession: %v", err)
			}
			if ended.IsCurrent || !ended.EndedAt.Equal(at) || ended.EndReason != reason || ended.Version != s.Version+1 {
				t.Fatalf("ended = %+v, want ended at %v with %s at version %d", ended, at, reason, s.Version+1)
			}
			if _, err := domain.EndSession(ended, reason, at); !errors.Is(err, domain.ErrSessionEnded) {
				t.Fatalf("second end = %v, want ErrSessionEnded", err)
			}
		})
	}
	if _, err := domain.EndSession(current(ids.New()), "paused", at); err == nil {
		t.Fatal("an unknown end reason passed")
	}
}

func TestApplyReport(t *testing.T) {
	started := current(ids.New())
	unstarted := started
	unstarted.Started = false
	cases := []struct {
		name    string
		row     domain.Session
		report  domain.SessionReport
		kind    domain.ReportKind
		ref     string
		model   string
		started bool
		changed bool
		reason  string
	}{
		{name: "reference change", row: started, report: domain.SessionReport{ExternalSessionRef: "sess-new"}, kind: domain.ReportChanged,
			ref: "sess-new", model: "model-large", started: true, changed: true},
		{name: "model change", row: started, report: domain.SessionReport{ExternalSessionRef: "sess-7f3a", Model: "model-small"},
			kind: domain.ReportChanged, ref: "sess-7f3a", model: "model-small", started: true},
		{name: "started true", row: unstarted, report: domain.SessionReport{ExternalSessionRef: "sess-7f3a", Started: true},
			kind: domain.ReportChanged, ref: "sess-7f3a", model: "model-large", started: true},
		{name: "started false on a started row", row: started, report: domain.SessionReport{ExternalSessionRef: "sess-7f3a"},
			kind: domain.ReportUnchanged, ref: "sess-7f3a", model: "model-large", started: true},
		{name: "empty report", row: unstarted, report: domain.SessionReport{ExternalSessionRef: "sess-7f3a"},
			kind: domain.ReportUnchanged, ref: "sess-7f3a", model: "model-large"},
		{name: "same model", row: started, report: domain.SessionReport{ExternalSessionRef: "sess-7f3a", Model: "model-large", Started: true},
			kind: domain.ReportUnchanged, ref: "sess-7f3a", model: "model-large", started: true},
		{name: "bind_failed on a started row", row: started,
			report: domain.SessionReport{ExternalSessionRef: "sess-new", BindFailed: domain.BindFailedSessionMissing, Started: true},
			kind:   domain.ReportBindFailed, ref: "sess-new", started: true},
		{name: "bind_failed on an unstarted row", row: unstarted,
			report: domain.SessionReport{ExternalSessionRef: "sess-new", BindFailed: domain.BindFailedSessionMissing},
			reason: domain.ReasonSessionNotBound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := domain.ApplyReport(c.row, c.report)
			if c.reason != "" {
				fe, ok := errs.As(err)
				if !ok || fe.Code != errs.CodeConflict || fe.Reason != c.reason {
					t.Fatalf("ApplyReport = %v, want conflict %s", err, c.reason)
				}
				return
			}
			if err != nil {
				t.Fatalf("ApplyReport: %v", err)
			}
			if out.Kind != c.kind || out.ExternalSessionRef != c.ref || out.Model != c.model || out.Started != c.started ||
				out.RefChanged != c.changed {
				t.Fatalf("outcome = %+v, want kind %d ref %q model %q started %v ref changed %v", out, c.kind, c.ref, c.model,
					c.started, c.changed)
			}
		})
	}
}

// importRow is an import row of agent created at offset.
func importRow(thread, agent ids.ID, offset time.Duration) domain.Session {
	return domain.Session{
		ID: ids.New(), ThreadID: thread, AgentID: agent, ExternalSessionRef: "sess-import", Started: true,
		CreatedAt: sessionStart.Add(offset), EndedAt: sessionStart.Add(offset), EndReason: domain.EndImport, Version: 1,
	}
}

func TestImportBindingCandidate(t *testing.T) {
	thread, lotse, runner := ids.New(), ids.New(), ids.New()
	if _, ok := domain.ImportBindingCandidate(nil, lotse); ok {
		t.Fatal("no rows gave a candidate")
	}
	only := importRow(thread, lotse, 0)
	if got, ok := domain.ImportBindingCandidate([]domain.Session{only}, lotse); !ok || got.ID != only.ID {
		t.Fatalf("only import rows of the agent = %v %v, want the row", got.ID, ok)
	}
	if _, ok := domain.ImportBindingCandidate([]domain.Session{importRow(thread, runner, 0)}, lotse); ok {
		t.Fatal("an import row of another agent gave a candidate")
	}
	reset := current(lotse)
	reset.ThreadID, reset.IsCurrent, reset.EndedAt, reset.EndReason = thread, false, sessionStart.Add(time.Hour), domain.EndReset
	if _, ok := domain.ImportBindingCandidate([]domain.Session{reset, only}, lotse); ok {
		t.Fatal("a thread with an earlier reset row bound an import row")
	}
	older, newest := importRow(thread, lotse, time.Minute), importRow(thread, lotse, 2*time.Minute)
	if got, ok := domain.ImportBindingCandidate([]domain.Session{older, newest}, lotse); !ok || got.ID != newest.ID {
		t.Fatalf("two import rows = %v, want the newest %v", got.ID, newest.ID)
	}
	same := importRow(thread, lotse, time.Minute)
	if got, _ := domain.ImportBindingCandidate([]domain.Session{same, older}, lotse); got.ID != same.ID {
		t.Fatalf("equal created_at = %v, want the higher id %v", got.ID, same.ID)
	}
}

func TestCostBaseline(t *testing.T) {
	if micros, currency := domain.CostBaseline(nil); micros != 0 || currency != "" {
		t.Fatalf("no row = %d %q, want 0 and the default", micros, currency)
	}
	prev := domain.Session{CostBaselineMicros: 1_000, CumulativeCostMicros: 250, Currency: "EUR"}
	if micros, currency := domain.CostBaseline(&prev); micros != 1_250 || currency != "EUR" {
		t.Fatalf("baseline = %d %q, want 1250 EUR", micros, currency)
	}
}

// fieldCode returns the pointer and code of the one field error of err.
func fieldCode(t *testing.T, err error) (string, string) {
	t.Helper()
	fe, ok := errs.As(err)
	if !ok || fe.Code != errs.CodeValidationFailed || len(fe.Fields) != 1 {
		t.Fatalf("error = %v, want validation_failed with one field", err)
	}
	return fe.Fields[0].Pointer, fe.Fields[0].Code
}

func TestValidateExternalSessionRef(t *testing.T) {
	const limit = 16
	cases := []struct {
		name, ref, code string
	}{
		{"empty", "", "required"},
		{"white space only", "   ", "required"},
		{"invalid utf8", "sess-\xff", "invalid_utf8"},
		{"control character", "sess-\x07", "control_character"},
		{"bidi control", "sess-\u202e", "bidi_control"},
		{"one byte too long", strings.Repeat("a", limit+1), "too_long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pointer, code := fieldCode(t, domain.CheckExternalSessionRef(c.ref, limit))
			if pointer != domain.PointerExternalSessionRef || code != c.code {
				t.Fatalf("field = %s %s, want %s %s", pointer, code, domain.PointerExternalSessionRef, c.code)
			}
		})
	}
	for _, ok := range []string{strings.Repeat("a", limit), "sitzung-über", "sess-\U0001F680"} {
		if err := domain.CheckExternalSessionRef(ok, limit); err != nil {
			t.Fatalf("CheckExternalSessionRef(%q) = %v, want nil", ok, err)
		}
	}
}

func TestValidateModel(t *testing.T) {
	for _, ok := range []string{"", "model-large", "vendor/model:1.5@latest"} {
		if err := domain.CheckModel(ok); err != nil {
			t.Fatalf("CheckModel(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"model large", "model\n", "modell-ü"} {
		pointer, code := fieldCode(t, domain.CheckModel(bad))
		if pointer != domain.PointerModel || code != "pattern" {
			t.Fatalf("CheckModel(%q) field = %s %s, want /model pattern", bad, pointer, code)
		}
	}
	pointer, code := fieldCode(t, domain.CheckBindFailed("gone"))
	if pointer != domain.PointerBindFailed || code != "invalid" {
		t.Fatalf("CheckBindFailed field = %s %s", pointer, code)
	}
	if err := domain.CheckBindFailed(domain.BindFailedSessionMissing); err != nil {
		t.Fatalf("CheckBindFailed(session_missing) = %v", err)
	}
}
