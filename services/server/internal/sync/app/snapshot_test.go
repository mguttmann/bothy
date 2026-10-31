// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	syncmod "github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestNewSnapshot_Refuses: a missing collaborator is refused.
func TestNewSnapshot_Refuses(t *testing.T) {
	t.Parallel()
	if _, err := app.NewSnapshot(app.SnapshotDeps{}); err == nil {
		t.Fatal("no collaborators accepted")
	}
	if _, err := app.NewCatchUp(app.CatchUpConfig{}, app.CatchUpDeps{}); err == nil {
		t.Fatal("no collaborators accepted")
	}
}

// TestSnapshot_UseCase (M1-63): Build called directly with the caller in the context: a person
// gets the parts of design note 9 with an unread thread and its mention, an agent the empty
// personal parts, an anonymous caller 401; catch-up over the same world reaches the head.
func TestSnapshot_UseCase(t *testing.T) {
	t.Parallel()
	w := mw.NewSync(t, noSubscriber, mw.SyncOptions{CatchUp: app.CatchUpConfig{PageSize: 5, MaxEvents: 1000}, PartitionSize: 50})
	space, ch, main := w.TextChannel()
	w.Channel(space, "forum", "ideas")
	other := w.Space("Other")
	w.Channel(other, "text", "elsewhere")
	alex := w.Person("alex", false)
	w.Grant("user", alex, space, "read")
	helper := w.Agent("helper")
	w.SetOperator(ch, helper)
	w.Send("owner", main, "one", http.StatusCreated)
	w.Send("owner", main, "two @alex", http.StatusCreated)
	w.Want(w.Do("alex", http.MethodPut, "/api/v1/threads/"+main.String()+"/read-state", `{"last_read_seq":1}`), http.StatusOK, "")
	w.Want(w.Do("alex", http.MethodPut, "/api/v1/threads/"+main.String()+"/draft", `{"content_md":"d"}`), http.StatusOK, "")

	ctx := actor.WithActor(context.Background(), w.Actor("alex"))
	v, err := w.Sync.Snapshot.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Principal.ID != alex || v.IsOwner || len(v.Spaces) != 1 || len(v.Channels) != 2 || len(v.Drafts) != 1 || len(v.Features) == 0 {
		t.Fatalf("snapshot %+v", v)
	}
	if len(v.UnreadThreads) != 1 || v.UnreadThreads[0] != (syncmod.UnreadThread{ThreadID: main, ChannelID: ch, UnreadCount: 1, MentionCount: 1,
		LastMessageSeq: 2, LastReadSeq: 1}) {
		t.Fatalf("unread %+v", v.UnreadThreads)
	}
	if len(v.Agents) != 1 || v.Agents[0].AgentID != helper || v.Agents[0].State != "unknown" || len(v.ActiveRuns) != 0 {
		t.Fatalf("agents %+v, runs %+v", v.Agents, v.ActiveRuns)
	}

	ag, err := w.Sync.Snapshot.Build(actor.WithActor(context.Background(), w.Actor("helper")))
	if err != nil || len(ag.Sidebar)+len(ag.SidebarChannels)+len(ag.UnreadThreads)+len(ag.Settings)+len(ag.Drafts) != 0 || ag.Principal.Kind != "agent" {
		t.Fatalf("agent snapshot %+v %v", ag, err)
	}
	if _, err := w.Sync.Snapshot.Build(context.Background()); errs.KindOf(err) != errs.KindUnauthenticated {
		t.Fatalf("anonymous: %v", err)
	}
	owner, err := w.Sync.Snapshot.Build(actor.WithActor(context.Background(), w.Actor("owner")))
	if err != nil || !owner.IsOwner || len(owner.Spaces) != 2 {
		t.Fatalf("owner snapshot %+v %v", owner, err)
	}

	res, err := w.Sync.CatchUp.Check(ctx, syncmod.CheckInput{})
	if err != nil || res.Reason != "" || res.HeadSeq != v.HeadSeq {
		t.Fatalf("check %+v %v", res, err)
	}
	after := int64(0)
	for after < res.HeadSeq {
		page, err := w.Sync.CatchUp.Page(ctx, syncmod.PageInput{After: after, Head: res.HeadSeq})
		if err != nil || page.NextCursor <= after {
			t.Fatalf("page after %d: %+v %v", after, page, err)
		}
		after = page.NextCursor
	}
}
