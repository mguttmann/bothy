// SPDX-License-Identifier: Apache-2.0

package device

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// fakeSidecar answers the requests of a Sidecar from a table: op -> answer line. It records every
// request it got.
type fakeSidecar struct {
	answers map[string]string
	got     []map[string]any
}

// serve reads JSON lines from in and writes the answer of each op to out until in closes.
func (f *fakeSidecar) serve(in io.Reader, out io.WriteCloser) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		var req map[string]any
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_, _ = io.WriteString(out, `{"ok":false,"error":{"kind":"bad_json"}}`+"\n")
			continue
		}
		f.got = append(f.got, req)
		op, _ := req["op"].(string)
		a, ok := f.answers[op]
		if !ok {
			a = `{"ok":false,"error":{"kind":"unknown_op"}}`
		}
		_, _ = io.WriteString(out, a+"\n")
	}
	_ = out.Close()
}

// TestSidecarProtocolRoundTrip proves the driver of the sidecar protocol of Design note 6 against
// a fake sidecar: every op sends its fields in base64url without padding and decodes the bytes of
// its answer, and every error kind arrives as a SidecarError with that kind.
func TestSidecarProtocolRoundTrip(t *testing.T) {
	enc := func(s string) string { return b64([]byte(s)) }
	f := &fakeSidecar{answers: map[string]string{
		"keypair":  `{"ok":true,"ephemeral_public":"` + enc("eph-keypair") + `","cek":"cek-1"}`,
		"start":    `{"ok":true,"ephemeral_public":"` + enc("eph-start") + `","cek":"cek-2"}`,
		"message1": `{"ok":true,"message":"` + enc("m1") + `"}`,
		"message2": `{"ok":true,"payload":"` + enc("p2") + `"}`,
		"seal":     `{"ok":true,"message":"` + enc("sealed") + `"}`,
		"open":     `{"ok":true,"frame":"` + enc("frame") + `"}`,
	}}
	sc := newFakeSidecar(t, f)
	if e, cek, err := sc.Keypair(); err != nil || string(e) != "eph-keypair" || cek != "cek-1" {
		t.Errorf("keypair = %q %q %v", e, cek, err)
	}
	if e, cek, err := sc.Start([]byte("static"), "s-id", "h-id"); err != nil || string(e) != "eph-start" || cek != "cek-2" {
		t.Errorf("start = %q %q %v", e, cek, err)
	}
	for _, c := range []struct {
		name string
		call func([]byte) ([]byte, error)
		want string
	}{
		{"message1", sc.Message1, "m1"}, {"message2", sc.Message2, "p2"}, {"seal", sc.Seal, "sealed"}, {"open", sc.Open, "frame"},
	} {
		got, err := c.call([]byte("in-" + c.name))
		if err != nil || string(got) != c.want {
			t.Errorf("%s = %q %v, want %q", c.name, got, err, c.want)
		}
	}
	wantFields := map[string]map[string]string{
		"start":    {"host_static": enc("static"), "session_id": "s-id", "host_id": "h-id"},
		"message1": {"payload": enc("in-message1")},
		"message2": {"message": enc("in-message2")},
		"seal":     {"frame": enc("in-seal")},
		"open":     {"message": enc("in-open")},
	}
	for _, req := range f.got {
		op, _ := req["op"].(string)
		for k, v := range wantFields[op] {
			if req[k] != v {
				t.Errorf("%s sent %s=%v, want %s", op, k, req[k], v)
			}
		}
	}
	for _, kind := range []string{"ErrHandshake", "ErrDecrypt", "ErrState", "ErrBroken", "ErrUnknownType", "ErrWrongDirection", "ErrMalformed",
		"ErrTooLong", "ErrInvalidKey", "ErrInvalidID", "bad_json", "unknown_op", "out_of_order", "bad_field"} {
		f.answers["open"] = `{"ok":false,"error":{"kind":"` + kind + `"}}`
		_, err := sc.Open([]byte("x"))
		var se *SidecarError
		if !errors.As(err, &se) || se.Kind != kind {
			t.Errorf("error kind %s arrives as %v", kind, err)
		}
	}
	if err := sc.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
	if _, err := sc.Open([]byte("x")); !errors.Is(err, ErrSidecarGone) {
		t.Errorf("a call after Close = %v, want ErrSidecarGone", err)
	}
}

// newFakeSidecar connects a Sidecar to f over in-process pipes.
func newFakeSidecar(t *testing.T, f *fakeSidecar) *Sidecar {
	t.Helper()
	reqR, reqW := io.Pipe()
	ansR, ansW := io.Pipe()
	done := make(chan struct{})
	go func() { f.serve(reqR, ansW); close(done) }()
	t.Cleanup(func() { _ = reqW.Close(); <-done })
	return NewSidecarIO(reqW, ansR)
}

// TestDialerMapsInstanceOrigin proves the dialer of Design note 3: chat.example.com:8443 reaches
// the mapped port on 127.0.0.1 and verifies against the test CA only; another address is refused,
// and a server whose certificate the test CA did not sign fails the TLS check.
func TestDialerMapsInstanceOrigin(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host) //nolint:gosec // G705: a test server echoes the Host of the test client
	}))
	srv.Config.ErrorLog = nil
	srv.StartTLS()
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	// httptest signs for example.com and its subdomains; its CA is the root of this test.
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	d := &Dialer{Host: "example.com", Port: 8443, Mapped: mapped, Roots: pool}
	resp, err := d.Client().Get("https://example.com:8443/") //nolint:noctx // a unit test of the dialer
	if err != nil {
		t.Fatalf("the instance origin does not reach the mapped port: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "example.com:8443" {
		t.Errorf("the request carried Host %q, want example.com:8443", body)
	}
	if resp, err := d.Client().Get("https://example.com:9443/"); err == nil || !strings.Contains(err.Error(), "only the instance origin") { //nolint:noctx // a unit test of the dialer
		if resp != nil {
			_ = resp.Body.Close()
		}
		t.Errorf("another port is dialed: %v", err)
	}
	other := &Dialer{Host: "example.com", Port: 8443, Mapped: mapped, Roots: x509.NewCertPool()}
	resp, err = other.Client().Get("https://example.com:8443/") //nolint:noctx // a unit test of the dialer
	if resp != nil {
		_ = resp.Body.Close()
	}
	var unknown x509.UnknownAuthorityError
	if !errors.As(err, &unknown) && !strings.Contains(err.Error(), "certificate") {
		t.Errorf("a server outside the test CA is trusted: %v", err)
	}
	if cfg := d.TLSConfig(); cfg.ServerName != "example.com" || cfg.MinVersion < tls.VersionTLS12 || cfg.InsecureSkipVerify {
		t.Errorf("TLS configuration %+v", cfg)
	}
}
