// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// TestComputerEnds holds the closed lists of 27 8.3: details only with open_failed,
// stopped_by_user and error, each from its own part; the reasons a host may send.
func TestComputerEnds(t *testing.T) {
	t.Parallel()
	ok := []struct {
		r domain.ComputerCloseReason
		d domain.ComputerCloseDetail
	}{
		{domain.ComputerDone, ""}, {domain.ComputerRunEnded, ""}, {domain.ComputerStoppedByUser, domain.ComputerClientStop},
		{domain.ComputerOpenFailed, domain.ComputerOpenTimeout}, {domain.ComputerError, domain.ComputerStreamFault},
		{domain.ComputerOpenFailed, domain.ComputerTokenReplayed},
	}
	for _, c := range ok {
		if err := domain.ValidateComputerEnd(c.r, c.d); err != nil {
			t.Errorf("%s %s: %v", c.r, c.d, err)
		}
	}
	bad := []struct {
		r domain.ComputerCloseReason
		d domain.ComputerCloseDetail
	}{
		{"bored", ""}, {domain.ComputerDone, domain.ComputerClientStop}, {domain.ComputerRunEnded, domain.ComputerOpenTimeout},
		{domain.ComputerError, domain.ComputerOpenTimeout}, {domain.ComputerStoppedByUser, "screen_text"},
	}
	for _, c := range bad {
		if err := domain.ValidateComputerEnd(c.r, c.d); err == nil {
			t.Errorf("%s %s accepted", c.r, c.d)
		}
	}
	if domain.HostMayEndComputer(domain.ComputerRunEnded, "") || domain.HostMayEndComputer(domain.ComputerAccessRevoked, "") ||
		domain.HostMayEndComputer(domain.ComputerStoppedByUser, domain.ComputerClientStop) {
		t.Error("a server reason accepted from the host")
	}
	if !domain.HostMayEndComputer(domain.ComputerStoppedByUser, domain.ComputerLocalEscape) || !domain.HostMayEndComputer(domain.ComputerDone, "") {
		t.Error("a host reason refused")
	}
	if len(domain.ComputerCloseReasons()) != 12 || len(domain.ComputerCloseDetails()) != 14 {
		t.Errorf("lists %d %d", len(domain.ComputerCloseReasons()), len(domain.ComputerCloseDetails()))
	}
}

// enabledComputer is a connected helper, enabled with generation 3 and every permission granted.
func enabledComputer() *domain.ReportedComputer {
	return &domain.ReportedComputer{Helper: "connected", Enabled: true, Generation: 3, OSAccount: "alex", Capture: "granted",
		Input: "granted", HelperVersion: "1.0.0", Signature: "verified"}
}

// TestDesktopListed holds 27 section 4 item 3 and the disabled generation (D-CU-01-25).
func TestDesktopListed(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	base := domain.Host{ID: ids.New(), State: domain.StateActive, ReportedComputer: enabledComputer()}
	gen := func(n int64) *int64 { return &n }
	cases := []struct {
		name      string
		mut       func(*domain.Host)
		connected bool
		want      bool
	}{
		{"listed", func(*domain.Host) {}, true, true},
		{"not connected", func(*domain.Host) {}, false, false},
		{"disabled host", func(h *domain.Host) { h.State = domain.StateDisabled }, true, false},
		{"no report", func(h *domain.Host) { h.ReportedComputer = nil }, true, false},
		{"helper absent", func(h *domain.Host) { h.ReportedComputer.Helper = "absent" }, true, false},
		{"switched off", func(h *domain.Host) { h.ReportedComputer.Enabled = false }, true, false},
		{"capture denied", func(h *domain.Host) { h.ReportedComputer.Capture = "denied" }, true, false},
		{"input unknown", func(h *domain.Host) { h.ReportedComputer.Input = "unknown" }, true, false},
		{"disabled remotely at this generation", func(h *domain.Host) { h.ComputerDisabledGeneration = gen(3) }, true, false},
		{"switched on again locally", func(h *domain.Host) { h.ComputerDisabledGeneration = gen(2) }, true, true},
		{"assigned to another person", func(h *domain.Host) { h.AssignedUserID = sam }, true, false},
		{"assigned to the person", func(h *domain.Host) { h.AssignedUserID = alex }, true, true},
	}
	for _, c := range cases {
		h := base
		rc := *base.ReportedComputer
		h.ReportedComputer = &rc
		c.mut(&h)
		if got := domain.DesktopListed(h, c.connected, alex); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

// TestReportedComputer: the stored form nests the permissions, the check finds every bad value.
func TestReportedComputer(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(enabledComputer())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"helper":"connected","enabled":true,"generation":3,"os_account":"alex","permissions":{"capture":"granted","input":"granted"},` +
		`"helper_version":"1.0.0","signature":"verified","locked":false}`
	if string(b) != want {
		t.Fatalf("stored form %s", b)
	}
	var back domain.ReportedComputer
	if err := json.Unmarshal(b, &back); err != nil || back != *enabledComputer() {
		t.Fatalf("read back %+v %v", back, err)
	}
	lim := domain.ComputerLimits{OSUserMaxChars: 32, LabelMaxChars: 64}
	if f := domain.ValidateComputer(*enabledComputer(), lim); len(f) != 0 {
		t.Fatalf("valid report: %v", f)
	}
	bad := domain.ReportedComputer{Helper: "maybe", Capture: "yes", Input: "", Signature: "unsigned", Generation: -1,
		OSAccount: "../root", HelperVersion: "1.0‮"}
	if f := domain.ValidateComputer(bad, lim); len(f) != 7 {
		t.Fatalf("findings %d: %v", len(f), f)
	}
}

// TestComputerClaims holds the claims of 27 5.2 and the checks of 27 5.4 (CU-T17 in the domain).
func TestComputerClaims(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0)
	g := domain.ComputerGrant{InstanceID: ids.New(), HostID: ids.New(), UserID: ids.New(), SessionID: ids.New(), RunID: ids.New(),
		AgentID: ids.New(), CEK: "cek-hash", Generation: 4}
	jti := ids.New()
	c, err := domain.NewComputerClaims(g, jti, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.Payload()
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"iss", "aud", "sub", "sid", "jti", "iat", "exp", "run", "agt", "cek", "own", "gen"} {
		if _, ok := m[k]; !ok {
			t.Errorf("claim %s missing", k)
		}
	}
	if len(m) != 12 || c.Exp-c.Iat != 30 || !c.Own || c.Gen != 4 {
		t.Fatalf("claims %s", b)
	}
	if _, err := domain.NewComputerClaims(domain.ComputerGrant{}, jti, now, 30*time.Second); err == nil {
		t.Fatal("an empty grant became claims")
	}
	ses := domain.ComputerSession{ID: g.SessionID, HostID: g.HostID, UserID: g.UserID, RunID: g.RunID, AgentID: g.AgentID, TokenJTI: jti, Generation: 4}
	if v := domain.CheckAttachClaims(c, ses, now); v != domain.AttachOK {
		t.Fatalf("valid token: %s", v)
	}
	if v := domain.CheckAttachClaims(c, ses, now.Add(31*time.Second)); v != domain.AttachTokenExpired {
		t.Fatalf("expired: %s", v)
	}
	attached := ses
	attached.AttachedAt = now
	if v := domain.CheckAttachClaims(c, attached, now); v != domain.AttachReplayed {
		t.Fatalf("second attach: %s", v)
	}
	other := ses
	other.RunID = ids.New()
	if v := domain.CheckAttachClaims(c, other, now); v != domain.AttachTokenInvalid {
		t.Fatalf("other run: %s", v)
	}
}

// TestAppCodeAndFrontApp: app identities become audit codes; the frontmost app is a display text
// of at most host.name_max_chars characters.
func TestAppCodeAndFrontApp(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"com.example.Editor": "com.example.editor", "ABCDE12345": "abcde12345", "Example App.exe": "example_app.exe",
		"..hidden": "hidden", "": "", "‮": "",
	} {
		if got := domain.AppCode(in); got != want {
			t.Errorf("AppCode(%q) = %q, want %q", in, got, want)
		}
	}
	if !domain.IsAuditCode("left_click:3") || domain.IsAuditCode("Left") {
		t.Error("IsAuditCode")
	}
	if domain.CheckFrontApp("com.example.editor", 100) != nil {
		t.Error("a bundle id refused")
	}
	for _, bad := range []string{"", " lead", "a‮b", "x\ny"} {
		if domain.CheckFrontApp(bad, 100) == nil {
			t.Errorf("front app %q accepted", bad)
		}
	}
	if domain.CheckFrontApp("abcdef", 5) == nil {
		t.Error("a too long front app accepted")
	}
}

// TestComputerEndForHost maps the reasons of 24 10.3 to 27 8.3.
func TestComputerEndForHost(t *testing.T) {
	t.Parallel()
	for in, want := range map[domain.CloseReason]domain.ComputerCloseReason{
		domain.CloseHostDisabled: domain.ComputerHostDisabled, domain.CloseHostRetired: domain.ComputerHostRetired,
		domain.CloseHostRekeyed: domain.ComputerHostRekeyed, domain.CloseHostRestarted: domain.ComputerHostRestarted,
		domain.CloseUserDisabled: domain.ComputerAccessRevoked, domain.CloseAccessRevoked: domain.ComputerAccessRevoked,
		domain.CloseIdleTimeout: domain.ComputerConnectionLost,
	} {
		if got := domain.ComputerEndForHost(in); got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
}
