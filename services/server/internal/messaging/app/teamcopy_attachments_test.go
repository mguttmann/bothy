// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// copyTo runs CopyMessage of the message m into thread as Alex in a unit of work.
func (w *attWorld) copyTo(t *testing.T, m messaging.Message, thread ids.ID) messaging.Copied {
	t.Helper()
	var c messaging.Copied
	err := w.uow.Do(w.ctx(), func(ctx context.Context) error {
		var err error
		c, err = w.svc.CopyMessage(ctx, messaging.CopyCmd{SourceMessageID: m.ID, SourceVersion: m.Version, ThreadID: thread, AuthorID: w.alex.ID})
		return err
	})
	if err != nil {
		t.Fatalf("CopyMessage: %v", err)
	}
	return c
}

// TestCopyMessage_RelaysTheAttachments (FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-1, D-TCAT-5): a team copy
// of a message with attachments is written with has_attachments, calls files.Relay once in the
// unit of work with the source and the copy's place, and carries the relayed attachments in its
// message.created, its MessagePosted and its representation; a source of attachments only (no
// text) is copied too; a source without attachments makes no files call.
func TestCopyMessage_RelaysTheAttachments(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	work := ids.New()
	w.threads.mu.Lock()
	w.threads.infos[work] = chat.ThreadInfo{ID: work, ChannelID: w.newChann, SpaceID: w.threads.infos[w.thread].SpaceID, ChannelKind: "text",
		Kind: "thread", State: "open"}
	w.threads.mu.Unlock()
	f1, f2 := ids.New(), ids.New()
	src := w.send(t, "see the screenshots", f1, f2).Message

	c := w.copyTo(t, src, work)
	if len(w.files.relayed) != 1 || w.files.relayed[0].SourceMessageID != src.ID || w.files.relayed[0].MessageID != c.ID ||
		w.files.relayed[0].ThreadID != work || w.files.relayed[0].ChannelID != w.newChann {
		t.Fatalf("Relay calls %+v, want one with the source and the copy's place", w.files.relayed)
	}
	if row, _, _ := w.store.Message(context.Background(), c.ID); !row.HasAttachments || row.ContentMD != "see the screenshots" {
		t.Fatalf("stored copy %+v", row)
	}
	created := w.events.payloads(t, "message.created")
	wantAttachments(t, created[len(created)-1], []ids.ID{f1, f2})
	if ev := w.posted.last(); ev.MessageID != c.ID || !ev.Relay || !ev.HasAttachments {
		t.Fatalf("MessagePosted %+v, want the relay with attachments", ev)
	}
	got, err := w.svc.Get(w.ctx(), c.ID)
	if err != nil || len(got.Attachments) != 2 || got.Attachments[1].File.ID != f2 {
		t.Fatalf("the copy read back: %+v %v", got.Attachments, err)
	}

	// An owner's screenshot without words is copied with its file.
	only := w.send(t, "", ids.New()).Message
	if oc := w.copyTo(t, only, work); oc.ContentMD != "" || len(w.files.relayed) != 2 {
		t.Fatalf("copy of attachments only: %+v, relay calls %d", oc, len(w.files.relayed))
	}

	plain := w.send(t, "no files").Message
	pc := w.copyTo(t, plain, work)
	if row, _, _ := w.store.Message(context.Background(), pc.ID); row.HasAttachments || len(w.files.relayed) != 2 {
		t.Fatalf("copy without attachments: has_attachments %v, relay calls %d", row.HasAttachments, len(w.files.relayed))
	}
}
