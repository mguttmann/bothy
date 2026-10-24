// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// lifecycleWorld is the world of the cases that change runs and grants: Alex's running run main,
// whose token T binds Lotse's requests until G-1 is revoked.
type lifecycleWorld struct {
	*env
	main run
}

// TestAZP11_SR9RunLifecycle runs the cases of brief M2-14 that change runs and grants on one
// server, in an order where each finds the state it needs: the unbound run operations, the end of
// a run, a foreign run, the revocation of G-1 and, on a new trigger grant of Alex, AZ-P11 for
// writes; last the token check. agent.cancel_grace is long, so a revocation's cancel request does
// not end a run before the case that needs it has finished.
func TestAZP11_SR9RunLifecycle(t *testing.T) {
	t.Parallel()
	e := newEnv(t, harness.Options{}, map[string]string{"agent.cancel_grace": "30m"})
	w := &lifecycleWorld{env: e}
	w.main = e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Hauptlauf")

	t.Run("TestStartRun_HeaderUnaffected", func(t *testing.T) { w.startUnaffected(e.in(t)) })
	t.Run("TestSR9_RunInactive", func(t *testing.T) { w.runInactive(e.in(t)) })
	t.Run("TestSR9_ForeignRunIsBound", func(t *testing.T) { w.foreignRun(e.in(t)) })
	t.Run("TestSR9_FinishOwnRunAfterRevocation", func(t *testing.T) { w.finishAfterRevocation(e.in(t)) })
	t.Run("TestAZP11_WritesEndToEnd", func(t *testing.T) { w.writes(e.in(t)) })
	t.Run("TestSR9_TokenNeverLogged", func(t *testing.T) { e.in(t).noTokenLeak(e.secrets()) })
	t.Run("Contract_NoNullInSuccessBodies", func(t *testing.T) { e.in(t).noNullInSuccess() })
}

// sameOutcome requires that a and b have the same status and, for problems, the same code and
// reason; a and b answer the same operation on two runs, one with the header and one without.
func (e *env) sameOutcome(a, b harness.Response, label string) {
	e.t.Helper()
	pa, pb := problemOf(a), problemOf(b)
	if a.Status != b.Status || pa.Code != pb.Code || pa.Reason != pb.Reason {
		e.t.Fatalf("%s: with the header %d %s %s, without %d %s %s", label, a.Status, pa.Code, pa.Reason, b.Status, pb.Code, pb.Reason)
	}
	if a.Status >= 400 {
		e.wantProblem(a, a.Status, pa.Code, pa.Reason, label+" with the header")
	}
}

// problemOf decodes the problem members of r; a success body gives the zero problem.
func problemOf(r harness.Response) problem {
	var p problem
	if r.Status >= 400 {
		_ = json.Unmarshal(r.Body, &p)
	}
	return p
}

// startUnaffected (brief "TestStartRun_HeaderUnaffected", design note 5): renewInvocationToken,
// startRun and resumeRun of run A with T in the header answer exactly as those of run B without
// it. resumeRun of a waiting run needs the token of an agent.followup, which no module of this
// base emits yet, so both runs resume with their consumed start token and get the same answer
// (403 invocation_token_invalid superseded, AG-31).
func (w *lifecycleWorld) startUnaffected(e *env) {
	a := e.newRun(e.alex, e.lotse, "Support/Allgemein", "Lauf A")
	b := e.newRun(e.alex, e.lotse, "Support/Allgemein", "Lauf B")
	renew := func(r run, tokens ...string) (harness.Response, string) {
		resp := e.do(e.lotse, http.MethodPost, "/runs/"+r.id+"/invocation-token", map[string]any{"invocation_token": r.token}, tokens...)
		if resp.Status != http.StatusOK {
			return resp, ""
		}
		tok := str(e.ok(resp, http.StatusOK, "renewInvocationToken")["invocation_token"])
		e.remember(tok)
		return resp, tok
	}
	ra, ta := renew(a, w.main.token)
	rb, tb := renew(b)
	e.sameOutcome(ra, rb, "renewInvocationToken")
	if ta == "" || tb == "" {
		e.t.Fatal("renewInvocationToken gave no token")
	}
	a.token, b.token = ta, tb
	sa := e.do(e.lotse, http.MethodPost, "/runs/"+a.id+"/start", startBody(a.token), w.main.token)
	sb := e.do(e.lotse, http.MethodPost, "/runs/"+b.id+"/start", startBody(b.token))
	e.sameOutcome(sa, sb, "startRun")
	if st := e.ok(sa, http.StatusOK, "startRun with the header")["state"]; st != "running" {
		e.t.Fatalf("run A is %v after startRun", st)
	}
	for _, r := range []run{a, b} {
		e.must(e.lotse, http.MethodPost, "/runs/"+r.id+"/await-input", map[string]any{"status_line": "Warte auf Antwort"}, http.StatusOK)
	}
	e.sameOutcome(e.do(e.lotse, http.MethodPost, "/runs/"+a.id+"/resume", startBody(a.token), w.main.token),
		e.do(e.lotse, http.MethodPost, "/runs/"+b.id+"/resume", startBody(b.token)), "resumeRun")
	e.sameOutcome(e.do(e.lotse, http.MethodPost, "/runs/"+a.id+"/start", startBody(a.token), w.main.token),
		e.do(e.lotse, http.MethodPost, "/runs/"+b.id+"/start", startBody(b.token)), "startRun of a waiting run")
}

// runInactive (brief "TestSR9_RunInactive", design note 4c): after finishRun(completed) of a run of
// Alex, a bound request with its token answers 403 invocation_token_invalid reason run_inactive and
// the denial carries token_verdict run_inactive with Alex and the run; startRun of that run with
// the token in the header and in the body answers 409 already_started as without the header.
func (w *lifecycleWorld) runInactive(e *env) {
	r := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Lauf C")
	e.must(e.lotse, http.MethodPost, "/runs/"+r.id+"/finish", map[string]any{"state": "completed"}, http.StatusOK)
	msg := e.w.Messages["Support/Allgemein"][0]
	e.wantProblem(e.do(e.lotse, http.MethodGet, "/messages/"+msg, nil, r.token), http.StatusForbidden, "invocation_token_invalid", "run_inactive",
		"getMessage after the end of the run")
	with := e.do(e.lotse, http.MethodPost, "/runs/"+r.id+"/start", startBody(r.token), r.token)
	without := e.do(e.lotse, http.MethodPost, "/runs/"+r.id+"/start", startBody(r.token))
	e.wantProblem(with, http.StatusConflict, "already_started", "", "startRun of the ended run with the header")
	e.sameOutcome(with, without, "startRun of the ended run")
	for _, d := range e.denials("message.read", "", e.lotse.PrincipalID, "invocation_token_invalid") {
		if d.details["token_verdict"] == "run_inactive" {
			if d.details["acting_for_user_id"] != e.alex.PrincipalID || d.details["run_id"] != r.id {
				e.t.Fatalf("deny entry %v, want Alex and run %s", d.details, r.id)
			}
			return
		}
	}
	e.t.Fatal("no deny entry with token_verdict run_inactive")
}

// foreignRun (brief "TestSR9_ForeignRunIsBound", design note 5): the owner makes Lotse the
// operator of Engineering/Werkzeuge and triggers it there (run R2, started by Lotse with its own
// token); getRunContext, patchRun and finishRun(completed) of R2 with Alex's T answer 404 like an
// unknown run, without the header 200.
func (w *lifecycleWorld) foreignRun(e *env) {
	path := "/channels/" + e.w.Channels["Engineering/Werkzeuge"]
	cur := e.must(e.owner, http.MethodGet, path, nil, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	set, err := harness.DoWithStepUp(e.t.Context(), e.owner, harness.Request{Method: http.MethodPut, Path: path + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": e.lotse.PrincipalID, "reason": "acting-for test"}})
	if err != nil {
		e.t.Fatal(err)
	}
	e.ok(set, http.StatusOK, "setChannelOperator")
	r2 := e.startedRun(e.owner, e.lotse, "Engineering/Werkzeuge", "Werkzeug bauen")
	// getRunContext (bound read, M2-13): the context of R2 with T answers 404 like an unknown run,
	// without the header 200.
	got := e.do(e.lotse, http.MethodGet, "/runs/"+r2.id+"/context", nil, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "getRunContext of R2 with T")
	e.sameAnswer(got, e.do(e.lotse, http.MethodGet, "/runs/"+ids.New().String()+"/context", nil, w.main.token), "getRunContext of R2 with T")
	e.ok(e.do(e.lotse, http.MethodGet, "/runs/"+r2.id+"/context", nil), http.StatusOK, "getRunContext of R2 without the header")
	patch := e.statusPatch(r2.id, "Baue das Werkzeug")
	got = e.do(e.lotse, http.MethodPatch, "/runs/"+r2.id, patch, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "patchRun of R2 with T")
	e.sameAnswer(got, e.do(e.lotse, http.MethodPatch, "/runs/"+ids.New().String(), patch, w.main.token), "patchRun of R2 with T")
	e.ok(e.do(e.lotse, http.MethodPatch, "/runs/"+r2.id, patch), http.StatusOK, "patchRun of R2 without the header")
	finish := map[string]any{"state": "completed"}
	got = e.do(e.lotse, http.MethodPost, "/runs/"+r2.id+"/finish", finish, w.main.token)
	unknown := e.do(e.lotse, http.MethodPost, "/runs/"+ids.New().String()+"/finish", finish, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "finishRun of R2 with T")
	e.sameAnswer(got, unknown, "finishRun of R2 with T")
	e.ok(e.do(e.lotse, http.MethodPost, "/runs/"+r2.id+"/finish", finish), http.StatusOK, "finishRun of R2 without the header")
	// reportAgentSession (exception agent_session.report, M2-12): the session report on the
	// thread of R2 with T answers 404 like an unknown thread.
	report := map[string]any{"external_session_ref": "sess-foreign"}
	got = e.do(e.lotse, http.MethodPost, "/threads/"+r2.thread+"/agent-sessions/report", report, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "reportAgentSession on R2's thread with T")
	e.sameAnswer(got, e.do(e.lotse, http.MethodPost, "/threads/"+ids.New().String()+"/agent-sessions/report", report, w.main.token),
		"reportAgentSession on R2's thread with T")
}

// finishAfterRevocation (brief "TestSR9_FinishOwnRunAfterRevocation"): the owner revokes G-1, the
// run gets run.cancel_requested with access_revoked (M2-09); a Support message outside the run's
// thread and getRunContext of the own run (no exception, M2-13) answer 404 with T, while patchRun
// (exception run.update, M2-10), reportAgentSession (exception agent_session.report, M2-12) and
// finishRun(cancelled, access_revoked) of the own run with T answer 200 (exception run.finish).
func (w *lifecycleWorld) finishAfterRevocation(e *env) {
	e.revoke(e.w.Grants["G-1"])
	e.eventually("run.cancel_requested of the run", func() bool {
		return e.textOr("0", `SELECT count(*)::text FROM event_log WHERE type = 'run.cancel_requested' AND resource_id::text = $1
			AND payload->>'reason' = 'access_revoked'`, w.main.id) == "1"
	})
	got := e.do(e.lotse, http.MethodGet, "/messages/"+e.w.Messages["Support/Allgemein"][0], nil, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "getMessage outside the run's thread after the revocation")
	e.sameAnswer(got, e.do(e.lotse, http.MethodGet, "/messages/"+ids.New().String(), nil, w.main.token), "getMessage after the revocation")
	// getRunContext is no SR9 exception (07 SR9): with T the context of the own run answers 404
	// like an unknown run once Alex lost every grant, without the header 200.
	got = e.do(e.lotse, http.MethodGet, "/runs/"+w.main.id+"/context", nil, w.main.token)
	e.wantProblem(got, http.StatusNotFound, "not_found", "", "getRunContext of the own run with T after the revocation")
	e.sameAnswer(got, e.do(e.lotse, http.MethodGet, "/runs/"+ids.New().String()+"/context", nil, w.main.token),
		"getRunContext of the own run with T after the revocation")
	e.ok(e.do(e.lotse, http.MethodGet, "/runs/"+w.main.id+"/context", nil), http.StatusOK,
		"getRunContext of the own run without the header after the revocation")
	e.ok(e.do(e.lotse, http.MethodPatch, "/runs/"+w.main.id, e.statusPatch(w.main.id, "Breche ab"), w.main.token), http.StatusOK,
		"patchRun of the own run with T after the revocation")
	e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+w.main.thread+"/agent-sessions/report", map[string]any{"external_session_ref": "sess-main"},
		w.main.token), http.StatusOK, "reportAgentSession of the own run with T after the revocation")
	m := e.ok(e.do(e.lotse, http.MethodPost, "/runs/"+w.main.id+"/finish", map[string]any{"state": "cancelled", "reason": "access_revoked"},
		w.main.token), http.StatusOK, "finishRun of the own run with T")
	if m["state"] != "cancelled" {
		e.t.Fatalf("the run is %v", m["state"])
	}
}

// writes (AZ-P11 for writes, brief "TestAZP11_WritesEndToEnd"): with a new trigger grant of Alex on
// Support and a new run W, Lotse with write on Engineering/Werkzeuge writes there with W's token:
// 404 while Alex has no grant there, 403 forbidden acting_for_insufficient once Alex reads there,
// 201 without the header. After the owner lowers Alex to read on Support (revoke, new grant), the
// reply in W's own thread answers 201 (exception message.create) and a message into another
// thread of Support/Allgemein answers 403 forbidden acting_for_insufficient.
func (w *lifecycleWorld) writes(e *env) {
	trigger := e.grant("user", e.alex.PrincipalID, "space", e.w.Spaces["Support"], "trigger")
	run := e.startedRun(e.alex, e.lotse, "Support/Allgemein", "Lauf W")
	e.grant("agent", e.lotse.PrincipalID, "channel", e.w.Channels["Engineering/Werkzeuge"], "write")
	werk := e.w.Threads["Engineering/Werkzeuge"]
	msg := func(text string) map[string]any {
		return map[string]any{"client_msg_id": ids.New().String(), "content_md": text}
	}
	e.wantProblem(e.do(e.lotse, http.MethodPost, "/threads/"+werk+"/messages", msg("Ohne Recht von Alex"), run.token),
		http.StatusNotFound, "not_found", "", "createMessage in Werkzeuge with T, Alex without a grant")
	e.grant("user", e.alex.PrincipalID, "channel", e.w.Channels["Engineering/Werkzeuge"], "read")
	e.wantProblem(e.do(e.lotse, http.MethodPost, "/threads/"+werk+"/messages", msg("Alex liest nur"), run.token),
		http.StatusForbidden, "forbidden", "acting_for_insufficient", "createMessage in Werkzeuge with T, Alex reads")
	e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+werk+"/messages", msg("Ohne Kopfzeile")), http.StatusCreated, "createMessage without the header")
	e.revoke(trigger)
	e.grant("user", e.alex.PrincipalID, "space", e.w.Spaces["Support"], "read")
	e.ok(e.do(e.lotse, http.MethodPost, "/threads/"+run.thread+"/messages", msg("Antwort im eigenen Thread"), run.token),
		http.StatusCreated, "the reply in the run's own thread with T")
	e.wantProblem(e.do(e.lotse, http.MethodPost, "/threads/"+e.w.Threads["Support/Allgemein"]+"/messages", msg("Fremder Thread"), run.token),
		http.StatusForbidden, "forbidden", "acting_for_insufficient", "createMessage in another Support thread with T")
}

// statusPatch is a patchRun body (M2-10) that sets the status line of run on its current version,
// read without the header (the owner of the run is the agent).
func (e *env) statusPatch(run, line string) map[string]any {
	e.t.Helper()
	cur := e.ok(e.do(e.lotse, http.MethodGet, "/runs/"+run, nil), http.StatusOK, "getRun")
	return map[string]any{"base_version": cur["version"],
		"ops": []any{map[string]any{"op": "replace", "path": "/status_line", "value": line}}}
}
