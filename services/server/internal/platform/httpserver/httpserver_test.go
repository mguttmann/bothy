// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

var cfg = httpserver.Config{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 65536}

// serve runs srv on a fresh loopback listener and returns its base URL.
func serve(t *testing.T, srv *http.Server) string {
	t.Helper()
	ln, err := httpserver.Listen(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}

func TestServerErrorLogHasNoClientAddress(t *testing.T) {
	t.Parallel()
	logs := &syncBuffer{}
	log := slog.New(slog.NewJSONHandler(logs, nil))
	srv := httpserver.NewServer(cfg, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler broke")
	}), log)
	base := serve(t, srv)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/", nil)
	// The client sees the connection drop; its own local address is what net/http would log.
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "http server error") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "http server error") || !strings.Contains(out, `"error.type":"panic"`) {
		t.Fatalf("logs lack the server error line:\n%s", out)
	}
	if strings.Contains(out, "127.0.0.1") || strings.Contains(out, "handler broke") {
		t.Fatalf("logs carry a client address or the panic text:\n%s", out)
	}
}

func TestNewServerTimeouts(t *testing.T) {
	t.Parallel()
	srv := httpserver.NewServer(cfg, http.NotFoundHandler(), nil)
	if srv.ReadHeaderTimeout != 5*time.Second || srv.IdleTimeout != 2*time.Minute || srv.MaxHeaderBytes != 65536 {
		t.Fatalf("server = %+v", srv)
	}
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatal("ReadTimeout and WriteTimeout belong to the per-route deadlines of M1-18")
	}
	if srv.ErrorLog == nil {
		t.Fatal("no ErrorLog")
	}
}

func TestInternalMuxServesHealthOnly(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	reg.SetStarted()
	base := serve(t, httpserver.NewServer(cfg, httpserver.Internal(reg), nil))
	for path, want := range map[string]int{"/healthz": 200, "/readyz": 200, "/": 404, "/api/v1/threads": 404} {
		if code, _ := get(t, base+path); code != want {
			t.Errorf("%s = %d, want %d", path, code, want)
		}
	}
}

func TestPublicAnswersUnmatchedWithEmpty404(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/ping", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	base := serve(t, httpserver.NewServer(cfg, httpserver.Public(mux), nil))
	for _, path := range []string{"/healthz", "/readyz", "/", "/api/v1/other"} {
		code, body := get(t, base+path)
		if code != http.StatusNotFound || body != "" {
			t.Errorf("%s = %d %q, want 404 with an empty body", path, code, body)
		}
	}
	if code, body := get(t, base+"/api/v1/ping"); code != 200 || body != "pong" {
		t.Errorf("mounted route = %d %q", code, body)
	}
}

func TestListen(t *testing.T) {
	t.Parallel()
	ln, err := httpserver.Listen(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	if _, port, _ := net.SplitHostPort(ln.Addr().String()); port == "0" {
		t.Fatal("port 0 not resolved")
	}
	if _, err := httpserver.Listen(context.Background(), ln.Addr().String()); err == nil {
		t.Fatal("second listen on a bound port succeeded")
	}
}
