// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestComputerSessions runs the server cases of docs/spec/27-computer-use.md 15 on one server.
func TestComputerSessions(t *testing.T) {
	w := newWorld(t)

	var first grant
	var firstRun run
	t.Run("CU-T01 open in the owner's run on a listed desktop", func(t *testing.T) {
		firstRun = w.newRun(w.owner, w.runner, w.desk, "do X on desk-a")
		var list struct {
			Items []struct {
				HostID string `json:"host_id"`
				Online bool   `json:"online"`
				Busy   bool   `json:"busy"`
			} `json:"items"`
		}
		w.must(w.runner, http.MethodGet, "/computer/desktops", nil, http.StatusOK, firstRun.token).JSON(t, &list)
		if len(list.Items) != 1 || list.Items[0].HostID != w.hostA.id.String() || !list.Items[0].Online || list.Items[0].Busy {
			t.Fatalf("desktops %+v, want desk-a only (desk-b is assigned to Alex)", list.Items)
		}
		first = w.open(firstRun, w.hostA)
		if st, _, _ := w.state(first.SessionID); st != "opening" {
			t.Fatalf("state %s", st)
		}
		if first.AttachPath != "/api/v1/computer/sessions/"+first.SessionID+"/attach" || first.NoisePublicKey == "" || first.Token == "" {
			t.Fatalf("grant %+v", first)
		}
		claims := tokenClaims(t, first.Token)
		for k, v := range map[string]string{"aud": w.hostA.id.String(), "sid": first.SessionID, "sub": w.owner.PrincipalID,
			"run": firstRun.id, "agt": w.runner.PrincipalID, "cek": cek} {
			if claims[k] != v {
				t.Errorf("claim %s = %v, want %s", k, claims[k], v)
			}
		}
		if claims["own"] != true || claims["gen"] != float64(1) || tokenHeader(t, first.Token)["typ"] != "computer-session+jwt" {
			t.Errorf("claims %v", claims)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'computer.open' AND decision = 'allow' AND resource_id = $1
			AND details->>'run_id' = $2 AND details->>'generation' = '1'`, first.SessionID, firstRun.id); n != 1 {
			t.Fatalf("allow entries of computer.open %d", n)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'computer_session.changed' AND resource_id = $1
			AND audience_principal_id = $2`, first.SessionID, w.owner.PrincipalID); n < 1 {
			t.Fatalf("computer_session.changed %d", n)
		}
		open := w.hostA.waitOp("computer.open", first.SessionID)
		var op struct {
			UserID   string         `json:"user_id"`
			DataPath string         `json:"data_path"`
			Settings map[string]any `json:"settings"`
		}
		if err := json.Unmarshal(open, &op); err != nil || op.UserID != w.owner.PrincipalID ||
			op.DataPath != "/api/v1/hosts/computer-sessions/"+first.SessionID+"/stream" || len(op.Settings) != 8 {
			t.Fatalf("computer.open %s", open)
		}
	})

	t.Run("CU-T06 busy host and busy run", func(t *testing.T) {
		other := w.newRun(w.owner, w.runner, w.desk, "do Y on desk-a")
		resp := w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}, other.token)
		w.refused(resp, http.StatusConflict, "conflict", "computer_busy")
		third := w.pairHost("desk-c", "")
		third.connect(enabled(2))
		resp = w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": third.id.String(), "client_ephemeral_key_hash": cek}, firstRun.token)
		w.refused(resp, http.StatusConflict, "conflict", "computer_run_busy")
	})

	var agentSide, hostSide *conn
	t.Run("CU-T17 attach once, refusals audited", func(t *testing.T) {
		bad := w.attach(first.AttachPath, first.Token[:len(first.Token)-4]+"AAAA")
		if code, reason := bad.closed(t); code != 4004 || reason != "token_invalid" {
			t.Fatalf("forged token: %d %s", code, reason)
		}
		hostSide = w.hostA.dialData(first.SessionID)
		agentSide = w.attach(first.AttachPath, first.Token)
		w.eventually("attached_at", func() bool {
			return w.text(`SELECT attached_at::text FROM computer_sessions WHERE id = $1`, first.SessionID) != ""
		})
		again := w.attach(first.AttachPath, first.Token)
		if code, reason := again.closed(t); code != 4004 || reason != "token_replayed" {
			t.Fatalf("second attach: %d %s", code, reason)
		}
		// Denials are recorded outside the business transaction and aggregated per
		// audit.deny_aggregate_window, so they arrive a little later.
		w.eventually("the audited attach denials", func() bool {
			return w.count(`SELECT count(*) FROM audit_events WHERE action = 'computer.attach' AND decision = 'deny' AND resource_id = $1`,
				first.SessionID) >= 1
		})
	})

	t.Run("CU-T18 the relay carries the sentinel unchanged", func(t *testing.T) {
		agentSide.send(t, sentinel)
		if got := hostSide.recv(t); string(got) != string(sentinel) {
			t.Fatal("the host received other bytes")
		}
		hostSide.send(t, append([]byte("reply:"), sentinel...))
		if got := agentSide.recv(t); string(got) != "reply:"+string(sentinel) {
			t.Fatal("the agent received other bytes")
		}
	})

	t.Run("host ops started, state and pause", func(t *testing.T) {
		w.hostA.send("computer.started", map[string]any{"session_id": first.SessionID, "state": "active"})
		w.eventually("active", func() bool { st, _, _ := w.state(first.SessionID); return st == "active" })
		w.hostA.send("computer.state", map[string]any{"session_id": first.SessionID, "state": "paused", "reason": "locked", "action_count": 3,
			"front_app": map[string]any{"id": "", "name": "", "signer": ""}})
		w.eventually("paused", func() bool { st, _, _ := w.state(first.SessionID); return st == "paused" })
		w.hostA.send("computer.state", map[string]any{"session_id": first.SessionID, "state": "active", "action_count": 5,
			"reason": "", "front_app": map[string]any{"id": "com.example.editor", "name": "Editor", "signer": "ABCDE12345"}})
		w.eventually("active again", func() bool {
			return w.text(`SELECT state || ':' || action_count || ':' || front_app FROM computer_sessions WHERE id = $1`, first.SessionID) ==
				"active:5:com.example.editor"
		})
	})

	t.Run("CU-T16 stop by a client", func(t *testing.T) {
		var v struct {
			State       string `json:"state"`
			CloseReason string `json:"close_reason"`
			CloseDetail string `json:"close_detail"`
			StoppedBy   string `json:"stopped_by"`
			Desktop     string `json:"desktop"`
		}
		w.must(w.owner, http.MethodPost, "/computer/sessions/"+first.SessionID+"/stop", nil, http.StatusOK).JSON(t, &v)
		if v.State != "closed" || v.CloseReason != "stopped_by_user" || v.CloseDetail != "client_stop" || v.StoppedBy != w.owner.PrincipalID || v.Desktop != "desk-a" {
			t.Fatalf("stop %+v", v)
		}
		if code, reason := agentSide.closed(t); code != 4100 || reason != "stopped_by_user" {
			t.Fatalf("agent side %d %s", code, reason)
		}
		if code, _ := hostSide.closed(t); code != 4100 {
			t.Fatalf("host side %d", code)
		}
		cl := w.hostA.waitOp("computer.close", first.SessionID)
		if !strings.Contains(string(cl), `"reason":"stopped_by_user"`) {
			t.Fatalf("computer.close %s", cl)
		}
		d := w.terminateAudit(first.SessionID)
		if d["close_reason"] != "stopped_by_user" || d["close_detail"] != "client_stop" || d["duration_ms"] == nil || d["paused_ms"] == nil {
			t.Fatalf("computer.terminate %v", d)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'computer.stop' AND decision = 'allow' AND resource_id = $1`, first.SessionID); n != 1 {
			t.Fatalf("computer.stop entries %d", n)
		}
		// A stop of a closed session answers unchanged.
		w.must(w.owner, http.MethodPost, "/computer/sessions/"+first.SessionID+"/stop", nil, http.StatusOK)
		// Alex is no owner: the session is invisible to Alex.
		w.refused(w.do(w.alex, http.MethodPost, "/computer/sessions/"+first.SessionID+"/stop", nil), http.StatusNotFound, "not_found", "")
	})

	t.Run("list own sessions", func(t *testing.T) {
		var list struct {
			Items []map[string]any `json:"items"`
		}
		w.must(w.owner, http.MethodGet, "/computer/sessions?state=closed", nil, http.StatusOK).JSON(t, &list)
		if len(list.Items) == 0 || list.Items[0]["token_jti"] != nil {
			t.Fatalf("own sessions %v", list.Items)
		}
		w.refused(w.do(w.alex, http.MethodGet, "/computer/sessions", nil), http.StatusForbidden, "forbidden", "owner_only")
	})

	t.Run("CU-T16 remote disable holds until a new local switch-on", func(t *testing.T) {
		g := w.open(firstRun, w.hostA)
		var h struct {
			ComputerListed             *bool `json:"computer_listed"`
			ComputerDisabledGeneration *int  `json:"computer_disabled_generation"`
		}
		w.must(w.owner, http.MethodPost, "/hosts/"+w.hostA.id.String()+"/computer/disable", nil, http.StatusOK).JSON(t, &h)
		if h.ComputerDisabledGeneration == nil || *h.ComputerDisabledGeneration != 1 || h.ComputerListed == nil || *h.ComputerListed {
			t.Fatalf("host after disable %+v", h)
		}
		w.waitClosed(g.SessionID, "device_disabled", "")
		if d := w.hostA.waitOp("computer.disable", ""); !strings.Contains(string(d), `"generation":1`) {
			t.Fatalf("computer.disable %s", d)
		}
		resp := w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}, firstRun.token)
		w.refused(resp, http.StatusForbidden, "forbidden", "computer_not_enabled")
		// The host still reports generation 1 enabled (it missed nothing here, but it would stay
		// unlisted either way); a new local switch-on raises the generation and lists it again.
		w.hostA.report(enabled(2))
		g2 := w.open(firstRun, w.hostA)
		w.must(w.owner, http.MethodPost, "/computer/sessions/"+g2.SessionID+"/stop", nil, http.StatusOK)
		// A host that is not enabled is no desktop.
		w.hostA.report(map[string]any{"helper": "connected", "enabled": false, "generation": 2, "os_account": "alex",
			"permissions": map[string]any{"capture": "granted", "input": "granted"}, "helper_version": "1.0.0", "signature": "verified", "locked": false})
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek},
			firstRun.token), http.StatusForbidden, "forbidden", "computer_not_enabled")
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'host.connect' AND resource_id = $1 AND details ? 'to_enabled'`, w.hostA.id); n < 2 {
			t.Fatalf("enablement changes audited %d", n)
		}
		w.hostA.report(enabled(3))
	})

	t.Run("CU-T04 assigned to another person, not readable", func(t *testing.T) {
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostB.id.String(), "client_ephemeral_key_hash": cek},
			firstRun.token), http.StatusForbidden, "forbidden", "computer_not_enabled")
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": ids.New().String(), "client_ephemeral_key_hash": cek},
			firstRun.token), http.StatusNotFound, "not_found", "")
	})

	t.Run("CU-T03 a run below host", func(t *testing.T) {
		r := w.newRun(w.owner, w.texter, w.notes, "do Z with the scoped agent")
		w.refused(w.do(w.texter, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}, r.token),
			http.StatusForbidden, "forbidden", "computer_tool_class")
		w.refused(w.do(w.texter, http.MethodGet, "/computer/desktops", nil, r.token), http.StatusForbidden, "forbidden", "computer_tool_class")
	})

	t.Run("CU-T05 without the binding", func(t *testing.T) {
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}),
			http.StatusForbidden, "forbidden", "computer_not_owner_run")
		w.refused(w.do(w.owner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}),
			http.StatusForbidden, "forbidden", "subject_kind_not_allowed")
	})

	t.Run("CU-T16 the run's end ends the session and the agent session", func(t *testing.T) {
		r := w.newRun(w.owner, w.runner, w.desk, "do W and finish")
		w.must(w.runner, http.MethodPost, "/runs/"+r.id+"/start", map[string]any{"invocation_token": r.token, "billing_mode_used": "organization",
			"tool_class_used": "host", "external_session_ref": "cu-session-1"}, http.StatusOK)
		g := w.open(r, w.hostA)
		w.must(w.runner, http.MethodPost, "/runs/"+r.id+"/finish", map[string]any{"state": "completed"}, http.StatusOK, r.token)
		w.waitClosed(g.SessionID, "run_ended", "")
		if got := w.text(`SELECT end_reason FROM agent_sessions WHERE thread_id = $1 ORDER BY created_at DESC LIMIT 1`, r.thread); got != "computer_use" {
			t.Fatalf("end_reason of the agent session %q", got)
		}
		resp := w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}, r.token)
		w.refused(resp, http.StatusForbidden, "invocation_token_invalid", "run_inactive")
		// CU-T17: an attach after the run ended is refused.
		late := w.attach(g.AttachPath, g.Token)
		if code, _ := late.closed(t); code != 4100 {
			t.Fatalf("attach after the run's end: %d", code)
		}
	})

	t.Run("CU-T25 follow-ups of another person", func(t *testing.T) {
		// Only an owner triggers a host agent (tool_class_owner_only), so the other person is a
		// second owner: SR13 item 5 holds back even an owner's follow-up.
		w.must(w.owner, http.MethodPost, "/owners", map[string]any{"user_id": w.alex.PrincipalID, "reason": "computer test"}, http.StatusCreated)
		r := w.newRun(w.owner, w.runner, w.desk, "do V with a follow-up")
		w.must(w.runner, http.MethodPost, "/runs/"+r.id+"/start", map[string]any{"invocation_token": r.token, "billing_mode_used": "organization",
			"tool_class_used": "host"}, http.StatusOK)
		g := w.open(r, w.hostA)
		msg := w.created(w.must(w.alex, http.MethodPost, "/threads/"+r.thread+"/messages",
			map[string]any{"client_msg_id": ids.New().String(), "content_md": "also do U"}, http.StatusCreated))
		w.eventually("the follow-up row", func() bool {
			return w.count(`SELECT count(*) FROM run_followups WHERE message_id = $1`, msg) == 1
		})
		for _, ev := range w.page(w.runner).Events {
			if ev.Type == "agent.followup" && strings.Contains(string(ev.Payload), msg) {
				t.Fatal("a follow-up of another person reached the run with a computer session")
			}
		}
		if st := w.text(`SELECT state FROM run_followups WHERE message_id = $1`, msg); st != "queued" {
			t.Fatalf("follow-up state %s", st)
		}
		w.must(w.owner, http.MethodPost, "/computer/sessions/"+g.SessionID+"/stop", nil, http.StatusOK)
		w.must(w.runner, http.MethodPost, "/runs/"+r.id+"/finish", map[string]any{"state": "completed"}, http.StatusOK, r.token)
		w.eventually("deferred", func() bool { return w.text(`SELECT state FROM run_followups WHERE message_id = $1`, msg) != "queued" })
	})

	t.Run("CU-T16 lost owner role ends the session", func(t *testing.T) {
		// Alex is a second owner since CU-T25.
		r := w.newRun(w.alex, w.runner, w.desk, "do T on desk-b")
		w.hostB.report(enabled(2))
		g := w.open(r, w.hostB)
		w.must(w.owner, http.MethodPost, "/owners/"+w.alex.PrincipalID+"/revoke", map[string]any{"reason": "computer test"}, http.StatusNoContent)
		w.waitClosed(g.SessionID, "access_revoked", "")
		// The run is still active, but its person is no owner now (SR13 item 3); desk-b stays
		// readable to Alex as its assignee, so the answer is 403, not 404.
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostB.id.String(), "client_ephemeral_key_hash": cek}, r.token),
			http.StatusForbidden, "forbidden", "computer_not_owner_run")
		w.refused(w.do(w.runner, http.MethodPost, "/computer/sessions", map[string]any{"host_id": w.hostA.id.String(), "client_ephemeral_key_hash": cek}, r.token),
			http.StatusNotFound, "not_found", "")
	})

	t.Run("CU-T16 host ops end and a lost control connection", func(t *testing.T) {
		r := w.newRun(w.owner, w.runner, w.desk, "do S")
		g := w.open(r, w.hostA)
		w.hostA.send("computer.ended", map[string]any{"session_id": g.SessionID, "close_reason": "stopped_by_user", "close_detail": "local_escape",
			"action_counts": map[string]any{"left_click": 2, "type": 1}, "apps": []any{map[string]any{"id": "com.example.Editor", "name": "Editor", "signer": "ABCDE12345"}}, "paused_ms": 0})
		w.waitClosed(g.SessionID, "stopped_by_user", "local_escape")
		d := w.terminateAudit(g.SessionID)
		if !strings.Contains(asJSON(d["action_counts"]), "left_click:2") || !strings.Contains(asJSON(d["apps"]), "com.example.editor:abcde12345") {
			t.Fatalf("computer.terminate %v", d)
		}
		g2 := w.open(r, w.hostA)
		w.hostA.drop()
		w.waitClosed(g2.SessionID, "connection_lost", "")
		w.hostA.connect(enabled(3))
	})

	t.Run("open timeout through the sweep", func(t *testing.T) {
		r := w.newRun(w.owner, w.runner, w.desk, "do R and never attach")
		g := w.open(r, w.hostA)
		w.clk.Advance(3 * time.Minute)
		w.sweep()
		w.waitClosed(g.SessionID, "open_failed", "open_timeout")
		// CU-T17: an attach after exp is refused (the session is closed now as well).
		late := w.attach(g.AttachPath, g.Token)
		if code, _ := late.closed(t); code != 4100 && code != 4004 {
			t.Fatalf("attach after exp: %d", code)
		}
	})

	t.Run("CU-T18 no sentinel byte on the server", func(t *testing.T) {
		rows, err := w.db.Query(w.ctx(), `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`)
		if err != nil {
			t.Fatal(err)
		}
		var tables []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			tables = append(tables, n)
		}
		rows.Close()
		hexForm := "%" + hexOf(sentinel) + "%"
		for _, table := range tables {
			var n int
			q := `SELECT count(*) FROM "` + table + `" x WHERE row_to_json(x)::text LIKE '%SCREEN-BYTES-7f3c9a51%'
				OR encode(convert_to(row_to_json(x)::text, 'UTF8'), 'hex') LIKE $1`
			if err := w.db.QueryRow(w.ctx(), q, hexForm).Scan(&n); err != nil {
				t.Fatalf("%s: %v", table, err)
			}
			if n > 0 {
				t.Errorf("table %s holds the sentinel", table)
			}
		}
		if containsAny(w.logs.String()) {
			t.Error("the log holds the sentinel")
		}
	})
}

// asJSON returns v as JSON text.
func asJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// hexOf returns the lower-case hex of b.
func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(b))
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&15])
	}
	return string(out)
}

// tokenClaims decodes the payload of a compact JWS (the test reads its own token).
func tokenClaims(t *testing.T, tok string) map[string]any {
	t.Helper()
	return segment(t, tok, 1)
}

// tokenHeader decodes the protected header of a compact JWS.
func tokenHeader(t *testing.T, tok string) map[string]any {
	t.Helper()
	return segment(t, tok, 0)
}

// segment decodes segment i of a compact JWS as a JSON object.
func segment(t *testing.T, tok string, i int) map[string]any {
	t.Helper()
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatal("no compact JWS")
	}
	b, err := base64RawURL(parts[i])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
