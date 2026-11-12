// SPDX-License-Identifier: Apache-2.0

package privacy_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestExport_AsTheAppAsksForIt follows the calls of the row "Export of your data" in the settings
// of the apps (apps/client/lib/settings/data_export.dart) on the real server: who the caller is,
// the caller's requests by subject_user_id, the request for the export, a second one while the
// first is open, the list again after the job, the archive, and the answers after
// privacy.export_ttl. The answers are logged: the test of the app
// (apps/client/test/settings/data_export_port_test.dart) sends them through the generated client
// and its decoder.
func TestExport_AsTheAppAsksForIt(t *testing.T) {
	t.Parallel()
	const ttl = time.Hour
	w := newWorld(t, map[string]string{"privacy.export_ttl": "1h"}, nil)
	o, alex := w.s.Owner, w.w.People["Alex"]

	var me struct {
		Principal struct {
			ID string `json:"id"`
		} `json:"principal"`
	}
	meResp := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/me"}, http.StatusOK)
	meResp.JSON(t, &me)
	if me.Principal.ID != alex.PrincipalID {
		t.Fatalf("getMe names %q, want %q", me.Principal.ID, alex.PrincipalID)
	}
	t.Logf("getMe: %s", meResp.Body)

	// An owner files a deletion for Alex: a request of another kind about the same person, which
	// the row of the export skips.
	create(t, o, map[string]any{"kind": "deletion", "reason": "Asked by mail", "subject_user_id": alex.PrincipalID})

	list := func() ([]request, []byte) {
		t.Helper()
		var page struct {
			Items      []request `json:"items"`
			NextCursor string    `json:"next_cursor"`
		}
		resp := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet,
			Path: "/gdpr-requests?subject_user_id=" + alex.PrincipalID}, http.StatusOK)
		resp.JSON(t, &page)
		if page.NextCursor != "" {
			t.Fatalf("the requests of one person need a second page: %s", resp.Body)
		}
		return page.Items, resp.Body
	}
	exports := func(items []request) []request {
		var out []request
		for _, r := range items {
			if r.Kind == "export" {
				out = append(out, r)
			}
		}
		return out
	}
	before, body := list()
	if len(before) != 1 || len(exports(before)) != 0 {
		t.Fatalf("list before the request = %s", body)
	}
	t.Logf("listGdprRequests before: %s", body)

	// Without a step-up the request is refused; the app's client then confirms and repeats.
	body0 := map[string]any{"kind": "export", "reason": "Requested by the person in the app"}
	first := alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/gdpr-requests",
		Header: http.Header{"Idempotency-Key": {ids.New().String()}}, Body: body0})
	if first.Status != http.StatusUnauthorized || !strings.Contains(string(first.Body), "step_up_required") {
		t.Fatalf("request without step-up = %d %s", first.Status, first.Body)
	}
	created := harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests", Body: body0}, http.StatusCreated)
	var r request
	created.JSON(t, &r)
	if r.Kind != "export" || r.State != "approved" {
		t.Fatalf("created = %s", created.Body)
	}
	t.Logf("createGdprRequest: %s", created.Body)

	// The newest request is first.
	approved, body := list()
	if len(approved) != 2 || approved[0].ID != r.ID || approved[0].State != "approved" {
		t.Fatalf("list after the request = %s", body)
	}
	t.Logf("listGdprRequests approved: %s", body)

	// A second request while the first is open names the open one.
	again, err := harness.DoWithStepUp(t.Context(), alex, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests", Body: body0})
	if err != nil {
		t.Fatal(err)
	}
	var problem struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	again.JSON(t, &problem)
	if again.Status != http.StatusConflict || problem.Code != "conflict" || problem.Reason != "request_open" {
		t.Fatalf("second open request = %d %s", again.Status, again.Body)
	}
	t.Logf("createGdprRequest again: %d %s", again.Status, again.Body)

	// The archive of a request that is not completed is not there yet.
	early := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusConflict)
	t.Logf("downloadGdprExport early: %d %s", early.Status, early.Body)

	if err := w.uc().RunExport(t.Context(), ids.MustParse(r.ID)); err != nil {
		t.Fatal(err)
	}
	done, body := list()
	if len(done) != 2 || done[0].ID != r.ID || done[0].State != "completed" || done[0].ResultExpiresAt == "" {
		t.Fatalf("list after the job = %s", body)
	}
	if until, err := time.Parse(time.RFC3339, done[0].ResultExpiresAt); err != nil || !until.After(w.clk.Now()) {
		t.Fatalf("result_expires_at = %q (%v)", done[0].ResultExpiresAt, err)
	}
	t.Logf("listGdprRequests completed: %s", body)

	archive := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusOK)
	if ct := archive.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if _, err := zip.NewReader(bytes.NewReader(archive.Body), int64(len(archive.Body))); err != nil {
		t.Fatalf("the archive is no ZIP: %v", err)
	}

	// After privacy.export_ttl the archive is gone, and the list still tells when it expired, so
	// the row can say "no longer available" without asking for the archive.
	w.clk.Advance(ttl)
	if n, err := w.uc().ExpirePackages(t.Context(), 10); err != nil || n != 1 {
		t.Fatalf("expiry = %d, %v", n, err)
	}
	gone := harness.MustDo(t, alex, harness.Request{Method: http.MethodGet, Path: "/gdpr-requests/" + r.ID + "/export"}, http.StatusGone)
	t.Logf("downloadGdprExport gone: %d %s", gone.Status, gone.Body)
	expired, body := list()
	if len(expired) != 2 || expired[0].State != "completed" {
		t.Fatalf("list after the expiry = %s", body)
	}
	if expired[0].ResultExpiresAt != "" {
		at, err := time.Parse(time.RFC3339, expired[0].ResultExpiresAt)
		if err != nil || at.After(w.clk.Now()) {
			t.Fatalf("result_expires_at after the expiry = %q (%v)", expired[0].ResultExpiresAt, err)
		}
	}
	t.Logf("listGdprRequests expired: %s", body)

	// A new export can be requested once the first is completed.
	var second request
	harness.MustDo(t, alex, harness.Request{Method: http.MethodPost, Path: "/gdpr-requests", Body: body0}, http.StatusCreated).JSON(t, &second)
	if second.ID == r.ID || second.State != "approved" {
		raw, _ := json.Marshal(second)
		t.Fatalf("second export = %s", raw)
	}
}
