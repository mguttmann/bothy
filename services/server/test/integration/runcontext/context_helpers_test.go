// SPDX-License-Identifier: Apache-2.0

package runcontext_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// ctxWorld is the world of the brief M2-13 ("Tests"): the world of 07 11.1 with the extensions of
// M2-05, Sam's trigger on Support/Allgemein (grant sam), a person writer with write only on
// Allgemein, Texter's write on Allgemein, an app Tracker, and on Allgemein and Eskalation
// thread_require_mention (a message in a thread triggers only with a mention, so the earlier
// messages of a test open no runs) and on Allgemein allow_mentioned_agents (guest runs).
type ctxWorld struct {
	*world
	writer, tracker, samGrant ids.ID
}

// newCtxWorld builds the world with the default byte bounds of 09 section 21.
func newCtxWorld(t *testing.T) *ctxWorld {
	t.Helper()
	return newCtxWorldWith(t, defaultContextMax, defaultContextMessageMax)
}

// newCtxWorldWith builds the world with agent.context_max_bytes contextMax and
// agent.context_message_max_bytes contextMessageMax.
func newCtxWorldWith(t *testing.T, contextMax, contextMessageMax int) *ctxWorld {
	t.Helper()
	w := newLimitedWorld(t, chapterSearch(t), contextMax, contextMessageMax)
	w.build()
	// The instance row of bothy migrate up (snapshot and catch-up read head and epoch).
	w.sql(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 1, $2) ON CONFLICT (id) DO NOTHING`,
		ids.New().String(), ids.New().String())
	c := &ctxWorld{world: w}
	c.samGrant = w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"channel","target_id":"` +
		w.allgemein.String() + `","level":"trigger","reason":"Second support writer"`)
	c.writer = w.person("writer", false)
	w.sql(`UPDATE principals SET display_name = 'Writer' WHERE id = $1`, c.writer.String())
	w.grant(`"subject_kind":"user","subject_id":"` + c.writer.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"write","reason":"Writer"`)
	w.grant(`"subject_kind":"agent","subject_id":"` + w.texter.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"write","reason":"Guest in general"`)
	c.tracker = ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', 'tracker', 'Tracker')`, c.tracker.String())
	w.behavior(w.allgemein, `"thread_require_mention":true,"allow_mentioned_agents":true`)
	w.behavior(w.eskalation, `"thread_require_mention":true`)
	return c
}

// insert writes a message of origin and kind directly into thread the way the import and the
// apps paths will (they have no real path in this build): the next thread_seq of the thread, the
// thread's channel, a server client_msg_id; a system line is silent and empty. It returns the id.
func (c *ctxWorld) insert(thread, author ids.ID, origin, kind, systemCode, content string) string {
	c.t.Helper()
	id := ids.New()
	c.sql(`WITH t AS (UPDATE threads SET last_message_seq = last_message_seq + 1, message_count = message_count + 1 WHERE id = $1
			RETURNING channel_id, last_message_seq)
		INSERT INTO messages (id, thread_id, channel_id, thread_seq, author_principal_id, client_msg_id, client_payload_sha256, origin, kind,
			system_code, content_md, silent)
		SELECT $2, $1, t.channel_id, t.last_message_seq, $3, $2, '\x00', $4, $5, NULLIF($6, ''), $7, $5 = 'system' FROM t`,
		thread.String(), id.String(), author.String(), origin, kind, systemCode, content)
	return id.String()
}

// say posts a text message as name into thread and returns its id.
func (c *ctxWorld) say(name string, thread ids.ID, text string) string {
	c.t.Helper()
	return c.post(name, thread, text)["id"].(string)
}

// sayReply posts a reply to message as name into thread and returns its id.
func (c *ctxWorld) sayReply(name string, thread ids.ID, text, replyTo string) string {
	c.t.Helper()
	m := c.want(c.do(name, http.MethodPost, "/api/v1/threads/"+thread.String()+"/messages",
		`{"client_msg_id":"`+ids.New().String()+`","content_md":"`+text+`","reply_to_message_id":"`+replyTo+`"}`, key()...), http.StatusCreated, "")
	return m["id"].(string)
}

// triggerWith posts text as name into thread and returns the run it opened and the message.
func (c *ctxWorld) triggerWith(name string, thread ids.ID, text string) (run, message string) {
	c.t.Helper()
	message = c.say(name, thread, text)
	r, ok := c.runOf(message)
	if !ok {
		c.t.Fatalf("the message %q opened no run", text)
	}
	return r.ID, message
}

// readContext sends getRunContext as name.
func (c *ctxWorld) readContext(name, run string) *httpservertest.Response {
	return c.do(name, http.MethodGet, "/api/v1/runs/"+run+"/context", "")
}

// contextOf reads the context of run as name, expects 200 and checks the body: no JSON null
// (want), no optional member that does not apply (absent rather than false or empty) and no
// e-mail address of an author.
func (c *ctxWorld) contextOf(name, run string) map[string]any {
	c.t.Helper()
	body := c.wantOK(c.readContext(name, run))
	for _, e := range allElements(body) {
		for _, k := range []string{"truncated", "already_delivered"} {
			if v, ok := e[k]; ok && v != true {
				c.t.Errorf("element %v: %s is %v instead of absent", e["thread_seq"], k, v)
			}
		}
		if v, ok := e["reply_to_message_id"]; ok && v == "" {
			c.t.Errorf("element %v: empty reply_to_message_id", e["thread_seq"])
		}
		a := e["author"].(map[string]any)
		for _, k := range []string{"id", "kind", "handle", "display_name"} {
			if s, _ := a[k].(string); s == "" {
				c.t.Errorf("element %v: author without %s", e["thread_seq"], k)
			}
		}
		raw, _ := json.Marshal(a)
		if strings.Contains(string(raw), "@") || strings.Contains(string(raw), "email") {
			c.t.Errorf("element %v: author carries an address: %s", e["thread_seq"], raw)
		}
	}
	return body
}

// part returns the elements of the member name ("thread_context" or "channel_context") of a
// context; nil when the member is absent.
func part(body map[string]any, name string) []map[string]any {
	list, ok := body[name].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		out = append(out, e.(map[string]any))
	}
	return out
}

// allElements returns the triggering message and every element of both parts.
func allElements(body map[string]any) []map[string]any {
	out := []map[string]any{body["trigger_message"].(map[string]any)}
	out = append(out, part(body, "channel_context")...)
	return append(out, part(body, "thread_context")...)
}

// byID indexes elements by message_id.
func byID(list []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, e := range list {
		out[e["message_id"].(string)] = e
	}
	return out
}

// seqOf reads thread_seq of message.
func (c *ctxWorld) seqOf(message string) int64 {
	return c.count(`SELECT thread_seq FROM messages WHERE id = $1`, message)
}

// num reads a JSON number member as int64.
func num(m map[string]any, k string) int64 {
	v, _ := m[k].(float64)
	return int64(v)
}

// contents lists content_md of elements.
func contents(list []map[string]any) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e["content_md"].(string))
	}
	return out
}

// wantMark fails unless element e has role and trust.
func wantMark(t *testing.T, what string, e map[string]any, role, trust string) {
	t.Helper()
	if e == nil {
		t.Fatalf("%s: missing", what)
	}
	if e["role"] != role || e["trust"] != trust {
		t.Errorf("%s: role %v trust %v, want %s %s", what, e["role"], e["trust"], role, trust)
	}
}

// startWithSession starts run as Lotse with the external session ref and reports it started, so
// the next run of the thread resumes it (09 9.1).
func (c *ctxWorld) startWithSession(run, ref string, thread ids.ID) {
	c.t.Helper()
	c.wantOK(c.start("lotse", run, c.deliver(run), `"external_session_ref":"`+ref+`"`))
	c.want(c.report("lotse", thread, `{"external_session_ref":"`+ref+`","started":true}`, append(match(1), key()...)...), http.StatusOK, "")
}

// sameNotFound fails unless a and b are 404 with equal status, body (without correlation_id and
// instance) and headers (without Correlation-Id and Date) (AZ-P4 status and body part).
func sameNotFound(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != http.StatusNotFound || b.StatusCode != http.StatusNotFound {
		t.Fatalf("statuses %d %d: %s %s", a.StatusCode, b.StatusCode, a.Body, b.Body)
	}
	ha, hb := a.Header.Clone(), b.Header.Clone()
	for _, k := range []string{"Correlation-Id", "Date"} {
		ha.Del(k)
		hb.Del(k)
	}
	if fa, fb := fmt.Sprint(ha), fmt.Sprint(hb); fa != fb {
		t.Fatalf("404 headers differ:\n%s\n%s", fa, fb)
	}
	pa, pb := a.Problem(t), b.Problem(t)
	for _, k := range []string{"correlation_id", "instance"} {
		delete(pa, k)
		delete(pb, k)
	}
	ja, _ := json.Marshal(pa)
	jb, _ := json.Marshal(pb)
	if string(ja) != string(jb) {
		t.Fatalf("404 bodies differ:\n%s\n%s", ja, jb)
	}
}

// patchFollowup sends patchRun as Lotse: replace /followups/<message>/state with state at the
// current version of the run (M2-11).
func (c *ctxWorld) patchFollowup(run, message, state string) *httpservertest.Response {
	v := c.count(`SELECT version FROM runs WHERE id = $1`, run)
	body := `{"base_version":` + fmt.Sprint(v) + `,"ops":[{"op":"replace","path":"/followups/` + message + `/state","value":"` + state + `"}]}`
	return c.do("lotse", http.MethodPatch, "/api/v1/runs/"+run, body)
}
