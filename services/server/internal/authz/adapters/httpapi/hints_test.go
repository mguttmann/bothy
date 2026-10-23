// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestPermissionHintsREST (M6-07): GET /access/hints answers the owner with the hint
// agent_personal of Assistent and the labels of the page, validated against the contract; an
// unknown kind is 400; a manager of Support reads Support's list but neither the instance (403)
// nor a space without read (404).
func TestPermissionHintsREST(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	m := w.want(w.boss.call(http.MethodGet, "/api/v1/access/hints?kind=agent_personal", ""), http.StatusOK, "")
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items %v", m)
	}
	h := items[0].(map[string]any)
	if h["kind"] != "agent_personal" || h["agent_id"] != w.assistent.String() || h["credential_holder_id"] != w.ownerID.String() {
		t.Fatalf("hint %v", h)
	}
	labels := m["labels"].(map[string]any)
	if !strings.Contains(principalNames(labels), "assistent") {
		t.Fatalf("labels %v", labels)
	}
	for _, k := range []string{"groups", "spaces", "channels"} {
		if _, ok := labels[k].([]any); !ok {
			t.Errorf("labels without %s: %v", k, labels)
		}
	}
	w.want(w.boss.call(http.MethodGet, "/api/v1/access/hints?kind=other", ""), http.StatusBadRequest, "validation_failed")
	sam := w.manager(w.sam, "read")
	w.want(sam.call(http.MethodGet, "/api/v1/access/hints?space_id="+w.support.String(), ""), http.StatusOK, "")
	w.want(sam.call(http.MethodGet, "/api/v1/access/hints", ""), http.StatusForbidden, "forbidden")
	w.want(sam.call(http.MethodGet, "/api/v1/access/hints?space_id="+w.engineering.String(), ""), http.StatusNotFound, "not_found")
}

// principalNames joins the handles of the principals of labels.
func principalNames(labels map[string]any) string {
	var out []string
	for _, p := range labels["principals"].([]any) {
		out = append(out, p.(map[string]any)["handle"].(string))
	}
	return strings.Join(out, ",")
}

// TestExpiringAccessREST (M6-07): GET /access/expiring lists the owner a token of Lotse expiring
// within authz.expiry_reminder_lead with its id, holder and expiry, never its hash or prefix, and
// leaves a personal token out; a manager of Support gets no tokens.
func TestExpiringAccessREST(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	tok := ids.New()
	insert := func(id, holder ids.ID, prefix string, exp time.Time) {
		w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, expires_at, created_by, created_at)
			VALUES ($1, $2, 'ci', $3, $4, $5, $6, $7)`, id.String(), holder.String(), prefix, []byte(id.String()), exp, w.ownerID.String(), start)
	}
	insert(tok, w.lotse, "bothy_agt_EXAMPL", start.Add(time.Hour))
	insert(ids.New(), w.sam, "bothy_pat_EXAMPL", start.Add(time.Hour))
	r := w.boss.call(http.MethodGet, "/api/v1/access/expiring", "")
	m := w.want(r, http.StatusOK, "")
	items := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items %v", m)
	}
	it := items[0].(map[string]any)
	if it["kind"] != "token" || it["token_id"] != tok.String() || it["principal_id"] != w.lotse.String() || it["expires_at"] == nil {
		t.Fatalf("item %v", it)
	}
	body := string(r.Body)
	if strings.Contains(body, "bothy_") || strings.Contains(body, "token_hash") {
		t.Fatalf("the answer shows token material: %s", body)
	}
	sam := w.manager(w.sam, "read")
	m = w.want(sam.call(http.MethodGet, "/api/v1/access/expiring?space_id="+w.support.String(), ""), http.StatusOK, "")
	if items := m["items"].([]any); len(items) != 0 {
		t.Fatalf("a manager sees tokens or more: %v", items)
	}
}
