// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFrontConfigRendering proves the TLS front of Design note 3: the three routing modes name
// their upstreams, and the timeouts follow the profile's keys (everything else
// terminal.host_heartbeat_interval times terminal.host_heartbeat_miss_limit, the stream paths
// terminal.idle_timeout plus terminal.idle_grace plus that window).
func TestFrontConfigRendering(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	timeouts := FrontTimeouts{IdleTimeout: 10 * time.Minute, IdleGrace: time.Minute, HeartbeatInterval: 15 * time.Second, HeartbeatMissLimit: 4}
	if err := RenderFront(root, dir, ModeSingle, timeouts); err != nil {
		t.Fatal(err)
	}
	conf, err := os.ReadFile(filepath.Join(dir, "nginx.conf")) //nolint:gosec // G304: the rendered file in the temp dir
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"listen 8443 ssl;", "proxy_read_timeout 720s;", "proxy_send_timeout 720s;", "proxy_read_timeout 60s;",
		"X-Forwarded-Proto https", "access_log off;", "client_max_body_size 0;", "proxy_buffering off;", "proxy_request_buffering off;",
		"pid /tmp/nginx.pid;", "include /etc/termsuite-front/routing.conf;"} {
		if !strings.Contains(string(conf), want) {
			t.Errorf("nginx.conf lacks %q", want)
		}
	}
	if strings.Contains(string(conf), "{{") {
		t.Error("nginx.conf keeps a template placeholder")
	}
	want := map[Mode][3]string{
		ModeSingle: {"api:8080", "api:8080", "api:8080"},
		ModeHash:   {"edge:8080", "edge:8080", "edge:8080"},
		ModeSplit:  {"api:8080", "api-2:8080", "edge:8080"},
	}
	for mode, servers := range want {
		if err := SetFrontRouting(root, dir, mode); err != nil {
			t.Fatal(err)
		}
		got := upstreams(t, filepath.Join(dir, "routing.conf"))
		if got != servers {
			t.Errorf("mode %s routes device, host data and rest to %v, want %v", mode, got, servers)
		}
	}
	if err := SetFrontRouting(root, dir, "other"); err == nil {
		t.Error("an unknown routing mode is accepted")
	}
}

// upstreams returns the servers of the upstreams termsuite_device, termsuite_hostdata and
// termsuite_rest of a routing file.
func upstreams(t *testing.T, path string) [3]string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // G304: the rendered file in the temp dir
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{"termsuite_device": 0, "termsuite_hostdata": 1, "termsuite_rest": 2}
	var out [3]string
	cur := ""
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "upstream "):
			cur = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(l, "upstream ")), " {")
		case strings.HasPrefix(l, "server "):
			if i, ok := names[cur]; ok {
				out[i] = strings.TrimSuffix(strings.TrimPrefix(l, "server "), ";")
			}
		}
	}
	return out
}

// TestTestCA proves the test CA of Design note 3: an ECDSA P-256 CA valid one day, a server
// certificate for chat.example.com that verifies against the CA only, and a key file of mode
// 0600.
func TestTestCA(t *testing.T) {
	dir := t.TempDir()
	ca, err := NewCA(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "front.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("front.key has mode %o, want 0600", info.Mode().Perm())
	}
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, "front.crt"), filepath.Join(dir, "front.key"))
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "chat.example.com", Roots: ca.Pool()}); err != nil {
		t.Errorf("the front certificate does not verify against the test CA: %v", err)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "chat.example.com"}); err == nil {
		t.Error("the front certificate verifies against the system roots")
	}
	block, _ := pem.Decode(ca.PEM())
	if block == nil {
		t.Fatal("the CA PEM is not a PEM block")
	}
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsCA || root.NotAfter.Sub(root.NotBefore) > 25*time.Hour {
		t.Errorf("the CA is not a CA valid about one day: IsCA=%v, %s", root.IsCA, root.NotAfter.Sub(root.NotBefore))
	}
}
