// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// The unit tests of M3-07 run the use cases over in-memory fakes with a fake files.Attachments,
// so they can count the calls of the files port (brief M3-07 design notes 5 d and 6). The HTTP
// and PostgreSQL behaviour is tested end to end elsewhere.

// attTrace records the order of the writes of the delete path across the store and files.
type attTrace struct {
	mu    sync.Mutex
	steps []string
}

func (t *attTrace) add(step string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, step)
}

func (t *attTrace) list() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.steps)
}

// attStore keeps the message rows in memory; reactions and pins are empty (bareStore).
type attStore struct {
	bareStore
	trace *attTrace
	mu    sync.Mutex
	rows  map[ids.ID]domain.Message
}

func newAttStore(trace *attTrace) *attStore {
	return &attStore{trace: trace, rows: map[ids.ID]domain.Message{}}
}

func (s *attStore) Message(_ context.Context, id ids.ID) (domain.Message, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.rows[id]
	return m, ok, nil
}

func (s *attStore) MessageByClientID(_ context.Context, author, clientMsgID ids.ID) (domain.Message, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.rows {
		if m.AuthorID == author && m.ClientMsgID == clientMsgID {
			return m, true, nil
		}
	}
	return domain.Message{}, false, nil
}

func (s *attStore) InsertMessage(_ context.Context, m domain.Message) (domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.SearchLanguage = ""
	s.rows[m.ID] = m
	return m, nil
}

func (s *attStore) InsertMentions(context.Context, ids.ID, ids.ID, []ids.ID, time.Time) error {
	return nil
}

func (s *attStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

func (s *attStore) VisibleMessage(ctx context.Context, id ids.ID, _ []ids.ID) (domain.Message, bool, error) {
	return s.Message(ctx, id)
}

func (s *attStore) LockMessage(ctx context.Context, id ids.ID, _ []ids.ID) (domain.Message, bool, error) {
	return s.Message(ctx, id)
}

// History returns every row of the thread, newest first, as the newest page reads them.
func (s *attStore) History(_ context.Context, q app.HistoryRead) (app.HistoryRows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var below []domain.Message
	for _, m := range s.rows {
		if m.ThreadID == q.ThreadID {
			below = append(below, m)
		}
	}
	slices.SortFunc(below, func(a, b domain.Message) int { return int(b.ThreadSeq - a.ThreadSeq) })
	return app.HistoryRows{Below: below}, nil
}

func (s *attStore) UpdateContent(_ context.Context, id ids.ID, _ []ids.ID, content string, at time.Time) (domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.rows[id]
	m.ContentMD, m.EditedAt, m.Version = content, at, m.Version+1
	s.rows[id] = m
	return m, nil
}

func (s *attStore) SetTriggerState(_ context.Context, id ids.ID, state, reason string, at time.Time) (domain.Message, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.rows[id]
	m.TriggerState, m.TriggerStateReason, m.TriggerStateAt, m.Version = state, reason, at, m.Version+1
	s.rows[id] = m
	return m, true, nil
}

func (s *attStore) InsertRevision(context.Context, domain.Revision, []ids.ID) error { return nil }

func (s *attStore) Tombstone(_ context.Context, id ids.ID, _ []ids.ID, by ids.ID, at time.Time) (domain.Message, error) {
	s.trace.add("tombstone")
	s.mu.Lock()
	defer s.mu.Unlock()
	m := domain.Tombstone(s.rows[id], by, at)
	s.rows[id] = m
	return m, nil
}

func (s *attStore) DeleteRevisions(context.Context, ids.ID, []ids.ID) error {
	s.trace.add("revisions")
	return nil
}

func (s *attStore) DeleteMentionsExcept(context.Context, ids.ID, []ids.ID, []ids.ID) error {
	s.trace.add("mentions")
	return nil
}

func (s *attStore) DeleteReactions(context.Context, ids.ID, []ids.ID) error {
	s.trace.add("reactions")
	return nil
}

func (s *attStore) DeletePin(context.Context, ids.ID, []ids.ID) (app.PinRow, bool, error) {
	s.trace.add("pin")
	return app.PinRow{}, false, nil
}

// attFiles is the fake files.Attachments: it attaches every listed file and counts the calls.
type attFiles struct {
	trace *attTrace
	mu    sync.Mutex
	// fail, when set, is the error of Attach.
	fail        error
	attached    []files.AttachCmd
	forMessages [][]ids.ID
	deleted     []ids.ID
	byMessage   map[ids.ID][]files.Attachment
}

func newAttFiles(trace *attTrace) *attFiles {
	return &attFiles{trace: trace, byMessage: map[ids.ID][]files.Attachment{}}
}

// attachedFile is the attached form of a file of the fake: the first file of a message is
// complete with its digest and a thumbnail, every later one still pending without either.
func attachedFile(id, by ids.ID, position int) files.File {
	f := files.File{ID: id, SizeBytes: int64(100 * position), MIMEType: "image/png", OriginalName: "screenshot.png", UploadedBy: by,
		Purpose: "attachment", UploadState: "pending", ScanStatus: "pending", TranscriptStatus: "none",
		CreatedAt: time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)}
	if position == 1 {
		f.SHA256, f.UploadState, f.ScanStatus, f.Width, f.Height = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", "complete", "clean", 640, 480
		f.Thumbnails = []files.Thumbnail{{Variant: "small", MIMEType: "image/jpeg", Width: 320, Height: 240}}
	}
	return f
}

func (f *attFiles) Attach(_ context.Context, cmd files.AttachCmd) ([]files.Attachment, error) {
	f.trace.add("attach")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attached = append(f.attached, cmd)
	if f.fail != nil {
		return nil, f.fail
	}
	out := make([]files.Attachment, 0, len(cmd.FileIDs))
	for i, id := range cmd.FileIDs {
		out = append(out, files.Attachment{Position: i + 1, File: attachedFile(id, cmd.AuthorID, i+1)})
	}
	f.byMessage[cmd.MessageID] = out
	return slices.Clone(out), nil
}

func (f *attFiles) ForMessages(_ context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forMessages = append(f.forMessages, slices.Clone(messageIDs))
	out := map[ids.ID][]files.Attachment{}
	for _, id := range messageIDs {
		if list, ok := f.byMessage[id]; ok {
			out[id] = slices.Clone(list)
		}
	}
	return out, nil
}

func (f *attFiles) DeleteForMessage(_ context.Context, messageID ids.ID) error {
	f.trace.add("files")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, messageID)
	delete(f.byMessage, messageID)
	return nil
}

func (*attFiles) MessageOf(context.Context, ids.ID) (files.AttachmentRef, bool, error) {
	panic("messaging never calls MessageOf")
}

func (*attFiles) Pending(context.Context, ids.ID) (bool, error) {
	panic("messaging never calls Pending")
}

// counts returns the number of Attach and ForMessages calls.
func (f *attFiles) counts() (attach, forMessages int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.attached), len(f.forMessages)
}

// attAuthz allows everything, records message.create and reports one channel as visible.
type attAuthz struct {
	channels []ids.ID
	mu       sync.Mutex
	creates  []authorize.Resource
}

func (a *attAuthz) Authorize(_ context.Context, _ authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if act == authorize.ActionMessageCreate {
		a.mu.Lock()
		a.creates = append(a.creates, r)
		a.mu.Unlock()
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (a *attAuthz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{ChannelIDs: a.channels}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (*attAuthz) Observe(context.Context, authorize.LogEvent) {}

// attThreads knows the threads of the test; AppendToThread hands out the next number and reports
// the channel of the locked thread, which may differ from the one Info read before (a move).
type attThreads struct {
	chat.Threads
	mu      sync.Mutex
	infos   map[ids.ID]chat.ThreadInfo
	seq     map[ids.ID]int64
	created chat.ThreadInfo
}

func (t *attThreads) Info(_ context.Context, id ids.ID) (chat.ThreadInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.infos[id]
	if !ok {
		return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
	}
	return info, nil
}

func (t *attThreads) AppendToThread(_ context.Context, _ actor.Actor, cmd chat.AppendCmd) (chat.Appended, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seq[cmd.ThreadID]++
	return chat.Appended{Seq: t.seq[cmd.ThreadID], Thread: t.infos[cmd.ThreadID]}, nil
}

func (t *attThreads) Create(context.Context, actor.Actor, chat.CreateThreadCmd) (chat.Thread, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.infos[t.created.ID] = t.created
	return chat.Thread{ID: t.created.ID, ChannelID: t.created.ChannelID}, nil
}

func (*attThreads) LockForMessageChange(context.Context, ids.ID) error { return nil }

func (*attThreads) DecrementMessageCount(context.Context, ids.ID) error { return nil }

// attDirectory knows every principal as Alex and answers one search language.
type attDirectory struct{ identity.Directory }

func (attDirectory) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	return identity.PrincipalView{ID: id, Kind: actor.KindUser, Handle: "alex", DisplayName: "Alex"}, nil
}

func (attDirectory) SearchLanguage(context.Context, ids.ID) (string, error) { return "english", nil }

func (attDirectory) InstanceSearchLanguage(context.Context) (string, error) { return "english", nil }

// attEvents records the appended durable events.
type attEvents struct {
	mu     sync.Mutex
	events []eventlog.Event
}

func (e *attEvents) Append(_ context.Context, evs ...eventlog.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, evs...)
	return nil
}

// payloads returns the message member of the events of type typ.
func (e *attEvents) payloads(t *testing.T, typ string) []map[string]any {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []map[string]any
	for _, ev := range e.events {
		if ev.Type != typ {
			continue
		}
		var p struct {
			Message map[string]any `json:"message"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p.Message)
	}
	return out
}

// attRedactor applies the redaction to one stored message.created payload and keeps the result.
type attRedactor struct {
	mu       sync.Mutex
	redacted json.RawMessage
}

func (r *attRedactor) Redact(_ context.Context, _ string, _ ids.ID, fn eventlog.RedactFunc) (int, error) {
	out, err := fn("message.created", 1, json.RawMessage(`{"message":{}}`))
	if err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redacted = out
	return 1, nil
}

// attUoW runs a unit of work over a sealed registry with the test subscriber of MessagePosted;
// it has no database, so a failed unit leaves the fakes as they are.
type attUoW struct{ reg *txn.Registry }

func (u attUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	if txn.InUnit(ctx) {
		return fn(ctx)
	}
	unit, set := txn.NewUnit(u.reg, txn.Options{})
	set(txn.PhaseWork)
	return fn(txn.WithUnit(ctx, unit))
}

// attPosted records MessagePosted.
type attPosted struct {
	mu     sync.Mutex
	events []messaging.MessagePosted
}

func (p *attPosted) on(_ context.Context, ev messaging.MessagePosted) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	return nil
}

func (p *attPosted) last() messaging.MessagePosted {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.events[len(p.events)-1]
}

// attWorld is the service over the fakes: Alex writes in one thread of one channel.
type attWorld struct {
	svc                       *app.Service
	store                     *attStore
	files                     *attFiles
	authz                     *attAuthz
	threads                   *attThreads
	events                    *attEvents
	redactor                  *attRedactor
	posted                    *attPosted
	trace                     *attTrace
	alex                      actor.Actor
	thread, channel, newChann ids.ID
}

// attMaxAttachments is message.max_attachments of the fake world.
const attMaxAttachments = 3

// newAttWorld builds the world; withFiles false leaves Deps.Files nil (a world without files).
func newAttWorld(t *testing.T, withFiles bool) *attWorld {
	t.Helper()
	trace := &attTrace{}
	w := &attWorld{store: newAttStore(trace), files: newAttFiles(trace), events: &attEvents{}, redactor: &attRedactor{},
		posted: &attPosted{}, trace: trace, alex: actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		thread: ids.New(), channel: ids.New(), newChann: ids.New()}
	space := ids.New()
	w.authz = &attAuthz{channels: []ids.ID{w.channel}}
	w.threads = &attThreads{
		infos: map[ids.ID]chat.ThreadInfo{w.thread: {ID: w.thread, ChannelID: w.channel, SpaceID: space, ChannelKind: "text", Kind: "thread", State: "open"}},
		seq:   map[ids.ID]int64{},
		created: chat.ThreadInfo{ID: ids.New(), ChannelID: w.channel, SpaceID: space, ChannelKind: "text", Kind: "thread", State: "open",
			CreatedBy: w.alex.ID},
	}
	reg := txn.NewRegistry()
	if err := txn.Subscribe(reg, w.posted.on); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	deps := app.Deps{Store: w.store, UoW: attUoW{reg: reg}, Authorizer: w.authz, Events: w.events, Redactor: w.redactor,
		Audit: struct{ auditing.Recorder }{}, Threads: w.threads, Directory: attDirectory{}, Cursors: struct{ app.Cursors }{}, Symbols: anySymbol{}}
	if withFiles {
		deps.Files = w.files
	}
	svc, err := app.New(app.Config{MaxContentBytes: 1000, MaxMentions: 4, EventPayloadMaxBytes: 100000, TemplateNameMaxChars: 10,
		TemplateMaxPerScope: 5, ListContentMaxBytes: 1000, PageSizeDefault: 10, PageSizeMax: 10, EmojiMaxBytes: 64,
		MaxDistinctReactions: 20, MaxPinsPerThread: 50, MaxAttachments: attMaxAttachments}, deps)
	if err != nil {
		t.Fatal(err)
	}
	w.svc = svc
	return w
}

// ctx is a request context of Alex.
func (w *attWorld) ctx() context.Context { return actor.WithActor(context.Background(), w.alex) }

// send creates a message of Alex in the thread with the files fileIDs.
func (w *attWorld) send(t *testing.T, content string, fileIDs ...ids.ID) messaging.Created {
	t.Helper()
	c, err := w.svc.Create(w.ctx(), w.alex, messaging.CreateMessageCmd{ThreadID: w.thread, ClientMsgID: ids.New(), ContentMD: &content,
		AttachmentIDs: fileIDs})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return c
}

// wantAttachments checks the attachment members of a message payload against the file ids in
// position order: the File form without scope_channel_id, sha256 only for the complete file,
// thumbnails always an array, no JSON null anywhere.
func wantAttachments(t *testing.T, payload map[string]any, fileIDs []ids.ID) {
	t.Helper()
	list, ok := payload["attachments"].([]any)
	if !ok || len(list) != len(fileIDs) {
		t.Fatalf("attachments %v, want %d", payload["attachments"], len(fileIDs))
	}
	for i, raw := range list {
		a := raw.(map[string]any)
		f := a["file"].(map[string]any)
		if a["position"] != float64(i+1) || f["id"] != fileIDs[i].String() {
			t.Errorf("attachment %d: %v", i, a)
		}
		if _, has := f["scope_channel_id"]; has {
			t.Errorf("attachment %d shows scope_channel_id", i)
		}
		if _, has := f["sha256"]; has != (i == 0) {
			t.Errorf("attachment %d: sha256 present %v", i, has)
		}
		thumbs, ok := f["thumbnails"].([]any)
		if !ok || (i == 0) != (len(thumbs) == 1) {
			t.Errorf("attachment %d: thumbnails %v", i, f["thumbnails"])
		}
		for _, k := range []string{"size_bytes", "mime_type", "original_name", "uploaded_by", "purpose", "upload_state", "scan_status",
			"transcript_status", "created_at"} {
			if _, has := f[k]; !has {
				t.Errorf("attachment %d lacks %s", i, k)
			}
		}
	}
	b, err := json.Marshal(payload)
	if err != nil || bytes.Contains(b, []byte("null")) {
		t.Errorf("payload %s has a JSON null (%v)", b, err)
	}
}

// TestCreate_NoFilesCallWithoutAttachments (brief M3-07 design notes 5 d and 6): a message
// without attachments makes no call of the files port, neither at the send nor in a page; a
// message with attachments makes exactly one Attach in the unit of work, its message.created and
// its answer carry the result of Attach, and a page asks ForMessages once, for the flagged
// messages only.
func TestCreate_NoFilesCallWithoutAttachments(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	plain := w.send(t, "no files")
	if a, f := w.files.counts(); a != 0 || f != 0 {
		t.Fatalf("a message without attachments made %d Attach and %d ForMessages calls", a, f)
	}
	if plain.Message.HasAttachments || plain.Message.Attachments == nil || len(plain.Message.Attachments) != 0 {
		t.Fatalf("plain message: flag %v, attachments %#v", plain.Message.HasAttachments, plain.Message.Attachments)
	}
	if w.posted.last().HasAttachments {
		t.Error("MessagePosted.HasAttachments for a message without attachments")
	}
	if p := w.events.payloads(t, "message.created")[0]; len(p["attachments"].([]any)) != 0 {
		t.Errorf("message.created of the plain message lists %v", p["attachments"])
	}
	page, err := w.svc.History(w.ctx(), messaging.HistoryQuery{ThreadID: w.thread})
	if err != nil {
		t.Fatal(err)
	}
	if _, f := w.files.counts(); f != 0 || len(page.Items) != 1 {
		t.Fatalf("a page without flagged messages made %d ForMessages calls (%d items)", f, len(page.Items))
	}

	fileIDs := []ids.ID{ids.New(), ids.New()}
	with := w.send(t, "two files", fileIDs...)
	attach, forMessages := w.files.counts()
	if attach != 1 || forMessages != 0 {
		t.Fatalf("a send with attachments made %d Attach and %d ForMessages calls, want 1 and 0", attach, forMessages)
	}
	cmd := w.files.attached[0]
	if cmd.MessageID != with.Message.ID || cmd.ThreadID != w.thread || cmd.ChannelID != w.channel || cmd.AuthorID != w.alex.ID ||
		!slices.Equal(cmd.FileIDs, fileIDs) || cmd.Pointer != domain.AttachmentsPointer {
		t.Fatalf("AttachCmd %+v", cmd)
	}
	if row, _, _ := w.store.Message(context.Background(), with.Message.ID); !row.HasAttachments {
		t.Error("the row of a message with attachments has has_attachments false")
	}
	if !with.Message.HasAttachments || len(with.Message.Attachments) != 2 || with.Message.Attachments[1].File.ID != fileIDs[1] {
		t.Fatalf("answer %+v", with.Message.Attachments)
	}
	if !w.posted.last().HasAttachments {
		t.Error("MessagePosted.HasAttachments is false for a message with attachments")
	}
	created := w.events.payloads(t, "message.created")
	wantAttachments(t, created[len(created)-1], fileIDs)

	page, err = w.svc.History(w.ctx(), messaging.HistoryQuery{ThreadID: w.thread})
	if err != nil {
		t.Fatal(err)
	}
	if _, f := w.files.counts(); f != 1 || !slices.Equal(w.files.forMessages[0], []ids.ID{with.Message.ID}) {
		t.Fatalf("a page made ForMessages calls %v, want one for the flagged message", w.files.forMessages)
	}
	if len(page.Items) != 2 || len(page.Items[0].Attachments) != 0 || len(page.Items[1].Attachments) != 2 {
		t.Fatalf("page attachments %+v", page.Items)
	}
}

// TestCreate_AttachmentsAuthorizeHashAndReplay (brief M3-07 design note 5 b and c): message.create
// is authorized with HasAttachments exactly when the request lists files; the request hash holds
// the ids in request order, so another order is 422 idempotency_key_reuse; a replay attaches
// nothing and answers the stored message with its attachments.
func TestCreate_AttachmentsAuthorizeHashAndReplay(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	w.send(t, "plain")
	f1, f2 := ids.New(), ids.New()
	content, key := "with files", ids.New()
	cmd := messaging.CreateMessageCmd{ThreadID: w.thread, ClientMsgID: key, ContentMD: &content, AttachmentIDs: []ids.ID{f1, f2}}
	first, err := w.svc.Create(w.ctx(), w.alex, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.authz.creates) != 2 || w.authz.creates[0].HasAttachments || !w.authz.creates[1].HasAttachments {
		t.Fatalf("message.create resources %+v", w.authz.creates)
	}
	replay, err := w.svc.Create(w.ctx(), w.alex, cmd)
	if err != nil || !replay.Replay || replay.Message.ID != first.Message.ID {
		t.Fatalf("replay %+v %v", replay, err)
	}
	if a, f := w.files.counts(); a != 1 || f != 1 {
		t.Fatalf("after the replay: %d Attach and %d ForMessages calls, want 1 and 1", a, f)
	}
	if len(replay.Message.Attachments) != 2 || replay.Message.Attachments[0].File.ID != f1 {
		t.Fatalf("replay attachments %+v", replay.Message.Attachments)
	}
	cmd.AttachmentIDs = []ids.ID{f2, f1}
	_, err = w.svc.Create(w.ctx(), w.alex, cmd)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeIdempotencyKeyReuse {
		t.Fatalf("another order: %v, want idempotency_key_reuse", err)
	}
}

// TestCreate_AttachmentRules (brief M3-07 design note 5 a): more than message.max_attachments
// ids are limit_reached on the list without a call of files; an empty content passes with a file;
// a world without the files port keeps the M1-56 answer not_enabled; the refusal of Attach is the
// answer of the use case.
func TestCreate_AttachmentRules(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	tooMany := make([]ids.ID, attMaxAttachments+1)
	for i := range tooMany {
		tooMany[i] = ids.New()
	}
	empty := " "
	_, err := w.svc.Create(w.ctx(), w.alex, messaging.CreateMessageCmd{ThreadID: w.thread, ClientMsgID: ids.New(), ContentMD: &empty,
		AttachmentIDs: tooMany})
	wantFieldErr(t, err, domain.AttachmentsPointer, domain.FieldLimitReached)
	if a, _ := w.files.counts(); a != 0 {
		t.Fatal("a refused list reached files")
	}
	if c := w.send(t, "", ids.New()); c.Message.ContentMD != "" || len(c.Message.Attachments) != 1 {
		t.Fatalf("attachment only: %+v", c.Message)
	}
	_, err = w.svc.Create(w.ctx(), w.alex, messaging.CreateMessageCmd{ThreadID: w.thread, ClientMsgID: ids.New(), ContentMD: &empty})
	wantFieldErr(t, err, "/content_md", domain.FieldEmpty)

	w.files.fail = errs.New(errs.CodeConflict, "/attachment_file_ids/0").WithReason("file_not_attachable")
	text := "x"
	_, err = w.svc.Create(w.ctx(), w.alex, messaging.CreateMessageCmd{ThreadID: w.thread, ClientMsgID: ids.New(), ContentMD: &text,
		AttachmentIDs: []ids.ID{ids.New()}})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict {
		t.Fatalf("refused attach: %v", err)
	}

	bare := newAttWorld(t, false)
	_, err = bare.svc.Create(bare.ctx(), bare.alex, messaging.CreateMessageCmd{ThreadID: bare.thread, ClientMsgID: ids.New(), ContentMD: &text,
		AttachmentIDs: []ids.ID{ids.New()}})
	wantFieldErr(t, err, domain.AttachmentsPointer, domain.FieldNotEnabled)
}

// wantFieldErr checks that err is validation_failed with one field error on pointer with code.
func wantFieldErr(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed on %s with %s", err, pointer, code)
	}
}

// TestCreateThread_FirstMessageAttachments (brief M3-07 design note 5): the first message of a
// new thread authorizes message.create with HasAttachments on the new thread and attaches with the
// pointer inside first_message; its list over the limit names that pointer.
func TestCreateThread_FirstMessageAttachments(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	content, f := "first", ids.New()
	out, err := w.svc.CreateThread(w.ctx(), w.alex, messaging.CreateThreadCmd{ChannelID: w.channel,
		FirstMessage: &messaging.CreateMessageCmd{ClientMsgID: ids.New(), ContentMD: &content, AttachmentIDs: []ids.ID{f}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.authz.creates) != 1 || !w.authz.creates[0].HasAttachments {
		t.Fatalf("message.create resources %+v", w.authz.creates)
	}
	cmd := w.files.attached[0]
	if cmd.ThreadID != out.ThreadID || cmd.ThreadID != w.threads.created.ID || cmd.Pointer != "/first_message/attachment_file_ids" ||
		out.FirstMessage == nil || len(out.FirstMessage.Attachments) != 1 {
		t.Fatalf("AttachCmd %+v, first %+v", cmd, out.FirstMessage)
	}
	tooMany := make([]ids.ID, attMaxAttachments+1)
	for i := range tooMany {
		tooMany[i] = ids.New()
	}
	_, err = w.svc.CreateThread(w.ctx(), w.alex, messaging.CreateThreadCmd{ChannelID: w.channel,
		FirstMessage: &messaging.CreateMessageCmd{ClientMsgID: ids.New(), ContentMD: &content, AttachmentIDs: tooMany}})
	wantFieldErr(t, err, "/first_message/attachment_file_ids", domain.FieldLimitReached)
}

// TestDelete_AttachmentsGoWithTheMessage (brief M3-07 design note 7): the deletion of a message
// with attachments removes them through files after revisions, mentions, reactions and pin, and the
// redacted payload lists none; a message without attachments makes no files call.
func TestDelete_AttachmentsGoWithTheMessage(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	plain := w.send(t, "plain")
	if err := w.svc.Delete(w.ctx(), plain.Message.ID); err != nil {
		t.Fatal(err)
	}
	if len(w.files.deleted) != 0 {
		t.Fatal("deleting a message without attachments called files")
	}
	with := w.send(t, "with", ids.New())
	start := len(w.trace.list())
	if err := w.svc.Delete(w.ctx(), with.Message.ID); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.files.deleted, []ids.ID{with.Message.ID}) {
		t.Fatalf("DeleteForMessage calls %v", w.files.deleted)
	}
	if steps := w.trace.list()[start:]; !slices.Equal(steps, []string{"tombstone", "revisions", "mentions", "reactions", "pin", "files"}) {
		t.Fatalf("delete steps %v", steps)
	}
	if row, _, _ := w.store.Message(context.Background(), with.Message.ID); row.HasAttachments || row.DeletedAt.IsZero() {
		t.Fatalf("tombstone %+v", row)
	}
	var redacted struct {
		Message map[string]any `json:"message"`
	}
	if err := json.Unmarshal(w.redactor.redacted, &redacted); err != nil {
		t.Fatal(err)
	}
	if list, ok := redacted.Message["attachments"].([]any); !ok || len(list) != 0 {
		t.Fatalf("redacted attachments %v", redacted.Message["attachments"])
	}
	got, err := w.svc.Get(w.ctx(), with.Message.ID)
	if err != nil || got.Attachments == nil || len(got.Attachments) != 0 {
		t.Fatalf("tombstone attachments %#v (%v)", got.Attachments, err)
	}
}

// TestUpdateAndTrigger_KeepAttachments (brief M3-07 design note 6): an edit never changes the
// attachments, and message.updated of an edit and of SetTriggerState lists them.
func TestUpdateAndTrigger_KeepAttachments(t *testing.T) {
	t.Parallel()
	w := newAttWorld(t, true)
	f := ids.New()
	c := w.send(t, "before", f)
	text := "after"
	m, err := w.svc.Update(w.ctx(), messaging.UpdateCommand{MessageID: c.Message.ID, IfMatch: 1, Content: &text})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].File.ID != f {
		t.Fatalf("edited attachments %+v", m.Attachments)
	}
	if err := w.svc.SetTriggerState(txnCtx(t, w), messaging.SetTriggerStateCmd{MessageID: c.Message.ID, State: "received"}); err != nil {
		t.Fatal(err)
	}
	updated := w.events.payloads(t, "message.updated")
	if len(updated) != 2 {
		t.Fatalf("%d message.updated", len(updated))
	}
	for _, p := range updated {
		wantAttachments(t, p, []ids.ID{f})
	}
}

// txnCtx is a context inside a unit of work of the fake world, as runs calls SetTriggerState.
func txnCtx(t *testing.T, w *attWorld) context.Context {
	t.Helper()
	reg := txn.NewRegistry()
	reg.Seal()
	unit, set := txn.NewUnit(reg, txn.Options{})
	set(txn.PhaseWork)
	return txn.WithUnit(w.ctx(), unit)
}
