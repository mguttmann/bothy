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

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
)

func TestSubscriptionTransitions(t *testing.T) {
	t.Parallel()
	const (
		active   = domain.SubscriptionActive
		paused   = domain.SubscriptionPaused
		disabled = domain.SubscriptionDisabled
	)
	allowed := map[domain.SubscriptionState]map[domain.SubscriptionEvent]domain.SubscriptionState{
		active: {
			domain.EventOwnerPause: paused, domain.EventGone: disabled, domain.EventMaxFailures: disabled,
			domain.EventRetryMaxAge: disabled, domain.EventGap: disabled, domain.EventQuota: paused,
		},
		paused: {
			domain.EventOwnerResume: active, domain.EventGone: disabled, domain.EventMaxFailures: disabled,
			domain.EventRetryMaxAge: disabled, domain.EventGap: disabled,
		},
		disabled: {domain.EventOwnerResume: active, domain.EventOwnerPause: paused},
	}
	events := []domain.SubscriptionEvent{domain.EventOwnerPause, domain.EventOwnerResume, domain.EventGone,
		domain.EventMaxFailures, domain.EventRetryMaxAge, domain.EventGap, domain.EventQuota}
	for _, from := range []domain.SubscriptionState{active, paused, disabled} {
		for _, ev := range events {
			got, err := domain.NextSubscriptionState(from, ev)
			want, ok := allowed[from][ev]
			switch {
			case ok && (err != nil || got != want):
				t.Errorf("%s + %s = %s %v, want %s", from, ev, got, err, want)
			case !ok && (err == nil || got != from || !errors.Is(err, domain.ErrTransition)):
				t.Errorf("%s + %s = %s %v, want a refusal that keeps the state", from, ev, got, err)
			}
		}
	}
	if _, err := domain.NextSubscriptionState("gone", domain.EventOwnerPause); err == nil {
		t.Fatal("unknown state accepted")
	}
	if !domain.EventOwnerResume.Owner() || domain.EventGap.Owner() {
		t.Fatal("owner events")
	}
	for ev, cause := range map[domain.SubscriptionEvent]string{domain.EventGone: "gone", domain.EventMaxFailures: "max_consecutive_failures",
		domain.EventRetryMaxAge: "retry_max_age", domain.EventGap: "gap", domain.EventQuota: "max_pending"} {
		if ev.Cause() != cause {
			t.Errorf("cause of %s = %q, want %q", ev, ev.Cause(), cause)
		}
	}
}

func TestValidateTarget(t *testing.T) {
	t.Parallel()
	allowed, err := domain.ParseAllowedHosts([]string{"hooks.example.com", "alt.example.com:8443", "192.0.2.10", "[2001:db8::1]:9443"})
	if err != nil {
		t.Fatal(err)
	}
	ok := []struct{ raw, host string }{
		{"https://hooks.example.com/chat", "hooks.example.com"},
		{"https://hooks.example.com:443/chat?x=1", "hooks.example.com"},
		{"https://HOOKS.Example.COM./chat", "hooks.example.com"},
		{"https://alt.example.com:8443/hooks", "alt.example.com"},
		{"https://192.0.2.10/in", "192.0.2.10"},
		{"https://[2001:db8::1]:9443/in", "2001:db8::1"},
	}
	for _, c := range ok {
		tg, err := domain.ValidateTarget(c.raw, allowed)
		if err != nil || tg.Host != c.host || tg.URL.Scheme != "https" {
			t.Errorf("ValidateTarget(%q) = %+v %v", c.raw, tg, err)
		}
	}
	notListed := []string{
		"https://other.example.com/chat",
		"https://hooks.example.com:8443/chat",
		"https://alt.example.com/hooks",
		"https://192.0.2.11/in",
		"https://198.51.100.7/in",
		"https://[2001:db8::1]/in",
	}
	for _, raw := range notListed {
		if _, err := domain.ValidateTarget(raw, allowed); !errors.Is(err, domain.ErrHostNotAllowed) {
			t.Errorf("ValidateTarget(%q) = %v, want ErrHostNotAllowed", raw, err)
		}
	}
	invalid := []string{
		"", "http://hooks.example.com/chat", "ftp://hooks.example.com/", "https://user:pw@hooks.example.com/",
		"https://hooks.example.com/chat#frag", "hooks.example.com/chat", "https:///chat", "https://hooks.example.com:0/",
		"https://hooks.example.com:99999/", "https://hooks.example.com:abc/", "//hooks.example.com/x", "https:opaque",
		"https://hooks example.com/", "https://[fe80::1%25eth0]/",
	}
	for _, raw := range invalid {
		if _, err := domain.ValidateTarget(raw, allowed); !errors.Is(err, domain.ErrInvalidTarget) {
			t.Errorf("ValidateTarget(%q) = %v, want ErrInvalidTarget", raw, err)
		}
	}
	for _, bad := range [][]string{{"https://hooks.example.com"}, {"hooks.example.com/x"}, {"hooks.example.com:0"}, {""}, {"a:b:c"}} {
		if _, err := domain.ParseAllowedHosts(bad); err == nil {
			t.Errorf("ParseAllowedHosts(%q) accepted", bad)
		}
	}
	policy := domain.AllowsHost(allowed)
	if !policy("https", "hooks.example.com", 443) || policy("http", "hooks.example.com", 443) || policy("https", "hooks.example.com", 8443) ||
		!policy("https", "alt.example.com", 8443) {
		t.Fatal("host policy differs from ValidateTarget")
	}
}

func FuzzValidateTarget(f *testing.F) {
	for _, s := range []string{"https://hooks.example.com/x", "http://a", "https://u@h/", "https://[::1]:1/", "%", "https://hooks.example.com:443"} {
		f.Add(s)
	}
	allowed, _ := domain.ParseAllowedHosts([]string{"hooks.example.com", "[::1]:1"})
	f.Fuzz(func(t *testing.T, raw string) {
		tg, err := domain.ValidateTarget(raw, allowed)
		if err != nil {
			return
		}
		if tg.URL.Scheme != "https" || tg.URL.User != nil || tg.URL.Fragment != "" || tg.Port == 0 {
			t.Fatalf("accepted %q as %+v", raw, tg)
		}
	})
}

func TestValidateEventTypes(t *testing.T) {
	t.Parallel()
	granted := []string{"message.created", "message.updated"}
	if fe := domain.ValidateEventTypes([]string{"message.created"}, granted); len(fe) != 0 {
		t.Fatalf("valid list refused: %v", fe)
	}
	cases := []struct {
		name    string
		types   []string
		pointer string
		code    string
	}{
		{"empty", nil, "/event_types", domain.CodeTooFewItems},
		{"duplicate", []string{"message.created", "message.created"}, "/event_types/1", domain.CodeInvalid},
		{"not subscribable", []string{"message.created", "agent.invocation"}, "/event_types/1", domain.CodeNotSubscribable},
		{"unknown", []string{"no.such_type"}, "/event_types/0", domain.CodeNotSubscribable},
		{"not granted", []string{"thread.created"}, "/event_types/0", domain.CodeNotGranted},
	}
	for _, c := range cases {
		fe := domain.ValidateEventTypes(c.types, granted)
		if len(fe) != 1 || fe[0].Pointer != c.pointer || fe[0].Code != c.code {
			t.Errorf("%s: %+v, want %s at %s", c.name, fe, c.code, c.pointer)
		}
	}
}

// repoRoot is the root of the repository, found from this file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

func TestAppSubscribableMatchesContract(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	var spec struct {
		Webhooks map[string]any `yaml:"webhooks"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "docs", "api", "openapi.yaml")) //nolint:gosec // G304: a fixed path below the repository root, not input.
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Events []struct {
			Type      string `yaml:"type"`
			Receivers string `yaml:"receivers"`
		} `yaml:"events"`
	}
	raw, err = os.ReadFile(filepath.Join(root, "docs", "spec", "actions.yaml")) //nolint:gosec // G304: a fixed path below the repository root, not input.
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	agentOnly := map[string]bool{}
	for _, e := range catalog.Events {
		if e.Receivers == "agent" {
			agentOnly[e.Type] = true
		}
	}
	var want []string
	for k := range spec.Webhooks {
		if !agentOnly[k] {
			want = append(want, k)
		}
	}
	slices.Sort(want)
	got := domain.AppSubscribableTypes()
	if !slices.Equal(got, want) {
		t.Fatalf("AppSubscribable differs from the contract\n got %s\nwant %s", strings.Join(got, " "), strings.Join(want, " "))
	}
	for _, ty := range want {
		if !domain.AppSubscribable(ty) {
			t.Fatalf("%s not subscribable", ty)
		}
	}
	if domain.AppSubscribable("agent.invocation") || domain.AppSubscribable("") {
		t.Fatal("agent-only or empty type subscribable")
	}
}
