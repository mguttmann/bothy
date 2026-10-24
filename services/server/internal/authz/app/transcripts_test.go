// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestFileTranscriptSubmitRequirement (05 4.8, M3-09 design note 14): file.transcript.submit is
// All(ThreadOperator, Level(read)) on the file resource of M3-07, no longer deferred. The operator
// agent of the thread the file is attached to (the fixed operator of a thread, the channel's
// effective operator for a main thread) with read is allowed; an agent without read (Texter) gets
// not_found like an unknown file; a visible agent that is not the operator and a person get
// forbidden with the reason of the ThreadOperator atom; an unattached file is visible to its
// uploader only, so Lotse gets not_found. file.transcribe keeps Level(write).
func TestFileTranscriptSubmitRequirement(t *testing.T) {
	w := newWorld(t)
	r, ok := domain.RequirementOf(string(authorize.ActionFileTranscriptSubmit))
	if !ok {
		t.Fatal("file.transcript.submit: no entry")
	}
	if _, deferred := r.Expr.DeferredTo(); deferred || slices.Contains(domain.DeferredActions(), "file.transcript.submit") {
		t.Fatal("file.transcript.submit is still deferred")
	}
	if got, want := r.Expr.String(), domain.All(domain.ThreadOperator, domain.AtLeast(domain.LevelRead)).String(); got != want {
		t.Errorf("file.transcript.submit: %s, want %s", got, want)
	}
	if tr, _ := domain.RequirementOf(string(authorize.ActionFileTranscribe)); tr.Expr.String() != domain.AtLeast(domain.LevelWrite).String() {
		t.Errorf("file.transcribe: %s, want Level(write)", tr.Expr.String())
	}

	thread := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: thread, ChannelID: w.allgemein, CreatedBy: w.alex, OperatorAgentID: w.lotse})
	attached := w.file(w.alex, w.allgemein, "attachment", true)
	attached.ThreadID = thread
	inMain := w.file(w.alex, w.allgemein, "attachment", true)
	unattached := w.file(w.alex, w.allgemein, "attachment", false)
	// The assistant reads Support but operates none of its threads.
	w.grant(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.assistent}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support},
		domain.LevelRead, time.Time{})
	submit := authorize.ActionFileTranscriptSubmit
	for _, c := range []struct {
		name   string
		p      authorize.Principal
		r      authorize.Resource
		o      authorize.Outcome
		reason string
	}{
		{"Lotse operates the thread", w.agent(w.lotse), attached, authorize.OutcomeAllow, ""},
		{"Lotse operates the main thread through the channel", w.agent(w.lotse), inMain, authorize.OutcomeAllow, ""},
		{"Texter without read", w.agent(w.texter), attached, authorize.OutcomeNotFound, "no_grant"},
		{"an agent with read that is not the operator", w.agent(w.assistent), attached, authorize.OutcomeForbidden, "subject_kind_not_allowed"},
		{"Alex is a person", w.user(w.alex), attached, authorize.OutcomeForbidden, "subject_kind_not_allowed"},
		{"an unattached file of Alex", w.agent(w.lotse), unattached, authorize.OutcomeNotFound, "no_grant"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w.expect(t, c.p, submit, c.r, c.o, c.reason)
		})
	}
}
