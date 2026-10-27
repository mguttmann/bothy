// SPDX-License-Identifier: Apache-2.0

package fakepush_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/fakepush"
)

// TestRunServeAndHealth (brief M5-11 Design note 1): `fakepush serve` writes the run material,
// prints one ready line with the three addresses and no key or token, answers the capture API,
// and ends with 0 when its context ends; `fakepush health` exits 0 while the capture API answers,
// as the Compose healthcheck needs in an image without a shell (its failures:
// TestHealthFailsWithoutAHealthyCaptureAPI).
func TestRunServeAndHealth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	pr, pw := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- fakepush.Run(ctx, []string{"serve", "--dir", dir, "--host", "localhost", "--apns-addr", "127.0.0.1:0",
			"--fcm-addr", "127.0.0.1:0", "--capture-addr", "127.0.0.1:0"}, pw)
		_ = pw.Close()
	}()
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	ready := testkit.Receive(t, "the ready line", lines)
	if !strings.HasPrefix(ready, "fakepush: ready ") || strings.Contains(ready, "PRIVATE") {
		t.Fatalf("ready line %q", ready)
	}
	capture := ""
	for _, f := range strings.Fields(ready) {
		if v, ok := strings.CutPrefix(f, "capture=http://"); ok {
			capture = v
		}
	}
	if _, _, err := net.SplitHostPort(capture); err != nil {
		t.Fatalf("no capture address in %q", ready)
	}
	if _, err := os.Stat(filepath.Join(dir, fakepush.FileAPNsKey)); err != nil {
		t.Fatalf("serve wrote no APNs key: %v", err)
	}
	if status, _, body := get(t, "http://"+capture+"/captured"); status != http.StatusOK || strings.TrimSpace(string(body)) != `{"items":[]}` {
		t.Fatalf("captured: %d %s", status, body)
	}
	var stderr strings.Builder
	if code := fakepush.Run(t.Context(), []string{"health", "--addr", capture}, &stderr); code != 0 {
		t.Fatalf("health while serving: %d %s", code, stderr.String())
	}
	cancel()
	if code := testkit.Receive(t, "the end of serve", done); code != 0 {
		t.Fatalf("serve ended with %d", code)
	}
}

// TestHealthFailsWithoutAHealthyCaptureAPI: health exits 1 and says why when /healthz answers
// another status or nothing can be reached; the probe never follows a redirect.
func TestHealthFailsWithoutAHealthyCaptureAPI(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]string{http.StatusServiceUnavailable: "status 503", http.StatusFound: "status 302"} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/healthz" {
				t.Errorf("health probed %s", r.URL.Path)
			}
			w.Header().Set("Location", "/healthz-elsewhere")
			w.WriteHeader(status)
		}))
		var stderr strings.Builder
		code := fakepush.Run(t.Context(), []string{"health", "--addr", strings.TrimPrefix(srv.URL, "http://")}, &stderr)
		srv.Close()
		if code != 1 || !strings.Contains(stderr.String(), want) {
			t.Errorf("health against %d: exit %d, stderr %q", status, code, stderr.String())
		}
	}
	// Port 0 is never a listening port, so the dial fails at once.
	var stderr strings.Builder
	if code := fakepush.Run(t.Context(), []string{"health", "--addr", "127.0.0.1:0"}, &stderr); code != 1 || stderr.Len() == 0 {
		t.Errorf("health without a listener: exit %d, stderr %q", code, stderr.String())
	}
}

// TestRunUsage: no subcommand, an unknown one, an unknown flag or an argument after the flags is
// a usage error with exit code 2; serve without its directory fails with 1.
func TestRunUsage(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"other"}, {"serve", "--nope"}, {"health", "--nope"}, {"health", "extra"}} {
		var stderr strings.Builder
		if code := fakepush.Run(t.Context(), args, &stderr); code != 2 || !strings.Contains(stderr.String(), "Usage") {
			t.Errorf("%v: exit %d, stderr %q", args, code, stderr.String())
		}
	}
	var stderr strings.Builder
	code := fakepush.Run(t.Context(), []string{"serve", "--dir", filepath.Join(t.TempDir(), "missing"), "--host", "localhost",
		"--apns-addr", "127.0.0.1:0", "--fcm-addr", "127.0.0.1:0", "--capture-addr", "127.0.0.1:0"}, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "fakepush:") {
		t.Errorf("serve without directory: exit %d, stderr %q", code, stderr.String())
	}
}
