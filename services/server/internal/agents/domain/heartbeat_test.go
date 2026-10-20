// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// runA and runB are run ids of the form of R-DM-01 (UUID version 7).
var (
	runA = ids.New().String()
	runB = ids.New().String()
)

// pointers returns the pointers of fes.
func pointers(fes []errs.FieldError) []string {
	out := make([]string, 0, len(fes))
	for _, fe := range fes {
		out = append(out, fe.Pointer)
	}
	return out
}

// hasError reports whether fes holds a field error at pointer, with code when code is not "".
func hasError(fes []errs.FieldError, pointer, code string) bool {
	return slices.ContainsFunc(fes, func(fe errs.FieldError) bool {
		return fe.Pointer == pointer && (code == "" || fe.Code == code)
	})
}

// TestDecodeHeartbeat: the closed members of agent_heartbeat.v1 (brief M2-20 note 3); each
// rejection names its pointer.
func TestDecodeHeartbeat(t *testing.T) {
	t.Parallel()
	full := `{"state":"degraded","runtime_version":"2.3.1","reported_billing_mode":"organization",
		"billing_modes_supported":["organization","personal"],
		"billing_evidence":{"provider":"anthropic","key_type":"api_key","method":"resolved_credentials"},
		"reported_tool_class":"scoped",
		"details":{"capabilities":{"followups":"steer","components":true,"voice_input":false,"title":true,"files_out":false},
			"active_runs":["` + runA + `","` + runB + `"],"degraded_reasons":["rate_limited"],
			"adapter":{"name":"hermes-adapter","version":"1.0.0+build.7"},"runtime":{"name":"hermes"},"protocol_version":1},
		"project_readiness":[{"channel_id":"` + runA + `","ready":true}]}`
	t.Run("minimal", func(t *testing.T) {
		t.Parallel()
		r, fes := domain.DecodeHeartbeat([]byte(`{"state":"online"}`))
		if len(fes) > 0 {
			t.Fatalf("errors %+v", fes)
		}
		if r.State != domain.RuntimeOnline || r.BillingMode != "" || r.ToolClass != "" || r.Evidence != nil || r.ReadinessGiven {
			t.Fatalf("report %+v", r)
		}
	})
	t.Run("full", func(t *testing.T) {
		t.Parallel()
		r, fes := domain.DecodeHeartbeat([]byte(full))
		if len(fes) > 0 {
			t.Fatalf("errors %+v", fes)
		}
		if r.State != domain.RuntimeDegraded || r.RuntimeVersion != "2.3.1" || r.BillingMode != domain.Organization ||
			r.ToolClass != domain.ToolScoped || r.Evidence == nil || r.Evidence.KeyType != "api_key" || r.Evidence.Method != "resolved_credentials" ||
			len(r.BillingModesSupported) != 2 || len(r.Details.ActiveRuns) != 2 || r.Details.ProtocolVersion != 1 ||
			r.Details.Adapter == nil || r.Details.Adapter.Version != "1.0.0+build.7" || !r.ReadinessGiven || len(r.Readiness) != 1 ||
			!r.Readiness[0].Ready {
			t.Fatalf("report %+v", r)
		}
		if r.Details.Capabilities == nil || r.Details.Capabilities.Followups != "steer" || r.Details.Capabilities.Components == nil ||
			!*r.Details.Capabilities.Components {
			t.Fatalf("capabilities %+v", r.Details.Capabilities)
		}
	})
	cases := map[string]struct {
		body, pointer, code string
	}{
		"state offline":            {`{"state":"offline"}`, "/state", "invalid_enum"},
		"state missing":            {`{}`, "/state", "required"},
		"unknown top-level member": {`{"state":"online","extra":1}`, "/extra", "unknown_member"},
		"unknown details key":      {`{"state":"online","details":{"projects":[]}}`, "/details/projects", "unknown_member"},
		"unknown capability":       {`{"state":"online","details":{"capabilities":{"teleport":true}}}`, "/details/capabilities/teleport", "unknown_member"},
		"unknown adapter key":      {`{"state":"online","details":{"adapter":{"name":"a","path":"x"}}}`, "/details/adapter/path", "unknown_member"},
		"adapter without name":     {`{"state":"online","details":{"adapter":{"version":"1"}}}`, "/details/adapter/name", "required"},
		"runtime unknown key":      {`{"state":"online","details":{"runtime":{"name":"a","home":"x"}}}`, "/details/runtime/home", "unknown_member"},
		"followups always":         {`{"state":"online","details":{"capabilities":{"followups":"always"}}}`, "/details/capabilities/followups", "invalid_enum"},
		"capability not boolean":   {`{"state":"online","details":{"capabilities":{"title":"yes"}}}`, "/details/capabilities/title", "invalid_type"},
		"active run not uuid":      {`{"state":"online","details":{"active_runs":["run-1"]}}`, "/details/active_runs/0", "invalid_format"},
		"active run duplicate":     {`{"state":"online","details":{"active_runs":["` + runA + `","` + runA + `"]}}`, "/details/active_runs/1", "duplicate"},
		"protocol version 0":       {`{"state":"online","details":{"protocol_version":0}}`, "/details/protocol_version", "too_small"},
		"protocol version float":   {`{"state":"online","details":{"protocol_version":1.5}}`, "/details/protocol_version", "invalid_type"},
		"modes duplicate":          {`{"state":"online","billing_modes_supported":["personal","personal"]}`, "/billing_modes_supported/1", "duplicate"},
		"mode unknown":             {`{"state":"online","reported_billing_mode":"shared"}`, "/reported_billing_mode", "invalid_enum"},
		"tool class unknown":       {`{"state":"online","reported_tool_class":"root"}`, "/reported_tool_class", "invalid_enum"},
		"key type unknown":         {`{"state":"online","billing_evidence":{"key_type":"magic"}}`, "/billing_evidence/key_type", "invalid_enum"},
		"key type missing":         {`{"state":"online","billing_evidence":{"method":"x"}}`, "/billing_evidence/key_type", "required"},
		"evidence unknown key":     {`{"state":"online","billing_evidence":{"key_type":"oauth","key":"x"}}`, "/billing_evidence/key", "unknown_member"},
		"reasons duplicate":        {`{"state":"degraded","details":{"degraded_reasons":["slow","slow"]}}`, "/details/degraded_reasons/1", "duplicate"},
		"readiness duplicate": {`{"state":"online","project_readiness":[{"channel_id":"` + runA + `","ready":true},{"channel_id":"` + runA +
			`","ready":false}]}`, "/project_readiness", "duplicate"},
		"readiness unknown key": {`{"state":"online","project_readiness":[{"channel_id":"` + runA + `","ready":true,"path":"x"}]}`,
			"/project_readiness/0/path", "unknown_member"},
		"rate limit":      {`{"state":"online","rate_limit":{}}`, "/rate_limit", "not_enabled"},
		"not an object":   {`[1]`, "", "invalid_type"},
		"details null":    {`{"state":"online","details":null}`, "/details", "invalid_type"},
		"runtime_version": {`{"state":"online","runtime_version":7}`, "/runtime_version", "invalid_type"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, fes := domain.DecodeHeartbeat([]byte(c.body))
			if !hasError(fes, c.pointer, c.code) {
				t.Fatalf("errors %+v, want %s at %q", fes, c.code, c.pointer)
			}
		})
	}
}

// TestHeartbeatForms: names, versions and codes refuse separators, whitespace, upper case in
// codes, '=', control and bidi characters at their pointers (brief M2-20 note 3).
func TestHeartbeatForms(t *testing.T) {
	t.Parallel()
	bad := []string{"a/b", `a\b`, "a b", "a\tb", "ab=", "a\u0007b", "a‮b", ""}
	places := map[string]func(v string) string{
		"/runtime_version": func(v string) string { return `{"state":"online","runtime_version":` + quote(v) + `}` },
		"/details/adapter/version": func(v string) string {
			return `{"state":"online","details":{"adapter":{"name":"a","version":` + quote(v) + `}}}`
		},
		"/details/adapter/name": func(v string) string { return `{"state":"online","details":{"adapter":{"name":` + quote(v) + `}}}` },
		"/details/runtime/name": func(v string) string { return `{"state":"online","details":{"runtime":{"name":` + quote(v) + `}}}` },
		"/billing_evidence/provider": func(v string) string {
			return `{"state":"online","billing_evidence":{"key_type":"oauth","provider":` + quote(v) + `}}`
		},
		"/billing_evidence/method": func(v string) string {
			return `{"state":"online","billing_evidence":{"key_type":"oauth","method":` + quote(v) + `}}`
		},
		"/details/degraded_reasons/0": func(v string) string { return `{"state":"degraded","details":{"degraded_reasons":[` + quote(v) + `]}}` },
	}
	for pointer, body := range places {
		for _, v := range bad {
			if _, fes := domain.DecodeHeartbeat([]byte(body(v))); !hasError(fes, pointer, "") {
				t.Errorf("%s %q accepted: %+v", pointer, v, fes)
			}
		}
	}
	for _, pointer := range []string{"/billing_evidence/method", "/billing_evidence/provider", "/details/degraded_reasons/0",
		"/details/adapter/name", "/details/runtime/name"} {
		if _, fes := domain.DecodeHeartbeat([]byte(places[pointer]("Upper"))); !hasError(fes, pointer, "pattern_mismatch") {
			t.Errorf("%s accepted upper case: %+v", pointer, fes)
		}
	}
	for pointer, v := range map[string]string{"/billing_evidence/method": "resolved_credentials", "/billing_evidence/provider": "api_key",
		"/runtime_version": "2.3.1", "/details/adapter/version": "1.0.0+build.7"} {
		if _, fes := domain.DecodeHeartbeat([]byte(places[pointer](v))); len(fes) > 0 {
			t.Errorf("%s %q refused: %+v", pointer, v, fes)
		}
	}
}

// quote returns v as a JSON string.
func quote(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// TestReportedChange: a change of the reported mode or class, also from or to "not reported",
// gives one detail set with exactly the keys of brief M2-20 note 4.
func TestReportedChange(t *testing.T) {
	t.Parallel()
	ev := &domain.Evidence{KeyType: "oauth", Method: "resolved_credentials"}
	cases := map[string]struct {
		stored, next domain.Reported
		evidence     *domain.Evidence
		want         map[string]string
		changed      bool
	}{
		"first report": {domain.Reported{}, domain.Reported{BillingMode: domain.Organization}, nil, map[string]string{"billing_mode_new": "organization"}, true},
		"organization to personal": {domain.Reported{BillingMode: domain.Organization}, domain.Reported{BillingMode: domain.Personal}, ev,
			map[string]string{"billing_mode_old": "organization", "billing_mode_new": "personal", "key_type": "oauth", "method": "resolved_credentials"}, true},
		"personal to none": {domain.Reported{BillingMode: domain.Personal}, domain.Reported{}, nil, map[string]string{"billing_mode_old": "personal"}, true},
		"chat to host": {domain.Reported{ToolClass: domain.ToolChat}, domain.Reported{ToolClass: domain.ToolHost}, nil,
			map[string]string{"tool_class_old": "chat", "tool_class_new": "host"}, true},
		"both": {domain.Reported{BillingMode: domain.Organization, ToolClass: domain.ToolChat}, domain.Reported{BillingMode: domain.Personal, ToolClass: domain.ToolHost},
			&domain.Evidence{KeyType: "unknown"}, map[string]string{"billing_mode_old": "organization", "billing_mode_new": "personal",
				"tool_class_old": "chat", "tool_class_new": "host", "key_type": "unknown"}, true},
		"equal": {domain.Reported{BillingMode: domain.Personal, ToolClass: domain.ToolHost}, domain.Reported{BillingMode: domain.Personal, ToolClass: domain.ToolHost},
			ev, nil, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			next := c.next
			next.Evidence = c.evidence
			got, changed := domain.ReportedChange(c.stored, next)
			if changed != c.changed {
				t.Fatalf("changed %v, want %v", changed, c.changed)
			}
			if !changed {
				return
			}
			m := got.Map()
			if len(m) != len(c.want) {
				t.Fatalf("details %v, want %v", m, c.want)
			}
			for k, v := range c.want {
				if m[k] != v {
					t.Fatalf("details %v, want %v", m, c.want)
				}
			}
		})
	}
}

// TestStoredDetails: the reported_details form holds exactly the members of note 6, absent
// members omitted, never null, and reads back.
func TestStoredDetails(t *testing.T) {
	t.Parallel()
	r, fes := domain.DecodeHeartbeat([]byte(`{"state":"online","billing_modes_supported":["organization"],
		"billing_evidence":{"key_type":"api_key"},"details":{"active_runs":[],"adapter":{"name":"a"}}}`))
	if len(fes) > 0 {
		t.Fatal(fes)
	}
	raw := r.StoredDetails()
	if bytes.Contains(raw, []byte("null")) || !bytes.Contains(raw, []byte(`"active_runs":[]`)) || bytes.Contains(raw, []byte("capabilities")) {
		t.Fatalf("stored %s", raw)
	}
	back, err := domain.ReportFromStore(domain.RuntimeOnline, "2.0", domain.Organization, domain.ToolChat, raw)
	if err != nil {
		t.Fatal(err)
	}
	if back.Evidence == nil || back.Evidence.KeyType != "api_key" || len(back.BillingModesSupported) != 1 || back.Details.Adapter == nil {
		t.Fatalf("read back %+v", back)
	}
	empty, err := domain.ReportFromStore(domain.RuntimeUnknown, "", "", "", []byte(`{}`))
	if err != nil || empty.Evidence != nil {
		t.Fatalf("empty %+v %v", empty, err)
	}
}

// FuzzDecodeHeartbeat: arbitrary bytes never panic, and an accepted report never carries '/',
// '\' or whitespace in a name, version or code.
func FuzzDecodeHeartbeat(f *testing.F) {
	for _, s := range []string{`{"state":"online"}`, `{"state":"degraded","runtime_version":"1.0","details":{"adapter":{"name":"a","version":"b"}}}`,
		`{"state":"online","billing_evidence":{"key_type":"oauth","method":"m","provider":"p"},"details":{"degraded_reasons":["x"]}}`,
		`{"state":"online","details":{"runtime":{"name":"r"},"active_runs":["` + runA + `"]}}`, `[]`, `{"state":`, ``} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		r, fes := domain.DecodeHeartbeat(b)
		if len(fes) > 0 {
			return
		}
		vals := []string{r.RuntimeVersion}
		if r.Evidence != nil {
			vals = append(vals, r.Evidence.Provider, r.Evidence.Method)
		}
		if r.Details.Adapter != nil {
			vals = append(vals, r.Details.Adapter.Name, r.Details.Adapter.Version)
		}
		if r.Details.Runtime != nil {
			vals = append(vals, r.Details.Runtime.Name)
		}
		vals = append(vals, r.Details.DegradedReasons...)
		for _, v := range vals {
			if strings.ContainsAny(v, "/\\ \t\r\n") {
				t.Fatalf("accepted %q in %s", v, b)
			}
		}
	})
}
