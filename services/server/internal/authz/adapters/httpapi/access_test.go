// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// access paths of the four operations of M1-49.
func spaceAccessPath(id ids.ID) string   { return "/api/v1/spaces/" + id.String() + "/access" }
func channelAccessPath(id ids.ID) string { return "/api/v1/channels/" + id.String() + "/access" }
func viewAsPath(id ids.ID) string        { return "/api/v1/users/" + id.String() + "/view-as" }

const previewPath = "/api/v1/grants/preview"

// counters returns the rows of grants, event_log and audit_events and the sequence counters
// (event_seq, audit_seq, permission_version among them) as text.
func (w *world) counters() string {
	w.t.Helper()
	must(w.t, w.rec.Aggregator().Flush(context.Background()))
	var s string
	w.scalar(&s, `SELECT concat_ws('|',
		(SELECT count(*) FROM grants), (SELECT count(*) FROM event_log), (SELECT count(*) FROM audit_events),
		(SELECT coalesce(string_agg(c::text, ',' ORDER BY c.name), '') FROM sequence_counters c))`)
	return s
}

// TestPreviewRollsBackEverything: one preview leaves the rows of grants, event_log and
// audit_events and every sequence counter as they were, and a LISTEN event_log connection gets
// no notification; a real createGrant afterwards does notify (the listener works).
func TestPreviewRollsBackEverything(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	conn := w.db.SuperConn(t)
	ctx := context.Background()
	if _, err := conn.Exec(ctx, "LISTEN event_log"); err != nil {
		t.Fatal(err)
	}
	before := w.counters()
	m := w.want(w.boss.call(http.MethodPost, previewPath, grantBody("user", w.robin, "space", w.support, "write", "upload")), http.StatusOK, "")
	if m["allowed"] != true || len(m["effects"].([]any)) == 0 {
		t.Fatalf("preview %v", m)
	}
	if after := w.counters(); after != before {
		t.Fatalf("the preview left something behind:\n%s\n%s", before, after)
	}
	wait, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if n, err := conn.WaitForNotification(wait); err == nil {
		t.Fatalf("notification after a preview: %+v", n)
	}
	w.want(w.boss.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.support, "write", "upload")), http.StatusCreated, "")
	wait2, cancel2 := context.WithTimeout(ctx, testkit.Patience)
	defer cancel2()
	if _, err := conn.WaitForNotification(wait2); err != nil {
		t.Fatalf("no notification after createGrant: %v", err)
	}
}

// TestPreviewNeedsNoStepUpAndWritesNoAudit: the owner with a step-up older than
// auth.step_up_max_age gets 200, no audit row appears, and createGrant still asks for the step-up.
func TestPreviewNeedsNoStepUpAndWritesNoAudit(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	stale := w.login(w.ownerID)
	stale.stepUp()
	w.clk.Set(w.clk.Now().Add(idCfg.StepUpMaxAge + time.Minute))
	after := w.auditSeq()
	w.want(stale.call(http.MethodPost, previewPath, grantBody("user", w.robin, "space", w.support, "read")), http.StatusOK, "")
	if rows := w.audits("access.preview", after); len(rows) != 0 {
		t.Fatalf("audit %+v", rows)
	}
	if rows := w.audits("grant.create", after); len(rows) != 0 {
		t.Fatalf("grant.create audit %+v", rows)
	}
	// createGrant still needs the step-up.
	w.want(stale.call(http.MethodPost, "/api/v1/grants", grantBody("user", w.robin, "space", w.support, "read")),
		http.StatusUnauthorized, "step_up_required")
}

// TestPreviewAuthorization: no read on the target 404, read without manage 403, manage 200; a
// delegation limit is 200 with allowed false.
func TestPreviewAuthorization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.login(w.sam)
	w.want(sam.call(http.MethodPost, previewPath, grantBody("user", w.robin, "space", w.research, "read")), http.StatusNotFound, "not_found")
	w.want(sam.call(http.MethodPost, previewPath, grantBody("user", w.robin, "channel", w.allgemein, "read")), http.StatusForbidden, "forbidden")
	alex := w.alexWriteManager()
	m := w.want(alex.call(http.MethodPost, previewPath, grantBody("user", w.robin, "channel", w.allgemein, "read")), http.StatusOK, "")
	if m["allowed"] != true {
		t.Fatalf("manage %v", m)
	}
	m = w.want(alex.call(http.MethodPost, previewPath, grantBody("user", w.robin, "channel", w.allgemein, "trigger")), http.StatusOK, "")
	if m["allowed"] != false || m["denial_reason"] != "delegation_exceeds_own_level" {
		t.Fatalf("delegation %v", m)
	}
}

// TestExplainAuthorization: for both explain operations no read is 404, read without manage 403,
// manage 200.
func TestExplainAuthorization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sam := w.login(w.sam)
	w.want(sam.call(http.MethodGet, channelAccessPath(w.notizen), ""), http.StatusNotFound, "not_found")
	w.want(sam.call(http.MethodGet, spaceAccessPath(w.research), ""), http.StatusNotFound, "not_found")
	w.want(sam.call(http.MethodGet, channelAccessPath(w.allgemein), ""), http.StatusForbidden, "forbidden")
	w.want(sam.call(http.MethodGet, spaceAccessPath(w.marketing), ""), http.StatusForbidden, "forbidden")
	alex := w.manager(w.alex, "write")
	m := w.want(alex.call(http.MethodGet, channelAccessPath(w.allgemein), ""), http.StatusOK, "")
	if m["target_kind"] != "channel" || len(m["entries"].([]any)) == 0 {
		t.Fatalf("explanation %v", m)
	}
	w.want(alex.call(http.MethodGet, spaceAccessPath(w.support), ""), http.StatusOK, "")
}

// TestViewAsRequiresOwnerAndStepUp: a person who is no owner gets 403 owner_only, the owner
// without a fresh step-up 401 step_up_required, with one 200; every attempt has an audit row.
func TestViewAsRequiresOwnerAndStepUp(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	after := w.auditSeq()
	sam := w.login(w.sam)
	sam.stepUp()
	w.wantReason(sam.call(http.MethodGet, viewAsPath(w.alex), ""), http.StatusForbidden, "forbidden", "owner_only")
	stale := w.login(w.ownerID)
	w.want(stale.call(http.MethodGet, viewAsPath(w.alex), ""), http.StatusUnauthorized, "step_up_required")
	w.want(w.boss.call(http.MethodGet, viewAsPath(w.alex), ""), http.StatusOK, "")
	rows := w.audits("access.view_as", after)
	var decisions []string
	for _, r := range rows {
		decisions = append(decisions, r.Decision+"/"+r.Reason)
	}
	slices.Sort(decisions)
	if !slices.Equal(decisions, []string{"allow/", "deny/owner_only", "deny/step_up_required"}) {
		t.Fatalf("audit rows %v", decisions)
	}
	for _, r := range rows {
		if r.Decision == "allow" && (r.ResourceID != w.alex.String() || r.Details["target_user_id"] != w.alex.String() || r.Details["expires_at"] == nil) {
			t.Fatalf("allow row %+v", r)
		}
	}
	w.want(w.boss.call(http.MethodGet, viewAsPath(w.lotse), ""), http.StatusNotFound, "not_found")
	w.want(w.boss.call(http.MethodGet, viewAsPath(ids.New()), ""), http.StatusNotFound, "not_found")
}

// TestViewAsSnapshotOfAlex: with G-1 Alex sees only Support and its channels, with the level and
// capabilities authorize gives; expires_at is the request time plus authz.view_as_max_duration.
func TestViewAsSnapshotOfAlex(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	m := w.want(w.boss.call(http.MethodGet, viewAsPath(w.alex), ""), http.StatusOK, "")
	spaces := m["spaces"].([]any)
	if len(spaces) != 1 {
		t.Fatalf("spaces %v", spaces)
	}
	sp := spaces[0].(map[string]any)
	if sp["space"].(map[string]any)["id"] != w.support.String() || sp["level"] != "trigger" {
		t.Fatalf("space %v", sp)
	}
	var chans []string
	for _, c := range m["channels"].([]any) {
		cv := c.(map[string]any)
		chans = append(chans, cv["channel"].(map[string]any)["id"].(string))
		if cv["level"] != "trigger" || !slices.Equal(anyStrings(cv["capabilities"]), []string{"approve_actions", "upload"}) {
			t.Fatalf("channel %v", cv)
		}
	}
	slices.Sort(chans)
	want := strs(w.allgemein, w.eskalation)
	slices.Sort(want)
	if !slices.Equal(chans, want) {
		t.Fatalf("channels %v, want %v", chans, want)
	}
	exp, err := time.Parse(time.RFC3339Nano, m["expires_at"].(string))
	must(t, err)
	if !exp.Equal(w.clk.Now().Add(viewAsMax)) {
		t.Fatalf("expires_at %v", exp)
	}
}

// anyStrings converts a JSON array of strings.
func anyStrings(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// TestAccessOperationsMatchSpec: every response of the four operations validates against
// openapi.yaml (the harness validates responses) and holds no JSON null (want), also with
// optional members present: inactive, via_agent, guard_blocked, hints with counts, violations.
func TestAccessOperationsMatchSpec(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.robin.String())
	w.grantAs(w.boss.actor(), "user", w.robin, "space", w.support, "read")
	w.want(w.boss.call(http.MethodGet, channelAccessPath(w.allgemein), ""), http.StatusOK, "")
	w.want(w.boss.call(http.MethodGet, spaceAccessPath(w.support), ""), http.StatusOK, "")
	w.want(w.boss.call(http.MethodGet, spaceAccessPath(w.marketing), ""), http.StatusOK, "")
	for _, body := range []string{
		grantBody("user", w.sam, "channel", w.notizen, "trigger"),
		grantBody("group", w.leser, "space", w.support, "trigger", "upload"),
		grantBody("agent", w.lotse, "space", w.support, "trigger"),
		grantBody("user", w.alex, "instance", ids.ID{}, "read", "export", "manage"),
	} {
		w.want(w.boss.call(http.MethodPost, previewPath, body), http.StatusOK, "")
	}
	w.want(w.boss.call(http.MethodGet, viewAsPath(w.alex), ""), http.StatusOK, "")
	w.want(w.boss.call(http.MethodGet, viewAsPath(w.robin), ""), http.StatusOK, "")
}
