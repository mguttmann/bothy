// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestDecideCommand is 09 10.4: a control command goes to the agent at once, a model command
// during a run is rejected with run_active.
func TestDecideCommand(t *testing.T) {
	base := func() domain.CommandTrigger {
		return domain.CommandTrigger{CommandID: commandID, ThreadID: threadT, Owner: lotse()}
	}
	for name, run := range map[string]*domain.ActiveRun{
		"no run":             nil,
		"run of the owner":   {RunID: runR, AgentID: lotseID},
		"run of other agent": {RunID: runR, AgentID: texterID},
	} {
		t.Run("control/"+name, func(t *testing.T) {
			in := base()
			in.Control, in.ActiveRun = true, run
			d, err := domain.DecideCommand(in)
			if err != nil {
				t.Fatal(err)
			}
			if d.Outcome != domain.OutcomeControl || d.Check != domain.CheckInvoke || d.Step != domain.StepCommand ||
				d.Agent == nil || d.Agent.ID != lotseID || !d.RunID.IsZero() || d.CommandID != commandID ||
				d.TriggerKind != domain.TriggerKindCommand {
				t.Fatalf("got %+v, want control with CheckInvoke and no follow-up", d)
			}
		})
		t.Run("model/"+name, func(t *testing.T) {
			in := base()
			in.ActiveRun = run
			d, err := domain.DecideCommand(in)
			if err != nil {
				t.Fatal(err)
			}
			if run != nil {
				if d.Outcome != domain.OutcomeReject || d.Reason != domain.ReasonRunActive || d.Check != domain.CheckNone || d.Agent != nil {
					t.Fatalf("got %+v, want reject run_active", d)
				}
				return
			}
			if d.Outcome != domain.OutcomeInvoke || d.Check != domain.CheckInvoke || d.TriggerKind != "command" ||
				d.CommandID != commandID || d.Agent == nil || d.Agent.ID != lotseID || !d.TriggerMessageID.IsZero() {
				t.Fatalf("got %+v, want invoke of the command", d)
			}
		})
	}
	invalid := map[string]func(*domain.CommandTrigger){
		"zero command":    func(in *domain.CommandTrigger) { in.CommandID = ids.ID{} },
		"zero thread":     func(in *domain.CommandTrigger) { in.ThreadID = ids.ID{} },
		"zero owner":      func(in *domain.CommandTrigger) { in.Owner.ID = ids.ID{} },
		"unknown billing": func(in *domain.CommandTrigger) { in.Owner.BillingMode = "" },
		"zero run id":     func(in *domain.CommandTrigger) { in.ActiveRun = &domain.ActiveRun{AgentID: lotseID} },
	}
	for name, mod := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			in := base()
			in.Control = true
			mod(&in)
			d, err := domain.DecideCommand(in)
			wantInternal(t, err)
			if d != (domain.Decision{}) {
				t.Fatalf("decision with error = %+v, want zero", d)
			}
		})
	}
}
