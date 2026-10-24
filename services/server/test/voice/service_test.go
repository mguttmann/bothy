// SPDX-License-Identifier: Apache-2.0

package voice_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents/adapters/agentstest"
	agentsdomain "github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestVoice_AG25_NoTranscriptionPath (AG-25, 09 3.5): mode off; mode agent with Lotse without the
// permission or without the report; mode service with voice.enabled off: the voice message
// mentioning Lotse is failed with voice_unsupported, no run, no invocation, no auto thread, no
// audit entry, no request to the service. A voice message that triggers nobody keeps no
// trigger_state.
func TestVoice_AG25_NoTranscriptionPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		mode  string
		setup func(w *world)
	}{
		{"mode off", "off", func(*world) {}},
		{"agent without permission", "agent", func(w *world) { w.voiceInput(false, true) }},
		{"agent without report", "agent", func(w *world) { w.voiceInput(true, false) }},
		{"service with the switch off", "service", func(*world) {}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, worldOptions{mode: c.mode})
			c.setup(w)
			f := w.m4a(3 * time.Second)
			threads := w.count(`SELECT count(*) FROM threads`)
			audits := w.count(`SELECT count(*) FROM audit_events`)
			invocations := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`)
			m := w.voiceTo(f)
			if st, reason := w.triggerState(m.ID); st != "failed" || reason != "voice_unsupported" {
				t.Fatalf("trigger %s %s", st, reason)
			}
			if n := w.count(`SELECT count(*) FROM runs WHERE triggering_message_id = $1`, m.ID); n != 0 {
				t.Fatalf("%d runs", n)
			}
			if n := w.count(`SELECT count(*) FROM threads`); n != threads {
				t.Fatalf("%d threads opened", n-threads)
			}
			if n := w.count(`SELECT count(*) FROM audit_events`); n != audits {
				t.Fatalf("%d audit entries", n-audits)
			}
			if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.invocation'`); n != invocations {
				t.Fatalf("%d invocations", n-invocations)
			}
			if n := len(w.svc.Requests()); n != 0 || w.file(f).Status != "none" {
				t.Fatalf("%d requests to the service, transcript %s", n, w.file(f).Status)
			}
			plain := w.send(w.alex, w.main, "voice", "", w.m4a(time.Second))
			var pm message
			plain.JSON(t, &pm)
			if st, _ := w.triggerState(pm.ID); plain.Status != http.StatusCreated || st != "" {
				t.Fatalf("a voice message without a trigger: %d %s, trigger %q", plain.Status, plain.Body, st)
			}
		})
	}
}

// localeHint is the primary subtag of users.locale of the person in lower case.
func (w *world) localeHint(principalID string) string {
	w.t.Helper()
	var locale string
	if err := w.db.QueryRow(w.t.Context(), `SELECT locale FROM users WHERE principal_id = $1`, principalID).Scan(&locale); err != nil {
		w.t.Fatal(err)
	}
	primary, _, _ := strings.Cut(locale, "-")
	return strings.ToLower(primary)
}

// serviceVoice starts mode service with voice.enabled on and sends a voice message of Alex
// mentioning Lotse; it returns the file, the message and the run.
func (w *world) serviceVoice() (string, message, runRow) {
	w.t.Helper()
	w.voiceEnabled(true)
	f := w.m4a(3 * time.Second)
	m := w.voiceTo(f)
	return f, m, w.runOf(m.ID)
}

// TestVoice_ModeService_FakeService (plan): the voice message is stored with the transcript
// pending, one file.updated, one job and the run held until the send plus voice.transcript_hold_max;
// the job sends the audio with the language hint and writes done with one file.updated and
// exactly one agent.invocation in one transaction; Lotse's poll delivers it; the hold job writes
// nothing more.
func TestVoice_ModeService_FakeService(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{mode: "service"})
	w.voiceEnabled(true)
	f := w.m4a(3 * time.Second)
	before := len(w.fileUpdated(f))
	sent := w.clk.Now().UTC().Truncate(time.Microsecond)
	m := w.voiceTo(f)
	if len(m.Attachments) != 1 || m.Attachments[0].File.TranscriptStatus != "pending" {
		t.Fatalf("send answered %+v", m)
	}
	run := w.runOf(m.ID)
	if got := w.file(f); got.Status != "pending" || w.transcribeJobs(f) != 1 {
		t.Fatalf("after the send: %+v, %d jobs", got, w.transcribeJobs(f))
	}
	if p := w.fileUpdated(f)[before:]; len(p) != 1 || !strings.Contains(p[0], "pending") {
		t.Fatalf("file.updated %v", p)
	}
	if run.State != "queued" || !run.HeldUntil.Equal(sent.Add(transcriptMax)) || w.invocations(run.ID) != 0 {
		t.Fatalf("run %+v (sent %v), %d invocations", run, sent, w.invocations(run.ID))
	}
	if err := w.runTranscribe(f, 1); err != nil {
		t.Fatal(err)
	}
	reqs := w.svc.Requests()
	audio := filestest.M4A(1000, 3000, filestest.AudioOptions{})
	hint := w.localeHint(w.alex.PrincipalID)
	if len(reqs) != 1 || hint == "" || reqs[0].Fields["language"] != hint || string(reqs[0].FileBytes) != string(audio) || reqs[0].FileName != "audio.m4a" ||
		strings.Contains(string(reqs[0].Body), "note.m4a") {
		t.Fatalf("requests %+v", reqs)
	}
	got := w.file(f)
	if got.Status != "done" || got.Source != "service" || got.Text == "" || got.Language != "en" {
		t.Fatalf("after the job: %+v", got)
	}
	payloads := w.fileUpdated(f)[before+1:]
	if len(payloads) != 1 || !strings.Contains(payloads[0], "done") || strings.Contains(payloads[0], got.Text) {
		t.Fatalf("file.updated %v", payloads)
	}
	if w.invocations(run.ID) != 1 || !w.runOf(m.ID).HeldUntil.IsZero() {
		t.Fatalf("after done: %d invocations, %+v", w.invocations(run.ID), w.runOf(m.ID))
	}
	if d := delivered(w.poll(), run.ID); len(d) != 1 {
		t.Fatalf("%d invocations delivered", len(d))
	}
	w.clk.Advance(transcriptMax + time.Second)
	w.holdTimeout(run.ID)
	if w.invocations(run.ID) != 1 {
		t.Fatal("the hold job wrote a second invocation")
	}
}

// TestVoice_ServiceFailures (02 14.4): 503 until the last attempt gives failed with one
// file.updated and the held invocation released; 400 gives failed at once; a 307 gives failed
// without a second request; a deleted message leaves the job without effect.
func TestVoice_ServiceFailures(t *testing.T) {
	t.Parallel()
	t.Run("transient until the last attempt", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		w.svc.Respond(http.StatusServiceUnavailable, `{"error":"busy"}`)
		f, m, run := w.serviceVoice()
		for attempt := 1; attempt < maxAttempts; attempt++ {
			if err := w.runTranscribe(f, attempt); err == nil {
				t.Fatalf("attempt %d returned no error", attempt)
			}
			if w.file(f).Status != "pending" || w.invocations(run.ID) != 0 {
				t.Fatalf("attempt %d: %+v", attempt, w.file(f))
			}
		}
		before := len(w.fileUpdated(f))
		if err := w.runTranscribe(f, maxAttempts); err != nil {
			t.Fatalf("last attempt: %v", err)
		}
		if got := w.file(f); got.Status != "failed" || got.Text != "" || got.Source != "" || len(w.fileUpdated(f)) != before+1 {
			t.Fatalf("after the last attempt: %+v", got)
		}
		if w.invocations(run.ID) != 1 || !w.runOf(m.ID).HeldUntil.IsZero() || len(w.svc.Requests()) != maxAttempts {
			t.Fatalf("%d invocations, %d requests", w.invocations(run.ID), len(w.svc.Requests()))
		}
	})
	t.Run("permanent 400", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		w.svc.Respond(http.StatusBadRequest, `{"error":"bad"}`)
		f, _, run := w.serviceVoice()
		if err := w.runTranscribe(f, 1); err != nil || w.file(f).Status != "failed" || w.invocations(run.ID) != 1 {
			t.Fatalf("400: %v %+v", err, w.file(f))
		}
	})
	t.Run("redirect", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		w.svc.RespondFunc(func(rw http.ResponseWriter, r *http.Request) {
			http.Redirect(rw, r, w.svc.URL()+"?again=1", http.StatusTemporaryRedirect)
		})
		f, _, _ := w.serviceVoice()
		if err := w.runTranscribe(f, 1); err != nil || w.file(f).Status != "failed" || len(w.svc.Requests()) != 1 {
			t.Fatalf("307: %v %+v, %d requests", err, w.file(f), len(w.svc.Requests()))
		}
	})
	t.Run("message deleted", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		f, m, _ := w.serviceVoice()
		harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + m.ID}, http.StatusNoContent)
		if err := w.runTranscribe(f, 1); err != nil {
			t.Fatal(err)
		}
		if got := w.file(f); !got.Deleted || got.Status != "pending" || len(w.svc.Requests()) != 0 {
			t.Fatalf("after the delete: %+v, %d requests", got, len(w.svc.Requests()))
		}
	})
}

// TestVoice_ServiceHoldTimeout: the service hangs beyond voice.transcription_timeout; once the
// clock passes voice.transcript_hold_max the hold job writes exactly one invocation with the
// transcript still pending; a later done writes no second one.
func TestVoice_ServiceHoldTimeout(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{mode: "service", config: map[string]string{"voice.transcription_timeout": "1s"}})
	w.svc.Hang()
	f, m, run := w.serviceVoice()
	if err := w.runTranscribe(f, 1); err == nil {
		t.Fatal("a hanging service returned no error")
	}
	w.holdTimeout(run.ID)
	if w.invocations(run.ID) != 0 {
		t.Fatal("released before voice.transcript_hold_max")
	}
	w.clk.Advance(transcriptMax + time.Second)
	w.holdTimeout(run.ID)
	if w.invocations(run.ID) != 1 || w.file(f).Status != "pending" || !w.runOf(m.ID).HeldUntil.IsZero() {
		t.Fatalf("after the window: %d invocations, %+v", w.invocations(run.ID), w.file(f))
	}
	w.svc.Respond(http.StatusOK, `{"text":"late transcript","language":"en"}`)
	if err := w.runTranscribe(f, 2); err != nil {
		t.Fatal(err)
	}
	if w.file(f).Status != "done" || w.invocations(run.ID) != 1 {
		t.Fatalf("a late done: %+v, %d invocations", w.file(f), w.invocations(run.ID))
	}
}

// TestRunContext_TranscriptTrust (AG-20 transcript part, M2-13): the run context of the voice
// message lists the attachment with transcript and the element keeps trust trigger; while the
// transcript is pending the member is absent.
func TestRunContext_TranscriptTrust(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{mode: "service"})
	f, _, run := w.serviceVoice()
	w.clk.Advance(transcriptMax + time.Second)
	w.holdTimeout(run.ID)
	w.startRun(run.ID)
	ctx := w.runContext(run.ID)
	el, ok := ctx.attachment(f)
	if !ok || el.Trust != "trigger" || el.HasTranscript {
		t.Fatalf("pending: %+v %v", el, ok)
	}
	w.svc.Respond(http.StatusOK, `{"text":"Please call Robin","language":"en"}`)
	if err := w.runTranscribe(f, 2); err != nil {
		t.Fatal(err)
	}
	ctx = w.runContext(run.ID)
	el, ok = ctx.attachment(f)
	if !ok || el.Trust != "trigger" || !el.HasTranscript || el.Transcript != "Please call Robin" {
		t.Fatalf("done: %+v %v", el, ok)
	}
}

// startRun delivers the invocation of the run to Lotse and starts the run.
func (w *world) startRun(runID string) {
	w.t.Helper()
	d := delivered(w.poll(), runID)
	if len(d) != 1 || d[0].InvocationToken == "" {
		w.t.Fatalf("delivered %+v", d)
	}
	harness.MustDo(w.t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + runID + "/start",
		Body: map[string]any{"invocation_token": d[0].InvocationToken, "billing_mode_used": "organization", "tool_class_used": "chat"}}, http.StatusOK)
}

// contextElement is the part of an element of the run context the tests read.
type contextElement struct {
	Trust         string
	HasTranscript bool
	Transcript    string
}

// runCtx is the decoded run context.
type runCtx struct {
	raw []byte
}

// attachment returns the element whose attachment is the file.
func (c runCtx) attachment(fileID string) (contextElement, bool) {
	var doc map[string]any
	if json.Unmarshal(c.raw, &doc) != nil {
		return contextElement{}, false
	}
	var found *contextElement
	var walk func(v any, trust string)
	walk = func(v any, trust string) {
		switch x := v.(type) {
		case map[string]any:
			if t, ok := x["trust"].(string); ok {
				trust = t
			}
			if id, ok := x["file_id"].(string); ok && id == fileID {
				el := contextElement{Trust: trust}
				if tr, ok := x["transcript"].(string); ok {
					el.HasTranscript, el.Transcript = true, tr
				}
				found = &el
				return
			}
			for _, child := range x {
				walk(child, trust)
			}
		case []any:
			for _, child := range x {
				walk(child, trust)
			}
		}
	}
	walk(doc, "")
	if found == nil {
		return contextElement{}, false
	}
	return *found, true
}

// runContext reads getRunContext as Lotse.
func (w *world) runContext(runID string) runCtx {
	w.t.Helper()
	r := w.lotse.Do(w.t.Context(), harness.Request{Method: http.MethodGet, Path: "/runs/" + runID + "/context"})
	if r.Status != http.StatusOK {
		w.t.Fatalf("getRunContext: %d %s", r.Status, r.Body)
	}
	noNull(w.t, "getRunContext", r.Body)
	return runCtx{raw: r.Body}
}

// startedLotseRun opens a run of Lotse through a text mention of Alex, delivers and starts it.
func (w *world) startedLotseRun() runRow {
	w.t.Helper()
	r := w.send(w.alex, w.main, "", "@lotse please help")
	if r.Status != http.StatusCreated {
		w.t.Fatalf("mention: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(w.t, &m)
	run := w.runOf(m.ID)
	w.startRun(run.ID)
	return run
}

// TestVoice_HeldFollowOnRunAndFollowUp (M2-11 through invokeOrHold of M3-07): a voice message
// deferred into a follow-on run in mode service is held with voice.transcript_hold_max; a voice
// follow-up to a running run gets its agent.followup at once.
func TestVoice_HeldFollowOnRunAndFollowUp(t *testing.T) {
	t.Parallel()
	t.Run("follow-on run", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		w.voiceEnabled(true)
		agentstest.ReportCapabilities(t, w.db, ids.MustParse(w.lotse.PrincipalID), agentsdomain.Capabilities{Followups: "queue"})
		ended := w.startedLotseRun()
		f := w.m4a(2 * time.Second)
		r := w.send(w.alex, ended.Thread, "voice", "", f)
		if r.Status != http.StatusCreated {
			t.Fatalf("voice follow-up: %d %s", r.Status, r.Body)
		}
		var fu message
		r.JSON(t, &fu)
		finishedAt := w.clk.Now().UTC().Truncate(time.Microsecond)
		harness.MustDo(t, w.lotse, harness.Request{Method: http.MethodPost, Path: "/runs/" + ended.ID + "/finish",
			Body: map[string]any{"state": "completed"}}, http.StatusOK)
		next := w.runOf(fu.ID)
		if next.State != "queued" || !next.HeldUntil.Equal(finishedAt.Add(transcriptMax)) || w.invocations(next.ID) != 0 {
			t.Fatalf("follow-on run %+v, want held until %v", next, finishedAt.Add(transcriptMax))
		}
		if err := w.runTranscribe(f, 1); err != nil {
			t.Fatal(err)
		}
		if w.invocations(next.ID) != 1 {
			t.Fatal("the follow-on run was not released by the transcript")
		}
	})
	t.Run("follow-up", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, worldOptions{mode: "service"})
		w.voiceEnabled(true)
		agentstest.ReportCapabilities(t, w.db, ids.MustParse(w.lotse.PrincipalID), agentsdomain.Capabilities{Followups: "steer"})
		run := w.startedLotseRun()
		f := w.m4a(2 * time.Second)
		r := w.send(w.alex, run.Thread, "voice", "", f)
		if r.Status != http.StatusCreated {
			t.Fatalf("voice follow-up: %d %s", r.Status, r.Body)
		}
		var fu message
		r.JSON(t, &fu)
		n := w.count(`SELECT count(*) FROM event_log WHERE type = 'agent.followup' AND resource_id = $1 AND payload->>'message_id' = $2`, run.ID, fu.ID)
		if n != 1 || w.file(f).Status != "pending" {
			t.Fatalf("%d agent.followup at once, transcript %s", n, w.file(f).Status)
		}
	})
}

// TestVoice_NotReadyAndBlocked: with the clamd stopped the file is pending and the send is 409
// file_not_ready; an EICAR reply makes it infected and the send 409 file_blocked; a foreign file
// is still 409 file_not_attachable.
func TestVoice_NotReadyAndBlocked(t *testing.T) {
	t.Parallel()
	fake, err := clamdtest.NewFake()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fake.Close() })
	w := newWorld(t, worldOptions{clamd: fake.Address()})
	f := w.upload(w.alex, w.allgemein, "note.m4a", filestest.M4A(1000, 3000, filestest.AudioOptions{}))
	if r := w.send(w.alex, w.main, "voice", "", f); r.Status != http.StatusConflict || problemOf(t, r).Code != "file_not_ready" {
		t.Fatalf("pending: %d %s", r.Status, r.Body)
	}
	fake.SetScript(clamdtest.Script{Reply: "stream: Eicar-Test-Signature FOUND"})
	w.mustScan(f)
	if r := w.send(w.alex, w.main, "voice", "", f); r.Status != http.StatusConflict || problemOf(t, r).Code != "file_blocked" {
		t.Fatalf("infected: %d %s", r.Status, r.Body)
	}
	fake.SetScript(clamdtest.Script{})
	lotses := w.ready(w.lotse, w.allgemein, "lotse.m4a", filestest.M4A(1000, 3000, filestest.AudioOptions{}))
	if r := w.send(w.alex, w.main, "voice", "", lotses); r.Status != http.StatusConflict || problemOf(t, r).Reason != "file_not_attachable" {
		t.Fatalf("a foreign file: %d %s", r.Status, r.Body)
	}
}
