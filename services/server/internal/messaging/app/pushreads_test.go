// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestPushReads (M3-19 design notes 5 and 9 d): the four reads of messaging.Messages for the
// consumer push answer through the store without a principal in ctx (they never authorize): the
// anchor of a run below a position and by its run, the newest answer of the agent after the
// anchor, and the newest message of the thread, which is the agent's answer and not the anchor.
func TestPushReads(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	_, ch, main := w.TextChannel()
	lotse := w.Agent("lotse")
	w.SetOperator(ch, lotse)
	w.Send("owner", main, "hello", http.StatusCreated)
	run := seedRun(w, main, lotse, w.OwnerID)
	var anchor messaging.AnchorRef
	if err := inUnit(w, func(ctx context.Context) error {
		var err error
		anchor, err = w.Svc.CreateRunAnchor(ctx, messaging.RunAnchorCmd{ThreadID: main, RunID: run, AgentID: lotse})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	answer := ids.MustParse(w.Send("lotse", main, "an agent answer", http.StatusCreated)["id"].(string))
	ctx := context.Background()
	want := messaging.AnchorRef{MessageID: anchor.MessageID, ThreadSeq: 2, RunID: run}
	if got, found, err := w.Svc.LastAnchorBefore(ctx, main, 3); err != nil || !found || got != want {
		t.Fatalf("LastAnchorBefore = %+v %v %v, want %+v", got, found, err, want)
	}
	if _, found, err := w.Svc.LastAnchorBefore(ctx, main, 2); err != nil || found {
		t.Fatalf("LastAnchorBefore at the anchor: %v %v", found, err)
	}
	if got, found, err := w.Svc.AnchorOf(ctx, run); err != nil || !found || got != want {
		t.Fatalf("AnchorOf = %+v %v %v, want %+v", got, found, err, want)
	}
	if _, found, err := w.Svc.AnchorOf(ctx, ids.New()); err != nil || found {
		t.Fatalf("AnchorOf an unknown run: %v %v", found, err)
	}
	ref := messaging.MessageRef{ID: answer, ThreadSeq: 3}
	if got, found, err := w.Svc.LastAgentMessageBefore(ctx, main, lotse, 2, 1<<53); err != nil || !found || got != ref {
		t.Fatalf("LastAgentMessageBefore = %+v %v %v, want %+v", got, found, err, ref)
	}
	if got, found, err := w.Svc.Newest(ctx, main); err != nil || !found || got != ref {
		t.Fatalf("Newest = %+v %v %v, want %+v", got, found, err, ref)
	}
}
