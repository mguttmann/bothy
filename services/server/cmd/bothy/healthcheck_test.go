// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func healthcheck(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), append([]string{"healthcheck"}, args...), nil, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestHealthcheckCommand(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readyz":
			w.WriteHeader(http.StatusOK)
		case "/down":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/redirect":
			http.Redirect(w, r, "/readyz", http.StatusFound)
		case "/slow":
			<-release
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	base := srv.URL // http://127.0.0.1:<port>
	localhost := strings.Replace(base, "127.0.0.1", "localhost", 1)

	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"ready", []string{"--url", base + "/readyz"}, 0, ""},
		{"localhost", []string{"--url=" + localhost + "/readyz"}, 0, ""},
		{"unavailable", []string{"--url", base + "/down"}, 1, "status 503"},
		{"redirect is not followed", []string{"--url", base + "/redirect"}, 1, "status 302"},
		{"timeout", []string{"--url", base + "/slow", "--timeout", "100ms"}, 1, "timeout"},
		{"refused", []string{"--url", "http://127.0.0.1:1/readyz"}, 1, "connection failed"},
		{"non-loopback host", []string{"--url", "http://192.0.2.1:8081/readyz"}, 2, "loopback"},
		{"https", []string{"--url", "https://example.com/readyz"}, 2, "http://"},
		{"name that is not localhost", []string{"--url", "http://example.com/readyz"}, 2, "loopback"},
		{"user info", []string{"--url", "http://u:p@127.0.0.1:8081/readyz"}, 2, "user info"},
		{"no url", nil, 2, "--url"},
		{"bad url", []string{"--url", "http://[::1"}, 2, "--url"},
		{"bad timeout", []string{"--url", base, "--timeout", "0s"}, 2, "--timeout"},
		{"unknown flag", []string{"--verbose"}, 2, "verbose"},
		{"argument", []string{"--url", base, "extra"}, 2, "extra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := healthcheck(t, tc.args...)
			if code != tc.code || !strings.Contains(stderr, tc.stderr) || stdout != "" {
				t.Fatalf("exit %d, stdout %q, stderr %q; want %d with %q", code, stdout, stderr, tc.code, tc.stderr)
			}
			if code == 1 && strings.Count(stderr, "\n") != 1 {
				t.Fatalf("stderr has more than one line: %q", stderr)
			}
		})
	}
}

func TestHealthcheckHelp(t *testing.T) {
	t.Parallel()
	code, stdout, _ := healthcheck(t, "--help")
	if code != 0 || !strings.Contains(stdout, "Usage: bothy healthcheck --url URL") {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
}

// TestHealthcheckDefaultTimeoutMatchesCompose holds the default of --timeout equal to the
// timeout of the Compose healthcheck, so a probe never outlives the attempt Compose counts.
func TestHealthcheckDefaultTimeoutMatchesCompose(t *testing.T) {
	t.Parallel()
	if defaultHealthcheckTimeout != 5*time.Second {
		t.Fatalf("default = %v", defaultHealthcheckTimeout)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "compose", "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	i := strings.Index(text, "x-server-healthcheck:")
	if i < 0 || !strings.Contains(text[i:i+300], `"healthcheck", "--url"`) || !strings.Contains(text[i:i+300], "timeout: 5s") {
		t.Fatalf("x-server-healthcheck no longer calls bothy healthcheck with timeout 5s")
	}
}
