// SPDX-License-Identifier: Apache-2.0

// Package clientstack serves the terminal stack of M2-48 to the tests of the desktop app (brief
// M4-36 Design note 6, brief M4-38). TestClientStack runs only when BOTHY_CLIENTSTACK_OUT names a
// file: it starts the stack of the profile default with one host container, builds the world of
// the mode BOTHY_CLIENTSTACK_MODE, writes the connection data as JSON to that file and waits
// until the process gets SIGINT or SIGTERM or the file BOTHY_CLIENTSTACK_OUT.stop appears; then
// the stack is removed. Without the variable the test is skipped, so the Go tests of make check
// never start a stack here.
//
// Modes (control.go):
//   - "" (make terminal-stack, M4-36): a paired host assigned to Alex with a session user and a
//     grant terminal; an enrollment code for a device of Alex and one for a device of the case's
//     Owner (only the newest code of a person is valid, 08 3.2, so the Dart test lets the Owner's
//     device create the code of Alex's second device).
//   - "e2e" (make e2e-desktop, M4-38 Design note 1): Alex is an owner, the host container is
//     installed and configured but not paired, and the generator profiles are set; the helper
//     checks that the server knows no host and that no container publishes a port besides the TLS
//     front, then serves the control endpoint (pairing step, canary scan, enrollment codes) on
//     127.0.0.1.
//   - "perf" (make perf-desktop, M4-38 Design note 2): a paired host assigned to Alex, an owner,
//     with the generator profiles reported, and the control endpoint for enrollment codes.
//   - "mobile" (make e2e-mobile, M5-13 Design note 1): the host paired through the API (code,
//     bothy-host pair in the container, fingerprint confirmation) and assigned to Alex, an owner,
//     with his session user, renamed "Build host" with updateHost; the terminal windows of the
//     profile and the control endpoint for enrollment codes.
//
// The JSON holds only test data: the instance origin, the published port of the TLS front on
// 127.0.0.1, the certificate (public part) of the test CA, IDs, one-time enrollment codes, and in
// the mode e2e the loopback address and bearer token of the control endpoint and the canary the
// test types.
package clientstack

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/canary"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/stack"
	"github.com/OWNER/bothy/services/server/test/terminal/suite"
)

// outEnv names the file the connection data goes to; modeEnv the mode.
const (
	outEnv  = "BOTHY_CLIENTSTACK_OUT"
	modeEnv = "BOTHY_CLIENTSTACK_MODE"
)

// stopPoll is how often the test looks for the stop file (a test constant; the signal ends the
// wait at once).
const stopPoll = time.Second

// Connection is the JSON the Dart tests read.
type Connection struct {
	Mode          string `json:"mode"`
	Origin        string `json:"origin"`
	Host          string `json:"instance_host"`
	Port          int    `json:"published_port"`
	CACertificate string `json:"ca_certificate_pem"`
	InstanceID    string `json:"instance_id,omitempty"`
	HostID        string `json:"host_id,omitempty"`
	GrantID       string `json:"grant_id,omitempty"`
	AlexID        string `json:"alex_id"`
	OwnerID       string `json:"owner_id"`
	AlexCode      string `json:"alex_enrollment_code"`
	OwnerCode     string `json:"owner_enrollment_code,omitempty"`
	SessionUser   string `json:"session_user"`
	// ControlURL and ControlToken reach the control endpoint (modes e2e and perf).
	ControlURL   string `json:"control_url,omitempty"`
	ControlToken string `json:"control_token,omitempty"`
	// Canary is the terminal content the E2E test types and the scan looks for (mode e2e).
	Canary    string     `json:"canary,omitempty"`
	Generator *Generator `json:"generator,omitempty"`
	// Windows are the terminal windows of the profile (every mode).
	Windows Windows `json:"terminal_windows"`
}

// Generator describes the generator profiles (modes e2e and perf).
type Generator struct {
	RateProfile     string `json:"rate_profile"`
	RateLines       int    `json:"rate_lines"`
	RateIntervalMs  int64  `json:"rate_interval_ms"`
	BurstProfile    string `json:"burst_profile"`
	BurstChunkBytes int    `json:"burst_chunk_bytes"`
	BurstChunks     int    `json:"burst_chunks"`
}

func TestClientStack(t *testing.T) {
	out := os.Getenv(outEnv) //nolint:forbidigo // test input from make terminal-stack, not server configuration
	if out == "" {
		t.Skipf("%s is not set: the stack starts only for make terminal-stack, e2e-desktop or perf-desktop", outEnv)
	}
	mode := Mode(os.Getenv(modeEnv)) //nolint:forbidigo // test input of the make targets, not server configuration
	if !slices.Contains([]Mode{ModeStack, ModeE2E, ModePerf, ModeMobile}, mode) {
		t.Fatalf("%s=%q: want empty, e2e, perf or mobile", modeEnv, mode)
	}
	env := suite.StartEnv(t, "clientstack", profileOf(mode))
	live := suite.NewLive(t, env, "clientstack", world(mode))
	owner := live.Owner()
	alex := live.Person("alex")
	ctx := t.Context()
	issue := func(ctx context.Context, principal string) (string, error) {
		resp, err := harness.DoWithStepUp(ctx, owner, harness.Request{
			Method: http.MethodPost, Path: "/users/" + principal + "/enrollment-codes",
		})
		if err != nil {
			return "", err
		}
		if resp.Status != http.StatusCreated {
			return "", fmt.Errorf("createEnrollmentCode: %d", resp.Status)
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(resp.Body, &body); err != nil {
			return "", err
		}
		return body.Code, nil
	}
	code := func(principal string) string {
		t.Helper()
		c, err := issue(ctx, principal)
		if err != nil {
			t.Fatalf("createEnrollmentCode for %s: %v", principal, err)
		}
		return c
	}
	alexCode := func(ctx context.Context) (string, error) { return issue(ctx, alex.PrincipalID) }
	windows, err := TerminalWindows(env.Profile)
	if err != nil {
		t.Fatal(err)
	}
	conn := Connection{
		Mode:          string(mode),
		Origin:        stack.InstanceOrigin,
		Host:          stack.InstanceHost,
		Port:          env.Stack.FrontPort,
		CACertificate: string(env.Stack.CA.PEM()),
		AlexID:        alex.PrincipalID,
		OwnerID:       owner.PrincipalID,
		SessionUser:   sessionUser,
		Windows:       windows,
	}
	switch mode {
	case ModeStack:
		conn.HostID = live.HostID(hostName)
		conn.GrantID = live.GrantID("g")
		conn.AlexCode = code(alex.PrincipalID)
		conn.OwnerCode = code(owner.PrincipalID)
	case ModeE2E:
		host := live.Host(hostName)
		installGenerator(t, env, host)
		conn.Generator = generator()
		checkNoHost(t, owner)
		checkPublished(t, env)
		reg := canary.NewRegistry()
		conn.Canary = reg.New(canary.ClassContent, "", "terminal content of the E2E run")
		conn.ControlToken = randomHex(t, 24)
		conn.ControlURL = serveControl(t, &Control{
			Token: conn.ControlToken,
			Pair: func(ctx context.Context, code string) (PairResult, error) {
				return pairInProject(ctx, env, host, code)
			},
			CheckCanary: func(ctx context.Context) ([]string, error) {
				return scanCanary(ctx, t.TempDir(), env, host, reg)
			},
			EnrollmentCode: alexCode,
		})
		conn.AlexCode = code(alex.PrincipalID)
		conn.InstanceID = instanceID(t, owner)
	case ModePerf:
		host := live.Host(hostName)
		installGenerator(t, env, host)
		conn.Generator = generator()
		conn.HostID = live.HostID(hostName)
		waitProfiles(t, owner, conn.HostID)
		checkPublished(t, env)
		conn.ControlToken = randomHex(t, 24)
		conn.ControlURL = serveControl(t, &Control{Token: conn.ControlToken, EnrollmentCode: alexCode})
		conn.AlexCode = code(alex.PrincipalID)
		conn.InstanceID = instanceID(t, owner)
	case ModeMobile:
		conn.HostID = live.HostID(hostName)
		rename(t, owner, conn.HostID, mobileHostName)
		checkPublished(t, env)
		conn.ControlToken = randomHex(t, 24)
		conn.ControlURL = serveControl(t, &Control{Token: conn.ControlToken, EnrollmentCode: alexCode})
		conn.AlexCode = code(alex.PrincipalID)
		conn.InstanceID = instanceID(t, owner)
	}
	b, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Written under another name and renamed, so a reader never sees half a file.
	if err := os.WriteFile(out+".tmp", b, 0o600); err != nil { //nolint:gosec // G703: the path make terminal-stack chose
		t.Fatal(err)
	}
	if err := os.Rename(out+".tmp", out); err != nil { //nolint:gosec // G703: the path make terminal-stack chose
		t.Fatal(err)
	}
	t.Logf("client stack ready (mode %q): %s", mode, out)

	sctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	wait(sctx, out+".stop")
}

// generator returns the generator settings of the connection data.
func generator() *Generator {
	return &Generator{
		RateProfile: RateProfile, RateLines: RateLines, RateIntervalMs: RateInterval.Milliseconds(),
		BurstProfile: BurstProfile, BurstChunkBytes: BurstChunkBytes, BurstChunks: BurstChunks,
	}
}

// installGenerator builds termgen for the architecture of the Docker host, copies it into the
// host container next to bothy-host (owner root, mode 0755) and adds the generator profiles to
// the configuration of the host service, which bothy-host restart reloads.
func installGenerator(t *testing.T, env *suite.Env, h *stack.Host) {
	t.Helper()
	ctx := t.Context()
	bin := filepath.Join(t.TempDir(), "termgen")
	pkg := filepath.Join(env.Stack.Root, "services", "server", "test", "terminal", "clientstack", "termgen")
	build := osexec.CommandContext(ctx, "go", "build", "-trimpath", "-o", bin, ".") //nolint:gosec // G204: fixed program, a path of the test's temp dir
	build.Dir = pkg
	build.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+env.Stack.Arch, "CGO_ENABLED=0", "GOTOOLCHAIN=local") //nolint:forbidigo // go needs PATH, HOME and the caches of the machine
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build termgen: %v\n%s", err, out)
	}
	if err := h.CopyIn(ctx, bin, generatorPath); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Shell(ctx, "chown root:root "+generatorPath+" && chmod 0755 "+generatorPath); err != nil {
		t.Fatal(err)
	}
	conf, err := h.ReadConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]map[string]any
	if err := yaml.Unmarshal(conf, &doc); err != nil {
		t.Fatalf("host configuration: %v", err)
	}
	doc["host_service"]["profiles"] = generatorProfiles()
	next, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.WriteConfig(ctx, next); err != nil {
		t.Fatal(err)
	}
}

// checkNoHost fails unless the server knows no host: the E2E mode starts with an unpaired host
// container (brief M4-38 Tests).
func checkNoHost(t *testing.T, owner *harness.Client) {
	t.Helper()
	resp, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{Method: http.MethodGet, Path: "/hosts"})
	if err != nil || resp.Status != http.StatusOK {
		t.Fatalf("listHosts: %v %d", err, resp.Status)
	}
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	resp.JSON(t, &page)
	if len(page.Items) != 0 {
		t.Fatalf("the E2E mode starts with %d hosts on the server, want none", len(page.Items))
	}
}

// checkPublished fails unless the only published port of the project is the TLS front on
// 127.0.0.1 (brief M4-38 Tests: nothing new is published).
func checkPublished(t *testing.T, env *suite.Env) {
	t.Helper()
	ps, err := env.Stack.Compose(t.Context(), "ps", "--all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := PublishedPorts(ps)
	if err != nil {
		t.Fatal(err)
	}
	if err := OnlyFront(pub, env.Stack.FrontPort); err != nil {
		t.Fatal(err)
	}
}

// pairInProject runs bothy-host pair with code in the host container h after it checked that
// the container belongs to the stack's Compose project, so the step never reaches another
// machine.
func pairInProject(ctx context.Context, env *suite.Env, h *stack.Host, code string) (PairResult, error) {
	project, err := h.Inspect(ctx, `{{index .Config.Labels "com.docker.compose.project"}}`)
	if err != nil {
		return PairResult{}, err
	}
	if project != env.Stack.Project {
		return PairResult{}, fmt.Errorf("the host container belongs to project %q, not %q", project, env.Stack.Project)
	}
	fp, rc, err := h.Pair(ctx, code)
	if err != nil {
		return PairResult{}, err
	}
	return PairResult{Fingerprint: fp, ExitCode: rc}, nil
}

// scanCanary collects the sinks of TM-T06 (the container logs, a data dump of the database, the
// collector export and the journal of the host service) and scans them for the canary of reg.
func scanCanary(ctx context.Context, dir string, env *suite.Env, h *stack.Host, reg *canary.Registry) ([]string, error) {
	sources := map[string][]byte{}
	logs, err := env.Stack.Logs(ctx)
	if err != nil {
		return nil, err
	}
	sources[suite.SinkContainerLog+":compose logs"] = logs
	dump, err := env.Stack.Exec(ctx, "postgres", "pg_dump", "-U", "postgres", "--data-only", "bothy")
	if err != nil {
		return nil, err
	}
	sources[suite.SinkDatabaseDump+":pg_dump"] = dump
	export := filepath.Join(dir, "collector")
	if err := env.Stack.CopyFrom(ctx, "otel-collector", "/var/lib/otelcol-smoke/.", export); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(export)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(export, e.Name())) //nolint:gosec // G304: the copied export in the test's temp dir
		if err != nil {
			return nil, err
		}
		sources[suite.SinkCollector+":"+e.Name()] = b
	}
	j, err := h.Journal(ctx)
	if err != nil {
		return nil, err
	}
	sources[suite.SinkHostJournal+":"+h.Name] = j
	return suite.ScanCanaries(reg, suite.CanarySinks(sources, nil)), nil
}

// serveControl serves c on 127.0.0.1 port 0 until the test ends and returns its URL.
func serveControl(t *testing.T, c *Control) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: c, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}

// rename gives the host hostID the name the phone suite looks for (updateHost with the ETag of
// getHost, step-up); the suite names its hosts with a suffix of its own.
func rename(t *testing.T, owner *harness.Client, hostID, name string) {
	t.Helper()
	get, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{Method: http.MethodGet, Path: "/hosts/" + hostID})
	if err != nil || get.Status != http.StatusOK {
		t.Fatalf("getHost: %v %d", err, get.Status)
	}
	resp, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{
		Method: http.MethodPatch, Path: "/hosts/" + hostID,
		Header: http.Header{"If-Match": []string{get.Header.Get("ETag")}},
		Body:   map[string]any{"name": name, "reason": "Terminal of the phone E2E run"},
	})
	if err != nil || resp.Status != http.StatusOK {
		t.Fatalf("updateHost: %v %d %s", err, resp.Status, resp.Body)
	}
}

// waitProfiles waits until the host reports the generator profiles after the restart.
func waitProfiles(t *testing.T, owner *harness.Client, hostID string) {
	t.Helper()
	testkit.Until(t, "the generator profiles of the host", func() bool {
		resp, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{Method: http.MethodGet, Path: "/hosts/" + hostID})
		return err == nil && resp.Status == http.StatusOK && strings.Contains(string(resp.Body), `"`+RateProfile+`"`) &&
			strings.Contains(string(resp.Body), `"`+BurstProfile+`"`)
	})
}

// instanceID returns the instance ID of the discovery document.
func instanceID(t *testing.T, owner *harness.Client) string {
	t.Helper()
	resp := owner.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/instance/discovery"})
	if resp.Status != http.StatusOK {
		t.Fatalf("getDiscovery: %d", resp.Status)
	}
	var d struct {
		InstanceID string `json:"instance_id"`
	}
	resp.JSON(t, &d)
	if d.InstanceID == "" {
		t.Fatal("getDiscovery: no instance_id")
	}
	return d.InstanceID
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// wait returns when ctx ends or the file stopFile exists.
func wait(ctx context.Context, stopFile string) {
	tick := time.NewTicker(stopPoll)
	defer tick.Stop()
	for {
		if _, err := os.Stat(stopFile); err == nil || !errors.Is(err, os.ErrNotExist) { //nolint:gosec // G703: the path make terminal-stack chose
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
