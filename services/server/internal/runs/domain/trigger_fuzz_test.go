// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// fuzzTrigger builds a MessageTrigger from the fuzz input. Every enum has one value outside its
// list, so the fuzzer also reaches the validation.
func fuzzTrigger(origin, chKind, thKind, sw, ops, mention, flags, voice uint8) domain.MessageTrigger {
	origins := []domain.Origin{domain.OriginHuman, domain.OriginAgent, domain.OriginApp, domain.OriginImport, domain.OriginSystem, "bogus"}
	chKinds := []domain.ChannelKind{domain.ChannelText, domain.ChannelForum, domain.ChannelDM, "bogus"}
	thKinds := []domain.ThreadKind{domain.ThreadMain, domain.ThreadThread, "bogus"}
	modes := []domain.VoiceMode{domain.VoiceOff, domain.VoiceAgent, domain.VoiceService, "bogus"}

	tex := texter()
	tex.WriteOnChannel = flags&0x20 != 0
	robot := domain.AgentFacts{ID: robotID, Active: flags&0x80 == 0, BillingMode: domain.BillingOrganization, WriteOnChannel: flags&0x80 != 0}
	if flags&0x40 != 0 {
		robot.BillingMode = domain.BillingPersonal
	}
	pool := []domain.AgentFacts{lotse(), tex, assistent(), robot}
	for i := range pool {
		pool[i].VoiceInput = voice&0x10 != 0
	}

	in := domain.MessageTrigger{
		MessageID: msgM, ThreadID: threadT, ChannelID: channelC,
		Origin:      origins[int(origin)%len(origins)],
		ChannelKind: chKinds[int(chKind)%len(chKinds)],
		ThreadKind:  thKinds[int(thKind)%len(thKinds)],
		Switches: domain.Switches{
			RequireMention: sw&1 != 0, ThreadRequireMention: sw&2 != 0, FreeResponse: sw&4 != 0,
			AutoThread: sw&8 != 0, AllowMentionedAgents: sw&16 != 0,
		},
		AuthorHasTrigger: flags&1 != 0,
		Voice:            flags&2 != 0,
		VoiceMode:        modes[int(voice)%len(modes)],
	}
	if ops&1 != 0 {
		in.ThreadOperator = ptr(pool[(ops>>2)&3])
	}
	if ops&2 != 0 {
		in.ChannelOperator = ptr(pool[(ops>>4)&3])
	}
	for i := range pool {
		if mention&(1<<i) != 0 {
			in.Mentioned = append(in.Mentioned, pool[i])
		}
	}
	if mention&0x10 != 0 && len(in.Mentioned) > 0 {
		in.Mentioned = append(in.Mentioned, in.Mentioned[0])
	}
	if flags&4 != 0 {
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: pool[(flags>>3)&3].ID}
	}
	return in
}

// cloneTrigger deep-copies a MessageTrigger, so the fuzz test can see a mutated input.
func cloneTrigger(in domain.MessageTrigger) domain.MessageTrigger {
	out := in
	if in.ThreadOperator != nil {
		out.ThreadOperator = ptr(*in.ThreadOperator)
	}
	if in.ChannelOperator != nil {
		out.ChannelOperator = ptr(*in.ChannelOperator)
	}
	if in.ActiveRun != nil {
		r := *in.ActiveRun
		out.ActiveRun = &r
	}
	out.Mentioned = append([]domain.AgentFacts(nil), in.Mentioned...)
	return out
}

// FuzzDecide checks the invariants of Decide and Settle over generated inputs. The seed corpus
// mirrors the table cases (operator, guest, auto thread, active runs, voice, non-human origin).
func FuzzDecide(f *testing.F) {
	// origin, channel kind, thread kind, switches, operators, mentions, flags, voice
	seeds := [][8]uint8{
		{0, 0, 0, 0x09, 0x02, 0x01, 0x01, 0},    // main, operator mentioned, auto thread
		{0, 0, 1, 0x00, 0x03, 0x00, 0x01, 0},    // thread, operator without mention
		{0, 0, 1, 0x10, 0x03, 0x02, 0x21, 0},    // thread, eligible guest
		{0, 0, 1, 0x10, 0x03, 0x04, 0x01, 0},    // personal guest
		{0, 0, 1, 0x00, 0x03, 0x03, 0x01, 0},    // two agents
		{0, 0, 1, 0x10, 0x03, 0x02, 0x25, 0},    // guest during the operator's run
		{0, 0, 1, 0x00, 0x03, 0x00, 0x05, 0},    // operator follow-up
		{0, 0, 1, 0x00, 0x03, 0x00, 0x03, 0x11}, // voice, agent mode, voice_input
		{0, 0, 1, 0x00, 0x03, 0x00, 0x03, 0},    // voice, off
		{1, 0, 0, 0x04, 0x02, 0x01, 0x01, 0},    // agent origin, free response
		{2, 0, 0, 0x04, 0x02, 0x00, 0x01, 0},    // app origin, free response
		{0, 2, 0, 0x04, 0x00, 0x01, 0x01, 0},    // DM
		{0, 0, 0, 0x04, 0x00, 0x00, 0x00, 0},    // free response, no trigger right
		{5, 3, 2, 0x1f, 0xff, 0x1f, 0xff, 3},    // invalid enums
	}
	for _, s := range seeds {
		f.Add(s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7])
	}
	codes := map[domain.Reason]bool{}
	for _, c := range []domain.Reason{
		domain.ReasonMultipleAgentsMentioned, domain.ReasonAgentNotAllowedHere, domain.ReasonNoTriggerRight,
		domain.ReasonRunActiveOtherAgent, domain.ReasonAgentDisabled, domain.ReasonBillingModePersonalNotHolder,
		domain.ReasonProjectNotReady, domain.ReasonVoiceUnsupported, domain.ReasonRunActive,
	} {
		codes[c] = true
	}
	f.Fuzz(func(t *testing.T, origin, chKind, thKind, sw, ops, mention, flags, voice uint8) {
		in := fuzzTrigger(origin, chKind, thKind, sw, ops, mention, flags, voice)
		orig := cloneTrigger(in)
		d, err := domain.Decide(in)
		if !reflect.DeepEqual(in, orig) {
			t.Fatalf("Decide mutated its input")
		}
		if err != nil {
			if d != (domain.Decision{}) {
				t.Fatalf("error with a non-zero decision %+v", d)
			}
			return
		}
		d2, err2 := domain.Decide(cloneTrigger(orig))
		if err2 != nil || !reflect.DeepEqual(d, d2) {
			t.Fatalf("Decide is not deterministic: %+v vs %+v (%v)", d, d2, err2)
		}

		if (in.Origin != domain.OriginHuman || in.ChannelKind == domain.ChannelDM) && d.Outcome != domain.OutcomeNone {
			t.Fatalf("non-human or DM triggered: %+v", d)
		}
		if (d.Reason != "") != (d.Outcome == domain.OutcomeReject) {
			t.Fatalf("reason without reject or reject without reason: %+v", d)
		}
		if d.Reason != "" && !codes[d.Reason] {
			t.Fatalf("reason %q is not a code of 09 3.5", d.Reason)
		}
		switch d.Outcome {
		case domain.OutcomeInvoke, domain.OutcomeFollowUp:
			if d.Check == domain.CheckNone || d.Agent == nil {
				t.Fatalf("%v without check or candidate: %+v", d.Outcome, d)
			}
			op := in.ChannelOperator
			if in.ThreadKind == domain.ThreadThread {
				op = in.ThreadOperator
			}
			isOp := op != nil && op.ID == d.Agent.ID
			onlyMentioned := len(in.Mentioned) > 0
			for _, m := range in.Mentioned {
				if m.ID != d.Agent.ID {
					onlyMentioned = false
				}
			}
			if !isOp && !onlyMentioned {
				t.Fatalf("candidate %v is neither the operator nor the only mention", d.Agent.ID)
			}
			if d.Guest && (d.Agent.BillingMode != domain.BillingOrganization || !d.Agent.WriteOnChannel || !in.Switches.AllowMentionedAgents || isOp) {
				t.Fatalf("guest breaks the guest rule: %+v", d)
			}
		case domain.OutcomeNone, domain.OutcomeReject:
			if d.Check != domain.CheckNone || d.Agent != nil {
				t.Fatalf("%v with a check or candidate: %+v", d.Outcome, d)
			}
		case domain.OutcomeControl:
			t.Fatalf("Decide gave outcome %v", d.Outcome)
		default:
			t.Fatalf("Decide gave outcome %v", d.Outcome)
		}
		if d.NewThread != nil && (in.ThreadKind != domain.ThreadMain || !in.Switches.AutoThread) {
			t.Fatalf("new thread outside the main thread with auto_thread: %+v", d)
		}
		if d.Outcome == domain.OutcomeFollowUp && (in.ActiveRun == nil || in.ActiveRun.AgentID != d.Agent.ID || d.RunID != in.ActiveRun.RunID) {
			t.Fatalf("follow-up to a run that is not the candidate's: %+v", d)
		}
		for _, deny := range []string{"agent_not_reachable", "no_grant", "level_insufficient", "project_not_ready"} {
			s, err := domain.Settle(d, domain.Verdict{Reason: deny})
			if err != nil {
				t.Fatalf("Settle: %v", err)
			}
			if s.State == domain.TriggerStateReceived || s.Accepted {
				t.Fatalf("a denial was accepted: %+v", s)
			}
		}
	})
}
