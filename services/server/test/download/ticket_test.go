// SPDX-License-Identifier: Apache-2.0

package download_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// TestTicket runs the ticket cases that need a short file.ticket_ttl, a short secrets.cache_ttl,
// the log of the server and the in-memory span exporter (M1-10) on their own server. otel.Setup
// installs global providers, so this test never runs in parallel with another start.
func TestTicket(t *testing.T) {
	sinks := harness.NewTelemetrySinks()
	logs := &safeBuffer{}
	e := start(t, harness.Options{Telemetry: sinks, LogWriter: logs}, map[string]string{
		"file.ticket_ttl": "6s", "secrets.cache_ttl": "1s",
	})
	ttl := e.duration(t, "file.ticket_ttl")
	cacheTTL := e.duration(t, "secrets.cache_ttl")
	file := e.ready(t, "voice note.m4a", bytes.Repeat([]byte("0123456789"), 40))
	unknown := e.fetch(t, "/api/v1/files/"+ids.New().String()+"/content?ticket=x", nil)
	if unknown.Status != http.StatusNotFound {
		t.Fatalf("unknown id with a ticket: %d", unknown.Status)
	}
	var tickets []string
	issue := func(t *testing.T, c *harness.Client) ticket {
		t.Helper()
		before := time.Now()
		tk := newTicket(t, c, file, "original")
		if !strings.HasPrefix(tk.URL, "/api/v1/files/"+file+"/content?ticket=") {
			t.Fatalf("url prefix %q", strings.SplitN(tk.URL, "?", 2)[0])
		}
		if d := tk.ExpiresAt.Sub(before.Add(ttl)); d < -time.Second || d > 2*time.Second {
			t.Errorf("expires_at is %v off now plus file.ticket_ttl", d)
		}
		tickets = append(tickets, strings.SplitN(tk.URL, "?ticket=", 2)[1])
		return tk
	}
	same404 := func(t *testing.T, r harness.Response, what string) {
		t.Helper()
		if r.Status != http.StatusNotFound || problemOf(t, r) != problemOf(t, unknown) {
			t.Errorf("%s: %d %s", what, r.Status, r.Body)
		}
	}

	t.Run("RangeAfterRevocationSessionRevocationExpiry", func(t *testing.T) {
		tk := issue(t, e.alex)
		r := e.fetch(t, tk.URL, header("Range", "bytes=0-9"))
		if r.Status != http.StatusPartialContent || string(r.Body) != "0123456789" || r.Header.Get("Cache-Control") != noStore {
			t.Fatalf("ticket with range: %d %q", r.Status, r.Header.Get("Cache-Control"))
		}
		resp, err := harness.DoWithStepUp(t.Context(), e.w.Owner, harness.Request{Method: http.MethodPost, Path: "/grants/" + e.w.Grants["G-1"] + "/revoke",
			Body: map[string]any{"reason": "download test"}})
		if err != nil || resp.Status != http.StatusOK {
			t.Fatalf("revokeGrant: %v %d %s", err, resp.Status, resp.Body)
		}
		same404(t, e.fetch(t, tk.URL, nil), "after the grant revocation")

		resp, err = harness.DoWithStepUp(t.Context(), e.w.Owner, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
			"subject_kind": "user", "subject_id": e.alex.PrincipalID, "target_kind": "space", "target_id": e.w.Spaces["Support"],
			"level": "trigger", "capabilities": []string{"upload"}, "reason": "download test again"}})
		if err != nil || resp.Status != http.StatusCreated {
			t.Fatalf("createGrant: %v %d %s", err, resp.Status, resp.Body)
		}
		tk = issue(t, e.alex)
		if r := e.fetch(t, tk.URL, nil); r.Status != http.StatusOK {
			t.Fatalf("new ticket after the new grant: %d", r.Status)
		}
		harness.MustDo(t, e.w.Owner, harness.Request{Method: http.MethodPost, Path: "/sessions/" + e.alex.SessionID + "/revoke"}, http.StatusNoContent)
		same404(t, e.fetch(t, tk.URL, nil), "after the session revocation")

		alex, err := e.s.UserSession(t.Context(), e.alex.PrincipalID)
		if err != nil {
			t.Fatal(err)
		}
		tk = issue(t, alex)
		if r := e.fetch(t, tk.URL, nil); r.Status != http.StatusOK {
			t.Fatalf("new ticket of the new session: %d", r.Status)
		}
		time.Sleep(time.Until(tk.ExpiresAt))
		same404(t, e.fetch(t, tk.URL, nil), "after the expiry")

		deadline := time.Now().Add(30 * time.Second)
		for {
			noGrant, revoked := e.denials(t, "no_grant"), e.denials(t, "session_revoked")
			if noGrant > 0 && revoked > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("file.read denials of Alex: no_grant %d, session_revoked %d", noGrant, revoked)
			}
			time.Sleep(100 * time.Millisecond)
		}
		e.alex = alex
	})

	t.Run("KeyRotation", func(t *testing.T) {
		old := issue(t, e.alex)
		e.rotateKey(t)
		time.Sleep(cacheTTL + 500*time.Millisecond)
		if r := e.fetch(t, old.URL, nil); r.Status != http.StatusOK {
			t.Errorf("ticket of the previous key: %d", r.Status)
		}
		fresh := issue(t, e.alex)
		if r := e.fetch(t, fresh.URL, nil); r.Status != http.StatusOK {
			t.Errorf("ticket of the new key: %d", r.Status)
		}
		e.rotateKey(t)
		time.Sleep(cacheTTL + 500*time.Millisecond)
		if r := e.fetch(t, fresh.URL, nil); r.Status != http.StatusOK {
			t.Errorf("ticket of the previous key after the second change: %d", r.Status)
		}
		same404(t, e.fetch(t, old.URL, nil), "ticket of a key two versions back")
	})

	t.Run("NeverInLogsOrSpans", func(t *testing.T) {
		tk := issue(t, e.alex)
		if r := e.fetch(t, tk.URL, nil); r.Status != http.StatusOK {
			t.Fatalf("ticket fetch: %d", r.Status)
		}
		bad := e.fetch(t, "/api/v1/files/"+ids.New().String()+"/content?ticket="+tickets[len(tickets)-1], nil)
		if err := e.s.FlushTelemetry(context.Background()); err != nil {
			t.Fatal(err)
		}
		var forms []string
		for _, raw := range tickets {
			b, err := base64.RawURLEncoding.DecodeString(raw)
			if err != nil {
				t.Fatalf("a ticket of %d characters is not base64url", len(raw))
			}
			forms = append(forms, raw, base64.StdEncoding.EncodeToString(b), base64.RawStdEncoding.EncodeToString(b), hex.EncodeToString(b))
		}
		var hay strings.Builder
		hay.WriteString(logs.String())
		hay.Write(bad.Body)
		for _, s := range sinks.Spans.GetSpans() {
			hay.WriteString(s.Name)
			for _, a := range s.Attributes {
				hay.WriteString(" " + string(a.Key) + "=" + a.Value.Emit())
			}
			for _, ev := range s.Events {
				hay.WriteString(" " + ev.Name)
				for _, a := range ev.Attributes {
					hay.WriteString(" " + string(a.Key) + "=" + a.Value.Emit())
				}
			}
		}
		for _, r := range sinks.Logs.Records() {
			hay.WriteString(" " + r.Body().String())
		}
		all := hay.String()
		for i, f := range forms {
			if strings.Contains(all, f) {
				t.Errorf("form %d of a ticket (length %d) is in the logs, spans or problem bodies", i%4, len(f))
			}
		}
		if !strings.Contains(logs.String(), "/api/v1/files/{file_id}/content") {
			t.Error("the access log does not name the route pattern")
		}
		var n int
		for _, f := range tickets {
			if err := e.db.QueryRow(t.Context(), `SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, f).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d audit rows hold a ticket", n)
			}
		}
	})
}

// denials counts the file.read denials of Alex with reason.
func (e *env) denials(t *testing.T, reason string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(t.Context(), `SELECT count(*) FROM audit_events WHERE action = 'file.read' AND decision = 'deny'
		AND deny_reason = $1 AND actor_principal_id = $2`, reason, e.alex.PrincipalID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// rotateKey replaces file_ticket_key of the secrets directory with a fresh key of 32 bytes as hex.
func (e *env) rotateKey(t *testing.T) {
	t.Helper()
	dir := e.s.SecretsDir
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: the directory needs the search bit; owner only
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, fi.Mode().Perm()) }()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "file_ticket_key")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(hex.EncodeToString(key)), 0o400); err != nil {
		t.Fatal(err)
	}
}
