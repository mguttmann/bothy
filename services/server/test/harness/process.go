// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// ProcessOptions change a server process of StartProcess.
type ProcessOptions struct {
	// Config holds dotted keys of docs/operations/configuration.md for every command; they are
	// passed as BOTHY_ environment variables and override the harness defaults.
	Config map[string]string
	// Env are further NAME=value entries of the process environment.
	Env []string
	// GOMAXPROCS of bothy serve; 0 leaves the Go default.
	GOMAXPROCS int
	// DBCPUs and DBMemoryBytes limit the PostgreSQL container (pgtest.Limits); 0 is no limit.
	DBCPUs        float64
	DBMemoryBytes int64
}

// Process is bothy serve --role=all as a child process against a durable pgtest database
// (pgtest.Durable), prepared by the real commands bothy secrets init, bothy migrate up
// --bootstrap and bothy setup bootstrap-owner. The embedded Server gives the URLs, the enrolled
// Owner, NewUser, NewAgent and AdminDB, as for Start.
type Process struct {
	*Server
	// DB is the database of the process (Restart of the container in chaos tests).
	DB *pgtest.DB
	// LogPath is the file that receives stdout and stderr of bothy serve.
	LogPath string

	tb       testing.TB
	bin      string
	opts     ProcessOptions
	secrets  string // the secret volume of bothy secrets init
	signKeys string
	// files is file.fs.root of the server (M3-03).
	files    string
	public   int
	internal int

	mu      sync.Mutex
	cmd     *exec.Cmd
	exited  chan struct{}
	started time.Time
	ready   time.Duration
}

// build holds the server binary of this test binary, built once by StartProcess.
var build struct {
	once sync.Once
	dir  string
	bin  string
	err  error
}

// cleanupBuild removes the build directory; Main calls it after the tests.
func cleanupBuild() {
	if build.dir != "" {
		_ = os.RemoveAll(build.dir)
	}
}

// binary builds ./cmd/bothy once per test binary into a temp directory and returns its path.
func binary() (string, error) {
	build.once.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		module := filepath.Join(filepath.Dir(file), "..", "..")
		build.dir, build.err = os.MkdirTemp("", "bothy-harness-bin-")
		if build.err != nil {
			return
		}
		build.bin = filepath.Join(build.dir, "bothy")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", build.bin, "./cmd/bothy") // #nosec G204 -- fixed arguments
		cmd.Dir = module
		if out, err := cmd.CombinedOutput(); err != nil {
			build.err = fmt.Errorf("harness: go build ./cmd/bothy: %w\n%s", err, out)
		}
	})
	return build.bin, build.err
}

// processPortAttempts is how often a start is repeated on new ports after a bind conflict.
const processPortAttempts = 3

// StartProcess builds ./cmd/bothy once per test binary, runs bothy secrets init, bothy migrate up
// --bootstrap and bothy setup bootstrap-owner against a fresh durable pgtest database, then bothy
// serve --role=all with server.listen_addr and server.internal_listen_addr on free 127.0.0.1 ports
// picked just before the start (retried on a bind conflict), waits for /readyz and enrolls the
// owner with fresh device keys. The process is stopped by PID in tb.Cleanup. Errors fail tb.
func StartProcess(tb testing.TB, opts ProcessOptions) *Process {
	tb.Helper()
	bin, err := binary()
	if err != nil {
		tb.Fatal(err)
	}
	dbOpts := []pgtest.Option{pgtest.WithoutMigrations(), pgtest.Durable()}
	if opts.DBCPUs > 0 || opts.DBMemoryBytes > 0 {
		dbOpts = append(dbOpts, pgtest.Limits(opts.DBCPUs, opts.DBMemoryBytes))
	}
	db := pgtest.New(tb, dbOpts...)
	dir := tb.TempDir()
	p := &Process{DB: db, LogPath: filepath.Join(dir, "serve.log"), tb: tb, bin: bin, opts: opts,
		secrets: filepath.Join(dir, "secrets"), signKeys: filepath.Join(dir, "signing-keys"), files: filepath.Join(dir, "files")}
	for _, d := range []string{p.signKeys, p.files} {
		if err := os.Mkdir(d, 0o700); err != nil {
			tb.Fatal(err)
		}
	}
	tb.Cleanup(func() { p.stop(); makeRemovable(p.secrets) })
	if err := p.pickPorts(); err != nil {
		tb.Fatal(err)
	}
	if err := p.prepare(dir); err != nil {
		tb.Fatal(err)
	}
	code, err := p.run("setup", p.env(p.secrets+"/server", "bothy_app"), "setup", "bootstrap-owner", "--handle", ownerHandle, "--display-name", ownerDisplayName)
	if err != nil {
		tb.Fatal(err)
	}
	var lastErr error
	for range processPortAttempts {
		if lastErr = p.start(tb.Context()); !errors.Is(lastErr, errPortTaken) {
			break
		}
		if lastErr = p.pickPorts(); lastErr != nil {
			break
		}
	}
	if lastErr != nil {
		tb.Fatal(lastErr)
	}
	owner, err := p.redeem(tb.Context(), strings.TrimSpace(code), "Owner device")
	if err != nil {
		tb.Fatal(err)
	}
	p.Owner = owner
	return p
}

// prepare runs bothy secrets init with the passwords of the container as supplied source and
// bothy migrate up --bootstrap as bothy_owner.
func (p *Process) prepare(dir string) error {
	src := filepath.Join(dir, "source")
	if err := os.Mkdir(src, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"db_superuser_password", "db_owner_password", "db_app_password", "db_maint_password", "db_monitor_password"} {
		b, err := os.ReadFile(filepath.Join(p.DB.SecretsDir(), name)) // #nosec G304 -- a fixed name in the pgtest directory
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(src, name), b, 0o600); err != nil { //nolint:gosec // G703: a fixed name in a temp dir of the test
			return err
		}
	}
	me := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	if _, err := p.run("secrets init", nil, "secrets", "init", "--target", p.secrets, "--source", src,
		"--owner", "server="+me, "--owner", "migrate="+me); err != nil {
		return err
	}
	_, err := p.run("migrate", p.env(p.secrets+"/migrate", "bothy_owner"), "migrate", "up", "--bootstrap")
	return err
}

// run runs one command of the server binary to its end and returns its stdout; a non-zero exit is an error with
// stderr.
func (p *Process) run(name string, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(p.tb.Context(), readyWait)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, args...) // #nosec G204 -- the harness binary with fixed arguments
	cmd.Env = append(baseEnv(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("harness: bothy %s: %w\n%s", name, err, stderr.String())
	}
	return stdout.String(), nil
}

// baseEnv is the environment every command inherits: PATH, HOME and TMPDIR of the test process.
func baseEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "HOME", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok { //nolint:forbidigo // the child inherits PATH, HOME and TMPDIR of the test process
			env = append(env, k+"="+v)
		}
	}
	return env
}

// env builds the BOTHY_ environment of a command with secrets.dir secretsDir as database user.
func (p *Process) env(secretsDir, user string) []string {
	cfg := p.DB.Config(pg.RoleApp)
	public := "http://127.0.0.1:" + strconv.Itoa(p.public)
	set := map[string]string{
		"secrets.dir":                 secretsDir,
		"secrets.signing_key_dir":     p.signKeys,
		"file.fs.root":                p.files,
		"database.host":               cfg.Host,
		"database.port":               strconv.Itoa(cfg.Port),
		"database.name":               p.DB.Name(),
		"database.user":               user,
		"database.password_ref":       "file:" + secretsDir + "/db_" + strings.TrimPrefix(user, "bothy_") + "_password",
		"database.sslmode":            "disable",
		"server.public_url":           public,
		"server.listen_addr":          "127.0.0.1:" + strconv.Itoa(p.public),
		"server.internal_listen_addr": "127.0.0.1:" + strconv.Itoa(p.internal),
		"startup.dependency_wait":     "30s",
		"telemetry.enabled":           "false",
		"telemetry.logs.exporter":     "stdout",
		"shutdown.drain_delay":        "0s",
	}
	for k, v := range p.opts.Config {
		set[k] = v
	}
	env := make([]string, 0, len(set)+len(p.opts.Env))
	for k, v := range set {
		env = append(env, config.EnvName(k)+"="+v)
	}
	return append(env, p.opts.Env...)
}

// pickPorts reserves two free loopback ports for the public and the internal listener.
func (p *Process) pickPorts() error {
	public, err := freePort()
	if err != nil {
		return err
	}
	internal, err := freePort()
	if err != nil {
		return err
	}
	if internal == public {
		return errPortTaken
	}
	p.public, p.internal = public, internal
	return nil
}

// start runs bothy serve --role=all, waits for /readyz and points the embedded Server at it.
func (p *Process) start(ctx context.Context) error {
	env := p.env(p.secrets+"/server", "bothy_app")
	if p.opts.GOMAXPROCS > 0 {
		env = append(env, "GOMAXPROCS="+strconv.Itoa(p.opts.GOMAXPROCS))
	}
	log, err := os.OpenFile(p.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600) // #nosec G304 -- a file in the temp dir of the test
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()
	offset, _ := log.Seek(0, 2)
	// The process outlives ctx (a start inside a test step); stop and Cleanup end it by PID.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), p.bin, "serve", "--role=all") // #nosec G204 -- the harness binary with fixed arguments
	cmd.Env = append(baseEnv(), env...)
	cmd.Stdout, cmd.Stderr = log, log
	p.mu.Lock()
	p.started = time.Now()
	if err := cmd.Start(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("harness: start bothy serve: %w", err)
	}
	exited := make(chan struct{})
	p.cmd, p.exited = cmd, exited
	p.mu.Unlock()
	go func() { _ = cmd.Wait(); close(exited) }()
	public := "http://127.0.0.1:" + strconv.Itoa(p.public)
	if p.Server == nil {
		schema, err := wire.ConfigSchema()
		if err != nil {
			return err
		}
		values, err := config.Load(schema, config.Sources{Env: env})
		if err != nil {
			return err
		}
		p.Server = &Server{db: p.DB, tb: p.tb, clk: clock.System(), values: values, env: env,
			http: &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{MaxIdleConns: 4096, MaxIdleConnsPerHost: 4096}}}
	}
	p.PublicURL, p.APIBase = public, public+apiPrefix
	p.GatewayURL = "ws://127.0.0.1:" + strconv.Itoa(p.public) + apiPrefix + "/gateway"
	p.InternalURL = "http://127.0.0.1:" + strconv.Itoa(p.internal)
	p.SecretsDir = p.secrets + "/server"
	if err := p.WaitReady(ctx); err != nil {
		if strings.Contains(p.logSince(offset), "address already in use") {
			return errPortTaken
		}
		return err
	}
	return nil
}

// logSince returns the log file from offset on.
func (p *Process) logSince(offset int64) string {
	b, err := os.ReadFile(p.LogPath)
	if err != nil || int64(len(b)) < offset {
		return ""
	}
	return string(b[offset:])
}

// WaitReady polls /readyz of the internal listener until it answers 200 and records the time
// from the exec of the process to that answer (ReadyAfter). The error reports a process that
// exited first or ctx ending, with the log of the process.
func (p *Process) WaitReady(ctx context.Context) error {
	p.mu.Lock()
	exited, started := p.exited, p.started
	p.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, readyWait)
	defer cancel()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-exited:
			return fmt.Errorf("harness: bothy serve exited before ready:\n%s", p.tail())
		case <-ctx.Done():
			return fmt.Errorf("harness: bothy serve not ready: %w\n%s", ctx.Err(), p.tail())
		default:
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.InternalURL+"/readyz", nil)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				p.mu.Lock()
				p.ready = time.Since(started)
				p.mu.Unlock()
				return nil
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ReadyAfter returns the time from the exec of the last start to the first 200 of /readyz.
func (p *Process) ReadyAfter() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ready
}

// tail returns the last lines of the log of the process.
func (p *Process) tail() string {
	b, _ := os.ReadFile(p.LogPath)
	const keep = 8 << 10
	if len(b) > keep {
		b = b[len(b)-keep:]
	}
	return string(b)
}

// PID returns the process ID of the running bothy serve.
func (p *Process) PID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd.Process.Pid
}

// Signal sends sig to the process.
func (p *Process) Signal(sig os.Signal) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd.Process.Signal(sig)
}

// Exited returns a channel closed when the current process has ended.
func (p *Process) Exited() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

// Restart stops the process (SIGTERM, or nothing when it has ended already, for example after a
// SIGKILL) and starts it again on the same ports with the same configuration; it returns when
// /readyz answers 200.
func (p *Process) Restart(ctx context.Context) error {
	return p.RestartWith(ctx, p.opts.Config)
}

// RestartWith is Restart with config instead of ProcessOptions.Config from now on: the seeding
// server of a load test raises the rate limits, the measured server starts with the defaults.
func (p *Process) RestartWith(ctx context.Context, config map[string]string) error {
	p.stop()
	p.opts.Config = config
	var err error
	for range processPortAttempts {
		if err = p.start(ctx); !errors.Is(err, errPortTaken) {
			return err
		}
		time.Sleep(time.Second)
	}
	return err
}

// stop sends SIGTERM by PID and waits up to stopWait, then SIGKILL; a process that has ended is
// left alone.
func (p *Process) stop() {
	p.mu.Lock()
	cmd, exited := p.cmd, p.exited
	p.mu.Unlock()
	if cmd == nil {
		return
	}
	select {
	case <-exited:
		return
	default:
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(stopWait):
		_ = cmd.Process.Kill()
		<-exited
	}
}

// RSSBytes returns the resident set size of the process: VmRSS of /proc/<pid>/status on Linux,
// ps -o rss= on macOS.
func (p *Process) RSSBytes() (int64, error) {
	pid := p.PID()
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/status") // #nosec G304 -- the status file of the own child
		if err != nil {
			return 0, err
		}
		return ParseProcStatusRSS(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output() // #nosec G204 -- ps with the PID of the own child
	if err != nil {
		return 0, fmt.Errorf("harness: ps: %w", err)
	}
	return ParsePSRSS(out)
}

// ParseProcStatusRSS returns VmRSS of a /proc/<pid>/status text in bytes.
func ParseProcStatusRSS(b []byte) (int64, error) {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 3 && fields[0] == "VmRSS:" && fields[2] == "kB" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("harness: VmRSS: %w", err)
			}
			return kb << 10, nil
		}
	}
	return 0, errors.New("harness: no VmRSS line in kB")
}

// ParsePSRSS returns the output of ps -o rss= (kibibytes) in bytes.
func ParsePSRSS(out []byte) (int64, error) {
	kb, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || kb <= 0 {
		return 0, fmt.Errorf("harness: ps rss %q is not a size", strings.TrimSpace(string(out)))
	}
	return kb << 10, nil
}
