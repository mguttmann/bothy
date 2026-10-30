// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// outsideMsg is Alex's message in a thread whose fixed operator Lotse is no longer allowed in the
// channel (the channel has no operator now, and Lotse never completed a run in the thread).
func outsideMsg() domain.MessageTrigger {
	in := threadMsg()
	in.ChannelOperator = nil
	in.ThreadOperatorOutside = true
	return in
}

// TestDecide_ThreadOperatorOutsideRejects is the trigger rule of the addendum of 2026-10-30 (09
// 3.4 step 4a): a fixed thread operator that is no longer allowed in the channel never gets a
// run; the message carries operator_not_in_channel instead, mentioned or not.
func TestDecide_ThreadOperatorOutsideRejects(t *testing.T) {
	wantReject(t, mustDecide(t, outsideMsg()), domain.StepThreadOperator, domain.ReasonOperatorNotInChannel)

	mentioned := outsideMsg()
	mentioned.Mentioned = []domain.AgentFacts{lotse()}
	wantReject(t, mustDecide(t, mentioned), domain.StepThreadOperator, domain.ReasonOperatorNotInChannel)

	// Another channel operator now does not take the thread over (R-DM-12): still a rejection.
	other := outsideMsg()
	other.ChannelOperator = ptr(texter())
	wantReject(t, mustDecide(t, other), domain.StepThreadOperator, domain.ReasonOperatorNotInChannel)
}

// TestDecide_ThreadOperatorOutsideKeepsOtherRules: the rule applies only when the operator is the
// candidate; a person without trigger who did not mention it stays without answer (R-AG-05), a
// guest is decided as before, and a message that triggers nobody stays without trigger.
func TestDecide_ThreadOperatorOutsideKeepsOtherRules(t *testing.T) {
	silent := outsideMsg()
	silent.AuthorHasTrigger = false
	wantNone(t, mustDecide(t, silent), domain.StepTriggerRight)

	noRight := outsideMsg()
	noRight.AuthorHasTrigger = false
	noRight.Mentioned = []domain.AgentFacts{lotse()}
	wantReject(t, mustDecide(t, noRight), domain.StepTriggerRight, domain.ReasonNoTriggerRight)

	guest := outsideMsg()
	guest.Switches.AllowMentionedAgents = true
	guest.Mentioned = []domain.AgentFacts{texter()}
	wantInvoke(t, mustDecide(t, guest), texterID, true, true)

	quiet := outsideMsg()
	quiet.Switches.ThreadRequireMention = true
	wantNone(t, mustDecide(t, quiet), domain.StepMentions)

	allowed := threadMsg()
	allowed.ThreadOperatorOutside = false
	wantInvoke(t, mustDecide(t, allowed), lotseID, false, false)
}

// TestDecide_ThreadOperatorOutsideNeedsAThreadOperator: the flag on a main thread or a thread
// without operator is invalid input (D-M2-04-5).
func TestDecide_ThreadOperatorOutsideNeedsAThreadOperator(t *testing.T) {
	main := mainMsg()
	main.ThreadOperatorOutside = true
	_, err := domain.Decide(main)
	wantInternal(t, err)

	none := threadMsg()
	none.ThreadOperator = nil
	none.ThreadOperatorOutside = true
	_, err = domain.Decide(none)
	wantInternal(t, err)
}
