// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// transition is one allowed row of the table of Design note 2 of M2-39 (24 sections 3.1 to 3.4).
type transition struct {
	from     domain.State
	reason   string
	cause    domain.Cause
	to       domain.State
	toReason string
	ends     domain.CloseReason
}

// allowed is the table of 24 section 3.1, written out once more for the test. Ruling of M2-39:
// every transition into a state other than active ends the sessions of the host; where the table
// of the brief names no close reason (the host had no keys yet), the reason is host_rekeyed.
var allowed = []transition{
	{"", "", domain.CauseCreate, domain.StatePairing, "", ""},
	{domain.StatePairing, "", domain.CausePair, domain.StateConfirming, "", domain.CloseHostRekeyed},
	{domain.StatePairing, "", domain.CauseNewPairingCode, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateConfirming, "", domain.CauseConfirm, domain.StateActive, "", ""},
	{domain.StateConfirming, "", domain.CauseNewPairingCode, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateConfirming, "", domain.CauseFingerprintAttemptsExhausted, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateActive, "", domain.CauseNewPairingCode, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateActive, "", domain.CauseDisable, domain.StateDisabled, domain.DisabledOwner, domain.CloseHostDisabled},
	{domain.StateActive, "", domain.CauseDuplicateConnection, domain.StateDisabled, domain.DisabledDuplicateConnection, domain.CloseHostDisabled},
	{domain.StateActive, "", domain.CauseRetire, domain.StateRetired, "", domain.CloseHostRetired},
	{domain.StateDisabled, domain.DisabledOwner, domain.CauseEnable, domain.StateActive, "", ""},
	{domain.StateDisabled, domain.DisabledOwner, domain.CauseNewPairingCode, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateDisabled, domain.DisabledDuplicateConnection, domain.CauseNewPairingCode, domain.StatePairing, "", domain.CloseHostRekeyed},
	{domain.StateDisabled, domain.DisabledOwner, domain.CauseRetire, domain.StateRetired, "", domain.CloseHostRetired},
	{domain.StateDisabled, domain.DisabledDuplicateConnection, domain.CauseRetire, domain.StateRetired, "", domain.CloseHostRetired},
}

// TestHostTransition_Table (M2-39, 24 section 3.1): every pair of a state before (disabled with
// each reason) and a cause is allowed exactly as the table says, with target state, disabled
// reason and the close reason of the sessions it ends; every other pair is ErrInvalidTransition.
func TestHostTransition_Table(t *testing.T) {
	t.Parallel()
	type from struct {
		state  domain.State
		reason string
	}
	froms := []from{{"", ""}, {domain.StatePairing, ""}, {domain.StateConfirming, ""}, {domain.StateActive, ""},
		{domain.StateDisabled, domain.DisabledOwner}, {domain.StateDisabled, domain.DisabledDuplicateConnection}, {domain.StateRetired, ""}}
	var tried, ok int
	for _, f := range froms {
		for _, c := range domain.Causes() {
			tried++
			got, err := domain.Transition(f.state, f.reason, c)
			i := slices.IndexFunc(allowed, func(a transition) bool { return a.from == f.state && a.reason == f.reason && a.cause == c })
			if i < 0 {
				if !errors.Is(err, domain.ErrInvalidTransition) {
					t.Errorf("%q/%q %s: %+v, %v; want ErrInvalidTransition", f.state, f.reason, c, got, err)
				}
				if errs.KindOf(err) != errs.KindConflict || !strings.Contains(err.Error(), "invalid_state_transition") {
					t.Errorf("%q %s: error %v is not 409 invalid_state_transition", f.state, c, err)
				}
				continue
			}
			ok++
			a := allowed[i]
			want := domain.Result{To: a.to, DisabledReason: a.toReason, EndSessions: a.ends != "", CloseReason: a.ends}
			if err != nil || got != want {
				t.Errorf("%q/%q %s: %+v, %v; want %+v", f.state, f.reason, c, got, err, want)
			}
		}
	}
	if ok != len(allowed) || tried != len(froms)*len(domain.Causes()) {
		t.Fatalf("%d allowed of %d tried, want %d allowed", ok, tried, len(allowed))
	}
}

// TestNewHostName (M2-39, SEC-IN-4): trimmed, not blank, no control or bidi character, at most
// host.name_max_chars characters.
func TestNewHostName(t *testing.T) {
	t.Parallel()
	const maxChars = 10
	if got, err := domain.NewHostName("  Build server  ", 20); err != nil || got != "Build server" {
		t.Fatalf("trimmed: %q, %v", got, err)
	}
	if got, err := domain.NewHostName(strings.Repeat("ä", maxChars), maxChars); err != nil || got != strings.Repeat("ä", maxChars) {
		t.Fatalf("exactly the maximum: %v", err)
	}
	for name, c := range map[string]struct{ in, code string }{
		"blank":    {"   ", "required"},
		"empty":    {"", "required"},
		"too long": {strings.Repeat("a", maxChars+1), "too_long"},
		"control":  {"a\u0007b", "control_character"},
		"bidi":     {"a\u202eb", "bidi_control"},
	} {
		_, err := domain.NewHostName(c.in, maxChars)
		wantField(t, name, err, "/name", c.code)
	}
}

// TestNewOSUser (M2-39, 03 host_session_users.os_user): the pattern of 03, never a path, at most
// host.os_user_max_chars characters.
func TestNewOSUser(t *testing.T) {
	t.Parallel()
	const maxChars = 16
	for _, ok := range []string{"bothy-s-alex", "BOTHY-S-SHARED", "_svc", "a.b", strings.Repeat("x", maxChars)} {
		if got, err := domain.NewOSUser(ok, maxChars, "/os_user"); err != nil || got != ok {
			t.Errorf("%q: %q, %v", ok, got, err)
		}
	}
	for name, c := range map[string]struct{ in, code string }{
		"empty":       {"", "required"},
		"path":        {"../x", "pattern_mismatch"},
		"slash":       {"a/b", "pattern_mismatch"},
		"leading dot": {".hidden", "pattern_mismatch"},
		"space":       {"a b", "pattern_mismatch"},
		"too long":    {strings.Repeat("x", maxChars+1), "too_long"},
	} {
		_, err := domain.NewOSUser(c.in, maxChars, "/add_session_users/0/os_user")
		wantField(t, name, err, "/add_session_users/0/os_user", c.code)
	}
}

// wantField checks a validation_failed error with one field error.
func wantField(t *testing.T, name string, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Errorf("%s: %v, want validation_failed at %s with %s", name, err, pointer, code)
	}
}

// TestPlanAssignments (M2-39, ruling T12, TM-T28 (b) and (e)): exact match with a reported name,
// uniqueness per host over all rows without regard to case, at most one valid assignment per
// person, revocations before additions, all 400 before any 409.
func TestPlanAssignments(t *testing.T) {
	t.Parallel()
	alex, sam, robin := ids.New(), ids.New(), ids.New()
	reported := []string{"bothy-s-alex", "bothy-s-robin", "bothy-s-sam", "bothy-s-shared", "BOTHY-S-SHARED"}
	row := func(user ids.ID, name string, ended bool) domain.SessionUserRow {
		return domain.SessionUserRow{ID: ids.New(), UserID: user, OSUser: name, Revoked: ended}
	}
	add := func(user ids.ID, name string) domain.SessionUserAssign {
		return domain.SessionUserAssign{UserID: user, OSUser: name}
	}
	rows := []domain.SessionUserRow{row(alex, "bothy-s-alex", true), row(robin, "bothy-s-shared", false)}

	p, err := domain.PlanAssignments(reported, rows, nil, []domain.SessionUserAssign{add(sam, "bothy-s-sam")})
	if err != nil || len(p.Add) != 1 || p.Add[0].OSUser != "bothy-s-sam" || len(p.Revoke) != 0 {
		t.Fatalf("plain add: %+v, %v", p, err)
	}
	cases := []struct {
		name    string
		rows    []domain.SessionUserRow
		revoke  []ids.ID
		add     []domain.SessionUserAssign
		code    string
		pointer string
	}{
		{"name not reported", rows, nil, []domain.SessionUserAssign{add(sam, "bothy-s-typo")}, "session_user_not_on_host", "/add_session_users/0/os_user"},
		{"case variant not reported", rows, nil, []domain.SessionUserAssign{add(sam, "BOTHY-S-SAM")}, "session_user_not_on_host", "/add_session_users/0/os_user"},
		{"ended name of another person", rows, nil, []domain.SessionUserAssign{add(sam, "bothy-s-alex")}, "session_user_taken", ""},
		{"name of another person in another case", rows, nil, []domain.SessionUserAssign{add(sam, "BOTHY-S-SHARED")}, "session_user_taken", ""},
		{"second valid assignment", rows, nil, []domain.SessionUserAssign{add(robin, "bothy-s-robin")}, "session_user_taken", ""},
		{"duplicate inside the request", nil, nil, []domain.SessionUserAssign{add(sam, "bothy-s-sam"), add(alex, "BOTHY-S-SAM")}, "session_user_not_on_host", "/add_session_users/1/os_user"},
		{"duplicate person inside the request", nil, nil, []domain.SessionUserAssign{add(sam, "bothy-s-sam"), add(sam, "bothy-s-alex")}, "session_user_taken", ""},
		{"same name twice", nil, nil, []domain.SessionUserAssign{add(sam, "bothy-s-sam"), add(alex, "bothy-s-sam")}, "session_user_taken", ""},
		{"all 400 before any 409", rows, nil, []domain.SessionUserAssign{add(sam, "bothy-s-alex"), add(alex, "bothy-s-typo")}, "session_user_not_on_host", "/add_session_users/1/os_user"},
	}
	for _, c := range cases {
		_, err := domain.PlanAssignments(reported, c.rows, c.revoke, c.add)
		e, ok := errs.As(err)
		switch {
		case !ok:
			t.Errorf("%s: %v", c.name, err)
		case c.code == "session_user_taken":
			if e.Code != errs.CodeConflict || e.Reason != "session_user_taken" {
				t.Errorf("%s: %v, want 409 session_user_taken", c.name, err)
			}
		default:
			if e.Code != errs.CodeValidationFailed || len(e.Fields) == 0 || e.Fields[0].Code != c.code || e.Fields[0].Pointer != c.pointer {
				t.Errorf("%s: %v %+v, want %s at %s", c.name, err, e.Fields, c.code, c.pointer)
			}
		}
	}
	// Revoke Robin, then give Robin a new name in the same request.
	p, err = domain.PlanAssignments(reported, rows, []ids.ID{robin}, []domain.SessionUserAssign{add(robin, "bothy-s-robin")})
	if err != nil || len(p.Revoke) != 1 || p.Revoke[0].UserID != robin || len(p.Add) != 1 {
		t.Fatalf("revoke then add: %+v, %v", p, err)
	}
	// A listed person without a valid assignment has no effect.
	p, err = domain.PlanAssignments(reported, rows, []ids.ID{sam, alex}, nil)
	if err != nil || len(p.Revoke) != 0 || !p.Empty() {
		t.Fatalf("revoke without assignment: %+v, %v", p, err)
	}
}

// TestCloseLists (M2-39, 24 section 10.3): the closed lists of close_reason and close_detail
// equal the enums TerminalCloseReason and TerminalCloseDetail of openapi.yaml.
func TestCloseLists(t *testing.T) {
	t.Parallel()
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// enumOf reads the one-line enum of the schema name (the form of openapi.yaml).
	enumOf := func(name string) []string {
		text := string(raw)
		i := strings.Index(text, "\n    "+name+":\n")
		if i < 0 {
			t.Fatalf("schema %s not found", name)
		}
		rest := text[i:]
		j := strings.Index(rest, "enum: [")
		k := strings.Index(rest[j:], "]")
		var out []string
		for _, v := range strings.Split(rest[j+len("enum: ["):j+k], ",") {
			out = append(out, strings.TrimSpace(v))
		}
		return out
	}
	reasons := make([]string, 0)
	for _, r := range domain.CloseReasons() {
		reasons = append(reasons, string(r))
	}
	details := make([]string, 0)
	for _, d := range domain.CloseDetails() {
		details = append(details, string(d))
	}
	if want := enumOf("TerminalCloseReason"); !slices.Equal(reasons, want) {
		t.Errorf("close reasons\n got %v\nwant %v", reasons, want)
	}
	if want := enumOf("TerminalCloseDetail"); !slices.Equal(details, want) {
		t.Errorf("close details\n got %v\nwant %v", details, want)
	}
}

// TestReportedDecoders (M2-39): the reported jsonb columns decode into their Go mirrors; an empty
// or broken value is an empty list, never a panic, and never a path.
func TestReportedDecoders(t *testing.T) {
	t.Parallel()
	profiles := domain.DecodeProfiles([]byte(`[{"name":"shell","display_name":"Shell","kind":"shell"}]`))
	if len(profiles) != 1 || profiles[0].Name != "shell" || profiles[0].Kind != "shell" {
		t.Fatalf("profiles %+v", profiles)
	}
	users := domain.DecodeReportedSessionUsers([]byte(`[{"os_user":"bothy-s-sam","workdirs":["Project A"]}]`))
	if len(users) != 1 || users[0].OSUser != "bothy-s-sam" || !slices.Equal(users[0].Workdirs, []string{"Project A"}) {
		t.Fatalf("users %+v", users)
	}
	doctor := domain.DecodeDoctor([]byte(`[{"check":"hd6_llm_login","ok":false,"code":"subscription_login_detected","session_user":"bothy-s-sam"}]`))
	if len(doctor) != 1 || doctor[0].OK || doctor[0].SessionUser != "bothy-s-sam" {
		t.Fatalf("doctor %+v", doctor)
	}
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`not json`)} {
		if len(domain.DecodeProfiles(raw)) != 0 || len(domain.DecodeReportedSessionUsers(raw)) != 0 || len(domain.DecodeDoctor(raw)) != 0 {
			t.Errorf("%q decodes to a non-empty list", raw)
		}
	}
}

// TestHostApply (M2-39): a transition sets state and disabled reason, disabled_at describes only
// the current disabled state, retire sets retired_at, and the version rises; ReportedNames lists
// the reported names in order.
func TestHostApply(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	h := domain.Host{State: domain.StateActive, Version: 3}
	r, _ := domain.Transition(h.State, "", domain.CauseDisable)
	d := h.Apply(r, at)
	if d.State != domain.StateDisabled || d.DisabledReason != domain.DisabledOwner || !d.DisabledAt.Equal(at) || d.Version != 4 {
		t.Fatalf("disable %+v", d)
	}
	r, _ = domain.Transition(d.State, d.DisabledReason, domain.CauseEnable)
	if e := d.Apply(r, at); e.State != domain.StateActive || e.DisabledReason != "" || !e.DisabledAt.IsZero() || e.Version != 5 {
		t.Fatalf("enable %+v", e)
	}
	r, _ = domain.Transition(d.State, d.DisabledReason, domain.CauseRetire)
	if x := d.Apply(r, at); x.State != domain.StateRetired || !x.RetiredAt.Equal(at) || !x.DisabledAt.IsZero() || x.DisabledReason != "" {
		t.Fatalf("retire %+v", x)
	}
	if !(domain.Host{PairedAt: at}).Paired() || (domain.Host{}).Paired() {
		t.Fatal("Paired")
	}
	got := domain.ReportedNames([]domain.ReportedSessionUser{{OSUser: "a"}, {OSUser: "b"}})
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("ReportedNames %v", got)
	}
	if !domain.IsInvalidTransition(domain.ErrInvalidTransition) || domain.IsInvalidTransition(nil) {
		t.Fatal("IsInvalidTransition")
	}
}
