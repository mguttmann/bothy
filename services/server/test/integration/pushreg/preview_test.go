// SPDX-License-Identifier: Apache-2.0

package pushreg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// canaryWord is a word of the message text that must never appear in a none preview.
const canaryWord = "Zebrafink"

// samReader returns a world in which Sam has an Android phone with an fcm registration and Alex
// wrote a Markdown message in Support/Allgemein that mentions Sam; it returns Sam's preview token
// and the message.
func samReader(t *testing.T) (*world, string, string) {
	t.Helper()
	w := newWorld(t, nil)
	w.android(w.sam)
	reg := w.register(w.sam, "fcm", fcmToken(), fcmApp)
	msg := w.post(w.alex, w.allgemeinMain, "**Hello** @sam, the "+canaryWord+" `report` is [here](https://example.com/r)", w.sam.PrincipalID)
	return w, reg.PreviewToken, msg
}

// TestPreview_PreviewModeNoneHasNoNames: 13 6.4 table, PLAN row "preview_mode none delivers no
// names", design note 9 c.
func TestPreview_PreviewModeNoneHasNoNames(t *testing.T) {
	w, tok, msg := samReader(t)
	decode := func(r harness.Response) map[string]any {
		t.Helper()
		wantStatus(t, "preview", r, http.StatusOK, "")
		var m map[string]any
		if err := json.Unmarshal(r.Body, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	w.setPreviewMode(w.sam, "full")
	full := decode(w.fetch(w.sam, tok, msg))
	if full["space_name"] != "Support" || full["channel_name"] != "Allgemein" || full["sender_display_name"] != "Alex" {
		t.Errorf("full names %v", full)
	}
	ex, _ := full["excerpt"].(string)
	if !strings.Contains(ex, "Sam") || !strings.Contains(ex, canaryWord) || strings.ContainsAny(ex, "*`[]") || strings.Contains(ex, "@sam") {
		t.Errorf("full excerpt %q", ex)
	}
	if full["kind"] != "mention" || full["message_id"] != msg || full["thread_id"] != w.allgemeinMain {
		t.Errorf("full ids %v", full)
	}

	w.setPreviewMode(w.sam, "names")
	names := decode(w.fetch(w.sam, tok, msg))
	if names["space_name"] != "Support" || names["sender_display_name"] != "Alex" {
		t.Errorf("names %v", names)
	}
	for _, k := range []string{"excerpt", "attachment_count", "image_url"} {
		if _, ok := names[k]; ok {
			t.Errorf("names carries %s", k)
		}
	}

	w.setPreviewMode(w.sam, "none")
	r := w.fetch(w.sam, tok, msg)
	wantStatus(t, "none", r, http.StatusOK, "")
	if got, want := members(t, r.Body), []string{"badge", "kind", "message_id", "thread_id"}; !slices.Equal(got, want) {
		t.Errorf("none members %v, want %v", got, want)
	}
	for _, s := range []string{"Support", "Allgemein", "Alex", canaryWord} {
		if bytes.Contains(r.Body, []byte(s)) {
			t.Errorf("none body holds %q", s)
		}
	}

	// No stored value: notify.default_preview_mode (the owner stored none; a stored value that is
	// not a mode cannot pass putMySetting and counts as unset, TestPreviewBuilder_FieldsByMode).
	owner := w.register(w.s.Owner, "apns", apnsToken(), apnsTopic)
	def := decode(w.fetch(w.s.Owner, owner.PreviewToken, msg))
	wantDefault(t, w, def)
}

// wantDefault checks that m follows notify.default_preview_mode of the server.
func wantDefault(t *testing.T, w *world, m map[string]any) {
	t.Helper()
	switch w.s.ConfigValue("notify.default_preview_mode") {
	case "full":
		if _, ok := m["excerpt"]; !ok {
			t.Errorf("default full without excerpt: %v", m)
		}
	case "names":
		if _, ok := m["excerpt"]; ok || m["space_name"] == nil {
			t.Errorf("default names: %v", m)
		}
	case "none":
		if m["space_name"] != nil {
			t.Errorf("default none with names: %v", m)
		}
	}
}

// TestPreview_RevocationBetweenSendAndFetchIs404: PLAN row "a revocation between sending and
// fetching yields 404", 13 14, design note 8 g.
func TestPreview_RevocationBetweenSendAndFetchIs404(t *testing.T) {
	w, tok, msg := samReader(t)
	wantStatus(t, "before", w.fetch(w.sam, tok, msg), http.StatusOK, "")
	harness.MustDo(t, w.s.Owner, harness.Request{Method: http.MethodDelete, Path: "/groups/" + w.leser + "/members/" + w.sam.PrincipalID}, http.StatusNoContent)
	revoked := w.fetch(w.sam, tok, msg)
	unknown := w.fetch(w.sam, tok, ids.New().String())
	wantStatus(t, "after the revocation", revoked, http.StatusNotFound, "not_found")
	if a, b := normalized(t, revoked), normalized(t, unknown); a != b {
		t.Errorf("revoked and unknown differ:\n%s\n%s", a, b)
	}
}

// TestPreview_DeletedMessageIs404: design note 8 g.
func TestPreview_DeletedMessageIs404(t *testing.T) {
	w, tok, msg := samReader(t)
	harness.MustDo(t, w.alex, harness.Request{Method: http.MethodDelete, Path: "/messages/" + msg}, http.StatusNoContent)
	deleted := w.fetch(w.sam, tok, msg)
	unknown := w.fetch(w.sam, tok, ids.New().String())
	wantStatus(t, "deleted", deleted, http.StatusNotFound, "not_found")
	if a, b := normalized(t, deleted), normalized(t, unknown); a != b {
		t.Errorf("deleted and unknown differ:\n%s\n%s", a, b)
	}
}

// TestPreview_NoRightIs404: Alex's preview token for a message in Kampagnen, which Alex does not
// read, answers like an unknown message.
func TestPreview_NoRightIs404(t *testing.T) {
	w := newWorld(t, nil)
	reg := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	msg := w.post(w.s.Owner, w.kampagnenMain, "plan")
	foreign := w.fetch(w.alex, reg.PreviewToken, msg)
	unknown := w.fetch(w.alex, reg.PreviewToken, ids.New().String())
	wantStatus(t, "foreign", foreign, http.StatusNotFound, "not_found")
	if a, b := normalized(t, foreign), normalized(t, unknown); a != b {
		t.Errorf("foreign and unknown differ:\n%s\n%s", a, b)
	}
}

// TestPreview_AuthenticationFailures: design note 8 a to e.
func TestPreview_AuthenticationFailures(t *testing.T) {
	w, tok, msg := samReader(t)
	other := w.register(w.alex, "apns", apnsToken(), apnsTopic)
	malformed := "bothy_pvw_" + strings.Repeat("A", 43) + "_AAAAAA"
	last := byte('A')
	if tok[len(tok)-1] == 'A' {
		last = 'B'
	}
	badSum := tok[:len(tok)-1] + string(last)
	robinReg := w.register(w.robin, "apns", apnsToken(), apnsTopic)
	harness.MustDo(t, w.robin, harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + robinReg.ID}, http.StatusNoContent)
	unknownTok := robinReg.PreviewToken
	access, _ := w.sam.Tokens()

	for _, tc := range []struct {
		name   string
		p      previewRequest
		status int
		code   string
	}{
		{"no authorization", previewRequest{NoAuth: true, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"two values", previewRequest{Auth: []string{"DPoP " + tok, "DPoP " + tok}, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"bearer", previewRequest{Auth: []string{"Bearer " + tok}, Token: tok, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"malformed", previewRequest{Token: malformed, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"bad checksum", previewRequest{Token: badSum, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"unknown token", previewRequest{Token: unknownTok, Signer: w.robin}, http.StatusUnauthorized, "unauthenticated"},
		{"access token", previewRequest{Token: access, Signer: w.sam}, http.StatusUnauthorized, "unauthenticated"},
		{"other device key", previewRequest{Token: tok, Signer: w.alex}, http.StatusUnauthorized, "invalid_dpop_proof"},
		{"no ath", previewRequest{Token: tok, Signer: w.sam, NoATH: true}, http.StatusUnauthorized, "invalid_dpop_proof"},
		{"ath of another token", previewRequest{Token: tok, Signer: w.sam, ATH: other.PreviewToken}, http.StatusUnauthorized, "invalid_dpop_proof"},
		{"no nonce", previewRequest{Token: tok, Signer: w.sam, NoNonce: true}, http.StatusUnauthorized, "use_dpop_nonce"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.p.Message = msg
			r := w.preview(tc.p)
			wantStatus(t, tc.name, r, tc.status, tc.code)
			if tc.code == "use_dpop_nonce" && r.Header.Get("DPoP-Nonce") == "" {
				t.Error("use_dpop_nonce without DPoP-Nonce")
			}
			if tc.code == "unauthenticated" && r.Header.Get("WWW-Authenticate") == "" {
				t.Error("401 without WWW-Authenticate")
			}
		})
	}
	wantStatus(t, "valid", w.fetch(w.sam, tok, msg), http.StatusOK, "")

	// A registration whose session was revoked while the row stayed active (the row is enabled
	// again as a fixture to reach the session check): 401 session_revoked.
	var samReg string
	if err := w.s.AdminDB().QueryRow(t.Context(), `SELECT id::text FROM push_registrations WHERE user_id = $1`, w.sam.PrincipalID).Scan(&samReg); err != nil {
		t.Fatal(err)
	}
	harness.MustDo(t, w.sam, harness.Request{Method: http.MethodPost, Path: "/auth/logout"}, http.StatusNoContent)
	if _, err := w.s.AdminDB().Exec(t.Context(), `UPDATE push_registrations SET disabled_at = NULL, disabled_reason = NULL WHERE id = $1`, samReg); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, "revoked session", w.fetch(w.sam, tok, msg), http.StatusUnauthorized, "session_revoked")
}

// TestPreviewToken_OnlyOnPreviewEndpoint: 13 6.4 "It is valid only for this endpoint".
func TestPreviewToken_OnlyOnPreviewEndpoint(t *testing.T) {
	w, tok, _ := samReader(t)
	target := w.s.APIBase + "/me/devices"
	for _, scheme := range []string{"DPoP", "Bearer"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", scheme+" "+tok)
		w.mu.Lock()
		nonce := w.nonce
		w.mu.Unlock()
		req.Header.Set("DPoP", proof(t, w.sam, http.MethodGet, target, nonce, tok))
		resp, err := w.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s preview token on listMyDevices = %d, want 401", scheme, resp.StatusCode)
		}
	}
}

// TestPreview_KindParameter: design note 9 a.
func TestPreview_KindParameter(t *testing.T) {
	w, tok, msg := samReader(t)
	for _, k := range []string{"", "sync"} {
		p := previewRequest{Token: tok, Signer: w.sam, Message: msg, Kind: k, OmitKind: k == ""}
		r := w.preview(p)
		wantStatus(t, "kind "+k, r, http.StatusBadRequest, "validation_failed")
		if !strings.Contains(string(r.Body), "kind") {
			t.Errorf("kind %q: %s", k, r.Body)
		}
	}
	for _, k := range []string{"security", "admin"} {
		r := w.preview(previewRequest{Token: tok, Signer: w.sam, Message: msg, Kind: k})
		wantStatus(t, "kind "+k, r, http.StatusBadRequest, "validation_failed")
		if !strings.Contains(string(r.Body), "#query/kind") || !strings.Contains(string(r.Body), `"invalid"`) {
			t.Errorf("kind %s: %s", k, r.Body)
		}
	}
	for _, k := range []string{"message", "mention", "dm", "run_done", "run_failed", "input_needed"} {
		r := w.preview(previewRequest{Token: tok, Signer: w.sam, Message: msg, Kind: k})
		wantStatus(t, "kind "+k, r, http.StatusOK, "")
		var m struct {
			Kind string `json:"kind"`
		}
		r.JSON(t, &m)
		if m.Kind != k {
			t.Errorf("kind %s echoed as %s", k, m.Kind)
		}
	}
}

// TestFetchMetric_And_NoTokensInTelemetry: bothy.push.fetch counts a 200 and a 404 fetch with a
// valid token and nothing for a malformed or unknown token (design note 2); no device token or
// preview token reaches a log record, span or problem body (SEC-LOG-4, SEC-SEC-3).
func TestFetchMetric_And_NoTokensInTelemetry(t *testing.T) {
	sinks := harness.NewTelemetrySinks()
	var logs bytes.Buffer
	var logMu syncWriter
	logMu.w = &logs
	w := newWorld(t, nil, func(o *harness.Options) { o.Telemetry = sinks; o.LogWriter = &logMu })
	w.android(w.sam)
	devTok := fcmToken()
	reg := w.register(w.sam, "fcm", devTok, fcmApp)
	msg := w.post(w.alex, w.allgemeinMain, "hello @sam", w.sam.PrincipalID)
	// The first fetch learns the nonce; its use_dpop_nonce attempt counts too (design note 2).
	wantStatus(t, "nonce", w.fetch(w.sam, reg.PreviewToken, msg), http.StatusOK, "")
	before := fetchCount(t, sinks, "fcm")
	wantStatus(t, "200", w.fetch(w.sam, reg.PreviewToken, msg), http.StatusOK, "")
	wantStatus(t, "404", w.fetch(w.sam, reg.PreviewToken, ids.New().String()), http.StatusNotFound, "")
	bad := w.preview(previewRequest{Token: "bothy_pvw_" + strings.Repeat("A", 43) + "_AAAAAA", Signer: w.sam, Message: msg})
	wantStatus(t, "malformed", bad, http.StatusUnauthorized, "")
	noToken(t, "a problem body", bad.Body, devTok, reg.PreviewToken)
	if got := fetchCount(t, sinks, "fcm") - before; got != 2 {
		t.Errorf("bothy.push.fetch rose by %d, want 2", got)
	}
	harness.MustDo(t, w.sam, harness.Request{Method: http.MethodDelete, Path: "/push-registrations/" + reg.ID}, http.StatusNoContent)
	if err := w.s.FlushTelemetry(t.Context()); err != nil {
		t.Fatal(err)
	}
	logMu.mu.Lock()
	noToken(t, "the server log", logs.Bytes(), devTok, reg.PreviewToken)
	logMu.mu.Unlock()
	for _, s := range sinks.Spans.GetSpans() {
		var b strings.Builder
		b.WriteString(s.Name)
		for _, a := range s.Attributes {
			fmt.Fprintf(&b, " %s=%s", a.Key, a.Value.String())
		}
		for _, e := range s.Events {
			b.WriteString(" " + e.Name)
			for _, a := range e.Attributes {
				fmt.Fprintf(&b, " %s=%s", a.Key, a.Value.String())
			}
		}
		noToken(t, "a span", []byte(b.String()), devTok, reg.PreviewToken)
	}
	for _, r := range sinks.Logs.Records() {
		var b strings.Builder
		b.WriteString(r.Body().String())
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			b.WriteString(" " + string(kv.Key) + "=" + kv.Value.String())
			return true
		})
		noToken(t, "an OTLP log record", []byte(b.String()), devTok, reg.PreviewToken)
	}
}

// fetchCount returns the sum of bothy.push.fetch for provider.
func fetchCount(t *testing.T, sinks *harness.TelemetrySinks, provider string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := sinks.Metrics.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	var n int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "bothy.push.fetch" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("bothy.push.fetch is %T", m.Data)
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value("bothy.push.provider"); ok && v.AsString() == provider {
					n += dp.Value
				}
			}
		}
	}
	return n
}
