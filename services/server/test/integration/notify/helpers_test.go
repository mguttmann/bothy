// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// msg is the part of a created message the tests read.
type msg struct {
	ID        ids.ID
	ThreadSeq int64
}

// msgOf reads a Message body.
func msgOf(m map[string]any) msg {
	return msg{ID: ids.MustParse(m["id"].(string)), ThreadSeq: int64(m["thread_seq"].(float64))}
}

// postBody sends createMessage as name into thread with the extra JSON members (without the
// braces, "" for none) and the text.
func (w *world) postBody(name string, thread ids.ID, text, extra string) *httpservertest.Response {
	w.t.Helper()
	b, _ := json.Marshal(text)
	body := `{"client_msg_id":"` + ids.New().String() + `","content_md":` + string(b)
	if extra != "" {
		body += "," + extra
	}
	return w.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages", body+"}", key()...)
}

// post sends a text message as name into thread, expects 201 and returns the message.
func (w *world) post(name string, thread ids.ID, text string) msg {
	w.t.Helper()
	return msgOf(w.want(w.postBody(name, thread, text, ""), http.StatusCreated, ""))
}

// mainThread reads the main thread of a text or dm channel.
func (w *world) mainThread(channel ids.ID) ids.ID {
	return ids.MustParse(w.text(`SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, channel.String()))
}

// newThread creates a thread of kind thread in the text channel as the owner without a first
// message: a message of the main thread, then createThread on it.
func (w *world) newThread(channel ids.ID) ids.ID {
	w.t.Helper()
	parent := w.post("owner", w.mainThread(channel), "Question")
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/channels/"+channel.String()+"/threads",
		`{"parent_message_id":"`+parent.ID.String()+`"}`, key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string))
}

// createThread creates a thread of kind thread in the text channel as name with a first message
// of text: a message of name in the main thread, then createThread on it with first_message. It
// returns the thread and its first message.
func (w *world) createThread(name string, channel ids.ID, text string) (ids.ID, msg) {
	w.t.Helper()
	parent := w.post(name, w.mainThread(channel), "Topic")
	b, _ := json.Marshal(text)
	m := w.want(w.do(name, http.MethodPost, "/api/v1/channels/"+channel.String()+"/threads",
		`{"parent_message_id":"`+parent.ID.String()+`","first_message":{"client_msg_id":"`+ids.New().String()+`","content_md":`+string(b)+`}}`,
		key()...), http.StatusCreated, "")
	return ids.MustParse(m["thread"].(map[string]any)["id"].(string)), msgOf(m["first_message"].(map[string]any))
}

// readUpTo sends putReadState as name for thread with last_read_seq seq and expects 200.
func (w *world) readUpTo(name string, thread ids.ID, seq int64) {
	w.t.Helper()
	w.wantOK(w.do(name, http.MethodPut, "/api/v1/threads/"+thread.String()+"/read-state", `{"last_read_seq":`+strconv.FormatInt(seq, 10)+`}`))
}

// setLevel sends putNotificationPreference as name on the target of kind (space, channel or
// thread) with level and expects 200.
func (w *world) setLevel(name, kind string, target ids.ID, level string) {
	w.t.Helper()
	w.wantOK(w.do(name, http.MethodPut, "/api/v1/me/notification-preferences/"+kind+"/"+target.String(), `{"level":"`+level+`"}`))
}

// setMySetting sends putMySetting as name for key with the JSON value and expects 200.
func (w *world) setMySetting(name, key, value string) {
	w.t.Helper()
	w.wantOK(w.do(name, http.MethodPut, "/api/v1/me/settings/"+key, `{"value":`+value+`}`))
}

// removeMember sends removeGroupMember as the owner and expects 204.
func (w *world) removeMember(group, user ids.ID) {
	w.t.Helper()
	w.want(w.do("owner", http.MethodDelete, "/api/v1/groups/"+group.String()+"/members/"+user.String(), "", key()...), http.StatusNoContent, "")
}

// revoke sends revokeGrant as the owner and expects 200.
func (w *world) revoke(grant ids.ID) {
	w.t.Helper()
	w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/"+grant.String()+"/revoke", `{"reason":"narrow"}`, key()...))
}

// dmChannel writes a dm channel with its main thread as fixture rows (as the tests of M3-18 do)
// and gives the structure source its participants; it returns the channel and the main thread.
func (w *world) dmChannel(participants ...ids.ID) (ids.ID, ids.ID) {
	w.t.Helper()
	ch, th := ids.New(), ids.New()
	w.sql(`INSERT INTO channels (id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
		VALUES ($1, 'dm', '', 'a0', false, 20, $2)`, ch.String(), participants[0].String())
	w.sql(`INSERT INTO threads (id, channel_id, kind, created_by) VALUES ($1, $2, 'main', $3)`, th.String(), ch.String(), participants[0].String())
	w.structure.put(ch, participants...)
	return ch, th
}
