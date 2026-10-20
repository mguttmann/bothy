// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// safeBuffer is a log writer safe for the concurrent writes of the server.
type safeBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// TestPostIncomingWebhook_PathAndBearer: the same native body through both operations gives 201
// with message_id and thread_id; the message has origin app, author Tracker, kind text, silent
// false; one message.created with actor kind app (TestPostIncomingWebhook_MentionsNotResolved:
// @alex and @lotse stay text, no mentions row).
func TestPostIncomingWebhook_PathAndBearer(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	for name, send := range map[string]func(hook, any) reply{"path": h.postPath, "bearer": h.postBearer} {
		t.Run(name, func(t *testing.T) {
			content := "Disk usage above threshold, ping @alex and @lotse"
			r := send(k, native(content, "client_msg_id", newID()))
			if r.status != http.StatusCreated {
				t.Fatalf("%d %s", r.status, r.body)
			}
			msg, thread := r.result(t)
			m := h.message(msg)
			if m.origin != "app" || m.authorID != h.tracker || m.kind != "text" || m.silent || m.threadID != thread || thread != h.eskalationMain ||
				m.content != content || m.triggerState != nil {
				t.Fatalf("stored %+v", m)
			}
			if n := h.count(`SELECT 1 FROM mentions WHERE message_id = $1`, msg); n != 0 {
				t.Fatalf("%d mentions rows", n)
			}
			if n := h.events("message.created", msg, "app"); n != 1 {
				t.Fatalf("%d message.created events of an app", n)
			}
		})
	}
}

// TestPostIncomingWebhook_NotFoundEquality: every refusal of the token or of the app's rights
// answers exactly the 404 of an unknown webhook, for both operations.
func TestPostIncomingWebhook_NotFoundEquality(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	o := h.w.Owner
	good := h.create(h.tracker, h.eskalation, "")
	wrong, err := apitoken.Generate(nil, apitoken.KindIncomingWebhook)
	if err != nil {
		t.Fatal(err)
	}
	revoked := h.create(h.tracker, h.eskalation, "")
	h.must(o, http.MethodDelete, "/incoming-webhooks/"+revoked.id, nil, http.StatusNoContent)
	// A second app whose installation is uninstalled, one suspended, one whose grant is revoked.
	gone := h.install("gone", scopeMessagesWrite)
	h.grantApp(gone, h.eskalation, "write")
	goneHook := h.create(gone, h.eskalation, "")
	h.must(o, http.MethodPost, "/apps/"+gone+"/uninstall", map[string]any{"reason": "No longer used"}, http.StatusNoContent)
	paused := h.install("paused", scopeMessagesWrite)
	h.grantApp(paused, h.eskalation, "write")
	pausedHook := h.create(paused, h.eskalation, "")
	h.must(o, http.MethodPost, "/apps/"+paused+"/suspend", nil, http.StatusNoContent)
	lost := h.install("lost", scopeMessagesWrite)
	lostGrant := h.grantApp(lost, h.eskalation, "write")
	lostHook := h.create(lost, h.eskalation, "")
	h.must(o, http.MethodPost, "/grants/"+lostGrant+"/revoke", map[string]any{"reason": "Revoked"}, http.StatusOK)
	// An archived channel and a fixed thread moved to another channel.
	archived := created(t, h.must(o, http.MethodPost, "/spaces/"+h.w.Spaces["Support"]+"/channels", map[string]any{"kind": "text", "name": "Alt"}, http.StatusCreated))
	h.grantApp(h.tracker, archived, "write")
	archivedHook := h.create(h.tracker, archived, "")
	h.must(o, http.MethodPost, "/channels/"+archived+"/archive", map[string]any{"reason": "Done"}, http.StatusOK)
	thread := h.w.Threads["Support/Eskalation"]
	movedHook := h.create(h.tracker, h.eskalation, thread)
	h.must(o, http.MethodPost, "/threads/"+thread+"/move", map[string]any{"target_channel_id": h.w.Channels["Support/Allgemein"],
		"reason": "Belongs to the general channel"}, http.StatusOK)
	body := native("Ping")
	cases := map[string]hook{
		"unknown id":        {id: newID(), token: good.token},
		"wrong token":       {id: good.id, token: wrong},
		"malformed token":   {id: good.id, token: "not-a-token"},
		"revoked":           revoked,
		"uninstalled app":   goneHook,
		"suspended app":     pausedHook,
		"grant revoked":     lostHook,
		"archived channel":  archivedHook,
		"fixed thread gone": movedHook,
	}
	var ref reply
	have := false
	check := func(name string, r reply) {
		t.Helper()
		if r.status != http.StatusNotFound {
			t.Errorf("%s: %d %s", name, r.status, r.body)
			return
		}
		if !have {
			ref, have = r, true
			return
		}
		if norm(r.body) != norm(ref.body) || !sameHeaders(r.header, ref.header) {
			t.Errorf("%s: %s %v differs from %s %v", name, r.body, r.header, ref.body, ref.header)
		}
	}
	for name, k := range cases {
		check(name+" path", h.postPath(k, body))
		check(name+" bearer", h.postBearer(k, body))
	}
	basic := http.Header{}
	basic.Set("Authorization", "Basic "+good.token)
	check("basic scheme", h.send(h.s.APIBase+"/hooks/"+good.id, marshal(t, body), basic))
	check("bearer missing", h.send(h.s.APIBase+"/hooks/"+good.id, marshal(t, body), nil))
	if r := h.postPath(good, body); r.status != http.StatusCreated {
		t.Fatalf("control: %d %s", r.status, r.body)
	}
	// The uninstall revoked the webhook of the app (11 section 4.3), the archive the webhook of the
	// channel, while the suspended app keeps its row.
	if n := h.count(`SELECT 1 FROM incoming_webhooks WHERE id = ANY($1::uuid[]) AND revoked_at IS NOT NULL`, []string{goneHook.id, archivedHook.id}); n != 2 {
		t.Fatalf("%d of the uninstalled and archived webhooks revoked", n)
	}
	if n := h.count(`SELECT 1 FROM incoming_webhooks WHERE id = $1 AND revoked_at IS NULL`, pausedHook.id); n != 1 {
		t.Fatal("the suspended app lost its webhook")
	}
	h.must(o, http.MethodPost, "/apps/"+paused+"/resume", nil, http.StatusNoContent)
	if r := h.postPath(pausedHook, body); r.status != http.StatusCreated {
		t.Fatalf("after resume: %d %s", r.status, r.body)
	}
}

// norm drops the members that differ per request (correlation_id, instance).
func norm(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return string(body)
	}
	delete(m, "correlation_id")
	delete(m, "instance")
	b, _ := json.Marshal(m)
	return string(b)
}

// sameHeaders compares the header names and values except those that differ per request.
func sameHeaders(a, b http.Header) bool {
	keys := func(h http.Header) []string {
		var out []string
		for k, v := range h {
			switch k {
			case "Correlation-Id", "Ratelimit", "Ratelimit-Policy", "Date", "Content-Length":
				continue
			}
			out = append(out, k+"="+strings.Join(v, ","))
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(keys(a), keys(b))
}

// TestPostIncomingWebhook_OrderOfChecks (11 8.4): 429 before the token, 404 for the token before
// the size, 413 for webhook_in.max_body_bytes, 400 for the format, 404 for authorize, 201.
func TestPostIncomingWebhook_OrderOfChecks(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{cfg: map[string]string{
		"webhook_in.max_body_bytes": "2048", "webhook_in.formats_enabled": `["native"]`,
		"ratelimit.rl.webhook_in.per_minute": "1", "ratelimit.rl.webhook_in.burst": "8",
	}})
	k := h.create(h.tracker, h.eskalation, "")
	wrong := hook{id: k.id, token: "bothy_whi_wrong"}
	big := native(strings.Repeat("x", 4096))
	if r := h.postPath(wrong, big); r.status != http.StatusNotFound {
		t.Fatalf("wrong token, big body: %d %s", r.status, r.body)
	}
	r := h.postPath(k, big)
	if code, _, _, detail, _ := r.problem(); r.status != http.StatusRequestEntityTooLarge || code != "payload_too_large" || !strings.Contains(detail, "webhook_in.max_body_bytes") {
		t.Fatalf("big body: %d %s", r.status, r.body)
	}
	if r := h.postPath(k, map[string]any{"content_md": "x", "colour": "red"}); r.status != http.StatusBadRequest || !strings.Contains(string(r.body), "unknown_member") {
		t.Fatalf("unknown member: %d %s", r.status, r.body)
	}
	am := map[string]any{"version": "4", "status": "firing", "alerts": []any{map[string]any{"status": "firing", "labels": map[string]any{"alertname": "A"}}}}
	if r := h.postPath(k, am); r.status != http.StatusBadRequest || !strings.Contains(string(r.body), "format_disabled") {
		t.Fatalf("switched-off format: %d %s", r.status, r.body)
	}
	if r := h.postPath(k, native("ok")); r.status != http.StatusCreated {
		t.Fatalf("valid: %d %s", r.status, r.body)
	}
	h.must(h.w.Owner, http.MethodPost, "/grants/"+h.g6+"/revoke", map[string]any{"reason": "Revoked"}, http.StatusOK)
	if r := h.postPath(k, native("after the grant")); r.status != http.StatusNotFound {
		t.Fatalf("app without write: %d %s", r.status, r.body)
	}
	// The bucket of the webhook is now exhausted (burst 8, one per minute): even a wrong token
	// gets 429 before any token check.
	for range 8 {
		h.postPath(wrong, native("drain"))
	}
	r = h.postPath(wrong, native("x"))
	if code, _, _, _, _ := r.problem(); r.status != http.StatusTooManyRequests || code != "rate_limited" || r.header.Get("Retry-After") == "" ||
		!strings.Contains(r.header.Get("RateLimit-Policy"), "rl.webhook_in") {
		t.Fatalf("exhausted: %d %s %v", r.status, r.body, r.header)
	}
	// Another webhook keeps its own bucket (TestPostIncomingWebhook_RateLimitPerWebhook).
	h.grantApp(h.tracker, h.eskalation, "write")
	other := h.create(h.tracker, h.eskalation, "")
	if r := h.postPath(other, native("other")); r.status != http.StatusCreated {
		t.Fatalf("other webhook: %d %s", r.status, r.body)
	}
}

// TestPostIncomingWebhook_FailedTokensCountAgainstPublicAuth: wrong tokens of one client IP use up
// rl.public_auth; then a valid token from that IP gets 429 without a token check; another client
// IP still gets 201.
func TestPostIncomingWebhook_FailedTokensCountAgainstPublicAuth(t *testing.T) {
	t.Parallel()
	// The world is built from the loopback address; the failures come from another client IP.
	h2 := newHW(t, options{cfg: map[string]string{"ratelimit.rl.public_auth.per_minute": "1", "ratelimit.rl.public_auth.burst": "200"}})
	k := h2.create(h2.tracker, h2.eskalation, "")
	from := func(ip string) http.Header {
		hdr := http.Header{}
		hdr.Set("X-Forwarded-For", ip)
		return hdr
	}
	for range 200 {
		if r := h2.send(h2.s.APIBase+"/hooks/"+k.id+"/bothy_whi_wrong", marshal(t, native("x")), from("192.0.2.7")); r.status != http.StatusNotFound {
			t.Fatalf("wrong token: %d %s", r.status, r.body)
		}
	}
	r := h2.send(h2.s.APIBase+"/hooks/"+k.id+"/"+k.token, marshal(t, native("x")), from("192.0.2.7"))
	if code, _, _, _, _ := r.problem(); r.status != http.StatusTooManyRequests || code != "rate_limited" || !strings.Contains(r.header.Get("RateLimit-Policy"), "rl.public_auth") {
		t.Fatalf("valid token after failures: %d %s %v", r.status, r.body, r.header)
	}
	if n := h2.count(`SELECT 1 FROM messages WHERE author_principal_id = $1`, h2.tracker); n != 0 {
		t.Fatalf("%d messages after the refused call", n)
	}
	if r := h2.send(h2.s.APIBase+"/hooks/"+k.id+"/"+k.token, marshal(t, native("x")), from("192.0.2.8")); r.status != http.StatusCreated {
		t.Fatalf("other client IP: %d %s", r.status, r.body)
	}
}

// TestPostIncomingWebhook_ContentRules: content of exactly message.max_content_bytes is 201, one
// byte more 413 naming message.max_content_bytes; a mixed body format_mixed; a client_msg_id of
// version 4 not_uuid_v7.
func TestPostIncomingWebhook_ContentRules(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{cfg: map[string]string{"message.max_content_bytes": "1000"}})
	k := h.create(h.tracker, h.eskalation, "")
	if r := h.postPath(k, native(strings.Repeat("é", 500))); r.status != http.StatusCreated {
		t.Fatalf("exactly the limit: %d %s", r.status, r.body)
	}
	r := h.postPath(k, native(strings.Repeat("é", 500)+"x"))
	if code, _, _, detail, _ := r.problem(); r.status != http.StatusRequestEntityTooLarge || code != "payload_too_large" || !strings.Contains(detail, "message.max_content_bytes") {
		t.Fatalf("one byte more: %d %s", r.status, r.body)
	}
	r = h.postPath(k, map[string]any{"content_md": "x", "status": "firing"})
	if _, fc, ptr, _, _ := r.problem(); r.status != http.StatusBadRequest || fc != "format_mixed" || ptr != "/content_md" {
		t.Fatalf("mixed: %d %s", r.status, r.body)
	}
	r = h.postPath(k, native("x", "client_msg_id", "0192f4a0-7b1c-4f4e-8959-c3574b6ad93b"))
	if _, fc, ptr, _, _ := r.problem(); r.status != http.StatusBadRequest || fc != "not_uuid_v7" || ptr != "/client_msg_id" {
		t.Fatalf("version 4: %d %s", r.status, r.body)
	}
}

// TestPostIncomingWebhook_TargetRules (11 8.3, design note 5).
func TestPostIncomingWebhook_TargetRules(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	escThread := h.w.Threads["Support/Eskalation"]
	fixed := h.create(h.tracker, h.eskalation, escThread)
	plain := h.create(h.tracker, h.eskalation, "")
	forum := h.create(h.tracker, h.forum, "")
	t.Run("fixed thread", func(t *testing.T) {
		_, th := h.postPath(fixed, native("x", "thread_id", h.eskalationMain, "title", "ignored")).result(t)
		if th != escThread {
			t.Fatalf("thread %s, want the fixed one", th)
		}
	})
	t.Run("body thread of the channel", func(t *testing.T) {
		if _, th := h.postPath(plain, native("x", "thread_id", escThread)).result(t); th != escThread {
			t.Fatalf("thread %s", th)
		}
	})
	t.Run("body thread of another channel", func(t *testing.T) {
		if _, th := h.postPath(plain, native("x", "thread_id", h.w.Threads["Support/Allgemein"])).result(t); th != h.eskalationMain {
			t.Fatalf("thread %s, want the main thread", th)
		}
	})
	t.Run("title in the forum", func(t *testing.T) {
		r := h.postPath(forum, native("x", "title", "Disk alert"))
		msg, th := r.result(t)
		if m := h.message(msg); m.threadID != th || m.channelID != h.forum {
			t.Fatalf("stored %+v", m)
		}
		if h.events("thread.created", th, "app") != 1 {
			t.Fatal("no thread.created of the app")
		}
	})
	t.Run("title in a text channel", func(t *testing.T) {
		msg, th := h.postPath(plain, native("x", "title", "Disk alert")).result(t)
		if m := h.message(msg); m.threadID != h.eskalationMain {
			t.Fatalf("message in %s, want the main thread", m.threadID)
		}
		var parent string
		h.row(`SELECT parent_message_id::text FROM threads WHERE id = $1`, []any{th}, &parent)
		if parent != msg {
			t.Fatalf("the new thread starts from %s, want %s", parent, msg)
		}
	})
	t.Run("forum without title", func(t *testing.T) {
		r := h.postPath(forum, native("x"))
		if _, fc, ptr, _, _ := r.problem(); r.status != http.StatusBadRequest || fc != "required_for_forum_post" || ptr != "/title" {
			t.Fatalf("%d %s", r.status, r.body)
		}
	})
}

// TestPostIncomingWebhook_RevokeRace (design note 6 a): posts and a revoke run concurrently; every
// post answers 201 or 404 (no deadlock or conflict reaches a client), every 201 is a stored message
// and no other message exists, and once the revoke committed no post writes.
func TestPostIncomingWebhook_RevokeRace(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	k := h.create(h.tracker, h.eskalation, "")
	var wg sync.WaitGroup
	statuses := make(chan int, 16)
	start := make(chan struct{})
	for range 8 {
		wg.Go(func() {
			<-start
			statuses <- h.postPath(k, native("race")).status
		})
	}
	close(start)
	h.must(h.w.Owner, http.MethodDelete, "/incoming-webhooks/"+k.id, nil, http.StatusNoContent)
	wg.Wait()
	close(statuses)
	ok := 0
	for s := range statuses {
		switch s {
		case http.StatusCreated:
			ok++
		case http.StatusNotFound:
		default:
			t.Fatalf("status %d", s)
		}
	}
	if n := h.count(`SELECT 1 FROM messages WHERE author_principal_id = $1`, h.tracker); n != ok {
		t.Fatalf("%d messages for %d answers 201", n, ok)
	}
	for range 3 {
		if r := h.postPath(k, native("after")); r.status != http.StatusNotFound {
			t.Fatalf("after the revoke committed: %d", r.status)
		}
	}
	if n := h.count(`SELECT 1 FROM messages WHERE author_principal_id = $1`, h.tracker); n != ok {
		t.Fatal("a post wrote after the revoke committed")
	}
}

// TestPostIncomingWebhook_TokenOnlyAsPattern (06 5 point 1, SEC-LOG-2, SEC-LOG-3): with a token
// built at runtime, the exported access log records and spans hold the route pattern
// /api/v1/hooks/{webhook_id}/{token} and never the token; a 404, a 400 and a 413 carry instance
// /api/v1/hooks/<id>/{token}; the Bearer variant logs no Authorization; audit details and event
// payloads hold no token. It installs the global telemetry providers and so does not run in
// parallel.
func TestPostIncomingWebhook_TokenOnlyAsPattern(t *testing.T) {
	sinks := harness.NewTelemetrySinks()
	var logs safeBuffer
	h := newHW(t, options{cfg: map[string]string{"webhook_in.max_body_bytes": "2048"}, opts: func(o *harness.Options) {
		o.LogWriter, o.Telemetry = &logs, sinks
	}})
	k := h.create(h.tracker, h.eskalation, "")
	for _, c := range []struct {
		k    hook
		body any
		want int
	}{
		{hook{id: k.id, token: k.token + "x"}, native("x"), http.StatusNotFound},
		{k, map[string]any{"content_md": 1}, http.StatusBadRequest},
		{k, native(strings.Repeat("x", 4096)), http.StatusRequestEntityTooLarge},
	} {
		r := h.postPath(c.k, c.body)
		if _, _, _, _, inst := r.problem(); r.status != c.want || inst != "/api/v1/hooks/"+k.id+"/{token}" {
			t.Fatalf("%d %s, want %d with the pattern in instance", r.status, r.body, c.want)
		}
	}
	h.postBearer(k, native("bearer"))
	h.postPath(k, native("path"))
	if err := h.s.FlushTelemetry(h.ctx); err != nil {
		t.Fatal(err)
	}
	secret := k.token[10:40]
	var dump strings.Builder
	dump.WriteString(logs.String())
	pattern := false
	for _, rec := range sinks.Logs.Records() {
		dump.WriteString(rec.Body().String())
		rec.WalkAttributes(func(kv attribute.KeyValue) bool {
			v := kv.Value.Emit()
			dump.WriteString(" " + string(kv.Key) + "=" + v)
			if kv.Key == "http.route" && v == "/api/v1/hooks/{webhook_id}/{token}" {
				pattern = true
			}
			return true
		})
	}
	spanPattern := false
	for _, sp := range sinks.Spans.GetSpans() {
		dump.WriteString(" " + sp.Name)
		if sp.Name == "POST /api/v1/hooks/{webhook_id}/{token}" {
			spanPattern = true
		}
		for _, a := range sp.Attributes {
			dump.WriteString(" " + string(a.Key) + "=" + a.Value.Emit())
		}
	}
	if strings.Contains(dump.String(), secret) || strings.Contains(dump.String(), "Bearer ") {
		t.Fatal("a log record or span holds the token or the Authorization value")
	}
	pattern = pattern || strings.Contains(logs.String(), `"http.route":"/api/v1/hooks/{webhook_id}/{token}"`)
	if !pattern || !spanPattern {
		t.Fatalf("access log pattern %v, span pattern %v", pattern, spanPattern)
	}
	if n := h.count(`SELECT 1 FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, secret); n != 0 {
		t.Fatal("an audit entry holds the token")
	}
	if n := h.count(`SELECT 1 FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`, secret); n != 0 {
		t.Fatal("an event holds the token")
	}
}
