// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestIncomingWebhooksMigration: incoming_webhooks exactly as 03 states it (columns with types and
// nullability, foreign keys ON DELETE RESTRICT, unique token_hash, the partial index on
// channel_id).
func TestIncomingWebhooksMigration(t *testing.T) {
	t.Parallel()
	s := harness.Start(t, harness.Options{EgressDenyAll: true})
	ctx := t.Context()
	rows, err := s.AdminDB().Query(ctx, `SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
		FROM information_schema.columns WHERE table_name = 'incoming_webhooks' ORDER BY ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var name, typ, null, def string
		if err := rows.Scan(&name, &typ, &null, &def); err != nil {
			t.Fatal(err)
		}
		got = append(got, name+" "+typ+" "+null+" "+def)
	}
	rows.Close()
	want := []string{
		"id uuid NO ", "app_id uuid NO ", "channel_id uuid NO ", "thread_id uuid YES ", "name text NO ", "token_hash bytea NO ",
		"created_by uuid NO ", "created_at timestamp with time zone NO now()", "last_used_at timestamp with time zone YES ",
		"revoked_at timestamp with time zone YES ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("columns\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var fks string
	if err := s.AdminDB().QueryRow(ctx, `SELECT string_agg(pg_get_constraintdef(oid), ' | ' ORDER BY pg_get_constraintdef(oid))
		FROM pg_constraint WHERE conrelid = 'incoming_webhooks'::regclass AND contype IN ('f', 'u', 'p')`).Scan(&fks); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{
		"FOREIGN KEY (app_id) REFERENCES apps(principal_id) ON DELETE RESTRICT",
		"FOREIGN KEY (channel_id) REFERENCES channels(id) ON DELETE RESTRICT",
		"FOREIGN KEY (created_by) REFERENCES principals(id) ON DELETE RESTRICT",
		"FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE RESTRICT",
		"PRIMARY KEY (id)", "UNIQUE (token_hash)",
	} {
		if !strings.Contains(fks, w) {
			t.Errorf("constraints %q lack %q", fks, w)
		}
	}
	var idx string
	if err := s.AdminDB().QueryRow(ctx, `SELECT indexdef FROM pg_indexes WHERE tablename = 'incoming_webhooks' AND indexdef LIKE '%(channel_id)%'`).Scan(&idx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(idx, "WHERE (revoked_at IS NULL)") {
		t.Fatalf("index %q is not partial on revoked_at IS NULL", idx)
	}
}

// TestCreateIncomingWebhook (design note 8) with TestCreateIncomingWebhook_OneTimeSecret,
// TestListIncomingWebhooks and TestRevokeIncomingWebhook on one world.
func TestCreateIncomingWebhook(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	o := h.w.Owner
	sam, alex, robin := h.w.People["Sam"], h.w.People["Alex"], h.w.People["Robin"]
	path := "/channels/" + h.eskalation + "/incoming-webhooks"
	// Sam gets read with manage on Support/Eskalation from the owner.
	h.must(o, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": sam.PrincipalID, "target_kind": "channel",
		"target_id": h.eskalation, "level": "read", "capabilities": []string{"manage"}, "reason": "Manages the alert hooks"}, http.StatusCreated)

	var first hook
	t.Run("owner", func(t *testing.T) {
		first = h.create(h.tracker, h.eskalation, "")
		if !strings.HasPrefix(first.url, h.s.PublicURL+"/api/v1/hooks/"+first.id+"/bothy_whi_") {
			t.Fatalf("url is not absolute from server.public_url with a whi token")
		}
		var hash []byte
		h.row(`SELECT token_hash FROM incoming_webhooks WHERE id = $1`, []any{first.id}, &hash)
		if len(hash) != 32 || bytes.Contains(hash, []byte(first.token)) || bytes.Contains(hash, []byte(first.token[10:20])) {
			t.Fatal("token_hash is not a 32-byte HMAC free of the token")
		}
		var details string
		h.row(`SELECT details::text FROM audit_events WHERE action = 'webhook_in.create' AND decision = 'allow' AND resource_id = $1`,
			[]any{h.eskalation}, &details)
		var d map[string]any
		if err := json.Unmarshal([]byte(details), &d); err != nil {
			t.Fatal(err)
		}
		delete(d, "ip_hmac")
		delete(d, "ip_key_id")
		if d["webhook_id"] != first.id || d["app_id"] != h.tracker || len(d) != 2 || strings.Contains(details, first.token[10:]) {
			t.Fatalf("audit details %s", details)
		}
	})
	t.Run("sam with manage", func(t *testing.T) {
		h.must(sam, http.MethodPost, path, map[string]any{"app_id": h.tracker, "name": "Cron reports"}, http.StatusCreated)
	})
	t.Run("refusals", func(t *testing.T) {
		fresh := h.s.NewUser(t, harness.UserSpec{Handle: "kim", DisplayName: "Kim", Email: "kim@example.com"})
		h.must(o, http.MethodPost, "/grants", map[string]any{"subject_kind": "user", "subject_id": fresh.PrincipalID, "target_kind": "channel",
			"target_id": h.eskalation, "level": "read", "capabilities": []string{"manage"}, "reason": "Manages hooks"}, http.StatusCreated)
		if r := fresh.Do(h.ctx, harness.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"app_id": h.tracker, "name": "X"},
			Header: idem()}); r.Status != http.StatusUnauthorized || !strings.Contains(string(r.Body), "step_up_required") {
			t.Fatalf("without step-up: %d %s", r.Status, r.Body)
		}
		h.must(alex, http.MethodPost, path, map[string]any{"app_id": h.tracker, "name": "X"}, http.StatusForbidden)
		invisible := h.must(robin, http.MethodPost, path, map[string]any{"app_id": h.tracker, "name": "X"}, http.StatusNotFound)
		unknown := h.must(robin, http.MethodPost, "/channels/"+newID()+"/incoming-webhooks", map[string]any{"app_id": h.tracker, "name": "X"}, http.StatusNotFound)
		if problemCode(invisible.Body) != problemCode(unknown.Body) || problemCode(unknown.Body) != "not_found" {
			t.Fatalf("invisible %s and unknown %s differ", invisible.Body, unknown.Body)
		}
		reporter := h.install("reporter", scopeMessagesWrite)
		for _, c := range []struct {
			body          map[string]any
			pointer, code string
		}{
			{map[string]any{"app_id": newID(), "name": "X"}, "/app_id", "app_not_installed"},
			{map[string]any{"app_id": reporter, "name": "X"}, "/app_id", "app_cannot_write"},
			{map[string]any{"app_id": h.tracker, "name": "X", "thread_id": h.w.Threads["Support/Allgemein"]}, "/thread_id", "not_in_channel"},
			{map[string]any{"app_id": h.tracker, "name": "X", "thread_id": newID()}, "/thread_id", "not_in_channel"},
			{map[string]any{"app_id": h.tracker, "name": "Alerts \u202e"}, "/name", ""},
		} {
			r := h.must(o, http.MethodPost, path, c.body, http.StatusBadRequest)
			if !strings.Contains(string(r.Body), `"pointer":"`+c.pointer+`"`) || c.code != "" && !strings.Contains(string(r.Body), `"code":"`+c.code+`"`) {
				t.Errorf("%v: %s", c.body, r.Body)
			}
		}
	})
	t.Run("one-time secret", func(t *testing.T) {
		hdr := idem()
		req := harness.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"app_id": h.tracker, "name": "Once"}, Header: hdr}
		if r := o.Do(h.ctx, req); r.Status != http.StatusCreated {
			t.Fatalf("first: %d %s", r.Status, r.Body)
		}
		r := o.Do(h.ctx, req)
		if r.Status != http.StatusConflict || !strings.Contains(string(r.Body), `"reason":"one_time_secret"`) || r.Header.Get("Idempotent-Replay") != "true" {
			t.Fatalf("replay: %d %s %v", r.Status, r.Body, r.Header)
		}
		if n := h.count(`SELECT 1 FROM incoming_webhooks WHERE name = 'Once'`); n != 1 {
			t.Fatalf("%d rows", n)
		}
	})
	t.Run("list and revoke", func(t *testing.T) {
		second := h.create(h.tracker, h.eskalation, "")
		h.must(o, http.MethodDelete, "/incoming-webhooks/"+second.id, nil, http.StatusNoContent)
		var list struct {
			Items []map[string]any `json:"items"`
		}
		r := h.must(o, http.MethodGet, path, nil, http.StatusOK)
		r.JSON(t, &list)
		var seen []string
		for _, it := range list.Items {
			seen = append(seen, it["id"].(string))
			if _, ok := it["token_hash"]; ok {
				t.Fatal("the list shows a hash")
			}
		}
		if len(seen) == 0 || seen[0] != first.id || strings.Contains(strings.Join(seen, ","), second.id) || strings.Contains(string(r.Body), "bothy_whi_") {
			t.Fatalf("list %v", seen)
		}
		h.must(alex, http.MethodGet, path, nil, http.StatusForbidden)
		h.must(robin, http.MethodGet, path, nil, http.StatusNotFound)

		if r := h.postPath(second, native("after revoke")); r.status != http.StatusNotFound {
			t.Fatalf("post after revoke: %d", r.status)
		}
		h.must(o, http.MethodDelete, "/incoming-webhooks/"+second.id, nil, http.StatusNoContent)
		var noChange bool
		h.row(`SELECT coalesce((details->>'no_change')::boolean, false) FROM audit_events WHERE action = 'webhook_in.revoke' AND resource_id = $1
			ORDER BY seq DESC LIMIT 1`, []any{second.id}, &noChange)
		if !noChange {
			t.Fatal("the second revoke is not audited with no_change")
		}
		unknown := h.must(robin, http.MethodDelete, "/incoming-webhooks/"+newID(), nil, http.StatusNotFound)
		invisible := h.must(robin, http.MethodDelete, "/incoming-webhooks/"+first.id, nil, http.StatusNotFound)
		if problemCode(unknown.Body) != problemCode(invisible.Body) {
			t.Fatalf("unknown %s, invisible %s", unknown.Body, invisible.Body)
		}
	})
}

// idem returns a header with a new Idempotency-Key.
func idem() http.Header {
	h := http.Header{}
	h.Set("Idempotency-Key", newID())
	return h
}

// problemCode returns the code member of a problem body.
func problemCode(body []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &p)
	return p.Code
}
