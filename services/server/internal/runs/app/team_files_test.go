// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
)

// withFiles marks message as written now with attachments whose files are pending (p) or final.
func (w *teamWorld) withFiles(message ids.ID, pending bool) {
	w.t.Helper()
	w.msgFacts.mu.Lock()
	m := w.msgFacts.m[message]
	m.HasAttachments, m.CreatedAt = true, w.clk.Now()
	w.msgFacts.m[message] = m
	w.msgFacts.mu.Unlock()
	w.files.setPending(message, pending)
}

// TestTeam_InputWaitsForItsFiles (FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-4): an owner's message whose
// attachments are not final is not copied yet; the job schedules itself again teamFilesRecheck
// later. Once the files are final the job copies the message to every member. A message whose
// files are still pending when file.invocation_hold_max has passed since it was written is copied
// with the files in their current state, with an Info line naming the message.
func TestTeam_InputWaitsForItsFiles(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	msg := w.ownerMessage(w.alex, "see the screenshot")
	w.withFiles(msg, true)
	w.input(msg)
	if n := len(w.tmsgs.copies); n != 0 {
		t.Fatalf("copied %d times while the files are pending", n)
	}
	if len(w.sched.later) != 1 || w.sched.later[0].run != msg || !w.sched.later[0].at.Equal(w.clk.Now().Add(2*time.Second)) {
		t.Fatalf("scheduled again %+v, want the message 2s later", w.sched.later)
	}
	w.files.setPending(msg, false)
	w.input(msg)
	for _, m := range []app.TeamMemberRow{w.coding, w.review} {
		if copies := w.tmsgs.copiesInto(m.WorkThreadID); len(copies) != 1 {
			t.Fatalf("%s: %d copies once the files are final", m.Handle, len(copies))
		}
	}

	late := w.ownerMessage(w.alex, "and this one")
	w.withFiles(late, true)
	w.clk.Advance(holdMax)
	w.input(late)
	if copies := w.tmsgs.copiesInto(w.coding.WorkThreadID); len(copies) != 2 || len(w.sched.later) != 1 {
		t.Fatalf("after the window: %d copies, %d schedules; want the copy and no new schedule", len(copies), len(w.sched.later))
	}
	if log := w.log.String(); !strings.Contains(log, "files were final") || !strings.Contains(log, late.String()) {
		t.Fatalf("log %s, want the Info line with the message", log)
	}
}

// TestTeam_ReplyWaitsForItsFiles: a member's final reply whose attachments are not final is
// neither mirrored nor handed on until they are.
func TestTeam_ReplyWaitsForItsFiles(t *testing.T) {
	t.Parallel()
	w := newTeamWorld(t, defaultTeamConfig())
	w.setUp()
	reply := w.reply(w.coding, "@review here is the chart")
	w.withFiles(reply, true)
	w.input(reply)
	if n := len(w.tmsgs.copies); n != 0 || len(w.sched.later) != 1 {
		t.Fatalf("pending files: %d copies, %d schedules", n, len(w.sched.later))
	}
	w.files.setPending(reply, false)
	w.input(reply)
	if mirror := w.tmsgs.copiesInto(w.teamMain); len(mirror) != 1 {
		t.Fatalf("mirror %+v once the files are final", mirror)
	}
	if in := w.tmsgs.copiesInto(w.review.WorkThreadID); len(in) != 1 {
		t.Fatalf("input for @review %+v once the files are final", in)
	}
}
