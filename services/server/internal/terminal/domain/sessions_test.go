// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// repoRoot walks up from the working directory to the directory that holds docs/spec.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no docs/spec above the test")
		}
		dir = parent
	}
}

// specClaims extracts the backticked names of the first column of the claim table of 24 section
// 5.3: the table that follows the section heading and starts with the column "Claim".
func specClaims(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "spec", "24-terminal.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	start := strings.Index(text, "### 5.3 ")
	if start < 0 {
		t.Fatal("no section 5.3 in chapter 24")
	}
	var out []string
	inTable := false
	tick := regexp.MustCompile("`([a-z]+)`")
	for _, line := range strings.Split(text[start:], "\n")[1:] {
		switch {
		case strings.HasPrefix(line, "| Claim"):
			inTable = true
		case inTable && strings.HasPrefix(line, "|---"):
		case inTable && strings.HasPrefix(line, "|"):
			first := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0]
			for _, m := range tick.FindAllStringSubmatch(first, -1) {
				out = append(out, m[1])
			}
		case inTable:
			slices.Sort(out)
			return out
		}
	}
	t.Fatal("no claim table in section 5.3")
	return nil
}

// grant is a complete grant of a claude opening with index 0.
func grant() domain.TokenGrant {
	zero := 0
	return domain.TokenGrant{InstanceID: ids.New(), HostID: ids.New(), UserID: ids.New(), SessionID: ids.New(), DeviceID: ids.New(),
		Profile: "claude", ProfileKind: "claude", CEK: "cek-hash", OSUser: "bothy-s-alex", WorkdirIndex: &zero}
}

// members returns the JSON members of c in order.
func members(t *testing.T, c domain.SessionClaims) []string {
	t.Helper()
	b, err := c.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("null in %s", b)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	var out []string
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, k.(string))
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestSessionClaims_MatchSpec is the schema test of the plan row: the claims of a session token
// are exactly the claims of the table of 24 section 5.3, and the header constants are the ones the
// chapter states.
func TestSessionClaims_MatchSpec(t *testing.T) {
	t.Parallel()
	want := specClaims(t)
	got := members(t, mustClaims(t, grant(), time.Minute))
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("claims %v, chapter 24 section 5.3 lists %v", got, want)
	}
	if domain.TokenAlg != "EdDSA" || domain.TokenTyp != "terminal-session+jwt" {
		t.Fatalf("header %s %s", domain.TokenAlg, domain.TokenTyp)
	}
}

// mustClaims builds the claims of g.
func mustClaims(t *testing.T, g domain.TokenGrant, ttl time.Duration) domain.SessionClaims {
	t.Helper()
	c, err := domain.NewSessionClaims(g, ids.New(), time.Date(2026, 10, 14, 9, 0, 0, 700_000_000, time.UTC), ttl)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestSessionClaims_Shape (Design note 5): the members and their order for open shell, open claude
// with wdi 0 and 1, attach of a started session and the open token of an attach to an opening
// claude session; own present as false; exp - iat the ttl in seconds.
func TestSessionClaims_Shape(t *testing.T) {
	t.Parallel()
	full := []string{"iss", "aud", "sub", "sid", "jti", "iat", "exp", "dev", "prf", "cek", "own", "osu", "wdi", "att"}
	noWdi := slices.DeleteFunc(slices.Clone(full), func(s string) bool { return s == "wdi" })
	one := 1
	shell := grant()
	shell.Profile, shell.ProfileKind, shell.WorkdirIndex = "shell", "shell", nil
	claudeOne := grant()
	claudeOne.WorkdirIndex = &one
	attach := grant()
	attach.Attach, attach.WorkdirIndex = true, nil
	cases := map[string]struct {
		g       domain.TokenGrant
		members []string
		wdi     *int
		att     string
	}{
		"open shell":            {shell, noWdi, nil, "open"},
		"open claude 0":         {grant(), full, new(int), "open"},
		"open claude 1":         {claudeOne, full, &one, "open"},
		"attach started":        {attach, noWdi, nil, "attach"},
		"attach opening (open)": {claudeOne, full, &one, "open"},
	}
	for name, c := range cases {
		cl := mustClaims(t, c.g, 30*time.Second)
		if got := members(t, cl); !slices.Equal(got, c.members) {
			t.Errorf("%s: members %v, want %v", name, got, c.members)
		}
		if (cl.Wdi == nil) != (c.wdi == nil) || cl.Wdi != nil && *cl.Wdi != *c.wdi {
			t.Errorf("%s: wdi %v", name, cl.Wdi)
		}
		if cl.Att != c.att || cl.Own || cl.Exp-cl.Iat != 30 || cl.Iat != time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC).Unix() {
			t.Errorf("%s: %+v", name, cl)
		}
		b, _ := cl.Payload()
		if !strings.Contains(string(b), `"own":false`) {
			t.Errorf("%s: own absent in %s", name, b)
		}
	}
	bad := grant()
	bad.WorkdirIndex = nil
	if _, err := domain.NewSessionClaims(bad, ids.New(), time.Now(), time.Minute); err == nil {
		t.Fatal("a claude opening without an index must fail")
	}
	if _, err := domain.NewSessionClaims(grant(), ids.ID{}, time.Now(), time.Minute); err == nil {
		t.Fatal("a token without jti must fail")
	}
}

// TestProfileKnown (Design note 3 b, TM-T29 (g)).
func TestProfileKnown(t *testing.T) {
	t.Parallel()
	reported := []domain.Profile{{Name: "shell", Kind: "shell"}, {Name: "claude", Kind: "claude"}}
	own := []string{"Project A", "Project B"}
	idx := func(i int) *int { return &i }
	cases := []struct {
		name    string
		own     []string
		profile string
		index   *int
		want    bool
	}{
		{"profile missing", own, "missing-profile", nil, false},
		{"shell without index", own, "shell", nil, true},
		{"shell with index", own, "shell", idx(0), false},
		{"claude without index", own, "claude", nil, false},
		{"index 0", own, "claude", idx(0), true},
		{"last index", own, "claude", idx(1), true},
		{"index equal to the length", own, "claude", idx(2), false},
		{"negative index", own, "claude", idx(-1), false},
		{"index only in another person's list", []string{}, "claude", idx(0), false},
		{"empty own list", nil, "claude", idx(0), false},
	}
	for _, c := range cases {
		if got := domain.ProfileKnown(reported, c.own, c.profile, c.index); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	list := []domain.ReportedSessionUser{{OSUser: "bothy-s-alex", Workdirs: own}, {OSUser: "bothy-s-robin", Workdirs: []string{"Project C"}}}
	if got := domain.OwnWorkdirs(list, "bothy-s-alex"); !slices.Equal(got, own) {
		t.Fatalf("own workdirs %v", got)
	}
	if got := domain.OwnWorkdirs(list, "BOTHY-S-ALEX"); len(got) != 0 {
		t.Fatalf("the comparison is exact: %v", got)
	}
}

// TestStreamCodeForEnd (Design note 8): every detail of TerminalCloseDetail and a reason without
// a detail.
func TestStreamCodeForEnd(t *testing.T) {
	t.Parallel()
	want := map[domain.CloseDetail]domain.StreamCode{
		domain.DetailTokenExpired: 4101, domain.DetailTokenReplayed: 4101, domain.DetailTokenInvalid: 4104,
		domain.DetailHandshakeFailed: 4104, domain.DetailProfileUnknown: 4104, domain.DetailSessionUserUnknown: 4104,
		domain.DetailSessionUserMismatch: 4104, domain.DetailServerHostOwnerOnly: 4104,
	}
	for _, d := range domain.CloseDetails() {
		w, ok := want[d]
		if !ok {
			w = 4100
		}
		if got := domain.StreamCodeForEnd(domain.CloseOpenFailed, d); got != w {
			t.Errorf("%s: %d, want %d", d, got, w)
		}
	}
	for _, r := range domain.CloseReasons() {
		if got := domain.StreamCodeForEnd(r, ""); got != 4100 {
			t.Errorf("%s without detail: %d", r, got)
		}
	}
	if d, ok := domain.HostRefusal(4101, "token_replayed"); !ok || d != domain.DetailTokenReplayed {
		t.Fatal("4101 token_replayed is a refusal")
	}
	for _, c := range []struct {
		code   int
		reason string
	}{{4101, "token_invalid"}, {4104, "token_expired"}, {4104, "open_timeout"}, {1000, ""}, {4104, "free text"}} {
		if _, ok := domain.HostRefusal(c.code, c.reason); ok {
			t.Errorf("%d %q is no refusal", c.code, c.reason)
		}
	}
}

// TestAttachKind (Design note 6): opening attaches as open, active and detached as attach, closed
// is 409 invalid_state_transition.
func TestAttachKind(t *testing.T) {
	t.Parallel()
	for state, want := range map[string]string{"opening": "open", "active": "attach", "detached": "attach"} {
		if got, err := domain.AttachKind(state); err != nil || got != want {
			t.Errorf("%s: %s %v", state, got, err)
		}
	}
	if _, err := domain.AttachKind("closed"); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("closed: %v", err)
	}
}

// TestRecheckEnd (Design note 12): the close reasons of a recheck that no longer allows.
func TestRecheckEnd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		reason string
		state  domain.State
		want   domain.CloseReason
	}{
		{"capability_missing", domain.StateActive, domain.CloseAccessRevoked},
		{"no_grant", domain.StateActive, domain.CloseAccessRevoked},
		{"i6_holder_only", domain.StateActive, domain.CloseI6Violation},
		{"host_not_active", domain.StateDisabled, domain.CloseHostDisabled},
		{"host_not_active", domain.StateRetired, domain.CloseHostRetired},
		{"host_not_active", domain.StatePairing, domain.CloseHostRekeyed},
	}
	for _, c := range cases {
		if got := domain.RecheckEnd(c.reason, c.state); got != c.want {
			t.Errorf("%s %s: %s, want %s", c.reason, c.state, got, c.want)
		}
	}
}
