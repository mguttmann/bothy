// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// errBoom is the error a fake returns for a method named in its fail set.
var errBoom = errors.New("boom")

// faults names the fake methods that fail with errBoom ("Threads.Create", "Store.Item", ...).
type faults struct {
	mu  sync.Mutex
	set map[string]bool
}

func (f *faults) on(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.set = map[string]bool{name: true}
}

func (f *faults) off() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.set = nil
}

func (f *faults) err(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.set[name] {
		return errBoom
	}
	return nil
}

// faultyStore is the in-memory store with injectable errors on the reads and writes of items.
type faultyStore struct {
	*store
	f *faults
}

func (s faultyStore) Item(ctx context.Context, source ids.ID, key string) (app.ItemRow, bool, error) {
	if err := s.f.err("Store.Item"); err != nil {
		return app.ItemRow{}, false, err
	}
	return s.store.Item(ctx, source, key)
}

func (s faultyStore) InsertPendingItem(ctx context.Context, n app.NewItem) (app.ItemRow, bool, error) {
	if err := s.f.err("Store.InsertPendingItem"); err != nil {
		return app.ItemRow{}, false, err
	}
	return s.store.InsertPendingItem(ctx, n)
}

func (s faultyStore) FinishItem(ctx context.Context, id ids.ID, state, kind string, target ids.ID, at time.Time) (app.ItemRow, error) {
	if err := s.f.err("Store.FinishItem"); err != nil {
		return app.ItemRow{}, err
	}
	return s.store.FinishItem(ctx, id, state, kind, target, at)
}

func (s faultyStore) LockItem(ctx context.Context, source ids.ID, key string) (app.ItemRow, bool, error) {
	if err := s.f.err("Store.LockItem"); err != nil {
		return app.ItemRow{}, false, err
	}
	return s.store.LockItem(ctx, source, key)
}

func (s faultyStore) ItemOfTarget(ctx context.Context, source ids.ID, kind string, id ids.ID) (app.ItemRow, bool, error) {
	if err := s.f.err("Store.ItemOfTarget"); err != nil {
		return app.ItemRow{}, false, err
	}
	return s.store.ItemOfTarget(ctx, source, kind, id)
}

// threads fakes chat.ImportThreads.
type threads struct {
	f       *faults
	mu      sync.Mutex
	created []chat.ImportedThread
	state   map[ids.ID]chat.ImportedThreadState
	mirrors []chat.MirrorChange
	main    map[ids.ID]ids.ID
}

func (t *threads) Create(_ context.Context, in chat.ImportedThread) (ids.ID, error) {
	if err := t.f.err("Threads.Create"); err != nil {
		return ids.ID{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	id := ids.New()
	t.created = append(t.created, in)
	t.state[id] = chat.ImportedThreadState{ID: id, ChannelID: in.ChannelID, Title: in.Name, TitleSource: "manual", LastActivityAt: in.LastActivityAt}
	return id, nil
}

func (t *threads) MainThread(_ context.Context, channel ids.ID) (ids.ID, error) {
	if err := t.f.err("Threads.MainThread"); err != nil {
		return ids.ID{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id, ok := t.main[channel]; ok {
		return id, nil
	}
	id := ids.New()
	t.main[channel] = id
	return id, nil
}

func (t *threads) Lock(_ context.Context, id ids.ID) (chat.ImportedThreadState, error) {
	if err := t.f.err("Threads.Lock"); err != nil {
		return chat.ImportedThreadState{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state[id], nil
}

func (t *threads) ApplyMirror(_ context.Context, _ ids.ID, c chat.MirrorChange) error {
	if err := t.f.err("Threads.ApplyMirror"); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.mirrors = append(t.mirrors, c)
	return nil
}

func (t *threads) Reserve(context.Context, ids.ID, int, time.Time) (chat.Reserved, error) {
	return chat.Reserved{}, nil
}

func (t *threads) FinishBatch(context.Context, ids.ID, bool) (int64, error) {
	if err := t.f.err("Threads.FinishBatch"); err != nil {
		return 0, err
	}
	return 7, nil
}

// messages fakes messaging.ImportMessages.
type messages struct {
	f      *faults
	mu     sync.Mutex
	thread map[ids.ID]ids.ID
	pins   []ids.ID
}

func (m *messages) Append(_ context.Context, thread ids.ID, msgs []messaging.ImportedMessage) ([]ids.ID, error) {
	if err := m.f.err("Messages.Append"); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ids.ID, 0, len(msgs))
	for range msgs {
		id := ids.New()
		m.thread[id] = thread
		out = append(out, id)
	}
	return out, nil
}

func (m *messages) ThreadOf(_ context.Context, id ids.ID) (ids.ID, bool, error) {
	if err := m.f.err("Messages.ThreadOf"); err != nil {
		return ids.ID{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.thread[id]
	return t, ok, nil
}

func (m *messages) Pin(_ context.Context, _, message, _ ids.ID) error {
	if err := m.f.err("Messages.Pin"); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pins = append(m.pins, message)
	return nil
}

// filesPort fakes files.ImportFiles: a reserved file completes when Complete is called.
type filesPort struct {
	f    *faults
	mu   sync.Mutex
	done map[ids.ID]bool
}

func (p *filesPort) Reserve(context.Context, files.ImportReserve) (files.ImportSlot, error) {
	if err := p.f.err("Files.Reserve"); err != nil {
		return files.ImportSlot{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	id := ids.New()
	p.done[id] = false
	return files.ImportSlot{FileID: id, Upload: files.UploadSession{FileID: id, PartCount: 1}}, nil
}

func (p *filesPort) Slot(_ context.Context, id ids.ID) (files.ImportSlot, error) {
	if err := p.f.err("Files.Slot"); err != nil {
		return files.ImportSlot{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return files.ImportSlot{FileID: id, Completed: p.done[id], Upload: files.UploadSession{FileID: id, PartCount: 1}}, nil
}

func (p *filesPort) UploadPart(_ context.Context, cmd files.PutPart) (files.PartRef, error) {
	if err := p.f.err("Files.UploadPart"); err != nil {
		return files.PartRef{}, err
	}
	return files.PartRef{PartNumber: cmd.PartNumber, ETag: "e"}, nil
}

func (p *filesPort) Complete(_ context.Context, cmd files.CompleteUpload) (files.File, error) {
	if err := p.f.err("Files.Complete"); err != nil {
		return files.File{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done[cmd.FileID] = true
	return files.File{}, nil
}

// contentEnv is the env with the content ports and injectable faults.
type contentEnv struct {
	*env
	f        *faults
	threads  *threads
	messages *messages
	files    *filesPort
	src      importer.Source
	items    map[string]importer.Item
}

func newContentEnv(t *testing.T) *contentEnv {
	t.Helper()
	f := &faults{}
	c := &contentEnv{f: f, threads: &threads{f: f, state: map[ids.ID]chat.ImportedThreadState{}, main: map[ids.ID]ids.ID{}},
		messages: &messages{f: f, thread: map[ids.ID]ids.ID{}}, files: &filesPort{f: f, done: map[ids.ID]bool{}}}
	c.env = newEnvWith(t, func(d *app.Deps) {
		d.Store = faultyStore{store: d.Store.(*store), f: f}
		d.Threads, d.Messages, d.Files = c.threads, c.messages, c.files
	})
	c.src = c.running(t)
	res, err := c.svc.ApplyBatch(context.Background(), c.owner, c.src.ID, structureBatch())
	if err != nil {
		t.Fatal(err)
	}
	c.items = byKey(res.Items)
	return c
}

// thread is a thread batch.
func thread(key string, items ...importer.BatchObject) importer.Batch {
	return importer.Batch{ThreadOriginKey: key, ThreadOriginKeySet: true, Items: items}
}

const (
	forumThread = `{"channel_origin_key":"channel:21","name":"Q","created_at":"2025-03-01T08:00:00Z","last_activity_at":"2025-03-01T08:00:00Z",` +
		`"closed":false,"starter_message_origin_key":"message:50","tag_origin_keys":["forum_tag:21:5"]}`
	textThread = `{"channel_origin_key":"channel:20","name":"T","created_at":"2025-03-01T08:00:00Z","last_activity_at":"2025-03-01T08:00:00Z",` +
		`"closed":false}`
	alexMsg = `{"author_origin_key":"user:30","content_md":"hi","created_at":"2025-03-01T08:00:00Z"}`
)

// TestContentBatches writes main thread, new thread, mirror, message, reply and pin items through
// the ports and checks the refusals that need no port error.
func TestContentBatches(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	apply := func(b importer.Batch) (importer.BatchResult, error) {
		return c.svc.ApplyBatch(ctx, c.owner, c.src.ID, b)
	}
	// The main thread of the text channel gets the starter message of the forum thread.
	res, err := apply(thread("channel:20", o("message:50", "message", alexMsg)))
	if err != nil || res.UnreadFloorSeq == nil || *res.UnreadFloorSeq != 7 {
		t.Fatalf("main thread %+v %v", res, err)
	}
	res, err = apply(thread("thread:40", o("thread:40", "thread", forumThread),
		o("message:51", "message", `{"author_origin_key":"user:30","content_md":"x","created_at":"2025-03-01T08:00:00Z","reply_to_origin_key":"message:50"}`),
		o("pin:51", "pin", `{"message_origin_key":"message:51"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.threads.created) != 1 || c.threads.created[0].Parent == nil || len(c.threads.created[0].TagIDs) != 1 || len(c.messages.pins) != 1 {
		t.Errorf("created %+v pins %v", c.threads.created, c.messages.pins)
	}
	// A posted thread item mirrors a new name.
	if _, err := apply(thread("thread:40", o("thread:40", "thread", strings.Replace(forumThread, `"Q"`, `"Renamed"`, 1)))); err != nil {
		t.Fatal(err)
	}
	// The planned operator of channel 20 is Lotse.
	if _, err := apply(thread("thread:41", o("thread:41", "thread", textThread))); err != nil || c.threads.created[1].OperatorAgentID != c.lotse {
		t.Errorf("planned operator %+v %v", c.threads.created, err)
	}
	// A thread of a skipped channel is skipped; further items of its batch are refused.
	if r, err := apply(importer.Batch{Items: []importer.BatchObject{o("channel:23", "channel", `{"name":"gone","kind":"forum","sort_key":"a2"}`)}}); err != nil ||
		r.Items[0].State != importer.ItemSkipped {
		t.Fatalf("skipped channel %+v %v", r, err)
	}
	skipped := strings.Replace(forumThread, "channel:21", "channel:23", 1)
	skipped = strings.Replace(skipped, `"tag_origin_keys":["forum_tag:21:5"]`, `"tag_origin_keys":[]`, 1)
	if r, err := apply(thread("thread:45", o("thread:45", "thread", skipped))); err != nil || r.Items[0].State != importer.ItemSkipped || r.UnreadFloorSeq != nil {
		t.Errorf("skipped thread %+v %v", r, err)
	}
	refusals := []struct {
		name   string
		b      importer.Batch
		code   errs.Code
		reason string
	}{
		{"thread not imported", thread("thread:49", o("message:60", "message", alexMsg)), errs.CodeConflict, importer.ReasonThreadNotImported},
		{"channel not imported", thread("channel:29", o("message:61", "message", alexMsg)), errs.CodeConflict, importer.ReasonThreadNotImported},
		{"author not imported", thread("thread:40", o("message:62", "message", `{"author_origin_key":"user:99","content_md":"x","created_at":"2025-03-01T08:00:00Z"}`)),
			errs.CodeConflict, importer.ReasonAuthorNotImported},
		{"reply target", thread("thread:40", o("message:63", "message",
			`{"author_origin_key":"user:30","content_md":"x","created_at":"2025-03-01T08:00:00Z","reply_to_origin_key":"message:98"}`)), errs.CodeValidationFailed, ""},
		{"pin target", thread("thread:40", o("pin:97", "pin", `{"message_origin_key":"message:97"}`)), errs.CodeValidationFailed, ""},
		{"file not imported", thread("thread:40", o("message:64", "message",
			`{"author_origin_key":"user:30","content_md":"x","created_at":"2025-03-01T08:00:00Z","attachment_file_ids":["`+ids.New().String()+`"]}`)),
			errs.CodeValidationFailed, ""},
		{"tag of another channel", thread("thread:42", o("thread:42", "thread", strings.Replace(forumThread, "forum_tag:21:5", "forum_tag:20:5", 1))),
			errs.CodeValidationFailed, ""},
		{"skipped thread with items", thread("thread:46", o("thread:46", "thread", strings.Replace(skipped, "thread:45", "thread:46", 1)),
			o("message:65", "message", alexMsg)), errs.CodeConflict, importer.ReasonThreadNotImported},
		{"channel outside", thread("thread:43", o("thread:43", "thread", strings.Replace(textThread, "channel:20", "channel:28", 1))),
			errs.CodeConflict, importer.ReasonTargetOutsideSource},
	}
	for _, r := range refusals {
		_, err := apply(r.b)
		e, ok := errs.As(err)
		if !ok || e.Code != r.code || (r.reason != "" && e.Reason != r.reason) {
			t.Errorf("%s: %v", r.name, err)
		}
	}
}

// TestContentPortErrors: an error of any port or store call ends the thread batch with that error
// and writes nothing.
func TestContentPortErrors(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	if _, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, thread("channel:20", o("message:50", "message", alexMsg))); err != nil {
		t.Fatal(err)
	}
	if _, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, thread("thread:40", o("thread:40", "thread", forumThread))); err != nil {
		t.Fatal(err)
	}
	reply := `{"author_origin_key":"user:30","content_md":"x","created_at":"2025-03-01T08:00:00Z","reply_to_origin_key":"message:50"}`
	cases := []struct {
		fault string
		b     importer.Batch
	}{
		{"Store.Item", thread("thread:40", o("message:70", "message", alexMsg))},
		{"Store.Item", thread("channel:20", o("message:70", "message", alexMsg))},
		{"Store.Item", thread("thread:44", o("thread:44", "thread", forumThread))},
		{"Threads.MainThread", thread("channel:20", o("message:70", "message", alexMsg))},
		{"Store.InsertPendingItem", thread("thread:44", o("thread:44", "thread", forumThread))},
		{"Store.InsertPendingItem", thread("thread:40", o("message:70", "message", alexMsg))},
		{"Store.FinishItem", thread("thread:44", o("thread:44", "thread", forumThread))},
		{"Store.FinishItem", thread("thread:40", o("message:70", "message", alexMsg))},
		{"Threads.Create", thread("thread:44", o("thread:44", "thread", forumThread))},
		{"Messages.ThreadOf", thread("thread:44", o("thread:44", "thread", forumThread))},
		{"Threads.Lock", thread("thread:40", o("thread:40", "thread", strings.Replace(forumThread, `"Q"`, `"New"`, 1)))},
		{"Threads.ApplyMirror", thread("thread:40", o("thread:40", "thread", strings.Replace(forumThread,
			`"last_activity_at":"2025-03-01T08:00:00Z"`, `"last_activity_at":"2025-03-02T08:00:00Z"`, 1)))},
		{"Threads.FinishBatch", thread("thread:40", o("message:70", "message", alexMsg))},
		{"Messages.Append", thread("thread:40", o("message:70", "message", alexMsg))},
		{"Messages.Append", thread("thread:40", o("message:71", "message", reply))},
		{"Messages.Pin", thread("thread:40", o("pin:50", "pin", `{"message_origin_key":"message:50"}`))},
		{"Store.ItemOfTarget", thread("thread:40", o("message:72", "message",
			`{"author_origin_key":"user:30","content_md":"x","created_at":"2025-03-01T08:00:00Z","attachment_file_ids":["`+ids.New().String()+`"]}`))},
	}
	for _, tc := range cases {
		c.f.on(tc.fault)
		_, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, tc.b)
		c.f.off()
		if !errors.Is(err, errBoom) {
			t.Errorf("%s %s: %v", tc.fault, tc.b.ThreadOriginKey, err)
		}
	}
	if _, err := c.svc.ApplyBatch(ctx, c.owner, c.src.ID, thread("thread:40", o("message:70", "message", alexMsg))); err != nil {
		t.Errorf("after the faults: %v", err)
	}
}

// TestImportFiles: create, repeat, part, completion and the refusals of the file operations.
func TestImportFiles(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	channel, author := c.items["channel:21"].TargetID, c.items["user:30"].TargetID
	cmd := importer.FileCmd{OriginKey: "file:80", ChannelID: channel, AuthorID: author, Name: "a.txt", SizeBytes: 3, SHA256: strings.Repeat("ab", 32)}
	slot, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd)
	if err != nil || slot.Upload == nil || slot.Item.State != importer.ItemPending {
		t.Fatalf("create %+v %v", slot, err)
	}
	file := slot.Item.TargetID
	if again, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd); err != nil || again.Upload == nil || again.Item.TargetID != file {
		t.Errorf("repeat %+v %v", again, err)
	}
	if ref, err := c.svc.UploadFilePart(ctx, c.owner, c.src.ID, importer.FilePartCmd{FileID: file, PartNumber: 1}); err != nil || ref.ETag != "e" {
		t.Errorf("part %+v %v", ref, err)
	}
	done, err := c.svc.CompleteFile(ctx, c.owner, c.src.ID, importer.FileCompleteCmd{FileID: file, Parts: []importer.PartRef{{PartNumber: 1, ETag: "e"}}})
	if err != nil || done.Item.State != importer.ItemPosted {
		t.Errorf("complete %+v %v", done, err)
	}
	if again, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd); err != nil || again.Upload != nil || again.Item.State != importer.ItemPosted {
		t.Errorf("posted repeat %+v %v", again, err)
	}
	// A pending key whose session completed meanwhile is posted by the repeat.
	cmd2 := cmd
	cmd2.OriginKey = "file:81"
	slot2, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd2)
	if err != nil {
		t.Fatal(err)
	}
	c.files.done[slot2.Item.TargetID] = true
	if again, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd2); err != nil || again.Item.State != importer.ItemPosted {
		t.Errorf("completed meanwhile %+v %v", again, err)
	}
	bad := cmd
	bad.OriginKey, bad.SHA256 = "user:1", "xyz"
	if e := fieldsOf(t, mustErr(c.svc.CreateFile(ctx, c.owner, c.src.ID, bad))); !hasField(e, "/sha256", "invalid") || len(e.Fields) != 2 {
		t.Errorf("body %+v", e.Fields)
	}
	outside := cmd
	outside.OriginKey, outside.ChannelID = "file:82", ids.New()
	wantErr(t, mustErr(c.svc.CreateFile(ctx, c.owner, c.src.ID, outside)), errs.CodeConflict, importer.ReasonTargetOutsideSource)
	stranger := cmd
	stranger.OriginKey, stranger.AuthorID = "file:83", ids.New()
	wantErr(t, mustErr(c.svc.CreateFile(ctx, c.owner, c.src.ID, stranger)), errs.CodeConflict, importer.ReasonAuthorNotImported)
	_, err = c.svc.UploadFilePart(ctx, c.owner, c.src.ID, importer.FilePartCmd{FileID: ids.New(), PartNumber: 1})
	wantErr(t, err, errs.CodeNotFound, "")
	wantErr(t, mustErr(c.svc.CompleteFile(ctx, c.owner, c.src.ID, importer.FileCompleteCmd{FileID: ids.New()})), errs.CodeNotFound, "")
	// Port and store errors end the call with the error.
	cmd3 := cmd
	cmd3.OriginKey = "file:84"
	pending, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd3)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"Store.ItemOfTarget", "Store.InsertPendingItem", "Files.Reserve", "Store.FinishItem"} {
		c.f.on(fault)
		k := cmd
		k.OriginKey = "file:85"
		_, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, k)
		c.f.off()
		if !errors.Is(err, errBoom) {
			t.Errorf("create %s: %v", fault, err)
		}
	}
	for _, fault := range []string{"Store.LockItem", "Files.Slot"} {
		c.f.on(fault)
		_, err := c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd3)
		c.f.off()
		if !errors.Is(err, errBoom) {
			t.Errorf("repeat %s: %v", fault, err)
		}
	}
	for _, fault := range []string{"Store.ItemOfTarget", "Store.LockItem", "Files.UploadPart"} {
		c.f.on(fault)
		_, err := c.svc.UploadFilePart(ctx, c.owner, c.src.ID, importer.FilePartCmd{FileID: pending.Item.TargetID, PartNumber: 1})
		c.f.off()
		if !errors.Is(err, errBoom) {
			t.Errorf("part %s: %v", fault, err)
		}
	}
	for _, fault := range []string{"Store.ItemOfTarget", "Files.Complete", "Store.FinishItem"} {
		c.f.on(fault)
		_, err := c.svc.CompleteFile(ctx, c.owner, c.src.ID, importer.FileCompleteCmd{FileID: pending.Item.TargetID})
		c.f.off()
		if !errors.Is(err, errBoom) {
			t.Errorf("complete %s: %v", fault, err)
		}
	}
	// Another principal on the context, an unknown source.
	other := actor.WithActor(ctx, actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	wantErr(t, mustErr(c.svc.CreateFile(other, c.owner, c.src.ID, cmd3)), errs.CodeInternalError, "")
	_, err = c.svc.UploadFilePart(other, c.owner, c.src.ID, importer.FilePartCmd{FileID: pending.Item.TargetID, PartNumber: 1})
	wantErr(t, err, errs.CodeInternalError, "")
	wantErr(t, mustErr(c.svc.CompleteFile(other, c.owner, c.src.ID, importer.FileCompleteCmd{FileID: pending.Item.TargetID})), errs.CodeInternalError, "")
	wantErr(t, mustErr(c.svc.CreateFile(ctx, c.owner, ids.New(), cmd3)), errs.CodeNotFound, "")
	// A paused source refuses every file operation.
	if _, err := c.svc.Pause(ctx, c.owner, c.src.ID); err != nil {
		t.Fatal(err)
	}
	wantErr(t, mustErr(c.svc.CreateFile(ctx, c.owner, c.src.ID, cmd3)), errs.CodeConflict, importer.ReasonImportNotRunning)
	_, err = c.svc.UploadFilePart(ctx, c.owner, c.src.ID, importer.FilePartCmd{FileID: pending.Item.TargetID, PartNumber: 1})
	wantErr(t, err, errs.CodeConflict, importer.ReasonImportNotRunning)
	wantErr(t, mustErr(c.svc.CompleteFile(ctx, c.owner, c.src.ID, importer.FileCompleteCmd{FileID: pending.Item.TargetID})),
		errs.CodeConflict, importer.ReasonImportNotRunning)
}

// TestContentWithoutPorts: a service without the content ports answers internal for a thread
// batch and the file operations.
func TestContentWithoutPorts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	s := e.running(t)
	_, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, thread("channel:20", o("message:50", "message", alexMsg)))
	wantErr(t, err, errs.CodeInternalError, "")
	wantErr(t, mustErr(e.svc.CreateFile(ctx, e.owner, s.ID, importer.FileCmd{})), errs.CodeInternalError, "")
	_, err = e.svc.UploadFilePart(ctx, e.owner, s.ID, importer.FilePartCmd{})
	wantErr(t, err, errs.CodeInternalError, "")
	wantErr(t, mustErr(e.svc.CompleteFile(ctx, e.owner, s.ID, importer.FileCompleteCmd{})), errs.CodeInternalError, "")
}

// mustErr drops the value of a call and keeps its error.
func mustErr(_ importer.FileSlot, err error) error { return err }
