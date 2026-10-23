// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// readRepoFile reads a file below the repository root found by repoRoot.
func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// openapiEnum reads the one-line enum of the schema name in docs/api/openapi.yaml.
func openapiEnum(t *testing.T, name string) []string {
	t.Helper()
	text := readRepoFile(t, "docs", "api", "openapi.yaml")
	i := strings.Index(text, "\n    "+name+":\n")
	if i < 0 {
		t.Fatalf("schema %s not found", name)
	}
	rest := text[i:]
	j := strings.Index(rest, "enum: [")
	if j < 0 {
		t.Fatalf("schema %s has no enum", name)
	}
	k := strings.Index(rest[j:], "]")
	var out []string
	for _, v := range strings.Split(rest[j+len("enum: ["):j+k], ",") {
		out = append(out, strings.TrimSpace(v))
	}
	slices.Sort(out)
	return out
}

// spec103 returns the text of 24 section 10.3 up to the next heading.
func spec103(t *testing.T) string {
	t.Helper()
	text := readRepoFile(t, "docs", "spec", "24-terminal.md")
	start := strings.Index(text, "\n### 10.3 ")
	if start < 0 {
		t.Fatal("no section 10.3 in chapter 24")
	}
	text = text[start+1:]
	if end := strings.Index(text[1:], "\n#"); end >= 0 {
		text = text[:end+1]
	}
	return text
}

// tickCode matches one backticked lower-case code.
var tickCode = regexp.MustCompile("`([a-z0-9_]+)`")

// specReasons extracts the backticked codes of the first column of the table of 24 section 10.3.
func specReasons(t *testing.T) []string {
	t.Helper()
	var out []string
	inTable := false
	for _, line := range strings.Split(spec103(t), "\n") {
		switch {
		case !inTable && strings.HasPrefix(line, "|"):
			inTable = true // the header row
		case inTable && strings.HasPrefix(line, "|---"):
		case inTable && strings.HasPrefix(line, "|"):
			first := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)[0]
			for _, m := range tickCode.FindAllStringSubmatch(first, -1) {
				out = append(out, m[1])
			}
		case inTable:
			slices.Sort(out)
			return out
		}
	}
	t.Fatal("no reason table in section 10.3")
	return nil
}

// specDetails extracts the backticked codes of the paragraph of 24 section 10.3 that starts with
// `close_detail` and lists the closed list after its colon.
func specDetails(t *testing.T) []string {
	t.Helper()
	for _, para := range strings.Split(spec103(t), "\n\n") {
		if !strings.HasPrefix(para, "`close_detail`") {
			continue
		}
		var out []string
		for _, m := range tickCode.FindAllStringSubmatch(para[strings.Index(para, ":")+1:], -1) {
			out = append(out, m[1])
		}
		slices.Sort(out)
		return out
	}
	t.Fatal("no close_detail list in section 10.3")
	return nil
}

// migrationReasons extracts the CHECK list of terminal_sessions.close_reason from the migrations.
func migrationReasons(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "services", "server", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`'([a-z0-9_]+)'`)
	for _, f := range files {
		b, err := os.ReadFile(f) //nolint:gosec // G304: a migration file of the repository found by the glob above
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		i := strings.Index(text, "terminal_sessions_close_reason_check CHECK (close_reason IN (")
		if i < 0 {
			continue
		}
		rest := text[i:]
		if j := strings.Index(rest, "))"); j >= 0 {
			rest = rest[:j]
		}
		var out []string
		for _, m := range quoted.FindAllStringSubmatch(rest, -1) {
			out = append(out, m[1])
		}
		slices.Sort(out)
		return out
	}
	t.Fatal("no CHECK list of terminal_sessions.close_reason in the migrations")
	return nil
}

// sortedStrings returns the values as sorted strings.
func sortedStrings[T ~string](vs []T) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, string(v))
	}
	slices.Sort(out)
	return out
}

// TestCloseReasonsMatchSpec (M2-46): the Go set of close reasons equals the table of 24 section
// 10.3, the CHECK list of terminal_sessions.close_reason and the enum TerminalCloseReason.
func TestCloseReasonsMatchSpec(t *testing.T) {
	t.Parallel()
	got := sortedStrings(domain.CloseReasons())
	if len(got) == 0 {
		t.Fatal("empty Go set")
	}
	for name, want := range map[string][]string{
		"spec 24 10.3": specReasons(t),
		"migration":    migrationReasons(t),
		"openapi.yaml": openapiEnum(t, "TerminalCloseReason"),
	} {
		if !slices.Equal(got, want) {
			t.Errorf("%s\n got %v\nwant %v", name, got, want)
		}
	}
}

// TestCloseDetailsMatchSpec (M2-46): the Go set of close details equals the list of 24 section
// 10.3 and the enum TerminalCloseDetail. The migrations hold no CHECK list of close_detail.
func TestCloseDetailsMatchSpec(t *testing.T) {
	t.Parallel()
	got := sortedStrings(domain.CloseDetails())
	if len(got) == 0 {
		t.Fatal("empty Go set")
	}
	for name, want := range map[string][]string{
		"spec 24 10.3": specDetails(t),
		"openapi.yaml": openapiEnum(t, "TerminalCloseDetail"),
	} {
		if !slices.Equal(got, want) {
			t.Errorf("%s\n got %v\nwant %v", name, got, want)
		}
	}
}

// TestValidateEnd (Design note 2 of M2-46): the pair rule of 24 section 10.3 for every end.
func TestValidateEnd(t *testing.T) {
	t.Parallel()
	policy := []domain.CloseDetail{domain.DetailSubscriptionLoginDetected, domain.DetailHelperUntrusted, domain.DetailSharedLocation}
	for _, r := range domain.CloseReasons() {
		allowed, ok := map[domain.CloseReason][]domain.CloseDetail{domain.CloseOpenFailed: domain.CloseDetails(),
			domain.CloseHostPolicyViolated: policy}[r]
		if !ok {
			allowed = []domain.CloseDetail{""}
		}
		for _, d := range allowed {
			if err := domain.ValidateEnd(r, d); err != nil {
				t.Errorf("%s/%q: %v", r, d, err)
			}
		}
	}
	bad := []struct {
		r domain.CloseReason
		d domain.CloseDetail
	}{
		{domain.CloseOpenFailed, ""},
		{domain.CloseHostPolicyViolated, ""},
		{domain.CloseHostPolicyViolated, domain.DetailTokenInvalid},
		{domain.CloseHostPolicyViolated, domain.DetailOpenTimeout},
		{"", ""},
		{"vanished", ""},
		{"Client_Closed", ""},
		{domain.CloseOpenFailed, "no_such_detail"},
		{domain.CloseClientClosed, "no_such_detail"},
	}
	for _, d := range domain.CloseDetails() {
		bad = append(bad, struct {
			r domain.CloseReason
			d domain.CloseDetail
		}{domain.CloseAccessRevoked, d})
	}
	for _, c := range bad {
		err := domain.ValidateEnd(c.r, c.d)
		var e *errs.Error
		if !errors.As(err, &e) || e.Code != errs.CodeInternalError {
			t.Errorf("%q/%q: want internal_error, got %v", c.r, c.d, err)
			continue
		}
		if c.r != "" && strings.Contains(err.Error(), string(c.r)) || c.d != "" && strings.Contains(err.Error(), string(c.d)) {
			t.Errorf("%q/%q: the error names the values: %v", c.r, c.d, err)
		}
	}
}

// TestReasonForLostHost (Design note 4 c of M2-46).
func TestReasonForLostHost(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	plain := domain.Host{LLMCredentials: "none"}
	ofSam := domain.Host{LLMCredentials: domain.LLMPersonal, CredentialHolderID: sam}
	ofAlex := domain.Host{LLMCredentials: domain.LLMPersonal, CredentialHolderID: alex}
	for _, c := range []struct {
		name   string
		action string
		h      domain.Host
		want   domain.CloseReason
	}{
		{"user.disable", "user.disable", plain, domain.CloseUserDisabled},
		{"user.disable on a personal host of another holder", "user.disable", ofSam, domain.CloseUserDisabled},
		{"personal host of another holder", "grant.revoke", ofSam, domain.CloseI6Violation},
		{"personal host of the person", "grant.revoke", ofAlex, domain.CloseAccessRevoked},
		{"grant.revoke", "grant.revoke", plain, domain.CloseAccessRevoked},
		{"grant.expire", "grant.expire", plain, domain.CloseAccessRevoked},
		{"group.member.remove", "group.member.remove", plain, domain.CloseAccessRevoked},
		{"group.member.expire", "group.member.expire", plain, domain.CloseAccessRevoked},
		{"owner.remove", "owner.remove", plain, domain.CloseAccessRevoked},
		{"owner.remove on a personal host of another holder", "owner.remove", ofSam, domain.CloseI6Violation},
	} {
		if got := domain.ReasonForLostHost(c.action, c.h, alex); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

// TestSR10Lost (Design note 4 b of M2-46, 07 SR10).
func TestSR10Lost(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	yes, no := true, false
	for _, c := range []struct {
		name  string
		owner bool
		h     domain.Host
		want  bool
	}{
		{"owner keeps a server host of another assignee", true,
			domain.Host{AssignedUserID: sam, DeclaredServerHost: true, ReportedIsServerHost: &yes}, false},
		{"owner keeps a host without assignee", true, domain.Host{}, false},
		{"non-owner assignee keeps", false, domain.Host{AssignedUserID: alex}, false},
		{"non-owner not assignee loses", false, domain.Host{AssignedUserID: sam}, true},
		{"host without assignee loses", false, domain.Host{}, true},
		{"declared server host loses", false, domain.Host{AssignedUserID: alex, DeclaredServerHost: true}, true},
		{"reported server host loses", false, domain.Host{AssignedUserID: alex, ReportedIsServerHost: &yes}, true},
		{"reported no server host keeps", false, domain.Host{AssignedUserID: alex, ReportedIsServerHost: &no}, false},
	} {
		if got := domain.SR10Lost(c.owner, alex, c.h); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestReasonForDeviceRevoke (Design note 5 a of M2-46): the seven revoke_reason values of 03
// devices.
func TestReasonForDeviceRevoke(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]domain.CloseReason{
		"verification_failed": domain.CloseDeviceRevoked,
		"pending_expired":     domain.CloseDeviceRevoked,
		"pairing_expired":     domain.CloseDeviceRevoked,
		"revoked_by_self":     domain.CloseDeviceRevoked,
		"revoked_by_owner":    domain.CloseDeviceRevoked,
		"user_disabled":       domain.CloseUserDisabled,
		"inactive":            domain.CloseDeviceRevoked,
		"":                    domain.CloseDeviceRevoked,
	} {
		if got := domain.ReasonForDeviceRevoke(in); got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}
}

// TestReasonForSessionEnd (Design note 5 b of M2-46): the nine revoke_reason values of 03
// auth_sessions.
func TestReasonForSessionEnd(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]domain.CloseReason{
		"logout":                  domain.CloseLogout,
		"admin":                   domain.CloseSessionRevoked,
		"device_revoked":          domain.CloseDeviceRevoked,
		"refresh_reuse":           domain.CloseSessionRevoked,
		"user_disabled":           domain.CloseUserDisabled,
		"expired":                 domain.CloseSessionRevoked,
		"inactive":                domain.CloseSessionRevoked,
		"step_up_failed":          domain.CloseSessionRevoked,
		"trusted_header_mismatch": domain.CloseSessionRevoked,
		"":                        domain.CloseSessionRevoked,
	} {
		if got := domain.ReasonForSessionEnd(in); got != want {
			t.Errorf("%q: got %s, want %s", in, got, want)
		}
	}
}

// TestHostStateEnd (Design note 8 a of M2-46).
func TestHostStateEnd(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		state string
		want  domain.CloseReason
		ok    bool
	}{
		{"disabled", domain.CloseHostDisabled, true},
		{"retired", domain.CloseHostRetired, true},
		{"pairing", domain.CloseHostRekeyed, true},
		{"confirming", domain.CloseHostRekeyed, true},
		{"active", "", false},
		{"", "", false},
		{"vanished", "", false},
	} {
		got, ok := domain.HostStateEnd(c.state)
		if got != c.want || ok != c.ok {
			t.Errorf("%q: got (%s, %v), want (%s, %v)", c.state, got, ok, c.want, c.ok)
		}
	}
}
