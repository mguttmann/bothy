// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// computerRun is a message run of person by the host agent Runner, triggered by person's own
// message, as runs reads it for SR13 (CU-04).
func computerRun(run actingRun, agent, person ids.ID) *authorize.ComputerFacts {
	return &authorize.ComputerFacts{
		RunFound: true, RunID: run.id, RunAgentID: agent, TriggeredBy: person, RunActive: true, ToolClass: "host",
		TriggerKind: "message", TriggerOrigin: "human", TriggerAuthorID: person,
		Feeders:   []authorize.ComputerFeeder{{PrincipalID: person, Source: "trigger"}},
		HostState: "active", HostConnected: true, HostListed: true,
	}
}

// computerOpen is the descriptor of computer.open on host with the run facts f.
func computerOpen(host ids.ID, f *authorize.ComputerFacts) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceHost, ID: host, Computer: f}
}

// TestComputerOpen_SR13 (CU-T01 to CU-T05 in authz): the order of 27 5.1 step 5 through the
// Authorizer: the owner's own host run opens on any host; a run of a person who is no owner is
// computer_not_owner_run on a host the person reads and not_found on any other (R-AZ-01); a
// request without the binding, with an invalid binding or of an ended run is refused before any
// resource; a person is subject_kind_not_allowed; every denial is audited.
func TestComputerOpen_SR13(t *testing.T) {
	w := newHostWorld(t)
	run := w.newActingRun()
	owner := w.agentFor(w.runner, w.owner, run, actor.ActingForBound)
	w.allow(t, owner, authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.owner)))

	alex := w.agentFor(w.runner, w.alex, run, actor.ActingForBound)
	w.expect(t, alex, authorize.ActionComputerOpen, computerOpen(w.r1, computerRun(run, w.runner, w.alex)), forbidden, "computer_not_owner_run")
	w.expect(t, alex, authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.alex)), notFound, "no_grant")

	unbound := w.agent(w.runner)
	w.expect(t, unbound, authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.owner)), forbidden, "computer_not_owner_run")
	invalid := w.agentFor(w.runner, w.owner, run, actor.ActingForInvalid)
	w.expect(t, invalid, authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.owner)), forbidden, "invocation_token_invalid")
	ended := w.agentFor(w.runner, w.owner, run, actor.ActingForRunInactive)
	d := w.decide(t, context.Background(), ended, authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.owner)))
	if d.Reason != "invocation_token_invalid" || d.Detail != "run_inactive" {
		t.Fatalf("ended run: %+v", d)
	}
	w.expect(t, w.user(w.owner), authorize.ActionComputerOpen, computerOpen(w.r2, computerRun(run, w.runner, w.owner)), forbidden, "subject_kind_not_allowed")

	tool := computerRun(run, w.runner, w.owner)
	tool.ToolClass = "write"
	w.expect(t, owner, authorize.ActionComputerOpen, computerOpen(w.r2, tool), forbidden, "computer_tool_class")
	off := computerRun(run, w.runner, w.owner)
	off.HostConnected = false
	w.expect(t, owner, authorize.ActionComputerOpen, computerOpen(w.r2, off), forbidden, "host_not_active")
	unlisted := computerRun(run, w.runner, w.owner)
	unlisted.HostListed = false
	w.expect(t, owner, authorize.ActionComputerOpen, computerOpen(w.r2, unlisted), forbidden, "computer_not_enabled")
	w.expect(t, owner, authorize.ActionComputerOpen, computerOpen(ids.New(), computerRun(run, w.runner, w.owner)), notFound, "no_grant")
	if denials, _ := w.rec.counts(); denials < 9 {
		t.Fatalf("denials audited %d, want every refusal", denials)
	}
}

// TestComputerOpen_WithoutFactsIsAnError: a computer action without its facts never allows.
func TestComputerOpen_WithoutFactsIsAnError(t *testing.T) {
	w := newHostWorld(t)
	run := w.newActingRun()
	owner := w.agentFor(w.runner, w.owner, run, actor.ActingForBound)
	if _, err := w.az.Authorize(context.Background(), owner, authorize.ActionComputerOpen, host(w.r2)); err == nil {
		t.Fatal("a computer action without facts was decided")
	}
}

// TestComputerDesktopsAndAttach: the list decides on the run, the attach on the session's host,
// both with SR13.
func TestComputerDesktopsAndAttach(t *testing.T) {
	w := newHostWorld(t)
	run := w.newActingRun()
	owner := w.agentFor(w.runner, w.owner, run, actor.ActingForBound)
	list := authorize.Resource{Kind: authorize.ResourceRun, ID: run.id, ThreadID: run.thread, OwnerID: w.runner, Computer: computerRun(run, w.runner, w.owner)}
	w.allow(t, owner, authorize.ActionComputerDesktopsList, list)
	wake := computerRun(run, w.runner, w.owner)
	wake.TriggerKind = "wake"
	list.Computer = wake
	w.expect(t, owner, authorize.ActionComputerDesktopsList, list, forbidden, "computer_not_owner_run")

	att := authorize.Resource{Kind: authorize.ResourceComputerSession, ID: ids.New(), HostID: w.r2, OwnerID: w.owner, Computer: computerRun(run, w.runner, w.owner)}
	w.allow(t, owner, authorize.ActionComputerAttach, att)
	att.Computer.HostListed = false
	w.expect(t, owner, authorize.ActionComputerAttach, att, forbidden, "computer_not_enabled")
}

// TestComputerStopAndRead: stop for every active owner in a session, the read for the
// triggering owner; others do not see the session.
func TestComputerStopAndRead(t *testing.T) {
	w := newHostWorld(t)
	ses := authorize.Resource{Kind: authorize.ResourceComputerSession, ID: ids.New(), OwnerID: w.owner}
	w.allow(t, w.user(w.owner), authorize.ActionComputerStop, ses)
	w.allow(t, w.user(w.owner), authorize.ActionComputerSessionRead, ses)
	w.expect(t, w.user(w.alex), authorize.ActionComputerStop, ses, notFound, "not_self")
	w.expect(t, w.agent(w.runner), authorize.ActionComputerStop, ses, forbidden, "subject_kind_not_allowed")
	w.allow(t, w.user(w.owner), authorize.ActionComputerDeviceDisable, host(w.r1))
	w.expect(t, w.user(w.alex), authorize.ActionComputerDeviceDisable, host(w.r1), forbidden, "owner_only")
}
