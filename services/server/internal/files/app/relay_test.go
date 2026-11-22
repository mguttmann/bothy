// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// relay runs Relay of the source to the copy in a unit of work.
func (e *env) relay(source ids.ID, copied place) ([]files.Attachment, error) {
	var out []files.Attachment
	err := e.uow.Do(e.ctx, func(ctx context.Context) error {
		var err error
		out, err = e.svc.Relay(ctx, files.RelayCmd{SourceMessageID: source, MessageID: copied.message, ThreadID: copied.thread, ChannelID: copied.channel})
		return err
	})
	return out, err
}

// positionsAndIDs returns the positions and file ids of list.
func positionsAndIDs(list []files.Attachment) ([]int, []ids.ID) {
	var pos []int
	var fileIDs []ids.ID
	for _, a := range list {
		pos, fileIDs = append(pos, a.Position), append(fileIDs, a.File.ID)
	}
	return pos, fileIDs
}

// TestRelayPutsTheSourceFilesOnTheCopy holds FIX-TEAM-CHANNEL-ATTACHMENTS D-TCAT-1: the live files of
// the source become relays of the copy at the source's positions with the copy's thread and
// channel, in the attached form; a deleted file is skipped; the original row stays the one row
// of message_attachments (MessageOf still names the source); the copy of a copy relays the same
// files; a source without files inserts nothing; Relay needs a unit of work.
func TestRelayPutsTheSourceFilesOnTheCopy(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	src, work, other := newPlace(), newPlace(), newPlace()
	f1 := e.seedFile(e.alex.ID, src.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	gone := e.seedFile(e.alex.ID, src.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	f3 := e.seedFile(e.alex.ID, src.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	if _, err := e.attach(src, e.alex.ID, f1.ID, gone.ID, f3.ID); err != nil {
		t.Fatal(err)
	}
	g := e.repo.files[gone.ID]
	g.Deleted = true
	e.repo.files[gone.ID] = g

	list, err := e.relay(src.message, work)
	if err != nil {
		t.Fatal(err)
	}
	if pos, got := positionsAndIDs(list); !slices.Equal(pos, []int{1, 3}) || !slices.Equal(got, []ids.ID{f1.ID, f3.ID}) {
		t.Fatalf("relayed %v %v, want positions 1 and 3 of the live files", pos, got)
	}
	for _, a := range list {
		if !a.File.ScopeChannelID.IsZero() {
			t.Errorf("relayed file %s shows its scope channel", a.File.ID)
		}
	}
	for _, row := range e.repo.relays {
		if row.MessageID != work.message || row.ThreadID != work.thread || row.ChannelID != work.channel {
			t.Errorf("relay %+v, want the copy's place", row)
		}
	}
	if ref, ok, err := e.svc.MessageOf(e.ctx, f1.ID); err != nil || !ok || ref.MessageID != src.message {
		t.Errorf("MessageOf after the relay = %+v %v %v, want the source", ref, ok, err)
	}
	// The input made from a mirror relays the mirror's files, which are the reply's.
	list, err = e.relay(work.message, other)
	if pos, got := positionsAndIDs(list); err != nil || !slices.Equal(pos, []int{1, 3}) || !slices.Equal(got, []ids.ID{f1.ID, f3.ID}) {
		t.Fatalf("copy of a copy: %v %v %v", pos, got, err)
	}
	e.repo.calls = nil
	if list, err := e.relay(ids.New(), newPlace()); err != nil || len(list) != 0 || slices.Contains(e.repo.callList(), "InsertRelays") {
		t.Errorf("a source without files: %v %v, calls %v", list, err, e.repo.callList())
	}
	if _, err := e.svc.Relay(e.ctx, files.RelayCmd{SourceMessageID: src.message, MessageID: ids.New()}); err == nil {
		t.Error("Relay outside a unit of work succeeded")
	}
}

// TestDeleteForMessageWithRelays holds D-TCAT-3: deleting a team copy removes its relays and keeps
// the files and the source's rows; deleting the source removes the relays of its files on every
// copy and marks the files deleted.
func TestDeleteForMessageWithRelays(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	src, work, other := newPlace(), newPlace(), newPlace()
	f := e.seedFile(e.alex.ID, src.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	if _, err := e.attach(src, e.alex.ID, f.ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range []place{work, other} {
		if _, err := e.relay(src.message, p); err != nil {
			t.Fatal(err)
		}
	}
	del := func(m ids.ID) {
		t.Helper()
		if err := e.uow.Do(e.ctx, func(ctx context.Context) error { return e.svc.DeleteForMessage(ctx, m) }); err != nil {
			t.Fatal(err)
		}
	}
	e.repo.calls = nil
	del(work.message)
	if calls := e.repo.callList(); !reflect.DeepEqual(calls, []string{"DeleteRelaysOfMessage", "LockMessageFiles"}) {
		t.Errorf("deleting a copy: calls %v", calls)
	}
	if e.repo.files[f.ID].Deleted || len(e.repo.relays) != 1 || e.repo.relays[0].MessageID != other.message {
		t.Errorf("after deleting a copy: deleted %v, relays %+v", e.repo.files[f.ID].Deleted, e.repo.relays)
	}
	if _, ok := e.repo.attachments[f.ID]; !ok {
		t.Error("deleting a copy removed the source's row")
	}
	e.repo.calls = nil
	del(src.message)
	want := []string{"DeleteRelaysOfMessage", "LockMessageFiles", "DeleteMessageAttachments", "DeleteRelaysOfFiles", "MarkFilesDeleted",
		"DeleteTextExtractsOfDeleted"}
	if calls := e.repo.callList(); !reflect.DeepEqual(calls, want) {
		t.Errorf("deleting the source: calls %v, want %v", calls, want)
	}
	if !e.repo.files[f.ID].Deleted || len(e.repo.relays) != 0 {
		t.Errorf("after deleting the source: deleted %v, relays %+v", e.repo.files[f.ID].Deleted, e.repo.relays)
	}
}

// TestFileReadThroughRelay holds D-TCAT-2: file.read is decided on the original row first; when it
// denies, on the place of each team copy the file is relayed to, and an allow there serves the
// file; without a relay, or with every place denied, the original row's answer stands. file.delete
// never reads a relay.
func TestFileReadThroughRelay(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	src, work := newPlace(), newPlace()
	f := e.seedFile(e.alex.ID, src.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	if _, err := e.attach(src, e.alex.ID, f.ID); err != nil {
		t.Fatal(err)
	}
	e.az.rules[authorize.ActionFileRead] = authorize.Decision{Outcome: authorize.OutcomeNotFound}
	e.az.rules[authorize.ActionFileDelete] = authorize.Decision{Outcome: authorize.OutcomeNotFound}
	e.az.allowIn = map[ids.ID]bool{work.channel: true}
	if _, err := e.svc.Get(e.ctx, f.ID); problem(err) != "not_found" {
		t.Fatalf("a file relayed nowhere: %v, want not_found", err)
	}
	if _, err := e.relay(src.message, work); err != nil {
		t.Fatal(err)
	}
	got, err := e.svc.Get(e.ctx, f.ID)
	if err != nil || got.ID != f.ID || !got.ScopeChannelID.IsZero() {
		t.Fatalf("a file relayed to a readable copy: %+v %v", got, err)
	}
	last := e.az.resources[len(e.az.resources)-1]
	want := authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: work.channel, ThreadID: work.thread, OwnerID: e.alex.ID,
		File: &authorize.FileFacts{Purpose: "attachment", Attached: true}}
	if !reflect.DeepEqual(last, want) {
		t.Errorf("relay resource %+v %+v, want the copy's place", last, last.File)
	}
	e.az.allowIn = nil
	if _, err := e.svc.Get(e.ctx, f.ID); problem(err) != "not_found" {
		t.Errorf("every place denied: %v, want not_found", err)
	}
	e.az.allowIn = map[ids.ID]bool{work.channel: true}
	e.az.resources = nil
	if err := e.svc.Delete(e.ctx, f.ID); problem(err) != "not_found" {
		t.Errorf("delete through a relay: %v, want not_found", err)
	}
	for _, r := range e.az.resources {
		if r.ChannelID == work.channel {
			t.Errorf("file.delete asked on the copy's place: %+v", r)
		}
	}
}
