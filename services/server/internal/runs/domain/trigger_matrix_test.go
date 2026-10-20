// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"fmt"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// TestDecide_AG02_SwitchMatrix is AG-02: every combination of the five switches of 09 3.3 x main
// thread and thread x the mention variants gives exactly the candidate of 09 3.4. The expected
// result is an oracle written here from the 03 channels "Mention rule" and the guest rule of
// 05 section 5 point 3 as plain boolean expressions; it calls no production helper.
func TestDecide_AG02_SwitchMatrix(t *testing.T) {
	noWrite := texter()
	noWrite.WriteOnChannel = false
	variants := []struct {
		name     string
		mentions []domain.AgentFacts
	}{
		{"none", nil},
		{"operator", []domain.AgentFacts{lotse()}},
		{"eligible guest", []domain.AgentFacts{texter()}},
		{"guest without write", []domain.AgentFacts{noWrite}},
		{"personal guest with write", []domain.AgentFacts{assistent()}},
		{"two agents", []domain.AgentFacts{lotse(), texter()}},
	}
	type expect struct {
		outcome   domain.Outcome
		agent     ids.ID
		guest     bool
		mentioned bool
		reason    domain.Reason
		newThread bool
	}
	count := 0
	for bits := 0; bits < 32; bits++ {
		sw := domain.Switches{
			RequireMention:       bits&1 != 0,
			ThreadRequireMention: bits&2 != 0,
			FreeResponse:         bits&4 != 0,
			AutoThread:           bits&8 != 0,
			AllowMentionedAgents: bits&16 != 0,
		}
		for _, main := range []bool{true, false} {
			for _, v := range variants {
				// The oracle.
				var want expect
				ruleMet := sw.FreeResponse || (main && !sw.RequireMention) || (!main && !sw.ThreadRequireMention)
				switch v.name {
				case "none":
					if ruleMet {
						want = expect{outcome: domain.OutcomeInvoke, agent: lotseID}
					}
				case "operator":
					want = expect{outcome: domain.OutcomeInvoke, agent: lotseID, mentioned: true}
				case "eligible guest":
					if sw.AllowMentionedAgents {
						want = expect{outcome: domain.OutcomeInvoke, agent: texterID, mentioned: true, guest: true}
					} else {
						want = expect{outcome: domain.OutcomeReject, reason: domain.ReasonAgentNotAllowedHere}
					}
				case "guest without write", "personal guest with write":
					want = expect{outcome: domain.OutcomeReject, reason: domain.ReasonAgentNotAllowedHere}
				case "two agents":
					want = expect{outcome: domain.OutcomeReject, reason: domain.ReasonMultipleAgentsMentioned}
				}
				want.newThread = want.outcome == domain.OutcomeInvoke && main && sw.AutoThread

				name := fmt.Sprintf("%+v/main=%v/%s", sw, main, v.name)
				t.Run(name, func(t *testing.T) {
					in := mainMsg()
					if !main {
						in = threadMsg()
					}
					in.Switches = sw
					in.Mentioned = v.mentions
					d := mustDecide(t, in)
					got := expect{outcome: d.Outcome, guest: d.Guest, mentioned: d.Mentioned, reason: d.Reason, newThread: d.NewThread != nil}
					if d.Agent != nil {
						got.agent = d.Agent.ID
					}
					if got != want {
						t.Fatalf("got %+v, want %+v", got, want)
					}
					if want.outcome == domain.OutcomeInvoke && d.Check != domain.CheckInvoke {
						t.Fatalf("invoke without CheckInvoke: %+v", d)
					}
				})
				count++
			}
		}
	}
	if count != 32*2*len(variants) {
		t.Fatalf("matrix covered %d cases", count)
	}
}
