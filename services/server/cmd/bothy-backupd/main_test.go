// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// lockedBuffer is a bytes.Buffer that serve may write while the test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakePgbackrest writes an executable shell script that stands in for pgbackrest: "server"
// runs until SIGTERM and records it in dir/term, "stanza-create" fails until it has been
// called failures+1 times and counts its calls in dir/stanza. serverExit, when not "", makes
// "server" exit with that code at once instead.
func fakePgbackrest(t *testing.T, failures int, serverExit string) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	server := `trap 'echo term > "$D/term"; exit 0' TERM
  while :; do sleep 0.05; done`
	if serverExit != "" {
		server = "exit " + serverExit
	}
	script := `#!/bin/sh
D='` + dir + `'
for a in "$@"; do last=$a; done
case "$last" in
server)
  ` + server + `
  ;;
stanza-create)
  n=$(cat "$D/stanza" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" > "$D/stanza"
  [ "$n" -gt ` + strconv.Itoa(failures) + ` ] || { echo "ERROR: [056]: unable to find primary cluster" >&2; exit 56; }
  echo "stanza-create command end: completed successfully"
  ;;
*) exit 99 ;;
esac
`
	bin = filepath.Join(dir, "pgbackrest")
	writeExecutable(t, bin, script)
	return bin, dir
}

// writeExecutable writes an executable file while no other goroutine of the test binary can
// fork. A child forked by a parallel test while the file is still open for writing inherits
// that descriptor, and exec of the file then fails on Linux with ETXTBSY ("text file busy",
// golang.org/issue/22315). os/exec holds syscall.ForkLock for reading while it forks.
func writeExecutable(t *testing.T, path, script string) {
	t.Helper()
	syscall.ForkLock.Lock()
	err := os.WriteFile(path, []byte(script), 0o700) //nolint:gosec // G306: the fake must be executable.
	syscall.ForkLock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}

// waitFile waits until path has content and returns it, failing the test after
// testkit.Patience.
func waitFile(t *testing.T, path string) string {
	t.Helper()
	var content string
	testkit.Until(t, path+" with content", func() bool {
		b, err := os.ReadFile(path) //nolint:gosec // G304: path from t.TempDir.
		content = strings.TrimSpace(string(b))
		return err == nil && content != ""
	})
	return content
}

func TestServeStopsChildOnSIGTERM(t *testing.T) {
	t.Parallel()
	bin, dir := fakePgbackrest(t, 2, "")
	signals := make(chan os.Signal, 1)
	stderr := &lockedBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- serve(context.Background(), serveOptions{pgbackrest: bin, stanza: "bothy", retry: 10 * time.Millisecond}, signals, stderr)
	}()
	// The stanza is created after two failed attempts while the server keeps running. The fake
	// counts an attempt before it succeeds, so the test waits for the line serve writes after the
	// third attempt returned; a SIGTERM before it would cancel that attempt.
	testkit.Until(t, "the line of the ready stanza", func() bool {
		return strings.Contains(stderr.String(), "stanza "+defaultStanza+" is ready")
	})
	if got := waitFile(t, filepath.Join(dir, "stanza")); got != "3" {
		t.Errorf("stanza-create ran %s times, want 3 (two failures, one success)", got)
	}
	signals <- syscall.SIGTERM
	if code := testkit.Receive(t, "the return of serve after SIGTERM", done); code != exitOK {
		t.Errorf("serve returned %d after SIGTERM, want 0; stderr:\n%s", code, stderr.String())
	}
	if got := waitFile(t, filepath.Join(dir, "term")); got != "term" {
		t.Errorf("child did not receive SIGTERM: %q", got)
	}
}

func TestServeFailsWhenTheServerExits(t *testing.T) {
	t.Parallel()
	bin, _ := fakePgbackrest(t, 0, "3")
	var stderr bytes.Buffer
	code := serve(context.Background(), serveOptions{pgbackrest: bin, stanza: "bothy", retry: 10 * time.Millisecond}, make(chan os.Signal), &stderr)
	if code != exitFailure {
		t.Errorf("serve returned %d when the TLS server exited, want 1", code)
	}
	if !strings.Contains(stderr.String(), "TLS server exited") {
		t.Errorf("stderr does not name the exit:\n%s", stderr.String())
	}
}

func TestServeFailsWithoutBinary(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	code := serve(context.Background(), serveOptions{pgbackrest: filepath.Join(t.TempDir(), "missing"), stanza: "bothy", retry: time.Millisecond}, make(chan os.Signal), &stderr)
	if code != exitFailure {
		t.Errorf("serve returned %d without a pgbackrest binary, want 1", code)
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"version"}, nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("version returned %d: %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != buildinfo.Version() {
		t.Errorf("version printed %q, want %q", got, buildinfo.Version())
	}
}

func TestUsage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, exitOK},
		{[]string{"unknown"}, exitUsage},
		{[]string{"--retry", "0s"}, exitUsage},
		{[]string{"--stanza", ""}, exitUsage},
		{[]string{"healthcheck", "--timeout", "0s"}, exitUsage},
		{[]string{"healthcheck", "extra"}, exitUsage},
		{[]string{"healthcheck", "--addr", "192.0.2.1:8432"}, exitUsage},
		{[]string{"healthcheck", "--addr", "backup:8432"}, exitUsage},
		{[]string{"healthcheck", "--addr", "127.0.0.1"}, exitUsage},
		{[]string{"version", "extra"}, exitUsage},
		{[]string{"--retry", "1s", "--help"}, exitOK},
		{[]string{"--bogus"}, exitUsage},
		{[]string{"--retry", "1s", "extra"}, exitUsage},
		{[]string{"--pgbackrest", ""}, exitUsage},
	} {
		var stdout, stderr bytes.Buffer
		if got := run(context.Background(), tc.args, nil, &stdout, &stderr); got != tc.code {
			t.Errorf("run(%q) = %d, want %d; stderr %s", tc.args, got, tc.code, stderr.String())
		}
	}
}

// tlsMaterial generates pgbackrest_tls/ with bothy secrets init and returns its directory.
func tlsMaterial(t *testing.T) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "target")
	me := stack.Owner{UID: os.Geteuid(), GID: os.Getegid()}
	if _, err := stack.Init(context.Background(), stack.InitOptions{Target: target, Owners: map[string]stack.Owner{"server": me, "migrate": me}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				_ = os.Chmod(path, 0o700) //nolint:gosec // G302: the directory must be removable.
			}
			return nil
		})
	})
	return filepath.Join(target, "_store", "pgbackrest_tls")
}

// tlsServer serves TLS with the certificate of host from dir on a loopback port, requiring a
// client certificate of the CA in caDir, and returns its address.
func tlsServer(t *testing.T, dir, caDir, host string) string {
	t.Helper()
	pair, err := tls.LoadX509KeyPair(filepath.Join(dir, host+".crt"), filepath.Join(dir, host+".key"))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pem, err := os.ReadFile(filepath.Join(caDir, "ca.crt")) //nolint:gosec // G304: path from t.TempDir.
	if err != nil || !pool.AppendCertsFromPEM(pem) {
		t.Fatalf("CA: %v", err)
	}
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{pair}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = c.(*tls.Conn).HandshakeContext(context.Background())
				_ = c.Close()
			}()
		}
	}()
	return l.Addr().String()
}

func healthArgs(dir, addr string, extra ...string) []string {
	return append([]string{"healthcheck", "--addr", addr,
		"--ca", filepath.Join(dir, "ca.crt"), "--cert", filepath.Join(dir, "backup.crt"), "--key", filepath.Join(dir, "backup.key"),
		"--timeout", testkit.Patience.String()}, extra...)
}

func TestHealthcheck(t *testing.T) {
	t.Parallel()
	dir := tlsMaterial(t)
	other := tlsMaterial(t)
	addr := tlsServer(t, dir, dir, "backup")
	foreign := tlsServer(t, other, other, "backup")
	wrongName := tlsServer(t, dir, dir, "postgres")
	// A closed port: listen, note the address, close.
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := l.Addr().String()
	_ = l.Close()

	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"answers", healthArgs(dir, addr), exitOK, ""},
		{"no TLS server", healthArgs(dir, closed), exitFailure, "no answer"},
		{"server of another CA", healthArgs(dir, foreign), exitFailure, "certificate"},
		{"server with another name", healthArgs(dir, wrongName), exitFailure, "certificate"},
		{"missing key", healthArgs(dir, addr, "--key", filepath.Join(dir, "missing.key")), exitFailure, "client certificate"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), tc.args, nil, &stdout, &stderr)
		if code != tc.code || !strings.Contains(stderr.String(), tc.stderr) {
			t.Errorf("%s: code %d stderr %q, want %d and %q", tc.name, code, stderr.String(), tc.code, tc.stderr)
		}
	}
}

// TestHealthcheckSilentServer: a port that accepts but never speaks TLS fails within --timeout.
func TestHealthcheckSilentServer(t *testing.T) {
	t.Parallel()
	dir := tlsMaterial(t)
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// The connections stay open until the test ends, so a check that ignored --timeout would
	// wait past testkit.Patience.
	silent := make(chan struct{})
	t.Cleanup(func() { close(silent); _ = l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				<-silent
				_ = c.Close()
			}()
		}
	}()
	var stdout, stderr bytes.Buffer
	args := healthArgs(dir, l.Addr().String())
	args[len(args)-1] = "200ms"
	// In a goroutine, so a check that ignored --timeout fails the test after testkit.Patience;
	// the cleanup then releases the silent connections.
	done := make(chan int, 1)
	go func() { done <- run(context.Background(), args, nil, &stdout, &stderr) }()
	if code := testkit.Receive(t, "the end of the healthcheck within --timeout", done); code != exitFailure {
		t.Errorf("silent server: code %d, want 1", code)
	}
}

// TestRunServesUntilTheServerFails: the default command of run starts serve with its own
// signal channel; without a pgBackRest binary it fails.
func TestRunServesUntilTheServerFails(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	args := []string{"--pgbackrest", filepath.Join(t.TempDir(), "missing"), "--retry", "1ms"}
	if code := run(context.Background(), args, nil, &stdout, &stderr); code != exitFailure {
		t.Errorf("run without a pgbackrest binary = %d, want %d; stderr %s", code, exitFailure, stderr.String())
	}
}

// TestServeAcceptsAServerEndedBySIGTERM: a TLS server that the SIGTERM itself ends (no trap)
// is the expected outcome of a stop and is not reported as an error.
func TestServeAcceptsAServerEndedBySIGTERM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do last=$a; done\n" +
		"case \"$last\" in\nserver) exec sleep 60 ;;\nstanza-create) echo ok > '" + dir + "/stanza' ;;\nesac\n"
	bin := filepath.Join(dir, "pgbackrest")
	writeExecutable(t, bin, script)
	signals := make(chan os.Signal, 1)
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- serve(context.Background(), serveOptions{pgbackrest: bin, stanza: "bothy", retry: 10 * time.Millisecond}, signals, &stderr)
	}()
	waitFile(t, filepath.Join(dir, "stanza"))
	signals <- syscall.SIGTERM
	if code := testkit.Receive(t, "the return of serve after SIGTERM", done); code != exitOK {
		t.Errorf("serve returned %d after SIGTERM, want 0; stderr:\n%s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "stopped with") {
		t.Errorf("a server ended by SIGTERM was reported as an error:\n%s", stderr.String())
	}
}
