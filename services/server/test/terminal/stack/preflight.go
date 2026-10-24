// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// The minimum versions of M1-73 (infra/compose/smoke.sh): Engine 26.0.0 for the volume subpath,
// Compose 2.26.0 for !reset, !override and configs.
const (
	MinEngine  = "26.0.0"
	MinCompose = "2.26.0"
)

// ErrPrecondition reports a machine that cannot run the suite; the message names the fix. The
// suite fails on it and never skips (P-09, R-QA-02).
var ErrPrecondition = errors.New("terminal suite precondition")

// precondition returns an ErrPrecondition with the message.
func precondition(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPrecondition, fmt.Sprintf(format, args...))
}

// Preflight checks the Docker runtime of the machine (requirement 18): docker on PATH, docker
// buildx, Engine and Compose at their minimum versions and cgroup version 2. The ptrace_scope of
// the Docker host needs the host image and is PtraceScope.
//
// Errors: ErrPrecondition with the fix in the message.
func Preflight(ctx context.Context, d *Docker) error {
	if _, err := exec.LookPath(d.bin()); err != nil {
		return precondition("docker is not on PATH; install Docker Engine %s or newer with Compose %s or newer and docker buildx", MinEngine, MinCompose)
	}
	if r, err := d.run(ctx, nil, nil, "buildx", "version"); err != nil || r.code != 0 {
		return precondition("docker buildx is missing; install the buildx plugin of Docker (the suite builds the server image with docker buildx build --load)")
	}
	r, err := d.run(ctx, nil, nil, "version", "--format", "{{.Server.Version}}")
	if err != nil || r.code != 0 {
		return precondition("no Docker runtime is reachable (docker version failed); start Docker or set DOCKER_HOST")
	}
	engine := strings.TrimSpace(string(r.stdout))
	if !versionAtLeast(engine, MinEngine) {
		return precondition("Docker Engine %s is too old; %s or newer is needed (volume subpath); update Docker", engine, MinEngine)
	}
	r, err = d.run(ctx, nil, nil, "compose", "version", "--short")
	if err != nil || r.code != 0 {
		return precondition("docker compose is missing; install the Compose plugin %s or newer", MinCompose)
	}
	compose := strings.TrimSpace(string(r.stdout))
	if !versionAtLeast(compose, MinCompose) {
		return precondition("Docker Compose %s is too old; %s or newer is needed (!reset, !override); update Compose", compose, MinCompose)
	}
	r, err = d.run(ctx, nil, nil, "info", "--format", "{{.CgroupVersion}}")
	if err != nil || r.code != 0 {
		return precondition("docker info failed; start Docker or set DOCKER_HOST")
	}
	if v := strings.TrimSpace(string(r.stdout)); v != "2" {
		return precondition("the Docker host runs cgroup version %s; the host containers need cgroup version 2 (systemd and the session cgroups of bothy-host, 24 6.5); run Docker on a host with the unified cgroup hierarchy", v)
	}
	return nil
}

// PtraceScope reads /proc/sys/kernel/yama/ptrace_scope of the Docker host in a throwaway container
// of image without network and fails below 1: HD10 of bothy-host refuses the start there with the
// code ptrace_scope_low (M2-42), so no session could run. The suite never changes the setting.
//
// Errors: ErrPrecondition with the fix in the message.
func PtraceScope(ctx context.Context, d *Docker, image string) error {
	r, err := d.run(ctx, nil, nil, "run", "--rm", "--network", "none", "--entrypoint", "cat", image, "/proc/sys/kernel/yama/ptrace_scope")
	if err != nil || r.code != 0 {
		return precondition("cannot read kernel.yama.ptrace_scope of the Docker host (yama missing?); the host service needs ptrace_scope of at least 1 (HD10, code ptrace_scope_low)")
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(r.stdout)))
	if err != nil || v < 1 {
		return precondition("kernel.yama.ptrace_scope is %s on the Docker host; the host service refuses to start below 1 (HD10, code ptrace_scope_low); set it to at least 1 on the Docker host (sysctl kernel.yama.ptrace_scope=1)", strings.TrimSpace(string(r.stdout)))
	}
	return nil
}

// versionAtLeast reports whether version v is at least min, per numeric component (a leading v and
// a suffix after the numbers are ignored), as smoke.sh compares.
//
// Example: versionAtLeast("v2.40.3-desktop.1", "2.26.0") == true.
func versionAtLeast(v, minimum string) bool {
	a, b := numericParts(v), numericParts(minimum)
	for i := range 3 {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return true
}

// numericParts returns the first three numeric components of v.
func numericParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexFunc(v, func(r rune) bool { return (r < '0' || r > '9') && r != '.' }); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}
