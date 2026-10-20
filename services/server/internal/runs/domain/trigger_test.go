// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The world of 07 10 "Examples" as facts: space Support with operator Lotse (organization),
// Texter (organization) and Assistent (personal), both with write on Support/Allgemein in the
// guest cases. Alex has trigger on Support (AuthorHasTrigger), Sam only write.
var (
	msgM      = ids.ID{0x0a, 1}
	threadT   = ids.ID{0x0b, 1}
	channelC  = ids.ID{0x0c, 1}
	runR      = ids.ID{0x0d, 1}
	lotseID   = ids.ID{0x01, 1}
	texterID  = ids.ID{0x02, 1}
	assistID  = ids.ID{0x03, 1}
	robotID   = ids.ID{0x04, 1}
	commandID = ids.ID{0x0e, 1}
)

func lotse() domain.AgentFacts {
	return domain.AgentFacts{ID: lotseID, Active: true, BillingMode: domain.BillingOrganization, WriteOnChannel: true}
}

func texter() domain.AgentFacts {
	return domain.AgentFacts{ID: texterID, Active: true, BillingMode: domain.BillingOrganization, WriteOnChannel: true}
}

func assistent() domain.AgentFacts {
	return domain.AgentFacts{ID: assistID, Active: true, BillingMode: domain.BillingPersonal, WriteOnChannel: true}
}

func ptr(a domain.AgentFacts) *domain.AgentFacts { return &a }

// mainMsg is Alex's message in the main thread of Support/Allgemein with Lotse as the channel's
// operator and the default switches of 09 3.3.
func mainMsg() domain.MessageTrigger {
	return domain.MessageTrigger{
		MessageID: msgM, ThreadID: threadT, ChannelID: channelC,
		Origin: domain.OriginHuman, ChannelKind: domain.ChannelText, ThreadKind: domain.ThreadMain,
		ChannelOperator:  ptr(lotse()),
		Switches:         domain.Switches{RequireMention: true, AutoThread: true},
		AuthorHasTrigger: true,
		VoiceMode:        domain.VoiceOff,
	}
}

// threadMsg is Alex's message in a thread with the fixed operator Lotse.
func threadMsg() domain.MessageTrigger {
	in := mainMsg()
	in.ThreadKind = domain.ThreadThread
	in.ThreadOperator = ptr(lotse())
	return in
}

func mustDecide(t *testing.T, in domain.MessageTrigger) domain.Decision {
	t.Helper()
	d, err := domain.Decide(in)
	if err != nil {
		t.Fatalf("Decide: unexpected error %v", err)
	}
	return d
}

func wantReject(t *testing.T, d domain.Decision, step domain.Step, r domain.Reason) {
	t.Helper()
	if d.Outcome != domain.OutcomeReject || d.Step != step || d.Reason != r {
		t.Fatalf("got %v/%q/%q, want reject/%q/%q", d.Outcome, d.Step, d.Reason, step, r)
	}
	if d.Check != domain.CheckNone || d.Agent != nil || d.NewThread != nil || !d.RunID.IsZero() {
		t.Fatalf("a rejection carries no check, candidate, thread or run: %+v", d)
	}
	if d.TriggerMessageID != msgM || d.TriggerKind != domain.TriggerKindMessage {
		t.Fatalf("rejection not on M: %+v", d)
	}
}

func wantNone(t *testing.T, d domain.Decision, step domain.Step) {
	t.Helper()
	if d.Outcome != domain.OutcomeNone || d.Step != step || d.Reason != "" || d.Check != domain.CheckNone || d.Agent != nil {
		t.Fatalf("got %+v, want none at %q without reason and check", d, step)
	}
}

func wantInvoke(t *testing.T, d domain.Decision, agent ids.ID, mentioned, guest bool) {
	t.Helper()
	if d.Outcome != domain.OutcomeInvoke || d.Step != domain.StepInvokeCheck || d.Check != domain.CheckInvoke || d.Reason != "" {
		t.Fatalf("got %+v, want invoke with CheckInvoke", d)
	}
	if d.Agent == nil || d.Agent.ID != agent || d.Mentioned != mentioned || d.Guest != guest {
		t.Fatalf("candidate %+v mentioned=%v guest=%v, want %v/%v/%v", d.Agent, d.Mentioned, d.Guest, agent, mentioned, guest)
	}
	if d.TriggerMessageID != msgM || d.TriggerKind != domain.TriggerKindMessage {
		t.Fatalf("invoke not on M: %+v", d)
	}
}

// TestDecide_AG01_NonHumanOriginNeverTriggers is AG-01: a message with origin agent, app, import
// or system in a free-response channel gives no run, with and without mentions. It mirrors AZ-M30
// (Lotse mentions Texter with origin agent) and AZ-M53 (app origin, free_response).
func TestDecide_AG01_NonHumanOriginNeverTriggers(t *testing.T) {
	origins := []domain.Origin{domain.OriginAgent, domain.OriginApp, domain.OriginImport, domain.OriginSystem}
	mentions := map[string][]domain.AgentFacts{
		"no mention":   nil,
		"lotse":        {lotse()},
		"two agents":   {lotse(), texter()},
		"texter guest": {texter()},
	}
	for _, o := range origins {
		for name, m := range mentions {
			t.Run(string(o)+"/"+name, func(t *testing.T) {
				in := mainMsg()
				in.Origin = o
				in.Switches.FreeResponse, in.Switches.AllowMentionedAgents = true, true
				in.Mentioned = m
				wantNone(t, mustDecide(t, in), domain.StepOrigin)
			})
		}
	}
}

// TestDecide_DirectMessageNeverTriggers is 09 3.4 step 2: DM channels have no agents.
func TestDecide_DirectMessageNeverTriggers(t *testing.T) {
	in := mainMsg()
	in.ChannelKind, in.ChannelOperator = domain.ChannelDM, nil
	in.Switches.FreeResponse = true
	in.Mentioned = []domain.AgentFacts{texter()}
	wantNone(t, mustDecide(t, in), domain.StepDirectMessage)
}

// TestDecide_AG03_MultipleAgentsMentioned is AG-03: more than one agent gives
// multiple_agents_mentioned, before the trigger right; one agent twice counts once.
func TestDecide_AG03_MultipleAgentsMentioned(t *testing.T) {
	cases := map[string]func(*domain.MessageTrigger){
		"operator and guest": func(in *domain.MessageTrigger) { in.Mentioned = []domain.AgentFacts{lotse(), texter()} },
		"two guests":         func(in *domain.MessageTrigger) { in.Mentioned = []domain.AgentFacts{texter(), assistent()} },
		"author without trigger": func(in *domain.MessageTrigger) {
			in.Mentioned = []domain.AgentFacts{lotse(), texter()}
			in.AuthorHasTrigger = false
		},
		"in a thread": func(in *domain.MessageTrigger) {
			*in = threadMsg()
			in.Mentioned = []domain.AgentFacts{texter(), lotse(), texter()}
		},
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			in := mainMsg()
			in.Switches.AllowMentionedAgents = true
			mod(&in)
			wantReject(t, mustDecide(t, in), domain.StepMentions, domain.ReasonMultipleAgentsMentioned)
		})
	}
	t.Run("same agent twice", func(t *testing.T) {
		in := mainMsg()
		in.Mentioned = []domain.AgentFacts{lotse(), lotse()}
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, true, false)
	})
}

// TestDecide_AG04_GuestNotAllowed is AG-04: a guest without allow_mentioned_agents or without
// write gives agent_not_allowed_here; so does a personal agent (R-AG-22) and the channel's current
// operator in a thread with another fixed operator (R-DM-12); step 3 comes before step 4.
func TestDecide_AG04_GuestNotAllowed(t *testing.T) {
	noWrite := texter()
	noWrite.WriteOnChannel = false
	cases := []struct {
		name  string
		build func() domain.MessageTrigger
	}{
		{"guests not allowed", func() domain.MessageTrigger {
			in := mainMsg()
			in.Mentioned = []domain.AgentFacts{texter()}
			return in
		}},
		{"guest without write", func() domain.MessageTrigger {
			in := mainMsg()
			in.Switches.AllowMentionedAgents = true
			in.Mentioned = []domain.AgentFacts{noWrite}
			return in
		}},
		{"personal agent with write", func() domain.MessageTrigger {
			in := mainMsg()
			in.Switches.AllowMentionedAgents = true
			in.Mentioned = []domain.AgentFacts{assistent()}
			return in
		}},
		{"current channel operator in a thread of another operator", func() domain.MessageTrigger {
			in := threadMsg()
			in.ThreadOperator = ptr(texter())
			in.Mentioned = []domain.AgentFacts{lotse()}
			return in
		}},
		{"author without trigger", func() domain.MessageTrigger {
			in := mainMsg()
			in.AuthorHasTrigger = false
			in.Mentioned = []domain.AgentFacts{texter()}
			return in
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantReject(t, mustDecide(t, c.build()), domain.StepMentions, domain.ReasonAgentNotAllowedHere)
		})
	}
}

// TestDecide_AG05_AutoThread is AG-05: a trigger in the main thread with auto_thread creates a new
// thread, the run goes there, trigger_state stays on M (09 3.7).
func TestDecide_AG05_AutoThread(t *testing.T) {
	t.Run("operator mentioned", func(t *testing.T) {
		in := mainMsg()
		in.Mentioned = []domain.AgentFacts{lotse()}
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, true, false)
		want := domain.NewThread{ChannelID: channelC, ParentMessageID: msgM, OperatorAgentID: lotseID, Title: "", TitleSource: "auto"}
		if d.NewThread == nil || *d.NewThread != want {
			t.Fatalf("NewThread = %+v, want %+v", d.NewThread, want)
		}
	})
	t.Run("no operator, guest mentioned", func(t *testing.T) {
		in := mainMsg()
		in.ChannelOperator = nil
		in.Switches.AllowMentionedAgents = true
		in.Mentioned = []domain.AgentFacts{texter()}
		d := mustDecide(t, in)
		wantInvoke(t, d, texterID, true, true)
		if d.NewThread == nil || !d.NewThread.OperatorAgentID.IsZero() || d.NewThread.ParentMessageID != msgM {
			t.Fatalf("NewThread = %+v, want one without operator", d.NewThread)
		}
	})
	t.Run("main thread without auto_thread", func(t *testing.T) {
		in := mainMsg()
		in.Switches.AutoThread = false
		in.Mentioned = []domain.AgentFacts{lotse()}
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, true, false)
		if d.NewThread != nil {
			t.Fatalf("NewThread = %+v, want the thread of M", d.NewThread)
		}
	})
	t.Run("thread with auto_thread", func(t *testing.T) {
		in := threadMsg()
		in.Mentioned = []domain.AgentFacts{lotse()}
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, true, false)
		if d.NewThread != nil {
			t.Fatalf("NewThread = %+v, want the thread of M", d.NewThread)
		}
	})
	t.Run("active run of another agent in the main thread", func(t *testing.T) {
		in := mainMsg()
		in.Mentioned = []domain.AgentFacts{lotse()}
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: texterID}
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, true, false)
		if d.NewThread == nil {
			t.Fatal("want the new thread")
		}
	})
}

// TestDecide_AG06_ActiveRunOtherAgent is AG-06: a guest mentioned during the operator's run gives
// run_active_other_agent; a run of the candidate takes M as a follow-up.
func TestDecide_AG06_ActiveRunOtherAgent(t *testing.T) {
	guestCase := func() domain.MessageTrigger {
		in := threadMsg()
		in.Switches.AllowMentionedAgents = true
		in.Mentioned = []domain.AgentFacts{texter()}
		return in
	}
	t.Run("guest during the operator's run", func(t *testing.T) {
		in := guestCase()
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		wantReject(t, mustDecide(t, in), domain.StepActiveRun, domain.ReasonRunActiveOtherAgent)
	})
	t.Run("main thread without auto_thread during another agent's run", func(t *testing.T) {
		in := mainMsg()
		in.Switches.AutoThread = false
		in.Mentioned = []domain.AgentFacts{lotse()}
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: texterID}
		wantReject(t, mustDecide(t, in), domain.StepActiveRun, domain.ReasonRunActiveOtherAgent)
	})
	t.Run("operator during a guest run", func(t *testing.T) {
		in := threadMsg()
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: texterID}
		wantReject(t, mustDecide(t, in), domain.StepActiveRun, domain.ReasonRunActiveOtherAgent)
	})
	followup := func(t *testing.T, in domain.MessageTrigger, agent ids.ID, guest bool) {
		t.Helper()
		d := mustDecide(t, in)
		if d.Outcome != domain.OutcomeFollowUp || d.Step != domain.StepActiveRun || d.RunID != runR || d.Check != domain.CheckFollowUp {
			t.Fatalf("got %+v, want followup to %v with CheckFollowUp", d, runR)
		}
		if d.Agent == nil || d.Agent.ID != agent || d.Guest != guest || d.Reason != "" || d.NewThread != nil {
			t.Fatalf("followup candidate %+v guest=%v", d.Agent, d.Guest)
		}
	}
	t.Run("operator during its own run", func(t *testing.T) {
		in := threadMsg()
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		followup(t, in, lotseID, false)
	})
	t.Run("guest during its own guest run", func(t *testing.T) {
		in := guestCase()
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: texterID}
		followup(t, in, texterID, true)
	})
}

// TestDecide_GuestRule is the guest rule of 09 3.6 and 05 section 5 point 3 as a table test: a
// guest exactly for allowed, write and organization when the agent is not the operator.
func TestDecide_GuestRule(t *testing.T) {
	for _, allow := range []bool{false, true} {
		for _, write := range []bool{false, true} {
			for _, billing := range []domain.BillingMode{domain.BillingOrganization, domain.BillingPersonal} {
				for _, isOp := range []bool{false, true} {
					a := domain.AgentFacts{ID: robotID, Active: true, BillingMode: billing, WriteOnChannel: write}
					in := threadMsg()
					if isOp {
						in.ThreadOperator = ptr(a)
					}
					in.Switches.AllowMentionedAgents = allow
					in.Mentioned = []domain.AgentFacts{a}
					d := mustDecide(t, in)
					switch {
					case isOp:
						wantInvoke(t, d, robotID, true, false)
					case allow && write && billing == domain.BillingOrganization:
						wantInvoke(t, d, robotID, true, true)
					default:
						wantReject(t, d, domain.StepMentions, domain.ReasonAgentNotAllowedHere)
					}
				}
			}
		}
	}
	t.Run("after a guest run the operator answers without mention", func(t *testing.T) {
		in := threadMsg()
		in.Switches.AllowMentionedAgents = true
		wantInvoke(t, mustDecide(t, in), lotseID, false, false)
	})
}

// TestDecide_TriggerRight is R-AG-05: without trigger only an explicit mention is rejected.
func TestDecide_TriggerRight(t *testing.T) {
	t.Run("mention without trigger", func(t *testing.T) {
		in := mainMsg()
		in.AuthorHasTrigger = false
		in.Mentioned = []domain.AgentFacts{lotse()}
		wantReject(t, mustDecide(t, in), domain.StepTriggerRight, domain.ReasonNoTriggerRight)
	})
	t.Run("free response without trigger", func(t *testing.T) {
		in := mainMsg()
		in.AuthorHasTrigger = false
		in.Switches.FreeResponse = true
		wantNone(t, mustDecide(t, in), domain.StepTriggerRight)
	})
	t.Run("without trigger during the operator's run", func(t *testing.T) {
		in := threadMsg()
		in.AuthorHasTrigger = false
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		wantNone(t, mustDecide(t, in), domain.StepTriggerRight)
	})
}

// TestDecide_OperatorRules is 09 3.1: a thread uses its fixed operator, the main thread the
// channel's effective operator.
func TestDecide_OperatorRules(t *testing.T) {
	t.Run("thread ignores a different channel operator", func(t *testing.T) {
		in := threadMsg()
		in.ChannelOperator = ptr(texter())
		wantInvoke(t, mustDecide(t, in), lotseID, false, false)
	})
	t.Run("main thread uses the channel operator", func(t *testing.T) {
		in := mainMsg()
		in.ChannelOperator = ptr(texter())
		in.Switches.RequireMention = false
		wantInvoke(t, mustDecide(t, in), texterID, false, false)
	})
	t.Run("thread without operator in a free response channel", func(t *testing.T) {
		in := threadMsg()
		in.ThreadOperator = nil
		in.Switches.FreeResponse = true
		wantNone(t, mustDecide(t, in), domain.StepMentions)
	})
}

// TestDecide_Voice is 09 13 and D-M2-04-2: the transcription path decides after the trigger right
// and before the target.
func TestDecide_Voice(t *testing.T) {
	voice := func(mode domain.VoiceMode, voiceInput bool) domain.MessageTrigger {
		in := threadMsg()
		in.Voice, in.VoiceMode = true, mode
		in.ThreadOperator.VoiceInput = voiceInput
		return in
	}
	t.Run("off", func(t *testing.T) {
		wantReject(t, mustDecide(t, voice(domain.VoiceOff, true)), domain.StepVoice, domain.ReasonVoiceUnsupported)
	})
	t.Run("agent with voice_input", func(t *testing.T) {
		d := mustDecide(t, voice(domain.VoiceAgent, true))
		wantInvoke(t, d, lotseID, false, false)
		if d.Transcript != domain.TranscriptAgent {
			t.Fatalf("Transcript = %q, want agent", d.Transcript)
		}
	})
	t.Run("agent without voice_input", func(t *testing.T) {
		wantReject(t, mustDecide(t, voice(domain.VoiceAgent, false)), domain.StepVoice, domain.ReasonVoiceUnsupported)
	})
	t.Run("service", func(t *testing.T) {
		d := mustDecide(t, voice(domain.VoiceService, false))
		wantInvoke(t, d, lotseID, false, false)
		if d.Transcript != domain.TranscriptService {
			t.Fatalf("Transcript = %q, want service", d.Transcript)
		}
	})
	t.Run("no trigger stays none", func(t *testing.T) {
		in := voice(domain.VoiceOff, false)
		in.Switches.ThreadRequireMention = true
		wantNone(t, mustDecide(t, in), domain.StepMentions)
	})
	t.Run("would be a follow-up without path", func(t *testing.T) {
		in := voice(domain.VoiceOff, false)
		in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: lotseID}
		wantReject(t, mustDecide(t, in), domain.StepVoice, domain.ReasonVoiceUnsupported)
	})
	t.Run("trigger right before voice", func(t *testing.T) {
		in := voice(domain.VoiceOff, false)
		in.AuthorHasTrigger = false
		wantNone(t, mustDecide(t, in), domain.StepTriggerRight)
	})
	t.Run("no auto thread without path", func(t *testing.T) {
		in := mainMsg()
		in.Voice = true
		in.Mentioned = []domain.AgentFacts{lotse()}
		wantReject(t, mustDecide(t, in), domain.StepVoice, domain.ReasonVoiceUnsupported)
	})
	t.Run("text message ignores the mode", func(t *testing.T) {
		in := threadMsg()
		d := mustDecide(t, in)
		wantInvoke(t, d, lotseID, false, false)
		if d.Transcript != domain.TranscriptNone {
			t.Fatalf("Transcript = %q, want none", d.Transcript)
		}
	})
}

// TestDecide_InvalidInput is D-M2-04-5: every broken input is an internal error with the zero
// decision, so it never yields an invocation.
func TestDecide_InvalidInput(t *testing.T) {
	cases := map[string]func(*domain.MessageTrigger){
		"unknown origin":          func(in *domain.MessageTrigger) { in.Origin = "robot" },
		"empty origin":            func(in *domain.MessageTrigger) { in.Origin = "" },
		"unknown channel kind":    func(in *domain.MessageTrigger) { in.ChannelKind = "voice" },
		"unknown thread kind":     func(in *domain.MessageTrigger) { in.ThreadKind = "sub" },
		"unknown voice mode":      func(in *domain.MessageTrigger) { in.VoiceMode = "" },
		"unknown billing mode":    func(in *domain.MessageTrigger) { in.ChannelOperator.BillingMode = "team" },
		"unknown mention billing": func(in *domain.MessageTrigger) { in.Mentioned = []domain.AgentFacts{{ID: robotID}} },
		"thread operator on main": func(in *domain.MessageTrigger) { in.ThreadOperator = ptr(lotse()) },
		"main thread in a forum":  func(in *domain.MessageTrigger) { in.ChannelKind = domain.ChannelForum },
		"operator on a DM":        func(in *domain.MessageTrigger) { in.ChannelKind = domain.ChannelDM },
		"zero message id":         func(in *domain.MessageTrigger) { in.MessageID = ids.ID{} },
		"zero thread id":          func(in *domain.MessageTrigger) { in.ThreadID = ids.ID{} },
		"zero channel id":         func(in *domain.MessageTrigger) { in.ChannelID = ids.ID{} },
		"zero mentioned id": func(in *domain.MessageTrigger) {
			in.Mentioned = []domain.AgentFacts{{BillingMode: domain.BillingOrganization}}
		},
		"zero channel operator id": func(in *domain.MessageTrigger) { in.ChannelOperator.ID = ids.ID{} },
		"zero active run id":       func(in *domain.MessageTrigger) { in.ActiveRun = &domain.ActiveRun{AgentID: lotseID} },
		"zero active run agent id": func(in *domain.MessageTrigger) { in.ActiveRun = &domain.ActiveRun{RunID: runR} },
		"thread operator on DM thread": func(in *domain.MessageTrigger) {
			*in = threadMsg()
			in.ChannelKind, in.ChannelOperator = domain.ChannelDM, nil
		},
		"zero thread operator id": func(in *domain.MessageTrigger) {
			*in = threadMsg()
			in.ThreadOperator.ID = ids.ID{}
		},
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			in := mainMsg()
			in.Switches.FreeResponse = true
			in.Mentioned = []domain.AgentFacts{lotse()}
			mod(&in)
			d, err := domain.Decide(in)
			wantInternal(t, err)
			if d != (domain.Decision{}) || d.Outcome != domain.OutcomeNone || d.Check != domain.CheckNone {
				t.Fatalf("decision with error = %+v, want the zero decision", d)
			}
		})
	}
}

func wantInternal(t *testing.T, err error) {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) || e.PublicCode() != errs.CodeInternalError {
		t.Fatalf("error = %v, want internal_error", err)
	}
}
