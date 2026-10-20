// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Profile names a stack profile of profiles.yaml.
type Profile string

// The stack profiles (Design note 4).
const (
	ProfileDefault Profile = "default"
	ProfileLimits  Profile = "limits"
	ProfileTwoNode Profile = "twonode"
)

// waitTimeout bounds every wait of compose up and down (a test constant, not a product limit).
const waitTimeout = 300 * time.Second

// Options configure Start.
type Options struct {
	// Package is the name of the e2e package (part of the project name).
	Package string
	// Profile selects the Compose files and the routing of the TLS front.
	Profile Profile
	// ServerKeys are the server keys of the profile as dotted names with their values; they reach
	// api, worker (and api-2) as BOTHY_* variables through the env_file of the terminal overlay.
	ServerKeys map[string]string
	// Front gives the timeouts of the TLS front.
	Front FrontTimeouts
	// Logf, when set, receives progress lines (no data of the stack).
	Logf func(format string, args ...any)
}

// Stack is one running Compose project of the suite.
type Stack struct {
	// Project is the Compose project name termsuite-<package>-<8 hex>.
	Project string
	// Root is the repository root, Dir the temporary folder of the stack (mode 0700).
	Root, Dir string
	// ServerImage and HostImage are the content-addressed local tags of the images.
	ServerImage, HostImage string
	// Arch is the GOARCH of the Docker host.
	Arch string
	// FrontPort is the published port of the TLS front on 127.0.0.1.
	FrontPort int
	// CA is the test CA of the stack.
	CA *CA
	// Profile is the profile of the stack.
	Profile Profile
	// Timings records wall times of the start (for the report).
	Timings map[string]time.Duration

	docker   *Docker
	files    []string
	env      []string
	frontDir string
	logf     func(string, ...any)
	hashes   map[string]string

	mu    sync.Mutex
	hosts map[string]*Host
}

// composeFiles returns the -f arguments of profile p in the order of Design note 3.
func composeFiles(p Profile) []string {
	files := []string{"compose.yaml", "test/compose.smoke.yaml"}
	if p == ProfileTwoNode {
		files = append(files, "compose.multi-node.yaml")
	}
	files = append(files, "test/compose.terminal.yaml")
	if p == ProfileTwoNode {
		files = append(files, "test/compose.terminal-twonode.yaml")
	}
	return files
}

// services returns the services that up starts for profile p.
func services(p Profile) []string {
	s := []string{"api", "worker", "otel-collector", "tls-front"}
	if p == ProfileTwoNode {
		s = append(s, "api-2", "edge")
	}
	return s
}

// EnvName maps a dotted configuration key to its BOTHY_ variable (docs/operations/configuration.md:
// a.b_c is BOTHY_A__B_C).
//
// Example: EnvName("terminal.idle_timeout") == "BOTHY_TERMINAL__IDLE_TIMEOUT".
func EnvName(key string) string {
	return "BOTHY_" + strings.ToUpper(strings.ReplaceAll(key, ".", "__"))
}

// Start runs the preflight, builds the images, renders the TLS front and starts the stack of o. It
// returns the stack even on an error after the project exists, so the caller can Down it.
//
// Errors: ErrPrecondition from the preflight, a failed build or start.
func Start(ctx context.Context, o Options) (*Stack, error) {
	root, err := RepoRoot()
	if err != nil {
		return nil, err
	}
	logf := o.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	d := &Docker{}
	if err := Preflight(ctx, d); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "termsuite-") //nolint:usetesting // the stack outlives one test: TestMain owns it
	if err != nil {
		return nil, err
	}
	s := &Stack{Root: root, Dir: dir, Profile: o.Profile, docker: d, logf: logf, Timings: map[string]time.Duration{}, hosts: map[string]*Host{}}
	if s.Arch, err = Arch(ctx, d); err != nil {
		return s, err
	}
	t0 := time.Now()
	if s.ServerImage, err = ServerImage(ctx, d, root, dir); err != nil {
		return s, err
	}
	s.Timings["server_image"] = time.Since(t0)
	logf("stack: server image %s built in %s", s.ServerImage, s.Timings["server_image"].Round(time.Second))
	t0 = time.Now()
	if s.HostImage, err = HostImage(ctx, d, root); err != nil {
		return s, err
	}
	s.Timings["host_image"] = time.Since(t0)
	if err := PtraceScope(ctx, d, s.HostImage); err != nil {
		return s, err
	}
	if err := s.prepare(o); err != nil {
		return s, err
	}
	if s.hashes, err = hashTree(filepath.Join(root, "infra", "compose")); err != nil {
		return s, err
	}
	t0 = time.Now()
	if err := s.up(ctx); err != nil {
		s.Diagnose(ctx, os.Stderr)
		return s, err
	}
	s.Timings["compose_up"] = time.Since(t0)
	logf("stack: %s up in %s, TLS front on 127.0.0.1:%d", s.Project, s.Timings["compose_up"].Round(time.Second), s.FrontPort)
	return s, nil
}

// prepare writes the files of the stack into its temporary folder and builds the environment of
// every Compose call.
func (s *Stack) prepare(o Options) error {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	s.Project = "termsuite-" + o.Package + "-" + hex.EncodeToString(suffix)
	s.frontDir = filepath.Join(s.Dir, "front")
	for _, sub := range []string{"front", "secrets-input", "out", "logs", "bin"} {
		if err := os.Mkdir(filepath.Join(s.Dir, sub), 0o700); err != nil {
			return err
		}
	}
	// The front runs as the uid of the test process and must read its folder.
	if err := os.Chmod(s.frontDir, 0o700); err != nil { //nolint:gosec // G302: only the test user reads it
		return err
	}
	var err error
	if s.CA, err = NewCA(s.frontDir); err != nil {
		return err
	}
	mode := ModeSingle
	if o.Profile == ProfileTwoNode {
		mode = ModeHash
	}
	if err := RenderFront(s.Root, s.frontDir, mode, o.Front); err != nil {
		return err
	}
	var env bytes.Buffer
	for _, k := range slices.Sorted(maps.Keys(o.ServerKeys)) {
		fmt.Fprintf(&env, "%s=%s\n", EnvName(k), o.ServerKeys[k])
	}
	serverEnv := filepath.Join(s.Dir, "server.env")
	if err := os.WriteFile(serverEnv, env.Bytes(), 0o600); err != nil {
		return err
	}
	s.files = composeFiles(o.Profile)
	s.env = []string{
		"COMPOSE_PROFILES=direct",
		"BOTHY_PUBLIC_URL=" + InstanceOrigin,
		"SMOKE_SERVER_IMAGE=" + s.ServerImage,
		"SMOKE_SECRETS_INPUT=" + filepath.Join(s.Dir, "secrets-input"),
		"SMOKE_RUN_ID=" + hex.EncodeToString(suffix),
		"SMOKE_OUT=" + filepath.Join(s.Dir, "out"),
		"SMOKE_TMP=" + s.Dir,
		"SMOKE_CANARY=termsuite",
		"TERMSUITE_SERVER_ENV=" + serverEnv,
		"TERMSUITE_FRONT_DIR=" + s.frontDir,
		"TERMSUITE_HOST_IMAGE=" + s.HostImage,
		"TERMSUITE_UID=" + strconv.Itoa(os.Getuid()),
		"TERMSUITE_GID=" + strconv.Itoa(os.Getgid()),
	}
	return nil
}

// composeArgs returns the arguments of a Compose call with args.
func (s *Stack) composeArgs(args ...string) []string {
	out := []string{"compose", "--project-name", s.Project, "--project-directory", filepath.Join(s.Root, "infra", "compose")}
	for _, f := range s.files {
		out = append(out, "--file", filepath.Join(s.Root, "infra", "compose", f))
	}
	out = append(out, "--env-file", filepath.Join(s.Root, "infra", "compose", "test", "smoke.env"))
	return append(out, args...)
}

// Compose runs docker compose with args in the project and returns stdout.
//
// Errors: ErrDocker with the last line of stderr.
func (s *Stack) Compose(ctx context.Context, args ...string) ([]byte, error) {
	return s.docker.must(ctx, s.env, s.composeArgs(args...)...)
}

// ComposeRaw runs docker compose with args and stdin in, returning stdout, stderr and the exit
// code without judging it.
func (s *Stack) ComposeRaw(ctx context.Context, in io.Reader, args ...string) (stdout, stderr []byte, code int, err error) {
	r, err := s.docker.run(ctx, in, s.env, s.composeArgs(args...)...)
	return r.stdout, r.stderr, r.code, err
}

// up creates the networks, reads the subnet of the default network for server.trusted_proxies and
// starts the services of the profile.
func (s *Stack) up(ctx context.Context) error {
	if _, err := s.Compose(ctx, "up", "--no-start"); err != nil {
		return err
	}
	out, err := s.docker.must(ctx, nil, "network", "inspect", "--format", "{{range .IPAM.Config}}{{.Subnet}} {{end}}", s.Project+"_default")
	if err != nil {
		return err
	}
	var subnets []string
	for _, f := range strings.Fields(string(out)) {
		if strings.Contains(f, ".") {
			subnets = append(subnets, f)
		}
	}
	if len(subnets) == 0 {
		return fmt.Errorf("stack: the network %s_default has no IPv4 subnet", s.Project)
	}
	s.env = append(s.env, "TERMSUITE_TRUSTED_PROXIES="+strings.Join(subnets, ","))
	args := append([]string{"up", "--detach", "--wait", "--wait-timeout", strconv.Itoa(int(waitTimeout / time.Second))}, services(s.Profile)...)
	if _, err := s.Compose(ctx, args...); err != nil {
		return err
	}
	port, err := s.Compose(ctx, "port", "tls-front", strconv.Itoa(InstancePort))
	if err != nil {
		return err
	}
	_, p, ok := strings.Cut(strings.TrimSpace(string(port)), ":")
	if !ok {
		return fmt.Errorf("stack: unexpected port answer %q", strings.TrimSpace(string(port)))
	}
	if s.FrontPort, err = strconv.Atoi(p); err != nil {
		return err
	}
	return nil
}

// BootstrapOwnerCode runs bothy setup bootstrap-owner in api (M1-37, as smoke.sh runs it) and
// returns the enrollment code of the owner. Stdout goes to a file of mode 0600 in the temporary
// folder and is never printed.
//
// Errors: a failed command, or an output that is not one enrollment code.
func (s *Stack) BootstrapOwnerCode(ctx context.Context, handle, displayName string) (string, error) {
	out, stderr, code, err := s.ComposeRaw(ctx, nil, "exec", "-T", "api", "/usr/local/bin/bothy", "setup", "bootstrap-owner",
		"--handle", handle, "--display-name", displayName)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("stack: bootstrap-owner exited %d: %s", code, lastLine(stderr))
	}
	path := filepath.Join(s.Dir, "out", "setup.out")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(out))
	if strings.Contains(line, "\n") || !strings.HasPrefix(line, "en_") {
		return "", errors.New("stack: the bootstrap-owner output is not one enrollment code")
	}
	return line, nil
}

// Exec runs args in the Compose service svc (compose exec -T) and returns stdout.
//
// Errors: ErrDocker with the last line of stderr.
func (s *Stack) Exec(ctx context.Context, svc string, args ...string) ([]byte, error) {
	return s.Compose(ctx, append([]string{"exec", "-T", svc}, args...)...)
}

// SQL runs query read-only (default_transaction_read_only) with psql in the postgres container
// and returns the rows as JSON objects (one per row, columns by name). A write needs bypass: it
// is allowed only for the steps of TM-T28 (c) that change the database bypassing the server.
//
// Errors: a failed query (the psql message names the SQL error).
func (s *Stack) SQL(ctx context.Context, query string, bypass bool) ([]map[string]any, error) {
	args := []string{"exec", "-T"}
	if !bypass {
		args = append(args, "-e", "PGOPTIONS=-c default_transaction_read_only=on")
		query = "SELECT coalesce(json_agg(t), '[]'::json) FROM (" + query + ") t"
	}
	args = append(args, "postgres", "psql", "-X", "-q", "-At", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "bothy", "-c", query)
	out, stderr, code, err := s.ComposeRaw(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("stack: psql exited %d: %s", code, lastLine(stderr))
	}
	if bypass {
		return nil, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out), &rows); err != nil {
		return nil, fmt.Errorf("stack: psql answer: %w", err)
	}
	return rows, nil
}

// CopyFrom copies path of the Compose service svc into dst on the machine of the tests (compose
// cp) and sets its mode to 0600.
func (s *Stack) CopyFrom(ctx context.Context, svc, path, dst string) error {
	if _, err := s.Compose(ctx, "cp", svc+":"+path, dst); err != nil {
		return err
	}
	return os.Chmod(dst, 0o600)
}

// Logs returns the container logs of the services (all when none is named), without colour.
func (s *Stack) Logs(ctx context.Context, svc ...string) ([]byte, error) {
	return s.Compose(ctx, append([]string{"logs", "--no-color"}, svc...)...)
}

// FrontMode switches the routing of the TLS front to mode and reloads nginx in its container.
func (s *Stack) FrontMode(ctx context.Context, mode Mode) error {
	if err := SetFrontRouting(s.Root, s.frontDir, mode); err != nil {
		return err
	}
	_, err := s.Exec(ctx, "tls-front", "nginx", "-c", "/etc/termsuite-front/nginx.conf", "-s", "reload")
	return err
}

// RestartService restarts the Compose service svc (docker compose restart) and waits until it is
// healthy again when it has a healthcheck.
func (s *Stack) RestartService(ctx context.Context, svc string) error {
	if _, err := s.Compose(ctx, "restart", svc); err != nil {
		return err
	}
	return s.WaitHealthy(ctx, svc)
}

// WaitHealthy polls the health of the container of svc until it is healthy or ctx ends.
func (s *Stack) WaitHealthy(ctx context.Context, svc string) error {
	for {
		out, err := s.Compose(ctx, "ps", "--format", "{{.Health}}", svc)
		if err == nil && strings.TrimSpace(string(out)) == "healthy" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("stack: %s not healthy: %w", svc, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Diagnose writes compose ps --all and the last log lines of api, worker and every host
// container, filtered to their codes, to w (on a failure, Design note 3).
func (s *Stack) Diagnose(ctx context.Context, w io.Writer) {
	if s.Project == "" {
		return
	}
	if out, err := s.Compose(ctx, "ps", "--all", "--format", "{{.Service}} {{.State}} {{.Status}}"); err == nil {
		_, _ = fmt.Fprintf(w, "stack %s:\n%s", s.Project, out)
	}
	for _, svc := range []string{"api", "worker"} {
		out, err := s.Compose(ctx, "logs", "--no-color", "--tail", "60", svc)
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "last log lines of %s (codes only):\n", svc)
		writeCodes(w, out)
	}
	s.mu.Lock()
	hosts := slices.Collect(maps.Values(s.hosts))
	s.mu.Unlock()
	for _, h := range hosts {
		r, err := s.docker.run(ctx, nil, nil, "exec", h.Container, "journalctl", "--no-pager", "-o", "cat", "-n", "40", "-u", "bothy-host*")
		if err != nil {
			continue
		}
		_, _ = fmt.Fprintf(w, "last journal lines of %s (codes only):\n", h.Container)
		writeCodes(w, r.stdout)
	}
}

// codeFields are the fields of a JSON log line that Diagnose keeps: level, message and the
// fields that carry codes, states and reasons, never a value of a request.
var codeFields = []string{"level", "msg", "code", "check", "ok", "session_user", "reason", "close_code", "close_reason", "close_detail", "state", "error.type", "op"}

// writeCodes writes the code fields of every JSON line of logs to w; other lines are counted only.
func writeCodes(w io.Writer, logs []byte) {
	sc := bufio.NewScanner(bytes.NewReader(logs))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	other := 0
	for sc.Scan() {
		line := sc.Text()
		i := strings.IndexByte(line, '{')
		if i < 0 {
			other++
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line[i:]), &m) != nil {
			other++
			continue
		}
		var parts []string
		for _, f := range codeFields {
			if v, ok := m[f]; ok {
				parts = append(parts, fmt.Sprintf("%s=%v", f, v))
			}
		}
		_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(parts, " "))
	}
	if other > 0 {
		_, _ = fmt.Fprintf(w, "  (%d lines without JSON not shown)\n", other)
	}
}

// Down removes the host containers and the project with its volumes and networks, asserts that
// nothing of the project remains, removes the temporary folder and compares the files under
// infra/compose with their state before the start.
//
// Errors: every object that remains, a changed installation file, or a failed removal.
func (s *Stack) Down(ctx context.Context) error {
	var errs []error
	if s.Project != "" {
		s.mu.Lock()
		hosts := slices.Collect(maps.Values(s.hosts))
		s.mu.Unlock()
		for _, h := range hosts {
			if err := h.Remove(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if _, err := s.Compose(ctx, "down", "--volumes", "--remove-orphans", "--timeout", "30"); err != nil {
			errs = append(errs, err)
		}
		left, err := s.leftovers(ctx)
		if err != nil {
			errs = append(errs, err)
		} else if len(left) > 0 {
			errs = append(errs, fmt.Errorf("stack: %s left behind: %s", s.Project, strings.Join(left, ", ")))
		}
	}
	if s.hashes != nil {
		now, err := hashTree(filepath.Join(s.Root, "infra", "compose"))
		if err != nil {
			errs = append(errs, err)
		} else if !maps.Equal(now, s.hashes) {
			errs = append(errs, errors.New("stack: the files under infra/compose changed while the stack ran"))
		}
	}
	if s.Dir != "" {
		if err := os.RemoveAll(s.Dir); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// leftovers lists the containers, volumes and networks that still carry the project label.
func (s *Stack) leftovers(ctx context.Context) ([]string, error) {
	label := "label=com.docker.compose.project=" + s.Project
	var left []string
	for _, args := range [][]string{
		{"ps", "--all", "--filter", label, "--format", "{{.Names}}"},
		{"volume", "ls", "--filter", label, "--format", "{{.Name}}"},
		{"network", "ls", "--filter", label, "--format", "{{.Name}}"},
	} {
		out, err := s.docker.must(ctx, nil, args...)
		if err != nil {
			return nil, err
		}
		left = append(left, strings.Fields(string(out))...)
	}
	return left, nil
}

// hashTree returns the SHA-256 of every regular file under dir by its relative path.
func hashTree(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || !e.Type().IsRegular() {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = sum
		return nil
	})
	return out, err
}
