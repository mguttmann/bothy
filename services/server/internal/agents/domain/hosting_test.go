// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The rules of hosting of docs/spec/26-builtin-agents.md 3.1 and 10 section 10.3 (A1-03).

// wantFieldCode requires a validation_failed error with exactly one field error at pointer with code.
func wantFieldCode(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed %s %s", err, pointer, code)
	}
}

// wantReason requires a 409 conflict error with reason.
func wantReason(t *testing.T, err error, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeConflict || e.Reason != reason {
		t.Fatalf("error %v, want conflict %s", err, reason)
	}
}

// runner returns a capable host: an active Claude Code runner, not hosted, reporting ceiling.
func runner(ceiling domain.ToolClass) domain.Agent {
	a := domain.NewAgent(ids.New(), domain.AdapterCodeRunner, "", domain.OperatorConfig{})
	a.Reported.HostsAgents = ceiling
	return a
}

// TestCheckHostedPairing: hosted_by is set exactly when adapter_kind is claude_agent (R-DM-58,
// BA-T07); a missing one is required, one with another kind not_allowed, both on /hosted_by.
func TestCheckHostedPairing(t *testing.T) {
	t.Parallel()
	host := ids.New()
	if err := domain.CheckHostedPairing(domain.AdapterClaudeAgent, host); err != nil {
		t.Fatalf("claude_agent with a host: %v", err)
	}
	if err := domain.CheckHostedPairing("hermes", ids.ID{}); err != nil {
		t.Fatalf("hermes without a host: %v", err)
	}
	wantFieldCode(t, domain.CheckHostedPairing(domain.AdapterClaudeAgent, ids.ID{}), "/hosted_by", domain.FieldRequired)
	wantFieldCode(t, domain.CheckHostedPairing(domain.AdapterCodeRunner, host), "/hosted_by", domain.FieldNotAllowed)
}

// TestCheckHost: only an active, not hosted Claude Code runner other than the agent itself that
// reports hosts_agents can host (26 3.1 item 2); everything else is host_not_capable.
func TestCheckHost(t *testing.T) {
	t.Parallel()
	self := ids.New()
	good := runner(domain.ToolScoped)
	if err := domain.CheckHost(self, good, true, true); err != nil {
		t.Fatalf("capable runner: %v", err)
	}
	hosted := runner(domain.ToolScoped)
	hosted.HostedBy = ids.New()
	noCap := runner("")
	other := runner(domain.ToolScoped)
	other.AdapterKind = "hermes"
	itself := runner(domain.ToolScoped)
	itself.ID = self
	for name, c := range map[string]struct {
		host          domain.Agent
		found, active bool
	}{
		"unknown":               {good, false, true},
		"disabled":              {good, true, false},
		"hosted itself":         {hosted, true, true},
		"without hosts_agents":  {noCap, true, true},
		"not a runner":          {other, true, true},
		"the agent itself":      {itself, true, true},
		"unknown and not found": {domain.Agent{}, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wantFieldCode(t, domain.CheckHost(self, c.host, c.found, c.active), "/hosted_by", domain.FieldHostNotCapable)
		})
	}
}

// TestCheckCeiling: a hosted agent's class may not exceed the host's hosts_agents (26 3.1 item
// 3); a host that stopped reporting it admits only chat (D-A1-03-6).
func TestCheckCeiling(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		class, ceiling domain.ToolClass
		ok             bool
	}{
		{domain.ToolChat, domain.ToolChat, true},
		{domain.ToolScoped, domain.ToolChat, false},
		{domain.ToolScoped, domain.ToolScoped, true},
		{domain.ToolHost, domain.ToolScoped, false},
		{domain.ToolHost, domain.ToolHost, true},
		{domain.ToolChat, "", true},
		{domain.ToolScoped, "", false},
	} {
		err := domain.CheckCeiling(c.class, c.ceiling)
		if c.ok && err != nil {
			t.Errorf("%s under %q: %v", c.class, c.ceiling, err)
		}
		if !c.ok {
			wantReason(t, err, domain.ReasonHostToolClassCeiling)
		}
	}
}

// TestCheckHostedHolder: BR-14, the hosted side. personal needs a personal runner with the same
// holder that reports personal; organization is free.
func TestCheckHostedHolder(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	host := runner(domain.ToolScoped)
	host.Policy = domain.Policy{Mode: domain.Personal, HolderID: alex}
	host.Reported.BillingModesSupported = []domain.BillingMode{domain.Personal, domain.Organization}
	if err := domain.CheckHostedHolder(domain.Policy{Mode: domain.Personal, HolderID: alex}, host); err != nil {
		t.Fatalf("same holder: %v", err)
	}
	if err := domain.CheckHostedHolder(domain.Policy{Mode: domain.Organization}, runner(domain.ToolChat)); err != nil {
		t.Fatalf("organization: %v", err)
	}
	wantReason(t, domain.CheckHostedHolder(domain.Policy{Mode: domain.Personal, HolderID: sam}, host), domain.ReasonHostedHolderMismatch)
	org := runner(domain.ToolScoped)
	wantReason(t, domain.CheckHostedHolder(domain.Policy{Mode: domain.Personal, HolderID: alex}, org), domain.ReasonHostedHolderMismatch)
	silent := host
	silent.Reported.BillingModesSupported = []domain.BillingMode{domain.Organization}
	wantReason(t, domain.CheckHostedHolder(domain.Policy{Mode: domain.Personal, HolderID: alex}, silent), domain.ReasonHostedHolderMismatch)
}

// TestCheckRunnerHolders: BR-14, the runner side. A runner's own mode or holder may not leave a
// personal hosted agent with another holder.
func TestCheckRunnerHolders(t *testing.T) {
	t.Parallel()
	alex, sam := ids.New(), ids.New()
	hostedPersonal := domain.NewAgent(ids.New(), domain.AdapterClaudeAgent, "", domain.OperatorConfig{})
	hostedPersonal.Policy = domain.Policy{Mode: domain.Personal, HolderID: alex}
	hostedOrg := domain.NewAgent(ids.New(), domain.AdapterClaudeAgent, "", domain.OperatorConfig{})
	if err := domain.CheckRunnerHolders(domain.Policy{Mode: domain.Personal, HolderID: alex}, []domain.Agent{hostedPersonal, hostedOrg}); err != nil {
		t.Fatalf("same holder: %v", err)
	}
	if err := domain.CheckRunnerHolders(domain.Policy{Mode: domain.Organization}, []domain.Agent{hostedOrg}); err != nil {
		t.Fatalf("only organization agents: %v", err)
	}
	wantReason(t, domain.CheckRunnerHolders(domain.Policy{Mode: domain.Organization}, []domain.Agent{hostedPersonal}), domain.ReasonHostedHolderMismatch)
	wantReason(t, domain.CheckRunnerHolders(domain.Policy{Mode: domain.Personal, HolderID: sam}, []domain.Agent{hostedPersonal}),
		domain.ReasonHostedHolderMismatch)
}

// TestSetHost: a new runner bumps the version once; the same runner changes nothing.
func TestSetHost(t *testing.T) {
	t.Parallel()
	a := domain.NewAgent(ids.New(), domain.AdapterClaudeAgent, "", domain.OperatorConfig{})
	h := ids.New()
	if !a.SetHost(h) || a.HostedBy != h || a.Version != 2 {
		t.Fatalf("first host: %+v", a)
	}
	if a.SetHost(h) || a.Version != 2 {
		t.Fatalf("same host: %+v", a)
	}
}

// TestDecodeHeartbeatHostsAgents: details.capabilities.hosts_agents takes a tool class (09 2.2,
// 26 3.1) and reaches the stored form; any other value is invalid_enum.
func TestDecodeHeartbeatHostsAgents(t *testing.T) {
	t.Parallel()
	r, fes := domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"hosts_agents":"scoped"}}}`))
	if len(fes) > 0 {
		t.Fatalf("errors %+v", fes)
	}
	if got := r.Reported().HostsAgents; got != domain.ToolScoped {
		t.Fatalf("reported hosts_agents %q", got)
	}
	back, err := domain.ReportFromStore(domain.RuntimeOnline, "", "", "", r.StoredDetails())
	if err != nil || back.Reported().HostsAgents != domain.ToolScoped {
		t.Fatalf("stored form %s: %+v %v", r.StoredDetails(), back.Reported(), err)
	}
	_, fes = domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"hosts_agents":"root"}}}`))
	if !hasError(fes, "/details/capabilities/hosts_agents", "") || len(fes) != 1 {
		t.Fatalf("errors %+v", fes)
	}
	plain, _ := domain.DecodeHeartbeat([]byte(`{"state":"online","details":{"capabilities":{"title":true}}}`))
	if plain.Reported().HostsAgents != "" {
		t.Fatalf("hosts_agents without a report: %q", plain.Reported().HostsAgents)
	}
}
