// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// postVoice accepts a voice message of Alex with its one attachment in the thread of the world that
// Lotse operates; reason is the answer of files.Attachments.Pending for it.
func postVoice(t *testing.T, w *world, reason files.PendingReason) messaging.MessagePosted {
	t.Helper()
	ev := w.posted(w.thread)
	ev.Kind, ev.HasAttachments = "voice", true
	w.files.setReason(ev.MessageID, reason)
	w.mustPost(ev)
	return ev
}

// withVoiceInput sets the voice facts of Lotse: the owner's supports_voice and the report.
func withVoiceInput(w *world, permitted *bool, reported bool) {
	f := w.agents[w.lotse]
	f.VoicePermitted, f.VoiceReported = permitted, reported
	w.agents[w.lotse] = f
}

// TestVoice_TextMessageReadsNoMode (M3-09 design note 12 b): a text message never asks files for
// the transcription mode, so the text path costs no extra statement.
func TestVoice_TextMessageReadsNoMode(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.mustPost(w.posted(w.thread))
	if w.j.count("files:mode") != 0 || len(invocations(w)) != 1 {
		t.Fatalf("text message: %v", w.j.whats())
	}
}

// TestVoice_Routing (09 sections 3.5 and 13, AG-25, M3-09 design note 12): a voice message without
// a transcription path (mode off, mode agent without the effective voice_input) is rejected with
// trigger_state failed and voice_unsupported, without a run, without agent.invoke in request mode
// and without an audit entry; mode agent with voice_input invokes at once; mode service holds the
// invocation for the pending transcript with voice.transcript_hold_max. The mode is read once for
// the author.
func TestVoice_Routing(t *testing.T) {
	t.Parallel()
	no, yes := false, true
	for name, c := range map[string]struct {
		mode      files.Mode
		permitted *bool
		reported  bool
	}{
		"mode off":                    {files.ModeOff, nil, true},
		"mode agent without a report": {files.ModeAgent, nil, false},
		"mode agent refused by owner": {files.ModeAgent, &no, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			w.modes.set(c.mode)
			withVoiceInput(w, c.permitted, c.reported)
			postVoice(t, w, files.PendingNone)
			failedWith(t, w, string(domain.ReasonVoiceUnsupported))
			if w.j.count("authz:agent.invoke:request") != 0 || len(w.audit.entries) != 0 || len(invocations(w)) != 0 {
				t.Fatalf("a voice rejection authorized or audited: %v, audit %+v", w.j.whats(), w.audit.entries)
			}
			if got := w.j.count("files:mode"); got != 1 {
				t.Fatalf("Mode calls %d", got)
			}
		})
	}
	t.Run("mode agent with voice_input invokes at once", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		withVoiceInput(w, &yes, true)
		postVoice(t, w, files.PendingNone)
		if len(w.repo.runs) != 1 || !w.repo.runs[0].InvocationHeldUntil.IsZero() || len(invocations(w)) != 1 || len(holds(w)) != 0 {
			t.Fatalf("runs %+v, journal %v", w.repo.runs, w.j.whats())
		}
		if got := w.triggers(); len(got) != 1 || got[0].State != "received" {
			t.Fatalf("trigger states %+v", got)
		}
		for _, c := range w.j.list() {
			if c.what == "files:mode" && c.arg != (subjectRef{w.alex, "user"}) {
				t.Fatalf("Mode subject %+v, want Alex as user", c.arg)
			}
		}
	})
	t.Run("mode service holds for the transcript", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeService)
		postVoice(t, w, files.PendingTranscript)
		if len(w.repo.runs) != 1 || len(invocations(w)) != 0 {
			t.Fatalf("runs %+v, journal %v", w.repo.runs, w.j.whats())
		}
		want := t0.Add(transcriptHoldMax)
		if got := w.repo.runs[0].InvocationHeldUntil; !got.Equal(want) {
			t.Fatalf("held until %v, want %v (voice.transcript_hold_max)", got, want)
		}
		if h := holds(w); len(h) != 1 || !h[0].at.Equal(want) {
			t.Fatalf("hold jobs %+v", h)
		}
	})
	t.Run("a voice message that triggers nobody keeps no trigger state", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeOff)
		ev := w.posted(w.main) // require_mention in the main thread: no candidate
		ev.Kind, ev.HasAttachments = "voice", true
		w.mustPost(ev)
		noWrites(t, w)
	})
	t.Run("a person without trigger stays unanswered", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeOff)
		w.authz.decide = func(_ authorize.Action, filter bool, _ authorize.Resource) decision {
			if filter {
				return decision{reason: "level_insufficient"}
			}
			return decision{allowed: true}
		}
		postVoice(t, w, files.PendingNone)
		noWrites(t, w)
	})
	t.Run("an error of the mode vetoes the message", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.err = errVeto
		ev := w.posted(w.thread)
		ev.Kind, ev.HasAttachments = "voice", true
		if err := w.post(ev); !errors.Is(err, errVeto) {
			t.Fatalf("post: %v, want the error of Mode", err)
		}
	})
}

// TestVoice_HoldWindowAndRelease (M3-09 design note 13): a voice message whose file is not final
// waits file.invocation_hold_max; ReleaseFinal releases only when nothing is pending, so a pending
// transcript keeps the hold; ReleaseTimeout releases with the transcript still pending and logs a
// line that names both windows.
func TestVoice_HoldWindowAndRelease(t *testing.T) {
	t.Parallel()
	t.Run("files pending", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeService)
		postVoice(t, w, files.PendingFiles)
		if got, want := w.repo.runs[0].InvocationHeldUntil, t0.Add(holdMax); !got.Equal(want) {
			t.Fatalf("held until %v, want %v (file.invocation_hold_max)", got, want)
		}
	})
	t.Run("final waits for the transcript", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeService)
		ev := postVoice(t, w, files.PendingFiles)
		run := w.repo.runs[0]
		w.files.setReason(ev.MessageID, files.PendingTranscript)
		if err := release(w, run.ID, domain.ReleaseFinal); err != nil || len(invocations(w)) != 0 {
			t.Fatalf("final with a pending transcript: %v %d", err, len(invocations(w)))
		}
		w.files.setReason(ev.MessageID, files.PendingNone)
		if err := release(w, run.ID, domain.ReleaseFinal); err != nil || len(invocations(w)) != 1 {
			t.Fatalf("final: %v %d", err, len(invocations(w)))
		}
	})
	t.Run("timeout with the transcript pending", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.modes.set(files.ModeService)
		postVoice(t, w, files.PendingTranscript)
		run := w.repo.runs[0]
		w.clk.Advance(transcriptHoldMax)
		if err := w.svc.InvocationHoldTimeout(context.Background(), run.ID); err != nil || len(invocations(w)) != 1 {
			t.Fatalf("timeout: %v %d", err, len(invocations(w)))
		}
		log := w.log.String()
		if !strings.Contains(log, "file.invocation_hold_max") || !strings.Contains(log, "voice.transcript_hold_max") {
			t.Fatalf("log %q does not name both windows", log)
		}
	})
}

// TestVoice_ConfigNeedsTranscriptHoldMax: voice.transcript_hold_max must be positive.
func TestVoice_ConfigNeedsTranscriptHoldMax(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, err := app.New(app.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: queuedMaxAge,
		ClockLeeway: 0, CancelGrace: 1, PresenceTimeout: 1, OrphanTimeout: 1, ReasonMaxChars: 1, ComponentDefaultTTL: 1,
		ComponentMaxPerMessage: 1, ComponentCustomIDMaxChars: 1, ComponentFormMaxFields: 1, MaxContentBytes: 1,
		ExcerptMaxChars: 1, MaxStepsPersisted: 1, EventPayloadMaxBytes: 1, InvocationHoldMax: holdMax}, w.deps())
	if err == nil || !strings.Contains(err.Error(), "voice.transcript_hold_max") {
		t.Fatalf("New without voice.transcript_hold_max: %v", err)
	}
}
