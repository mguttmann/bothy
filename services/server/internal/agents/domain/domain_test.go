// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantField checks that err is validation_failed with one field error on pointer.
func wantField(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v (%+v), want validation_failed on %s", err, e, pointer)
	}
}

// TestOperatorConfigV1Validation holds agent_operator_config.v1 (09 section 2.2): unknown keys,
// wrong types and a bad default_model are refused with the JSON pointer; the empty and the full
// object pass and come back canonical.
func TestOperatorConfigV1Validation(t *testing.T) {
	t.Parallel()
	bad := []struct {
		name    string
		in      map[string]any
		pointer string
	}{
		{"unknown key", map[string]any{"api_key": "x"}, "/operator_config/api_key"},
		{"model not a string", map[string]any{"default_model": 3.0}, "/operator_config/default_model"},
		{"model with space", map[string]any{"default_model": "big model"}, "/operator_config/default_model"},
		{"model empty", map[string]any{"default_model": ""}, "/operator_config/default_model"},
		{"flag not a boolean", map[string]any{"supports_voice": "yes"}, "/operator_config/supports_voice"},
		{"flag null", map[string]any{"supports_title": nil}, "/operator_config/supports_title"},
	}
	for _, c := range bad {
		_, err := domain.ParseOperatorConfig(c.in)
		wantField(t, err, c.pointer)
		if strings.Contains(err.Error(), "big model") {
			t.Errorf("%s: the error repeats the value", c.name)
		}
	}
	empty, err := domain.ParseOperatorConfig(map[string]any{})
	if err != nil || string(empty.JSON()) != `{}` {
		t.Fatalf("empty = %s, %v", empty.JSON(), err)
	}
	if none, err := domain.ParseOperatorConfig(nil); err != nil || string(none.JSON()) != `{}` {
		t.Fatalf("nil = %s, %v", none.JSON(), err)
	}
	full, err := domain.ParseOperatorConfig(map[string]any{
		"default_model": "vendor/model-1.5:fast@2", "supports_followups": false, "supports_components": true,
		"supports_voice": true, "supports_title": false, "supports_files_out": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"default_model":"vendor/model-1.5:fast@2","supports_followups":false,"supports_components":true,` +
		`"supports_voice":true,"supports_title":false,"supports_files_out":true}`
	if string(full.JSON()) != want {
		t.Fatalf("full = %s", full.JSON())
	}
	back, err := domain.OperatorConfigFromStore(full.JSON())
	if err != nil || string(back.JSON()) != want {
		t.Fatalf("from store = %s, %v", back.JSON(), err)
	}
	if !full.Allows(domain.CapabilityVoice) || full.Allows(domain.CapabilityFollowups) || !empty.Allows(domain.CapabilityTitle) {
		t.Fatal("a missing key must allow, a false key must forbid")
	}
}

// TestAdapterKindAndTexts holds the ruling on adapter_kind (^[a-z0-9][a-z0-9_-]*$), the
// description and the reason (SEC-IN-4, required reason).
func TestAdapterKindAndTexts(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"hermes", "code_runner", "custom", "a-1"} {
		if err := domain.CheckAdapterKind(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Hermes", "_x", "a b", "a.b"} {
		wantField(t, domain.CheckAdapterKind(bad), "/adapter_kind")
	}
	if err := domain.CheckDescription(""); err != nil {
		t.Fatal(err)
	}
	wantField(t, domain.CheckDescription("a\u202eb"), "/description")
	wantField(t, domain.CheckReason(""), "/reason")
	wantField(t, domain.CheckReason("   "), "/reason")
	wantField(t, domain.CheckReason("ok\x00"), "/reason")
	if err := domain.CheckReason("Team uses it"); err != nil {
		t.Fatal(err)
	}
}

// TestBillingPolicyTransitions is the table of 10 section 9: organization always, personal only
// with a holder, organization without a holder, the same target without change.
func TestBillingPolicyTransitions(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	org := domain.Policy{Mode: domain.Organization}
	personal := domain.Policy{Mode: domain.Personal, HolderID: alex}
	cases := []struct {
		name    string
		from    domain.Policy
		to      domain.Policy
		changed bool
		pointer string
	}{
		{"org to org", org, org, false, ""},
		{"org to personal", org, personal, true, ""},
		{"personal to same", personal, personal, false, ""},
		{"personal to other holder", personal, domain.Policy{Mode: domain.Personal, HolderID: sam}, true, ""},
		{"personal to org", personal, org, true, ""},
		{"personal without holder", org, domain.Policy{Mode: domain.Personal}, false, "/credential_holder_id"},
		{"org with holder", personal, domain.Policy{Mode: domain.Organization, HolderID: sam}, false, "/credential_holder_id"},
		{"unknown mode", org, domain.Policy{Mode: "shared"}, false, "/billing_mode"},
	}
	for _, c := range cases {
		changed, err := c.from.To(c.to)
		if c.pointer != "" {
			wantField(t, err, c.pointer)
			continue
		}
		if err != nil || changed != c.changed {
			t.Errorf("%s: changed %v, %v; want %v", c.name, changed, err, c.changed)
		}
	}
	a := domain.NewAgent(ids.New(), "hermes", "", domain.OperatorConfig{})
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	changed, err := a.SetPolicy(personal, sam, at)
	if err != nil || !changed || a.Version != 2 || a.Policy != personal || a.BillingModeChangedBy != sam || !a.BillingModeChangedAt.Equal(at) {
		t.Fatalf("SetPolicy = %+v, %v, %v", a, changed, err)
	}
	if changed, err := a.SetPolicy(personal, sam, at.Add(time.Hour)); err != nil || changed || a.Version != 2 {
		t.Fatalf("SetPolicy same = %v, %v, version %d", changed, err, a.Version)
	}
	if changed, err := a.SetHolder(sam); err != nil || !changed || a.Policy.HolderID != sam || a.Version != 3 {
		t.Fatalf("SetHolder = %v, %v, %+v", changed, err, a)
	}
	if changed, err := a.SetHolder(sam); err != nil || changed {
		t.Fatalf("SetHolder same = %v, %v", changed, err)
	}
	if _, err := a.SetPolicy(org, sam, at); err != nil || a.Policy.HolderID != (ids.ID{}) {
		t.Fatalf("to organization: %+v, %v", a.Policy, err)
	}
	_, err = a.SetHolder(sam)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict {
		t.Fatalf("SetHolder on organization = %v, want conflict", err)
	}
}

// TestOperatorConfigFromStore_IgnoresUnknownKeys (audit srv-agents-6, upgrade.md rule M8): a key a
// later release stored is ignored on read, so a rollback by one release keeps the lists of agents
// working; the known keys still count and a write stays strict.
func TestOperatorConfigFromStore_IgnoresUnknownKeys(t *testing.T) {
	t.Parallel()
	c, err := domain.OperatorConfigFromStore([]byte(`{"supports_title":false,"supports_future":true}`))
	if err != nil || c.Allows(domain.CapabilityTitle) || string(c.JSON()) != `{"supports_title":false}` {
		t.Fatalf("stored config %s, %v", c.JSON(), err)
	}
	if _, err := domain.ParseOperatorConfig(map[string]any{"supports_future": true}); err == nil {
		t.Fatal("a write accepted an unknown key")
	}
}
