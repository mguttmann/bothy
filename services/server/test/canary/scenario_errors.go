// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// errorPaths sends requests that fail, each with a canary, so the error sink holds every class
// of answer of brief M1-69 "Scenario" item 4.
func (sc *Scenario) errorPaths() {
	sc.T.Helper()
	o, alex, sam := sc.W.Owner, sc.W.People["Alex"], sc.Observers["Sam"]
	esk := sc.W.Channels["Support/Eskalation"]
	thread := sc.W.Threads["Support/Eskalation"]

	// 400: a display name with a bidi control (SEC-IN-4), duplicate JSON keys, an unknown member
	// whose name is a canary (W7).
	sc.patch(alex, "/me/profile", map[string]any{"display_name": "‮" + sc.Reg.New(ClassContent, "", "bidi display name")}, http.StatusBadRequest)
	dup := sc.Reg.New(ClassContent, "", "duplicate key value")
	sc.call(o, req{method: http.MethodPost, path: "/spaces", body: []byte(`{"name":"Doppelt","name":"` + dup + `"}`)}, http.StatusBadRequest)
	sc.post(o, "/spaces", map[string]any{"name": "Feld", sc.Reg.New(ClassContent, "", "unknown member name"): true}, http.StatusBadRequest)
	// 401 without credentials, with a canary in the body.
	sc.raw(http.MethodPost, "/spaces", nil, []byte(`{"name":"`+sc.Reg.New(ClassContent, "", "space name (401)")+`"}`), "anonymous")
	// 403: a person creates a space.
	sc.post(sam, "/spaces", map[string]any{"name": sc.Reg.New(ClassContent, "", "space name (403)")}, http.StatusForbidden)
	// 404: a foreign message and thread for Sam, with canary content.
	msg := sc.W.Messages["Support/Eskalation"][0]
	sc.call(sam, req{method: http.MethodPatch, path: "/messages/" + msg, header: ifMatch(1), body: map[string]any{
		"content_md": sc.Reg.New(ClassContent, esk, "foreign edit (404)"),
	}}, http.StatusNotFound)
	sc.post(sam, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, esk, "foreign message (404)")}, http.StatusNotFound)
	// 409: a unique conflict on a space name, so a database detail would carry it.
	taken := sc.Reg.New(ClassContent, "", "space name (409)")
	sp := id(sc.post(o, "/spaces", map[string]any{"name": taken}, created...))
	sc.Reg.Rescope(taken, sp)
	sc.post(o, "/spaces", map[string]any{"name": taken}, http.StatusConflict)
	// 412 and 428 on a message edit; 413 for a message over message.max_content_bytes.
	own := id(sc.post(alex, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": newID(), "content_md": sc.Reg.New(ClassContent, esk, "message text (412)")}, created...))
	sc.call(alex, req{method: http.MethodPatch, path: "/messages/" + own, header: ifMatch(999), body: map[string]any{
		"content_md": sc.Reg.New(ClassContent, esk, "message edit (412)"),
	}}, http.StatusPreconditionFailed)
	sc.call(alex, req{method: http.MethodPatch, path: "/messages/" + own, body: map[string]any{
		"content_md": sc.Reg.New(ClassContent, esk, "message edit (428)"),
	}}, http.StatusPreconditionRequired)
	limit, err := strconv.Atoi(sc.Server.ConfigValue("message.max_content_bytes"))
	if err != nil {
		sc.T.Errorf("message.max_content_bytes: %v", err)
	}
	big := sc.Reg.New(ClassContent, esk, "message text (413)")
	sc.post(alex, "/threads/"+thread+"/messages", map[string]any{"client_msg_id": newID(), "content_md": big + " " + strings.Repeat("x", limit)}, http.StatusRequestEntityTooLarge)
	// 422: an Idempotency-Key reused with another body.
	key := header("Idempotency-Key", newID())
	first := sc.Reg.New(ClassContent, "", "space name (idempotency)")
	sp2 := id(sc.call(o, req{method: http.MethodPost, path: "/spaces", header: key, body: map[string]any{"name": first}}, created...))
	sc.Reg.Rescope(first, sp2)
	sc.call(o, req{method: http.MethodPost, path: "/spaces", header: key, body: map[string]any{"name": sc.Reg.New(ClassContent, "", "space name (422)")}}, http.StatusUnprocessableEntity)
}

// gatewayOps sends every client gateway op M1 registers: identify, heartbeat, focus,
// typing.start and typing.stop on a connection of Alex, reauth and resume on further ones, and
// ack on a connection of the agent Lotse.
func (sc *Scenario) gatewayOps() {
	sc.T.Helper()
	alex := sc.W.People["Alex"]
	thread := sc.W.Threads["Support/Eskalation"]
	g, err := alex.Gateway(sc.Ctx)
	if err != nil {
		sc.T.Errorf("gateway of Alex: %v", err)
		return
	}
	for _, f := range []struct {
		op string
		d  any
	}{
		{"heartbeat", map[string]any{"last_seq": 0}},
		{"focus", map[string]any{"thread_ids": []string{thread}}},
		{"typing.start", map[string]any{"thread_id": thread}},
		{"typing.stop", map[string]any{"thread_id": thread}},
	} {
		if err := g.Send(f.op, f.d); err != nil {
			sc.T.Errorf("send %s: %v", f.op, err)
		}
	}
	if err := g.Reauth(); err != nil {
		sc.T.Errorf("reauth: %v", err)
	}
	sc.drain(g, "Alex")
	session := g.Ready.GatewaySessionId.String()
	head := g.Ready.HeadSeq
	g.Close()
	r, err := alex.GatewayResume(sc.Ctx, session, head)
	if err != nil {
		sc.T.Errorf("resume: %v", err)
	} else {
		sc.drain(r, "Alex resumed")
		r.Close()
	}
	lotse := sc.W.Agents["Lotse"]
	a, err := lotse.Gateway(sc.Ctx)
	if err != nil {
		sc.T.Errorf("gateway of Lotse: %v", err)
		return
	}
	if err := a.Send("ack", map[string]any{"seq": a.Ready.HeadSeq}); err != nil {
		sc.T.Errorf("ack: %v", err)
	}
	sc.drain(a, "Lotse")
	a.Close()
}

// drain reads the frames of g for a moment; frames to the principal itself are no sink, but a
// close of the server is a failure.
func (sc *Scenario) drain(g *harness.GatewayConn, who string) {
	sc.T.Helper()
	for {
		ctx, cancel := context.WithTimeout(sc.Ctx, time.Second)
		f, err := g.Next(ctx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				sc.T.Errorf("gateway of %s ended: %v", who, err)
			}
			return
		}
		if f.Op == "error" || f.Op == "invalid_session" {
			sc.T.Errorf("gateway of %s: %s %s", who, f.Op, clip(f.D))
		}
	}
}
