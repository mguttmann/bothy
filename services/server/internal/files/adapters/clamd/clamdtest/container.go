// SPDX-License-Identifier: Apache-2.0

package clamdtest

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // G501: the hash signature format of ClamAV (.hdb) is MD5; no security use
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Image is the clamd image of the test container: the repository and version of the Compose
// service clamav (BOTHY_CLAMAV_VERSION in infra/compose/.env.example and its default in
// infra/compose/compose.yaml), pinned by digest like pgtest.Image. Renovate updates tag and
// digest through the custom manager in renovate.json, which reads the comment on the next line;
// TestClamdImagePinned holds the version equal to the Compose stack.
//
// renovate: datasource=docker depName=clamav/clamav-debian
const Image = "clamav/clamav-debian:1.5.4@sha256:d7ede3cefeaf3b8a72ebcdee849d13f504bbca3f3b3cfade5bc7b6ee06a4f62c"

// The label of every clamd test container; `docker ps --filter label=org.bothy.test=clamdtest`
// finds one that a killed test binary left behind.
const (
	Label      = "org.bothy.test"
	LabelValue = "clamdtest"
)

// SignatureName is the name of the one test signature; clamd reports it with the suffix
// .UNOFFICIAL, so tests compare a prefix.
const SignatureName = "Local.Test.EICAR"

// The halves of the EICAR test string; EICAR joins them at run time, so the repository never holds
// the whole string.
const (
	eicarHead = `X5O!P%@AP[4\PZX54(P^)7CC)7}$`
	eicarTail = `EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`
)

// EICAR returns the EICAR test string, assembled at run time.
func EICAR() []byte { return []byte(eicarHead + eicarTail) }

// tcpPort is the loopback test seam inside the container (published on 127.0.0.1 only).
const tcpPort = "3310/tcp"

// Paths inside the container.
const (
	confPath = "/etc/bothy-test/clamd.conf"
	dbDir    = "/var/lib/bothy-test-db"
)

const startTimeout = 3 * time.Minute

// Options are the limits of the test clamd; zero values take the defaults below, all small and
// explicit so the limit tests stay fast.
type Options struct {
	// StreamMaxLength is clamd StreamMaxLength in bytes; default 1 MiB.
	StreamMaxLength int64
	// MaxScanSize is clamd MaxScanSize in bytes; default 1 MiB.
	MaxScanSize int64
	// MaxFileSize is clamd MaxFileSize in bytes; default 1 MiB.
	MaxFileSize int64
	// MaxRecursion is clamd MaxRecursion; default 3.
	MaxRecursion int
	// Restartable keeps the container after Stop, so Start can run it again (no AutoRemove);
	// Terminate removes it.
	Restartable bool
}

// The defaults of Options.
const (
	defaultLimitBytes   = 1 << 20
	defaultMaxRecursion = 3
)

func (o Options) withDefaults() Options {
	if o.StreamMaxLength == 0 {
		o.StreamMaxLength = defaultLimitBytes
	}
	if o.MaxScanSize == 0 {
		o.MaxScanSize = defaultLimitBytes
	}
	if o.MaxFileSize == 0 {
		o.MaxFileSize = defaultLimitBytes
	}
	if o.MaxRecursion == 0 {
		o.MaxRecursion = defaultMaxRecursion
	}
	return o
}

// Conf returns the clamd.conf of the test container for o: foreground, the test database only,
// a Unix socket inside the container and the TCP test seam, AlertExceedsMax yes and the limits of
// o.
func (o Options) Conf() string {
	o = o.withDefaults()
	lines := []string{
		"Foreground yes",
		"LogSyslog no",
		"DatabaseDirectory " + dbDir,
		"LocalSocket /tmp/clamd.sock",
		"TCPSocket 3310",
		"TCPAddr 0.0.0.0",
		"AlertExceedsMax yes",
		"StreamMaxLength " + strconv.FormatInt(o.StreamMaxLength, 10),
		"MaxScanSize " + strconv.FormatInt(o.MaxScanSize, 10),
		"MaxFileSize " + strconv.FormatInt(o.MaxFileSize, 10),
		"MaxRecursion " + strconv.Itoa(o.MaxRecursion),
	}
	return strings.Join(lines, "\n") + "\n"
}

// Signatures returns the hash database of the test clamd: one line MD5:size:name for EICAR.
func Signatures() string {
	e := EICAR()
	sum := md5.Sum(e) //nolint:gosec // G401: the .hdb format is MD5
	return hex.EncodeToString(sum[:]) + ":" + strconv.Itoa(len(e)) + ":" + SignatureName + "\n"
}

// Clamd is a running test clamd behind a Unix socket relay of the host.
type Clamd struct {
	ctr   *testcontainers.DockerContainer
	relay *relay
}

// Start runs Image with o and a relay socket in a fresh temporary directory, and waits until a
// PING through the relay answers PONG. The error names the step.
func Start(ctx context.Context, o Options) (*Clamd, error) {
	// testcontainers reads its configuration once; only the exact value "true" disables Ryuk.
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		return nil, err
	}
	port := network.MustParsePort(tcpPort)
	ctr, err := testcontainers.Run(ctx, Image,
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-clamdtest-"+strconv.FormatInt(time.Now().UnixNano(), 36)),
		testcontainers.WithLabels(map[string]string{Label: LabelValue}),
		testcontainers.WithEntrypoint("clamd"),
		testcontainers.WithCmd("--foreground", "--config-file="+confPath),
		testcontainers.WithFiles(
			testcontainers.ContainerFile{Reader: strings.NewReader(o.Conf()), ContainerFilePath: confPath, FileMode: 0o644},
			testcontainers.ContainerFile{Reader: strings.NewReader(Signatures()), ContainerFilePath: dbDir + "/bothy-test.hdb", FileMode: 0o644},
		),
		testcontainers.WithExposedPorts(tcpPort),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
			hc.AutoRemove = !o.Restartable
		}),
		testcontainers.WithWaitStrategy(wait.ForListeningPort(tcpPort).WithStartupTimeout(startTimeout)),
	)
	if err != nil {
		if ctr != nil {
			_ = testcontainers.TerminateContainer(ctr)
		}
		return nil, fmt.Errorf("clamdtest: start %s: %w", Image, err)
	}
	c := &Clamd{ctr: ctr}
	c.relay, err = newRelay(func(ctx context.Context) (string, error) {
		p, err := ctr.MappedPort(ctx, tcpPort)
		if err != nil {
			return "", err
		}
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(p.Num()))), nil
	})
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, err
	}
	if err := c.WaitReady(ctx); err != nil {
		_ = c.Terminate(context.WithoutCancel(ctx))
		return nil, err
	}
	return c, nil
}

// Address is the unix:// address of the relay socket.
func (c *Clamd) Address() string { return "unix://" + c.relay.path }

// ContainerID is the id of the container, for stopping exactly it.
func (c *Clamd) ContainerID() string { return c.ctr.GetContainerID() }

// Stop stops the container; the relay then refuses every connection. Only a Restartable clamd
// can be started again.
func (c *Clamd) Stop(ctx context.Context) error {
	timeout := 5 * time.Second
	return c.ctr.Stop(ctx, &timeout)
}

// StartAgain starts a stopped Restartable container and waits for PONG; it may get a new host
// port, which the relay resolves at every connection.
func (c *Clamd) StartAgain(ctx context.Context) error {
	if err := c.ctr.Start(ctx); err != nil {
		return fmt.Errorf("clamdtest: start again: %w", err)
	}
	return c.WaitReady(ctx)
}

// WaitReady polls PING through the relay until PONG or ctx ends (at most the start timeout).
func (c *Clamd) WaitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	var last error
	for {
		if last = ping(ctx, c.relay.path); last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("clamdtest: no PONG: %w", errors.Join(ctx.Err(), last))
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Terminate closes the relay and removes the container.
func (c *Clamd) Terminate(ctx context.Context) error {
	_ = ctx
	var err error
	if c.relay != nil {
		err = c.relay.close()
	}
	// TerminateContainer ignores the errors of a container that AutoRemove already removes.
	return errors.Join(err, testcontainers.TerminateContainer(c.ctr))
}

// ping sends zPING on the socket at path and wants PONG.
func ping(ctx context.Context, path string) error {
	var d net.Dialer // nosemgrep: bothy-go-egress-only -- test helper; the Unix socket of the clamd test container
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("zPING\x00")); err != nil {
		return err
	}
	buf := make([]byte, 16)
	n, _ := conn.Read(buf)
	if !bytes.HasPrefix(buf[:n], []byte("PONG")) {
		return errors.New("clamdtest: no PONG")
	}
	return nil
}

// relay offers a TCP address on a Unix socket of the host: every accepted connection dials the
// address resolve returns at that moment and copies both ways.
type relay struct {
	dir, path string
	ln        net.Listener
	resolve   func(ctx context.Context) (string, error)
	wg        sync.WaitGroup
	mu        sync.Mutex
	conns     map[net.Conn]struct{}
}

func newRelay(resolve func(ctx context.Context) (string, error)) (*relay, error) {
	dir, err := os.MkdirTemp("", "clamd")
	if err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "clamd.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", p)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	r := &relay{dir: dir, path: p, ln: ln, resolve: resolve, conns: map[net.Conn]struct{}{}}
	r.wg.Add(1)
	go r.serve()
	return r, nil
}

func (r *relay) serve() {
	defer r.wg.Done()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.pipe(c)
		}()
	}
}

// pipe connects one client to the container; a failed dial closes the client at once, which the
// client sees as an unreachable scanner.
func (r *relay) pipe(c net.Conn) {
	defer func() { _ = c.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	addr, err := r.resolve(ctx)
	var up net.Conn
	if err == nil {
		var d net.Dialer // nosemgrep: bothy-go-egress-only -- test helper; the mapped TCP port of the clamd test container
		up, err = d.DialContext(ctx, "tcp", addr)
	}
	cancel()
	if err != nil {
		return
	}
	defer func() { _ = up.Close() }()
	r.track(c, up)
	defer r.untrack(c, up)
	done := make(chan struct{})
	go func() {
		_ = copyClose(up, c)
		if tc, ok := up.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		close(done)
	}()
	_ = copyClose(c, up)
	_ = c.Close()
	<-done
}

// copyClose copies src to dst until one side fails or src ends.
func copyClose(dst, src net.Conn) error {
	buf := make([]byte, 32<<10)
	for {
		k, err := src.Read(buf)
		if k > 0 {
			if _, werr := dst.Write(buf[:k]); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func (r *relay) track(cs ...net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cs {
		r.conns[c] = struct{}{}
	}
}

func (r *relay) untrack(cs ...net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cs {
		delete(r.conns, c)
	}
}

func (r *relay) close() error {
	err := r.ln.Close()
	r.mu.Lock()
	for c := range r.conns {
		_ = c.Close()
	}
	r.mu.Unlock()
	r.wg.Wait()
	return errors.Join(err, os.RemoveAll(r.dir))
}
