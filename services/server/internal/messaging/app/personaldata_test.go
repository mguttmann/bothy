// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	messagingpg "github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// exportSchemas is docs/api/gdpr-export/v1 from the root of the repository (five levels above
// this package).
var exportSchemas = filepath.Join("..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1")

// checkSchema validates raw against the JSON Schema rel below exportSchemas, with format
// assertions (uuid, date-time).
func checkSchema(t *testing.T, rel string, raw []byte) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(exportSchemas, rel))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	sch, err := c.Compile(abs)
	if err != nil {
		t.Fatalf("schema %s: %v", rel, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("%s does not hold %s: %v", rel, raw, err)
	}
}

// fakeAttachments answers the attachments of the export from a map and records the messages it
// was asked for.
type fakeAttachments struct {
	mu    sync.Mutex
	files map[ids.ID][]ids.ID
	asked []ids.ID
}

func (f *fakeAttachments) ForMessages(_ context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, messageIDs...)
	out := map[ids.ID][]files.Attachment{}
	for _, m := range messageIDs {
		for i, id := range f.files[m] {
			out[m] = append(out[m], files.Attachment{Position: i + 1, File: files.File{ID: id}})
		}
	}
	return out, nil
}

// personalData returns the participant of the world as internal/wire builds it: the service, the
// repository of the module, the names of chat and the given attachments.
func personalData(t *testing.T, w *mw.World, att app.AttachmentReader) *app.PersonalData {
	t.Helper()
	names, ok := w.Threads.(app.PlaceNames)
	if !ok {
		t.Fatal("the chat service of the world gives no place names")
	}
	p, err := app.NewPersonalData(w.Svc, messagingpg.NewRepository(w.Pool), names, att)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// template inserts a template as the database owner: personal for owner, else of channel ch,
// created by by; deleted sets deleted_at.
func template(w *mw.World, owner, ch, by ids.ID, name, content string, deleted bool) ids.ID {
	id := ids.New()
	var o, c any
	if !owner.IsZero() {
		o = owner.String()
	}
	if !ch.IsZero() {
		c = ch.String()
	}
	w.SQL(`INSERT INTO message_templates (id, owner_user_id, channel_id, name, content_md, created_by, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $7 THEN now() END)`, id.String(), o, c, name, content, by.String(), deleted)
	return id
}

// directMessage inserts a direct message channel of author with its main thread and one message
// of author as the database owner and returns the message. The REST operation createDm is not
// served by the world, and the content step reads and deletes by author without a rights check.
func directMessage(w *mw.World, author ids.ID, text string) ids.ID {
	ch, th, m := ids.New(), ids.New(), ids.New()
	w.SQL(`INSERT INTO channels (id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, 'dm', '', 'a0', false, 50, $2)`, ch.String(), author.String())
	w.SQL(`INSERT INTO threads (id, channel_id, kind, created_by, last_message_seq, message_count) VALUES ($1, $2, 'main', $3, 1, 1)`,
		th.String(), ch.String(), author.String())
	w.SQL(`INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, content_md)
		VALUES ($1, $2, $3, 1, $4, $5, '\x00', 'human', $6)`, m.String(), th.String(), ch.String(), author.String(), ids.New().String(), text)
	return m
}

// lines decodes the lines of f.
func lines(t *testing.T, w *personaldata.MemoryWriter, f personaldata.File, schema string) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, raw := range w.Lines[f] {
		checkSchema(t, schema, raw)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

// keys returns the sorted member names of m.
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// TestPersonalDataExport (M7-06, 20 section 5.2, DP-T03): the export of Alex holds Alex's four
// messages (edited with its revision, in a channel Alex may no longer read without its names,
// deleted as a tombstone, with an attachment by its file ID), Alex's reaction, the mention of Alex
// with message ID and time only, and Alex's personal templates; no text of Sam, no channel
// template, no hash, and every line and section matches its schema.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	w, space, ch, main, alex, msg := world(t)
	sam := w.Person("sam", false)
	w.Grant("user", sam, space, "write")
	w.SQL(`UPDATE threads SET title = 'Release plan', title_source = 'manual' WHERE id = $1`, main.String())
	w.Clock.Advance(time.Minute)
	w.Want(patch(w, "alex", msg.id, v(msg.version), content("canary-alex-edited")), http.StatusOK, "")
	random := w.MainThread(w.Channel(space, "text", "random"))
	hidden := post(w, "alex", random, "canary-alex-hidden")
	gone := post(w, "alex", main, "canary-alex-deleted")
	w.Want(w.Do("alex", http.MethodDelete, messagePath(gone.id), ""), http.StatusNoContent, "")
	attached := post(w, "alex", main, "canary-alex-attached")
	w.SQL(`UPDATE messages SET has_attachments = true WHERE id = $1`, attached.id)
	fileID := ids.New()
	samMsg := post(w, "sam", main, "canary-sam-text @alex")
	post(w, "sam", main, "canary-sam-other @owner")
	w.Want(react(w, "alex", samMsg.id, thumbsUp), http.StatusNoContent, "")
	w.Want(react(w, "sam", msg.id, party), http.StatusNoContent, "")
	live := template(w, alex, ids.ID{}, alex, "Greeting", "canary-alex-template", false)
	old := template(w, alex, ids.ID{}, alex, "Old", "canary-alex-old-template", true)
	template(w, sam, ids.ID{}, sam, "Sam", "canary-sam-template", false)
	template(w, ids.ID{}, ch, alex, "Shared", "canary-alex-channel-template", false)

	att := &fakeAttachments{files: map[ids.ID][]ids.ID{ids.MustParse(attached.id): {fileID}}}
	p := personalData(t, w, att)
	if p.Module() != "messaging" {
		t.Fatalf("Module() = %q", p.Module())
	}
	if got, want := p.Tables(), []string{"messages", "message_revisions", "mentions", "reactions", "pins", "message_templates"}; !slices.Equal(got, want) {
		t.Fatalf("Tables() = %v, want %v", got, want)
	}
	out := personaldata.NewMemoryWriter()
	readable := func(_ context.Context, id ids.ID) (bool, error) { return id == ch, nil }
	if err := p.Export(context.Background(), personaldata.Export{Subject: alex, Readable: readable}, out); err != nil {
		t.Fatal(err)
	}

	msgs := lines(t, out, personaldata.FileMessages, "lines/messages.schema.json")
	byID := map[string]map[string]any{}
	for _, m := range msgs {
		byID[m["id"].(string)] = m
	}
	if len(msgs) != 4 || byID[msg.id] == nil || byID[hidden.id] == nil || byID[gone.id] == nil || byID[attached.id] == nil {
		t.Fatalf("messages.jsonl %v, want Alex's four messages", msgs)
	}
	edited := byID[msg.id]
	revs, _ := edited["revisions"].([]any)
	if edited["content_md"] != "canary-alex-edited" || edited["edited_at"] == nil || len(revs) != 1 ||
		revs[0].(map[string]any)["content_md"] != "first version" || revs[0].(map[string]any)["revision"] != float64(1) {
		t.Errorf("edited message %v", edited)
	}
	if edited["channel_name"] != "general" || edited["thread_title"] != "Release plan" || edited["channel_id"] != ch.String() ||
		edited["thread_id"] != main.String() || edited["deleted"] != false {
		t.Errorf("names and place of a readable channel %v", edited)
	}
	if _, ok := byID[hidden.id]["channel_name"]; ok {
		t.Errorf("a channel Alex may not read shows its name: %v", byID[hidden.id])
	}
	if _, ok := byID[hidden.id]["thread_title"]; ok {
		t.Errorf("a channel Alex may not read shows the thread title: %v", byID[hidden.id])
	}
	tomb := byID[gone.id]
	if tomb["deleted"] != true || tomb["deleted_at"] == nil || tomb["content_md"] != "" || len(tomb["revisions"].([]any)) != 0 {
		t.Errorf("deleted message %v", tomb)
	}
	if got := mw.JSONList(byID[attached.id]["attachment_file_ids"]); got != "["+fileID.String()+"]" {
		t.Errorf("attachment_file_ids %s", got)
	}
	if _, ok := edited["attachment_file_ids"]; ok {
		t.Errorf("a message without attachments lists attachment_file_ids: %v", edited)
	}
	if !slices.Equal(att.asked, []ids.ID{ids.MustParse(attached.id)}) {
		t.Errorf("attachments asked for %v, want only the message with attachments", att.asked)
	}

	reactions := lines(t, out, personaldata.FileReactions, "lines/reactions.schema.json")
	if len(reactions) != 1 || reactions[0]["message_id"] != samMsg.id || reactions[0]["emoji"] != thumbsUp {
		t.Errorf("reactions.jsonl %v, want Alex's one reaction", reactions)
	}
	mentions := lines(t, out, personaldata.FileMentions, "lines/mentions.schema.json")
	if len(mentions) != 1 || mentions[0]["message_id"] != samMsg.id || !slices.Equal(keys(mentions[0]), []string{"created_at", "message_id"}) {
		t.Errorf("mentions.jsonl %v, want the one mention of Alex with message ID and time only", mentions)
	}

	if len(out.Sections) != 1 || len(out.Sections[personaldata.FilePersonal]) != 1 || len(out.Contents) != 0 {
		t.Fatalf("the export wrote more than the section templates: %v", out.Sections)
	}
	raw := out.Sections[personaldata.FilePersonal]["templates"]
	checkSchema(t, "sections/personal.templates.schema.json", raw)
	var tpls []map[string]any
	if err := json.Unmarshal(raw, &tpls); err != nil {
		t.Fatal(err)
	}
	gotTpl := map[string]map[string]any{}
	for _, tp := range tpls {
		gotTpl[tp["id"].(string)] = tp
	}
	if len(tpls) != 2 || gotTpl[live.String()]["content_md"] != "canary-alex-template" || gotTpl[old.String()]["deleted_at"] == nil {
		t.Errorf("templates %s, want Alex's live and deleted personal template", raw)
	}
	if _, ok := gotTpl[live.String()]["deleted_at"]; ok {
		t.Errorf("a live template names deleted_at: %v", gotTpl[live.String()])
	}

	var all strings.Builder
	for _, f := range []personaldata.File{personaldata.FileMessages, personaldata.FileReactions, personaldata.FileMentions} {
		for _, l := range out.Lines[f] {
			all.Write(l)
		}
	}
	all.Write(raw)
	for _, banned := range []string{"canary-sam", "canary-alex-channel-template", "sha256", "hash", "client_msg_id", "first version\",\"edited_by"} {
		if strings.Contains(all.String(), banned) {
			t.Errorf("the export holds %q: %s", banned, all.String())
		}
	}

	// A person without data gets the empty section and no lines.
	empty := personaldata.NewMemoryWriter()
	if err := p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	if got := string(empty.Sections[personaldata.FilePersonal]["templates"]); got != "[]" || len(empty.Lines) != 0 {
		t.Errorf("export of a person without data: templates %s, lines %v", got, empty.Lines)
	}
}

// TestPersonalDataErase (M7-06, 20 sections 4.2 and 4.4): the steps 1 to 6 in order, twice (the
// second run is the resume after a crash). StepPersonal deletes Alex's personal templates, live
// and deleted, and keeps the channel template Alex wrote and Sam's template. StepContent with
// delete removes every message of Alex through the deletion path without an actor (tombstone,
// revisions gone, the payloads redacted, one message.deleted without deleted_by per message, no
// audit entry), also in a direct message, and nothing on the second run, and removes Alex's
// reactions on the messages of others (one reaction.removed each, without an actor); with
// keep_pseudonymized it changes nothing (20 section 4.4 item 5: messages, files and reactions
// stay). Sam's message, Sam's reaction and the mention of Alex stay in both modes.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			w, space, ch, main, alex, msg := world(t)
			sam := w.Person("sam", false)
			w.Grant("user", sam, space, "write")
			w.Want(patch(w, "alex", msg.id, v(msg.version), content("canary-erase-edited")), http.StatusOK, "")
			second := post(w, "alex", main, "canary-erase-second")
			direct := directMessage(w, alex, "canary-erase-direct")
			samMsg := post(w, "sam", main, "canary-sam-stays @alex")
			w.Want(react(w, "alex", samMsg.id, thumbsUp), http.StatusNoContent, "")
			w.Want(react(w, "sam", samMsg.id, thumbsUp), http.StatusNoContent, "")
			template(w, alex, ids.ID{}, alex, "Greeting", "canary-alex-template", false)
			template(w, alex, ids.ID{}, alex, "Old", "canary-alex-old", true)
			template(w, sam, ids.ID{}, sam, "Sam", "canary-sam-template", false)
			shared := template(w, ids.ID{}, ch, alex, "Shared", "canary-alex-shared", false)
			own := []string{msg.id, second.id, direct.String()}

			p := personalData(t, w, nil)
			e := personaldata.Erasure{Subject: alex, RequestID: ids.New(), DecidedBy: w.OwnerID, Content: mode}
			personal := func() int {
				return w.Count(`SELECT count(*)::int FROM message_templates WHERE owner_user_id = $1`, alex.String())
			}
			liveOwn := func() int {
				return w.Count(`SELECT count(*)::int FROM messages WHERE author_principal_id = $1 AND deleted_at IS NULL`, alex.String())
			}
			for run := range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					if err := p.Erase(context.Background(), step, e); err != nil {
						t.Fatalf("run %d step %s: %v", run, step, err)
					}
					wantTpl, wantLive := 2, 3
					if run > 0 || step >= personaldata.StepPersonal {
						wantTpl = 0
					}
					if mode == personaldata.DeleteContent && (run > 0 || step >= personaldata.StepContent) {
						wantLive = 0
					}
					if got := personal(); got != wantTpl {
						t.Fatalf("run %d after %s: %d personal templates of Alex, want %d", run, step, got, wantTpl)
					}
					if got := liveOwn(); got != wantLive {
						t.Fatalf("run %d after %s: %d live messages of Alex, want %d", run, step, got, wantLive)
					}
				}
			}
			if n := w.Count(`SELECT count(*)::int FROM message_templates WHERE id = $1 OR owner_user_id = $2`, shared.String(), sam.String()); n != 2 {
				t.Errorf("channel template of Alex and Sam's template: %d rows, want 2", n)
			}
			if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND deleted_at IS NULL AND content_md LIKE 'canary-sam-stays%'`, samMsg.id); n != 1 {
				t.Errorf("Sam's message changed")
			}
			wantReactions := 1
			if mode == personaldata.DeleteContent {
				wantReactions = 0
			}
			if n := w.Count(`SELECT count(*)::int FROM reactions WHERE principal_id = $1`, alex.String()); n != wantReactions {
				t.Errorf("Alex's reaction: %d rows, want %d (20 section 4.4 item 5)", n, wantReactions)
			}
			if n := w.Count(`SELECT count(*)::int FROM reactions WHERE principal_id = $1`, sam.String()); n != 1 {
				t.Errorf("Sam's reaction: %d rows, want 1", n)
			}
			removed := w.Events("reaction.removed")
			if len(removed) != wantReactions^1 {
				t.Errorf("reaction.removed: %d events, want %d", len(removed), wantReactions^1)
			}
			for _, ev := range removed {
				if ev.ResourceID != samMsg.id || ev.Payload["principal_id"] != alex.String() || ev.Payload["count"] != float64(1) {
					t.Errorf("reaction.removed %s %v, want Alex's reaction on Sam's message with one left", ev.ResourceID, ev.Payload)
				}
			}
			if n := w.Count(`SELECT count(*)::int FROM mentions WHERE principal_id = $1`, alex.String()); n != 1 {
				t.Errorf("mention of Alex in Sam's message: %d rows, want 1", n)
			}
			deleted := w.Events("message.deleted")
			w.FlushAudit()
			if n := w.Count(`SELECT count(*)::int FROM audit_events WHERE action = 'message.delete'`); n != 0 {
				t.Errorf("the deletion of the account content audited %d messages", n)
			}
			if mode == personaldata.KeepPseudonymized {
				if len(deleted) != 0 {
					t.Errorf("keep_pseudonymized deleted %d messages", len(deleted))
				}
				if n := w.Count(`SELECT count(*)::int FROM message_revisions WHERE message_id = $1`, msg.id); n != 1 {
					t.Errorf("keep_pseudonymized removed revisions: %d left", n)
				}
				return
			}
			var got []string
			for _, ev := range deleted {
				got = append(got, ev.ResourceID)
				if _, ok := ev.Payload["deleted_by"]; ok {
					t.Errorf("message.deleted of the account deletion names deleted_by: %v", ev.Payload)
				}
			}
			slices.Sort(got)
			want := slices.Clone(own)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("message.deleted for %v, want once for each of Alex's messages %v", got, want)
			}
			for _, id := range own {
				if n := w.Count(`SELECT count(*)::int FROM messages WHERE id = $1 AND content_md = '' AND deleted_by IS NULL`, id); n != 1 {
					t.Errorf("message %s is no tombstone without deleted_by", id)
				}
			}
			if n := w.Count(`SELECT count(*)::int FROM message_revisions WHERE message_id = $1`, msg.id); n != 0 {
				t.Errorf("revisions left: %d", n)
			}
			if n := w.Count(`SELECT count(*)::int FROM event_log WHERE payload::text LIKE '%canary-erase-%'`); n != 0 {
				t.Errorf("event payloads still hold the deleted content: %d", n)
			}
		})
	}
}
