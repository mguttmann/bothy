// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The paths of the host containers (Design note 5).
const (
	// HostBinDir holds the binaries the suite copies in (owned by root, mode 0755).
	HostBinDir = "/opt/termsuite/bin"
	// HostConfig is the configuration file of bothy-host on Linux (M2-42).
	HostConfig = "/etc/bothy-host/config.yaml"
	// HostStateDir is host_service.state_dir on Linux (M2-42).
	HostStateDir = "/var/lib/bothy-host"
	// MarkerDir collects the names of the marker programs that ran (mode 1777).
	MarkerDir = "/var/tmp/termsuite-markers"
	// FakeClaude is the path of M2-42's Fake-claude in a host container.
	FakeClaude = HostBinDir + "/claude"
	// ClaudeStartLog is the start log of the Fake-claude (one line per start: the path of its
	// program, and for an interactive start the working directory and CLAUDE_CONFIG_DIR; never a
	// key, only its SHA-256), writable by every user of the container (TM-T29).
	ClaudeStartLog = "/var/tmp/termsuite-claude-starts.log"
)

// Binaries are the programs the suite builds once per test binary for the host containers.
type Binaries struct {
	// HostService is the binary of the host service for linux/<arch> of the Docker host.
	HostService string
	// FakeClaude is M2-42's Fake-claude (services/host/testdata/fakeclaude) for the same platform.
	FakeClaude string
}

// BuildHostBinaries builds bothy-host and the Fake-claude in services/host for linux/arch into dir
// (GOTOOLCHAIN=local, CGO_ENABLED=0), as M2-42's TestHostPairAgainstServer builds bothy-host.
// bothy-host carries the mark of the platform suite (buildinfo.PlatformSuite=1, as make
// platform-test sets it): the host containers are disposable test machines with the marker file,
// and the mark makes HD10 measure the peer check of the local channel instead of assuming it
// (docs/DECISIONS.md D-M2-42-11 and D-M2-48-5).
//
// Errors: a failed build with its output.
func BuildHostBinaries(ctx context.Context, root, dir, arch string) (Binaries, error) {
	b := Binaries{HostService: filepath.Join(dir, "bothy-host"), FakeClaude: filepath.Join(dir, "claude")}
	builds := [][]string{
		{"build", "-trimpath", "-ldflags", "-X github.com/OWNER/bothy/services/host/internal/buildinfo.PlatformSuite=1", "-o", b.HostService, "./cmd/bothy-host"},
		{"build", "-trimpath", "-ldflags", "-X main.startLog=" + ClaudeStartLog, "-o", b.FakeClaude, "./testdata/fakeclaude"},
	}
	for _, args := range builds {
		if err := goBuild(ctx, filepath.Join(root, "services", "host"), []string{"GOOS=linux", "GOARCH=" + arch}, args...); err != nil {
			return b, err
		}
	}
	return b, nil
}

// goBuild runs go with args in dir with CGO_ENABLED=0 and GOTOOLCHAIN=local plus env.
func goBuild(ctx context.Context, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...) //nolint:gosec // G204: fixed program, arguments of the suite
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local"), env...) //nolint:forbidigo // go needs PATH, HOME and the caches of the machine
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stack: go %s in %s: %w\n%s", strings.Join(args, " "), filepath.Base(dir), err, out)
	}
	return nil
}

// Host is one host container: a machine with systemd on the network termhosts.
type Host struct {
	// Name is the host name of the scenario (r1, r2, ...), Container the container name.
	Name, Container string

	s *Stack
}

// ExecResult is the outcome of a command in a host container.
type ExecResult struct {
	// Stdout and Stderr are the output streams; Code is the exit code.
	Stdout, Stderr []byte
	Code           int
}

// containerName returns a Docker container name of the parts: lower case, every character other
// than [a-z0-9.-] replaced by a hyphen.
var nameRE = regexp.MustCompile(`[^a-z0-9.-]+`)

// NewHost starts a host container named <project>-<scope>-<name> (compose run --detach of the
// service termhost) and waits until systemd runs. scope is the case of the scenario.
//
// Errors: a failed start or a systemd that does not come up within ctx.
func (s *Stack) NewHost(ctx context.Context, scope, name string) (*Host, error) {
	container := nameRE.ReplaceAllString(strings.ToLower(s.Project+"-"+scope+"-"+name), "-")
	h := &Host{Name: name, Container: container, s: s}
	s.mu.Lock()
	s.hosts[container] = h
	s.mu.Unlock()
	if _, err := s.Compose(ctx, "run", "--detach", "--no-deps", "--name", container, "termhost"); err != nil {
		return h, err
	}
	return h, h.WaitBoot(ctx)
}

// WaitBoot polls systemctl is-system-running until systemd reports running or degraded.
func (h *Host) WaitBoot(ctx context.Context) error {
	for {
		r, err := h.Exec(ctx, nil, "systemctl", "is-system-running")
		if err == nil {
			switch strings.TrimSpace(string(r.Stdout)) {
			case "running", "degraded":
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("stack: systemd of %s did not start: %w", h.Container, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Exec runs args as root in the container with stdin from in (nil for none).
func (h *Host) Exec(ctx context.Context, in io.Reader, args ...string) (ExecResult, error) {
	base := []string{"exec"}
	if in != nil {
		base = append(base, "-i")
	}
	r, err := h.s.docker.run(ctx, in, nil, append(append(base, h.Container), args...)...)
	return ExecResult{Stdout: r.stdout, Stderr: r.stderr, Code: r.code}, err
}

// ExecAs runs args as user in the container (docker exec --user, with the home of the user as
// working directory and HOME set).
func (h *Host) ExecAs(ctx context.Context, user string, in io.Reader, args ...string) (ExecResult, error) {
	base := []string{"exec", "--user", user, "--env", "HOME=/home/" + user, "--workdir", "/home/" + user}
	if in != nil {
		base = append(base, "-i")
	}
	r, err := h.s.docker.run(ctx, in, nil, append(append(base, h.Container), args...)...)
	return ExecResult{Stdout: r.stdout, Stderr: r.stderr, Code: r.code}, err
}

// Shell runs script with bash -c as root and fails unless it exits 0.
func (h *Host) Shell(ctx context.Context, script string) (ExecResult, error) {
	r, err := h.Exec(ctx, nil, "bash", "-c", script)
	if err != nil {
		return r, err
	}
	if r.Code != 0 {
		return r, fmt.Errorf("stack: a root command in %s exited %d: %s", h.Name, r.Code, lastLine(r.Stderr))
	}
	return r, nil
}

// CopyIn copies the file src of the machine of the tests to dst in the container (docker cp).
func (h *Host) CopyIn(ctx context.Context, src, dst string) error {
	_, err := h.s.docker.must(ctx, nil, "cp", src, h.Container+":"+dst)
	return err
}

// Provision copies bothy-host and the Fake-claude into HostBinDir (owner root, mode 0755) and adds
// the test CA to the system trust store, so pair and the control client trust the TLS front.
func (h *Host) Provision(ctx context.Context, bins Binaries) error {
	if _, err := h.Shell(ctx, "install -d -o root -g root -m 0755 "+HostBinDir); err != nil {
		return err
	}
	for src, name := range map[string]string{bins.HostService: "bothy-host", bins.FakeClaude: "claude"} {
		if err := h.CopyIn(ctx, src, HostBinDir+"/"+name); err != nil {
			return err
		}
	}
	if _, err := h.Shell(ctx, "chown root:root "+HostBinDir+"/* && chmod 0755 "+HostBinDir+"/* && install -m 0666 /dev/null "+ClaudeStartLog); err != nil {
		return err
	}
	if _, err := h.Exec(ctx, bytes.NewReader(h.s.CA.PEM()), "bash", "-c", "cat > /usr/local/share/ca-certificates/termsuite-test-ca.crt"); err != nil {
		return err
	}
	_, err := h.Shell(ctx, "update-ca-certificates >/dev/null")
	return err
}

// Service runs bothy-host with args as root (the operator) and returns the result; a code other
// than 0 is no error here.
func (h *Host) Service(ctx context.Context, args ...string) (ExecResult, error) {
	bin := "/opt/bothy-host/bin/bothy-host"
	if len(args) > 0 && args[0] == "install" {
		bin = HostBinDir + "/bothy-host"
	}
	return h.Exec(ctx, nil, append([]string{bin}, args...)...)
}

// Install runs bothy-host install with the first session user and session-user add for the
// others (16 4.4 and M2-42's README).
//
// Errors: a command that exits other than 0, with its last line of stderr.
func (h *Host) Install(ctx context.Context, sessionUsers []string) error {
	if len(sessionUsers) == 0 {
		return errors.New("stack: a host needs at least one session user")
	}
	if r, err := h.Service(ctx, "install", "--session-user", sessionUsers[0]); err != nil || r.Code != 0 {
		return fmt.Errorf("stack: install on %s: %d %s: %w", h.Name, r.Code, lastLine(r.Stderr), errOr(err))
	}
	for _, u := range sessionUsers[1:] {
		if r, err := h.Service(ctx, "session-user", "add", u); err != nil || r.Code != 0 {
			return fmt.Errorf("stack: session-user add on %s: %d %s: %w", h.Name, r.Code, lastLine(r.Stderr), errOr(err))
		}
	}
	return nil
}

// errExit marks a command that ran but exited with a code other than 0.
var errExit = errors.New("exit code other than 0")

// errOr returns err, or errExit when err is nil.
func errOr(err error) error {
	if err != nil {
		return err
	}
	return errExit
}

// WriteConfig replaces the configuration file with content (YAML of host_service keys) and
// restarts the host service with bothy-host restart.
func (h *Host) WriteConfig(ctx context.Context, content []byte) error {
	if _, err := h.Exec(ctx, bytes.NewReader(content), "bash", "-c", "cat > "+HostConfig+".new && chmod 0644 "+HostConfig+".new && mv "+HostConfig+".new "+HostConfig); err != nil {
		return err
	}
	r, err := h.Service(ctx, "restart")
	if err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("stack: restart on %s exited %d: %s", h.Name, r.Code, lastLine(r.Stderr))
	}
	return nil
}

// ReadConfig returns the configuration file of the host.
func (h *Host) ReadConfig(ctx context.Context) ([]byte, error) {
	r, err := h.Exec(ctx, nil, "cat", HostConfig)
	if err != nil {
		return nil, err
	}
	if r.Code != 0 {
		return nil, fmt.Errorf("stack: no configuration on %s", h.Name)
	}
	return r.Stdout, nil
}

// fingerprintRE finds the fingerprint line of bothy-host pair.
var fingerprintRE = regexp.MustCompile(`(?m)^fingerprint: (\S+)$`)

// Pair runs bothy-host pair against the instance origin with code, passed as an argument of docker
// exec only, never written to the container's disk or the test output. It returns the fingerprint
// (empty when pair failed) and the exit code.
func (h *Host) Pair(ctx context.Context, code string) (string, int, error) {
	r, err := h.PairResult(ctx, code)
	if err != nil {
		return "", 0, err
	}
	return Fingerprint(r), r.Code, nil
}

// PairResult runs bothy-host pair like Pair and returns the whole result of the command.
func (h *Host) PairResult(ctx context.Context, code string) (ExecResult, error) {
	return h.Service(ctx, "pair", "--server", InstanceOrigin, "--code", code)
}

// Fingerprint returns the fingerprint of the output of a successful bothy-host pair, or "".
func Fingerprint(r ExecResult) string {
	m := fingerprintRE.FindSubmatch(r.Stdout)
	if r.Code != 0 || m == nil {
		return ""
	}
	return string(m[1])
}

// Doctor runs bothy-host doctor and returns its output and exit code.
func (h *Host) Doctor(ctx context.Context) (ExecResult, error) { return h.Service(ctx, "doctor") }

// Journal returns the journal of the bothy-host units (messages only).
func (h *Host) Journal(ctx context.Context) ([]byte, error) {
	r, err := h.Exec(ctx, nil, "journalctl", "--no-pager", "-o", "cat", "-u", "bothy-host*")
	return r.Stdout, err
}

// Reboot restarts the container (docker restart): the machine restarts with its file system.
func (h *Host) Reboot(ctx context.Context) error {
	if _, err := h.s.docker.must(ctx, nil, "restart", "--time", "30", h.Container); err != nil {
		return err
	}
	return h.WaitBoot(ctx)
}

// Pause and Unpause freeze and thaw every process of the container.
func (h *Host) Pause(ctx context.Context) error {
	_, err := h.s.docker.must(ctx, nil, "pause", h.Container)
	return err
}

// Unpause thaws the processes of the container.
func (h *Host) Unpause(ctx context.Context) error {
	_, err := h.s.docker.must(ctx, nil, "unpause", h.Container)
	return err
}

// NetDown disconnects the container from termhosts, so every connection of the host service is
// lost without a close; NetUp connects it again.
func (h *Host) NetDown(ctx context.Context) error {
	_, err := h.s.docker.must(ctx, nil, "network", "disconnect", "--force", h.s.Project+"_termhosts", h.Container)
	return err
}

// NetUp connects the container to termhosts again.
func (h *Host) NetUp(ctx context.Context) error {
	_, err := h.s.docker.must(ctx, nil, "network", "connect", h.s.Project+"_termhosts", h.Container)
	return err
}

// CloneState copies the state directory and the configuration of h as a tar stream into to (a
// container with the same session users, its host service stopped); the step that starts it is
// the caller's (TM-T23).
func (h *Host) CloneState(ctx context.Context, to *Host) error {
	r, err := h.Exec(ctx, nil, "tar", "-C", "/", "-cf", "-", strings.TrimPrefix(HostStateDir, "/"), strings.TrimPrefix(HostConfig, "/"))
	if err != nil {
		return err
	}
	if r.Code != 0 {
		return fmt.Errorf("stack: tar of the state of %s exited %d", h.Name, r.Code)
	}
	w, err := to.Exec(ctx, bytes.NewReader(r.Stdout), "tar", "-C", "/", "-xpf", "-")
	if err != nil {
		return err
	}
	if w.Code != 0 {
		return fmt.Errorf("stack: untar of the state into %s exited %d", to.Name, w.Code)
	}
	return nil
}

// Inspect returns a field of docker inspect of the container in Go template form.
func (h *Host) Inspect(ctx context.Context, format string) (string, error) {
	out, err := h.s.docker.must(ctx, nil, "inspect", "--format", format, h.Container)
	return strings.TrimSpace(string(out)), err
}

// Remove removes the container with its anonymous volumes.
func (h *Host) Remove(ctx context.Context) error {
	h.s.mu.Lock()
	delete(h.s.hosts, h.Container)
	h.s.mu.Unlock()
	r, err := h.s.docker.run(ctx, nil, nil, "rm", "--force", "--volumes", h.Container)
	if err != nil {
		return err
	}
	if r.code != 0 && !bytes.Contains(r.stderr, []byte("No such container")) {
		return fmt.Errorf("stack: rm %s exited %d: %s", h.Container, r.code, lastLine(r.stderr))
	}
	return nil
}
