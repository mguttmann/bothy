// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestThreadDeleteRights (05 thread.delete, D-DEL-4): write and (creator or manage); a person with
// write deletes their own chat, not one of somebody else; an owner deletes any. thread.restore
// follows the same rule.
func TestThreadDeleteRights(t *testing.T) {
	w := newWorld(t)
	own := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: own, ChannelID: w.allgemein, CreatedBy: w.alex})
	mine := authorize.Resource{Kind: authorize.ResourceThread, ID: own, OwnerID: w.alex}
	theirs := authorize.Resource{Kind: authorize.ResourceThread, ID: own, OwnerID: w.sam}
	for _, act := range []authorize.Action{authorize.ActionThreadDelete, authorize.ActionThreadRestore} {
		w.allow(t, w.user(w.alex), act, mine)
		w.expect(t, w.user(w.alex), act, theirs, authorize.OutcomeForbidden, "not_self")
		w.allow(t, w.user(w.owner), act, theirs)
	}
}

// TestDeletedThread (07 4.5a, D-DEL-3): a deleted thread is not_found for every action on it, its
// messages and its runs, except thread.read (its deletion state and its events),
// thread.delete and thread.restore.
func TestDeletedThread(t *testing.T) {
	w := newWorld(t)
	gone := ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: gone, ChannelID: w.allgemein, CreatedBy: w.alex, Deleted: true})
	th := authorize.Resource{Kind: authorize.ResourceThread, ID: gone, OwnerID: w.alex}
	msg := authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: gone, OwnerID: w.alex}
	for _, act := range []authorize.Action{authorize.ActionThreadRead, authorize.ActionThreadDelete, authorize.ActionThreadRestore} {
		w.allow(t, w.user(w.alex), act, th)
	}
	// The runner still ends a run the deletion cancelled.
	run := authorize.Resource{Kind: authorize.ResourceRun, ID: ids.New(), ThreadID: gone, OwnerID: w.lotse}
	w.allow(t, w.agent(w.lotse), authorize.ActionRunFinish, run)
	notFound := authorize.OutcomeNotFound
	for _, c := range []struct {
		act authorize.Action
		r   authorize.Resource
	}{
		{authorize.ActionMessageCreate, th},
		{authorize.ActionThreadTitleUpdate, th},
		{authorize.ActionThreadClose, th},
		{authorize.ActionMessageRead, msg},
		{authorize.ActionMessageUpdate, msg},
		{authorize.ActionReactionAdd, msg},
	} {
		w.expect(t, w.user(w.alex), c.act, c.r, notFound, "no_grant")
		w.expect(t, w.user(w.owner), c.act, c.r, notFound, "no_grant")
	}
	// The other threads of the channel are untouched.
	w.allow(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.allgemein))
}
