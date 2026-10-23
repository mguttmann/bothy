// SPDX-License-Identifier: Apache-2.0

package fakeapp_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/apps/fakeapp"
)

// client returns an egress client that reaches the fakes through the resolver, with an
// allow_private rule per fake as the integration tests configure it; it trusts the test authority
// of the first fake only, which is enough because redirects are never followed.
func client(t *testing.T, timeout time.Duration, apps ...*fakeapp.App) *egress.Client {
	t.Helper()
	cfg := egress.Config{Timeout: timeout, MaxResponseBytes: 1 << 20}
	pool := apps[0].RootCAs
	for _, a := range apps {
		rule, err := egress.ParseRule(a.HostPort(fakeapp.DefaultHost) + ";allow_private")
		if err != nil {
			t.Fatal(err)
		}
		cfg.AllowedHosts = append(cfg.AllowedHosts, rule)
	}
	c, err := egress.New(cfg, egress.Options{Purpose: "fakeapp_test", Redirects: egress.RedirectNever},
		egress.Deps{Resolver: fakeapp.NewResolver(fakeapp.DefaultHost), RootCAs: pool})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// post sends body to url with the webhook headers and returns status and body.
func post(ctx context.Context, c *egress.Client, url string, h map[string]string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	for k, v := range h {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}

// signed returns the webhook headers of body signed with key and extra signature entries.
func signed(key ed25519.PrivateKey, id string, ts int64, body []byte, extra string) map[string]string {
	content := append([]byte(id+"."+strconv.FormatInt(ts, 10)+"."), body...)
	sig := "v1a," + base64.StdEncoding.EncodeToString(ed25519.Sign(key, content))
	if extra != "" {
		sig += " " + extra
	}
	return map[string]string{"webhook-id": id, "webhook-timestamp": strconv.FormatInt(ts, 10), "webhook-signature": sig}
}

// TestFakeAppVerifiesSignature: a v1a entry that verifies with one of the keys marks the request
// verified; another key, a changed body or a missing header do not; a v1 entry is reported.
func TestFakeAppVerifiesSignature(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	app := fakeapp.Start(t, []ed25519.PublicKey{otherPub, pub})
	c := client(t, testkit.Patience, app)
	ctx := context.Background()
	body := []byte(`{"type":"command.invoked"}`)
	const id = "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b"
	h := signed(priv, id, 1760346000, body, "")
	if st, _, err := post(ctx, c, app.URL("/hooks/interactions"), h, body); err != nil || st != http.StatusOK {
		t.Fatalf("post = %d %v", st, err)
	}
	if _, _, err := post(ctx, c, app.URL("/hooks/interactions"), h, []byte(`{"type":"changed"}`)); err != nil {
		t.Fatal(err)
	}
	_, strangerPriv, _ := ed25519.GenerateKey(rand.Reader)
	if _, _, err := post(ctx, c, app.URL("/x"), signed(strangerPriv, id, 1760346000, body, "v1,AQID"), body); err != nil {
		t.Fatal(err)
	}
	noID := signed(otherPriv, id, 1760346000, body, "")
	delete(noID, "webhook-id")
	if _, _, err := post(ctx, c, app.URL("/x"), noID, body); err != nil {
		t.Fatal(err)
	}
	got := app.Requests()
	if len(got) != 4 {
		t.Fatalf("recorded %d requests, want 4", len(got))
	}
	want := []struct{ verified, v1 bool }{{true, false}, {false, false}, {false, true}, {false, false}}
	for i, w := range want {
		if got[i].Verified != w.verified || got[i].HasV1 != w.v1 {
			t.Errorf("request %d: verified %v v1 %v, want %v %v", i, got[i].Verified, got[i].HasV1, w.verified, w.v1)
		}
	}
	if got[0].Path != "/hooks/interactions" || !bytes.Equal(got[0].Body, body) || got[0].Header.Get("webhook-id") != id ||
		got[0].Method != http.MethodPost {
		t.Fatal("the first request was not recorded as sent")
	}
}

// TestFakeAppScript: the queue answers in order, then the handler; a redirect is not followed by
// an egress client without redirects, so its target records nothing; a hanging answer ends with
// the client's deadline; WaitRequests wakes on each request.
func TestFakeAppScript(t *testing.T) {
	t.Parallel()
	app := fakeapp.Start(t, nil)
	second := fakeapp.Start(t, nil)
	c := client(t, testkit.Patience, app, second)
	ctx := context.Background()
	app.Respond(
		fakeapp.Response{Status: http.StatusAccepted},
		fakeapp.Response{Status: http.StatusInternalServerError},
		fakeapp.Redirect(second.URL("/elsewhere")),
		fakeapp.Oversized(64),
		fakeapp.InvalidJSON(),
		fakeapp.Response{Status: http.StatusOK, Body: []byte(`{"choices":[]}`), Delay: time.Millisecond},
	)
	wantStatus := []int{http.StatusAccepted, http.StatusInternalServerError, http.StatusFound, http.StatusOK, http.StatusOK, http.StatusOK}
	for i, w := range wantStatus {
		st, b, err := post(ctx, c, app.URL("/hooks"), nil, []byte(`{}`))
		if err != nil || st != w {
			t.Fatalf("answer %d = %d %v, want %d", i, st, err, w)
		}
		switch i {
		case 3:
			if len(b) <= 64 {
				t.Fatalf("oversized body has %d bytes", len(b))
			}
		case 4:
			if len(b) == 0 || b[len(b)-1] == '}' {
				t.Fatal("invalid JSON answer is valid")
			}
		}
	}
	if n := len(second.Requests()); n != 0 {
		t.Fatalf("redirect target received %d requests", n)
	}
	app.Handle(func(r fakeapp.Request) fakeapp.Response {
		return fakeapp.Response{Status: http.StatusTeapot}
	})
	if st, _, err := post(ctx, c, app.URL("/hooks"), nil, []byte(`{}`)); err != nil || st != http.StatusTeapot {
		t.Fatalf("handler answer = %d %v", st, err)
	}
	app.Handle(nil)
	if st, _, err := post(ctx, c, app.URL("/hooks"), nil, []byte(`{}`)); err != nil || st != http.StatusOK {
		t.Fatalf("default answer = %d %v", st, err)
	}

	app.Respond(fakeapp.Response{Hang: true})
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, _, err := post(short, c, app.URL("/hooks"), nil, []byte(`{}`)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hanging answer ended with %v, want the deadline", err)
	}
	got, err := app.WaitRequests(testkit.Context(t), 9)
	if err != nil || len(got) != 9 {
		t.Fatalf("WaitRequests = %d %v", len(got), err)
	}
}

// TestResolver: the resolver maps its names to 127.0.0.1, follows Set and refuses other names.
func TestResolver(t *testing.T) {
	t.Parallel()
	r := fakeapp.NewResolver("Tracker.example.com")
	ctx := context.Background()
	got, err := r.LookupNetIP(ctx, "ip", "tracker.example.com")
	if err != nil || len(got) != 1 || got[0] != netip.MustParseAddr("127.0.0.1") {
		t.Fatalf("LookupNetIP = %v %v", got, err)
	}
	r.Set("tracker.example.com", netip.AddrFrom4([4]byte{10, 0, 0, 7}))
	if got, _ := r.LookupNetIP(ctx, "ip", "tracker.example.com"); got[0] != netip.AddrFrom4([4]byte{10, 0, 0, 7}) {
		t.Fatalf("after Set = %v", got)
	}
	var dnsErr *net.DNSError
	if _, err := r.LookupNetIP(ctx, "ip", "other.example.com"); !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
		t.Fatalf("unknown name = %v", err)
	}
}
