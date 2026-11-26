// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// computerBase is an open of Alex's own message run on Alex's listed desktop: allowed.
func computerBase() domain.ComputerFacts {
	alex, run, agent := ids.New(), ids.New(), ids.New()
	return domain.ComputerFacts{
		Action: domain.ActionComputerOpen, AgentID: agent, BindingRunID: run, BindingUserID: alex,
		RunFound: true, RunID: run, RunAgentID: agent, TriggeredBy: alex, RunActive: true, ToolClass: "host",
		TriggerKind: "message", TriggerOrigin: "human", TriggerAuthorID: alex,
		Feeders:     []domain.ComputerFeeder{{PrincipalID: alex, Source: "trigger"}},
		PersonOwner: true, HostVisible: true, HostState: "active", HostConnected: true, HostListed: true,
	}
}

// TestDecideComputer holds the order and the reasons of SR13 (CU-T02 to CU-T05, CU-T25 refusals).
func TestDecideComputer(t *testing.T) {
	t.Parallel()
	sam := ids.New()
	cases := []struct {
		name    string
		mut     func(*domain.ComputerFacts)
		outcome domain.Outcome
		reason  domain.Reason
		detail  string
	}{
		{"allowed", func(*domain.ComputerFacts) {}, domain.OutcomeAllow, "", ""},
		{"no binding", func(f *domain.ComputerFacts) { f.BindingRunID = ids.ID{} }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"run ended", func(f *domain.ComputerFacts) { f.RunActive = false }, domain.OutcomeForbidden, domain.ReasonInvocationTokenInvalid, "run_inactive"},
		{"host invisible", func(f *domain.ComputerFacts) { f.HostVisible = false; f.PersonOwner = false }, domain.OutcomeNotFound, domain.ReasonNoGrant, ""},
		{"tool class write", func(f *domain.ComputerFacts) { f.ToolClass = "write" }, domain.OutcomeForbidden, domain.ReasonComputerToolClass, ""},
		{"not an owner", func(f *domain.ComputerFacts) { f.PersonOwner = false }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"wake run", func(f *domain.ComputerFacts) { f.TriggerKind = "wake" }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"team run", func(f *domain.ComputerFacts) { f.TriggerKind = "team" }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"interaction run", func(f *domain.ComputerFacts) { f.TriggerKind = "interaction" }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"origin agent", func(f *domain.ComputerFacts) { f.TriggerOrigin = "agent" }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"message of another author", func(f *domain.ComputerFacts) { f.TriggerAuthorID = sam }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"command without command", func(f *domain.ComputerFacts) { f.TriggerKind = "command" }, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"follow-up of another person", func(f *domain.ComputerFacts) {
			f.Feeders = append(f.Feeders, domain.ComputerFeeder{PrincipalID: sam, Source: "followup"})
		}, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"feeder NULL", func(f *domain.ComputerFacts) {
			f.Feeders = append(f.Feeders, domain.ComputerFeeder{Source: "interaction"})
		}, domain.OutcomeForbidden, domain.ReasonComputerNotOwnerRun, ""},
		{"context of another person counts not", func(f *domain.ComputerFacts) {
			f.Feeders = append(f.Feeders, domain.ComputerFeeder{PrincipalID: sam, Source: "context"}, domain.ComputerFeeder{Source: "context"})
		}, domain.OutcomeAllow, "", ""},
		{"host disabled", func(f *domain.ComputerFacts) { f.HostState = "disabled" }, domain.OutcomeForbidden, domain.ReasonHostNotActive, ""},
		{"host not connected", func(f *domain.ComputerFacts) { f.HostConnected = false }, domain.OutcomeForbidden, domain.ReasonHostNotActive, ""},
		{"not connected before not enabled", func(f *domain.ComputerFacts) { f.HostConnected = false; f.HostListed = false }, domain.OutcomeForbidden, domain.ReasonHostNotActive, ""},
		{"not listed", func(f *domain.ComputerFacts) { f.HostListed = false }, domain.OutcomeForbidden, domain.ReasonComputerNotEnabled, ""},
		{"tool class before owner run", func(f *domain.ComputerFacts) { f.ToolClass = "read"; f.PersonOwner = false }, domain.OutcomeForbidden, domain.ReasonComputerToolClass, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := computerBase()
			c.mut(&f)
			d, err := domain.DecideComputer(f)
			if err != nil {
				t.Fatal(err)
			}
			if d.Outcome != c.outcome || d.Reason != c.reason || d.Detail != c.detail {
				t.Fatalf("got %+v, want %s %s %s", d, c.outcome, c.reason, c.detail)
			}
		})
	}
}

// TestDecideComputer_ListIgnoresHost: computer.desktops.list decides without a host.
func TestDecideComputer_ListIgnoresHost(t *testing.T) {
	t.Parallel()
	f := computerBase()
	f.Action, f.HostVisible, f.HostListed, f.HostConnected = domain.ActionComputerDesktopsList, false, false, false
	if d, err := domain.DecideComputer(f); err != nil || d.Outcome != domain.OutcomeAllow {
		t.Fatalf("got %+v %v", d, err)
	}
}

// TestDecideComputer_ForeignRunIsInternal: facts of another run than the binding fail closed.
func TestDecideComputer_ForeignRunIsInternal(t *testing.T) {
	t.Parallel()
	f := computerBase()
	f.RunID = ids.New()
	if _, err := domain.DecideComputer(f); err == nil {
		t.Fatal("facts of another run passed")
	}
}

// TestIsComputerAgentAction names exactly the three agent actions.
func TestIsComputerAgentAction(t *testing.T) {
	t.Parallel()
	for _, a := range []string{"computer.desktops.list", "computer.open", "computer.attach"} {
		if !domain.IsComputerAgentAction(a) {
			t.Fatalf("%s is no agent action", a)
		}
	}
	for _, a := range []string{"computer.stop", "computer.session.read", "computer.device.disable", "terminal.open"} {
		if domain.IsComputerAgentAction(a) {
			t.Fatalf("%s is an agent action", a)
		}
	}
}
