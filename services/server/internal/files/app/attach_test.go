// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// place is a message the tests attach to: the message, its thread and its channel.
type place struct{ message, thread, channel ids.ID }

func newPlace() place { return place{message: ids.New(), thread: ids.New(), channel: ids.New()} }

// seedFile stores a file of uploader for channel with a session in state and the upload and
// scan states, and returns it.
func (e *env) seedFile(uploader, channel ids.ID, state domain.SessionState, upload, scan string) app.FileRecord {
	id := ids.New()
	f := app.FileRecord{ID: id, ObjectKey: "f/" + id.String(), SHA256: [32]byte{1}, SizeBytes: 10, MIMEType: "text/plain",
		OriginalName: "notes.txt", UploadedBy: uploader, Purpose: string(domain.PurposeAttachment), ScopeChannelID: channel,
		UploadState: upload, ScanStatus: scan, TranscriptStatus: "none", CreatedAt: e.clk.Now()}
	e.repo.mu.Lock()
	defer e.repo.mu.Unlock()
	e.repo.files[id] = f
	e.repo.sessions[id] = app.SessionRecord{FileID: id, Mode: "direct", State: state, File: f}
	return f
}

// attach runs Attach in a unit of work.
func (e *env) attach(p place, author ids.ID, fileIDs ...ids.ID) ([]files.Attachment, error) {
	var out []files.Attachment
	err := e.uow.Do(e.ctx, func(ctx context.Context) error {
		var err error
		out, err = e.svc.Attach(ctx, files.AttachCmd{MessageID: p.message, ThreadID: p.thread, ChannelID: p.channel, AuthorID: author,
			FileIDs: fileIDs, Pointer: "/attachment_file_ids"})
		return err
	})
	return out, err
}

// TestAttachOrderPositionsAndCopies holds design notes 3 and 4 d and e: the rows in request order
// with position = index + 1 and the copies of the command, the result in the attached form
// (no scope_channel_id, sha256 only for a complete file, thumbnails included); blocked and pending
// files are attachable.
func TestAttachOrderPositionsAndCopies(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p := newPlace()
	pending := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	clean := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	infected := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanInfected)
	e.repo.thumbs[clean.ID] = []app.ThumbnailRecord{{Variant: domain.VariantSmall, MIMEType: "image/jpeg", Width: 2, Height: 1}}
	got, err := e.attach(p, e.alex.ID, clean.ID, pending.ID, infected.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []ids.ID{clean.ID, pending.ID, infected.ID}
	if len(got) != len(want) {
		t.Fatalf("%d attachments", len(got))
	}
	for i, a := range got {
		if a.Position != i+1 || a.File.ID != want[i] || !a.File.ScopeChannelID.IsZero() || a.File.Purpose != "attachment" {
			t.Errorf("attachment %d = %+v", i, a)
		}
		row := e.repo.attachments[want[i]]
		if row != (app.AttachmentRecord{MessageID: p.message, FileID: want[i], ThreadID: p.thread, ChannelID: p.channel, Position: i + 1}) {
			t.Errorf("row %d = %+v", i, row)
		}
	}
	if got[0].File.SHA256 == "" || got[1].File.SHA256 != "" || len(got[0].File.Thumbnails) != 1 || got[1].File.Thumbnails == nil {
		t.Errorf("attached form: clean %+v, pending %+v", got[0].File, got[1].File)
	}
	if calls := e.repo.callList(); !reflect.DeepEqual(calls, []string{"LockAttachTargets", "AttachedFileIDs", "InsertAttachments"}) {
		t.Errorf("calls %v: the lock comes before the read of message_attachments", calls)
	}
	if len(e.events.list) != 0 {
		t.Errorf("Attach appended %d events", len(e.events.list))
	}
}

// TestAttachNotAttachable holds 12 10 point 1 and design note 4 c (SEC-UP-9): every refused file
// answers the same 409 conflict file_not_attachable with the pointer of its entry, and nothing is
// stored.
func TestAttachNotAttachable(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p := newPlace()
	sam := ids.New()
	ok := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	foreign := e.seedFile(sam, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	otherChannel := e.seedFile(e.alex.ID, ids.New(), domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	open := e.seedFile(e.alex.ID, p.channel, domain.SessionOpen, domain.UploadPending, domain.ScanPending)
	aborted := e.seedFile(e.alex.ID, p.channel, domain.SessionAborted, domain.UploadFailed, domain.ScanPending)
	expired := e.seedFile(e.alex.ID, p.channel, domain.SessionExpired, domain.UploadPending, domain.ScanPending)
	failed := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadFailed, domain.ScanPending)
	avatar := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	avatar.Purpose = string(domain.PurposeAvatar)
	e.repo.files[avatar.ID] = avatar
	deleted := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	deleted.Deleted = true
	e.repo.files[deleted.ID] = deleted
	elsewhere := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.repo.attachments[elsewhere.ID] = app.AttachmentRecord{MessageID: ids.New(), FileID: elsewhere.ID, ThreadID: p.thread, ChannelID: p.channel, Position: 1}
	var first *errs.Error
	for _, c := range []struct {
		name  string
		list  []ids.ID
		index string
	}{
		{"unknown id", []ids.ID{ok.ID, ids.New()}, "1"},
		{"foreign uploader", []ids.ID{foreign.ID}, "0"},
		{"other channel (SEC-UP-9)", []ids.ID{ok.ID, otherChannel.ID}, "1"},
		{"open session", []ids.ID{open.ID}, "0"},
		{"aborted", []ids.ID{aborted.ID}, "0"},
		{"expired", []ids.ID{expired.ID}, "0"},
		{"upload failed", []ids.ID{failed.ID}, "0"},
		{"purpose avatar", []ids.ID{avatar.ID}, "0"},
		{"deleted", []ids.ID{deleted.ID}, "0"},
		{"attached to another message", []ids.ID{elsewhere.ID}, "0"},
		{"the same id twice", []ids.ID{ok.ID, ok.ID}, "1"},
	} {
		_, err := e.attach(p, e.alex.ID, c.list...)
		got, isErr := errs.As(err)
		if !isErr || problem(err) != "conflict/file_not_attachable" || got.Detail != "/attachment_file_ids/"+c.index {
			t.Errorf("%s: %v (%s)", c.name, err, problem(err))
			continue
		}
		if first == nil {
			first = got
		} else if got.Code != first.Code || got.Reason != first.Reason || !reflect.DeepEqual(got.Headers, first.Headers) ||
			got.RetryAfter != first.RetryAfter {
			t.Errorf("%s: answer %+v differs from %+v", c.name, got, first)
		}
	}
	if len(e.repo.attachments) != 1 {
		t.Errorf("%d rows after the refusals, want only the fixture", len(e.repo.attachments))
	}
}

// TestAttachUniqueViolation holds design note 4 d: a unique violation on file_id at the insert is
// the same 409 with the pointer of the list.
func TestAttachUniqueViolation(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p := newPlace()
	f := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	e.repo.insertErr = app.ErrAlreadyAttached
	_, err := e.attach(p, e.alex.ID, f.ID)
	if got, ok := errs.As(err); !ok || problem(err) != "conflict/file_not_attachable" || got.Detail != "/attachment_file_ids" {
		t.Fatalf("unique violation: %v (%s)", err, problem(err))
	}
	e.repo.insertErr = errors.New("connection lost")
	if _, err := e.attach(p, e.alex.ID, f.ID); err == nil || problem(err) == "conflict/file_not_attachable" {
		t.Fatalf("another insert error: %v", err)
	}
}

// TestAttachmentsNeedUnit holds design note 3: the writers and Pending refuse to run outside a
// unit of work with an internal error; ForMessages and MessageOf read on the pool.
func TestAttachmentsNeedUnit(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	ctx := context.Background()
	if _, err := e.svc.Attach(ctx, files.AttachCmd{FileIDs: []ids.ID{ids.New()}}); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Attach outside a unit: %v", err)
	}
	if err := e.svc.DeleteForMessage(ctx, ids.New()); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("DeleteForMessage outside a unit: %v", err)
	}
	if _, err := e.svc.Pending(ctx, ids.New()); errs.KindOf(err) != errs.KindInternal {
		t.Errorf("Pending outside a unit: %v", err)
	}
	if m, err := e.svc.ForMessages(ctx, []ids.ID{ids.New()}); err != nil || len(m) != 0 {
		t.Errorf("ForMessages on the pool: %v %v", m, err)
	}
	if _, ok, err := e.svc.MessageOf(ctx, ids.New()); ok || err != nil {
		t.Errorf("MessageOf on the pool: %v %v", ok, err)
	}
	e.repo.calls = nil
	if got, err := e.attach(newPlace(), e.alex.ID); err != nil || got == nil || len(got) != 0 || len(e.repo.callList()) != 0 {
		t.Errorf("an empty list: %v %v, calls %v", got, err, e.repo.callList())
	}
}

// TestForMessages holds design note 6: the attachments of several messages in position order in
// the attached form; a message without attachments is absent; no call for no message.
func TestForMessages(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p, q := newPlace(), newPlace()
	q.thread, q.channel = p.thread, p.channel
	f1 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	f2 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	f3 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	if _, err := e.attach(p, e.alex.ID, f2.ID, f1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.attach(q, e.alex.ID, f3.ID); err != nil {
		t.Fatal(err)
	}
	none := ids.New()
	got, err := e.svc.ForMessages(e.ctx, []ids.ID{p.message, q.message, none})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got[none]; ok || len(got) != 2 {
		t.Fatalf("messages %d", len(got))
	}
	pl := got[p.message]
	if len(pl) != 2 || pl[0].File.ID != f2.ID || pl[1].File.ID != f1.ID || pl[0].Position != 1 || pl[1].Position != 2 ||
		!pl[0].File.ScopeChannelID.IsZero() || pl[1].File.SHA256 == "" || pl[0].File.SHA256 != "" {
		t.Errorf("message p: %+v", pl)
	}
	if ql := got[q.message]; len(ql) != 1 || ql[0].File.ID != f3.ID {
		t.Errorf("message q: %+v", ql)
	}
	before := len(e.repo.callList())
	if m, err := e.svc.ForMessages(e.ctx, nil); err != nil || m == nil || len(m) != 0 || len(e.repo.callList()) != before {
		t.Errorf("no message: %v %v", m, err)
	}
}

// TestDeleteForMessage holds design note 7 b and c: the files locked, the rows gone, deleted_at at
// the clock time, no file.updated and no FileFinalized; a message without attachments writes
// nothing.
func TestDeleteForMessage(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p, other := newPlace(), newPlace()
	f1 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	f2 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	keep := e.seedFile(e.alex.ID, other.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	if _, err := e.attach(p, e.alex.ID, f1.ID, f2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.attach(other, e.alex.ID, keep.ID); err != nil {
		t.Fatal(err)
	}
	e.repo.calls = nil
	del := func(m ids.ID) error {
		return e.uow.Do(e.ctx, func(ctx context.Context) error { return e.svc.DeleteForMessage(ctx, m) })
	}
	if err := del(p.message); err != nil {
		t.Fatal(err)
	}
	if calls := e.repo.callList(); !reflect.DeepEqual(calls, []string{"LockMessageFiles", "DeleteMessageAttachments", "MarkFilesDeleted"}) {
		t.Errorf("calls %v: files first, then the rows, then deleted_at", calls)
	}
	for _, id := range []ids.ID{f1.ID, f2.ID} {
		if _, ok := e.repo.attachments[id]; ok || !e.repo.files[id].Deleted || !e.repo.deletedAt[id].Equal(e.clk.Now().UTC().Truncate(1000)) {
			t.Errorf("file %s: row %v deleted %v at %v", id, ok, e.repo.files[id].Deleted, e.repo.deletedAt[id])
		}
	}
	if _, ok := e.repo.attachments[keep.ID]; !ok || e.repo.files[keep.ID].Deleted {
		t.Error("the attachment of another message changed")
	}
	if len(e.events.list) != 0 || e.finalCount(f1.ID) != 0 {
		t.Errorf("%d events, %d FileFinalized", len(e.events.list), e.finalCount(f1.ID))
	}
	e.repo.calls = nil
	if err := del(ids.New()); err != nil || !reflect.DeepEqual(e.repo.callList(), []string{"LockMessageFiles"}) {
		t.Errorf("no attachments: %v, calls %v", err, e.repo.callList())
	}
}

// TestMessageOfAndPending holds design notes 3 and 11 a: MessageOf reports the copies and the
// position; Pending is true while one attachment is not final (12 2) and false for a message
// without attachments.
func TestMessageOfAndPending(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p := newPlace()
	f1 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	f2 := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadPending, domain.ScanPending)
	if _, err := e.attach(p, e.alex.ID, f1.ID, f2.ID); err != nil {
		t.Fatal(err)
	}
	ref, ok, err := e.svc.MessageOf(e.ctx, f2.ID)
	if err != nil || !ok || ref != (files.AttachmentRef{MessageID: p.message, ThreadID: p.thread, ChannelID: p.channel, Position: 2}) {
		t.Fatalf("MessageOf = %+v %v %v", ref, ok, err)
	}
	pending := func(m ids.ID) bool {
		t.Helper()
		var got bool
		if err := e.uow.Do(e.ctx, func(ctx context.Context) error {
			var err error
			got, err = e.svc.Pending(ctx, m)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if !pending(p.message) {
		t.Error("a pending attachment is not pending")
	}
	for _, final := range [][2]string{{domain.UploadComplete, domain.ScanInfected}, {domain.UploadComplete, domain.ScanError},
		{domain.UploadFailed, domain.ScanPending}, {domain.UploadComplete, domain.ScanSkipped}} {
		f := e.repo.files[f2.ID]
		f.UploadState, f.ScanStatus = final[0], final[1]
		e.repo.files[f2.ID] = f
		if pending(p.message) {
			t.Errorf("%v is final", final)
		}
	}
	f := e.repo.files[f2.ID]
	f.UploadState, f.ScanStatus = domain.UploadPending, domain.ScanClean
	e.repo.files[f2.ID] = f
	if !pending(p.message) {
		t.Error("an image waiting for its original is not final")
	}
	if pending(ids.New()) {
		t.Error("a message without attachments is pending")
	}
}

// TestFileResourceAttached holds design note 9: getFile of an attached file asks file.read on the
// channel and thread of its message with the attached facts and shows no scope_channel_id; an
// unattached file asks on its scope channel with the uploader as owner.
func TestFileResourceAttached(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	p := newPlace()
	f := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	got, err := e.svc.Get(e.ctx, f.ID)
	if err != nil || got.ScopeChannelID != p.channel {
		t.Fatalf("unattached Get = %+v %v", got, err)
	}
	want := authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: p.channel, OwnerID: e.alex.ID,
		File: &authorize.FileFacts{Purpose: "attachment"}}
	if r := e.az.resources[len(e.az.resources)-1]; !reflect.DeepEqual(r, want) {
		t.Errorf("unattached resource %+v %+v", r, r.File)
	}
	moved := ids.New()
	e.repo.attachments[f.ID] = app.AttachmentRecord{MessageID: p.message, FileID: f.ID, ThreadID: p.thread, ChannelID: moved, Position: 1}
	got, err = e.svc.Get(e.ctx, f.ID)
	if err != nil || !got.ScopeChannelID.IsZero() {
		t.Fatalf("attached Get = %+v %v", got, err)
	}
	want = authorize.Resource{Kind: authorize.ResourceFile, ID: f.ID, ChannelID: moved, ThreadID: p.thread, OwnerID: e.alex.ID,
		File: &authorize.FileFacts{Purpose: "attachment", Attached: true}}
	if r := e.az.resources[len(e.az.resources)-1]; !reflect.DeepEqual(r, want) {
		t.Errorf("attached resource %+v %+v", r, r.File)
	}
	e.az.rules[authorize.ActionFileRead] = authorize.Decision{Outcome: authorize.OutcomeNotFound}
	_, errAttached := e.svc.Get(e.ctx, f.ID)
	_, errUnknown := e.svc.Get(e.ctx, ids.New())
	if problem(errAttached) != "not_found" || problem(errUnknown) != "not_found" {
		t.Errorf("refused %s, unknown %s", problem(errAttached), problem(errUnknown))
	}
}

// TestRecordFileChangeTarget holds design note 10 a: file.updated of an unattached file goes to
// the uploader in the scope of its channel, of an attached file without audience to the scope of
// the message with the thread and without scope_channel_id in the payload.
func TestRecordFileChangeTarget(t *testing.T) {
	e := newScanEnv(t, app.ScanConfig{Mode: app.ScanModeRequired, Timeout: 5e9, HealthInterval: 1e9, RetryBackoffCap: 6e10, SniffBytes: 64})
	alone := e.seed(t, []byte("plain text"), []byte("plain text"), true)
	attached := e.seed(t, []byte("plain text"), []byte("plain text"), true)
	thread, channel := ids.New(), ids.New()
	e.repo.attachments[attached.FileID] = app.AttachmentRecord{MessageID: ids.New(), FileID: attached.FileID, ThreadID: thread, ChannelID: channel, Position: 1}
	for _, s := range []app.SessionRecord{alone, attached} {
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.events.list) != 2 {
		t.Fatalf("%d events", len(e.events.list))
	}
	one, two := e.events.list[0], e.events.list[1]
	if one.AudiencePrincipalID != alone.File.UploadedBy || one.ChannelID != alone.File.ScopeChannelID || !one.ThreadID.IsZero() ||
		one.SpaceID.IsZero() || !bytes.Contains(one.Payload, []byte(`"scope_channel_id"`)) {
		t.Errorf("unattached event %+v %s", one, one.Payload)
	}
	if !two.AudiencePrincipalID.IsZero() || two.ChannelID != channel || two.ThreadID != thread || two.SpaceID.IsZero() ||
		bytes.Contains(two.Payload, []byte(`"scope_channel_id"`)) || two.ActorKind != "" {
		t.Errorf("attached event %+v %s", two, two.Payload)
	}
	if e.finalCount(alone.FileID) != 1 || e.finalCount(attached.FileID) != 1 {
		t.Errorf("FileFinalized %d and %d, want once each", e.finalCount(alone.FileID), e.finalCount(attached.FileID))
	}
}

// TestAbortPublishesFileFinalized holds design note 10 b: the abort of an open upload makes the
// file failed, which is final, so it publishes FileFinalized once; aborting again publishes
// nothing.
func TestAbortPublishesFileFinalized(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	s, _ := uploadAll(t, e, e.ctx)
	for range 2 {
		if err := e.svc.Abort(e.ctx, s.FileID); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.finalCount(s.FileID); n != 1 {
		t.Fatalf("FileFinalized %d times, want once", n)
	}
	if ev := e.events.list[0]; ev.AudiencePrincipalID != e.alex.ID || ev.SpaceID != e.space || ev.ActorPrincipalID != e.alex.ID {
		t.Errorf("abort event %+v", ev)
	}
}

// TestSubscribersOnThreadMoved holds design note 8: the copies of the thread get the target
// channel inside the unit of the move; the subscriber needs no database connection to be built.
func TestSubscribersOnThreadMoved(t *testing.T) {
	if _, err := app.NewSubscribers(nil); err == nil {
		t.Fatal("NewSubscribers(nil) succeeded")
	}
	rows := &threadRows{}
	s, err := app.NewSubscribers(rows)
	if err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, "fs", testCfg)
	ev := chat.ThreadMoved{ThreadID: ids.New(), FromChannelID: ids.New(), ToChannelID: ids.New()}
	if err := e.uow.Do(e.ctx, func(ctx context.Context) error { return s.OnThreadMoved(ctx, ev) }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rows.moves, [][2]ids.ID{{ev.ThreadID, ev.ToChannelID}}) {
		t.Errorf("moves %v", rows.moves)
	}
}
