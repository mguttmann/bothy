// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HostDockerfile is the host image of Design note 5, relative to the repository root.
const HostDockerfile = "infra/compose/test/host/Dockerfile"

// ErrNoRepo reports a working directory outside the repository.
var ErrNoRepo = errors.New("stack: the repository root (infra/compose/compose.yaml) is not above the working directory")

// RepoRoot returns the repository root: the nearest folder above the working directory that holds
// infra/compose/compose.yaml.
//
// Errors: ErrNoRepo.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "infra", "compose", "compose.yaml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoRepo
		}
		dir = parent
	}
}

// Arch returns the architecture of the Docker host as GOARCH names it (docker version
// --format {{.Server.Arch}}: amd64 or arm64).
func Arch(ctx context.Context, d *Docker) (string, error) {
	out, err := d.must(ctx, nil, "version", "--format", "{{.Server.Arch}}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ServerImage builds the server image from the worktree root with services/server/Dockerfile for
// the platform of the Docker host, with the arguments of M1-23's image target for one platform
// (VERSION from services/server/VERSION, REVISION from git rev-parse HEAD), and tags it
// termsuite-server:<first 12 hex of the image ID>. An unchanged tree is a cache hit of BuildKit;
// parallel test packages arrive at the same content-addressed tag, so no lock is needed. tmp
// takes the iidfile.
//
// Errors: a failed build (with the last line of its output) or a missing VERSION or git.
func ServerImage(ctx context.Context, d *Docker, root, tmp string) (string, error) {
	arch, err := Arch(ctx, d)
	if err != nil {
		return "", err
	}
	version, err := os.ReadFile(filepath.Join(root, "services", "server", "VERSION")) //nolint:gosec // G304: a fixed file of the repository
	if err != nil {
		return "", err
	}
	head, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output() //nolint:gosec // G204: fixed program, the root of the repository
	if err != nil {
		return "", fmt.Errorf("stack: git rev-parse HEAD: %w", err)
	}
	iid := filepath.Join(tmp, "server.iid")
	build := *d
	build.Dir = root
	if _, err := build.must(ctx, nil, "buildx", "build", "--load", "--platform", "linux/"+arch,
		"--file", "services/server/Dockerfile",
		"--build-arg", "VERSION="+strings.TrimSpace(string(version)),
		"--build-arg", "REVISION="+strings.TrimSpace(string(head)),
		"--iidfile", iid, "."); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(iid) //nolint:gosec // G304: the iidfile of the build in the temp dir
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(raw))
	hexID := strings.TrimPrefix(id, "sha256:")
	if len(hexID) < 12 {
		return "", fmt.Errorf("stack: the build wrote no image ID (%d bytes)", len(raw))
	}
	tag := "termsuite-server:" + hexID[:12]
	if _, err := d.must(ctx, nil, "tag", id, tag); err != nil {
		return "", err
	}
	return tag, nil
}

// HostImage returns the tag termsuite-host:<first 12 hex of the SHA-256 of the host
// Dockerfile> and builds the image with BuildKit when the tag is missing.
//
// Errors: a failed build.
func HostImage(ctx context.Context, d *Docker, root string) (string, error) {
	file := filepath.Join(root, HostDockerfile)
	sum, err := fileSHA256(file)
	if err != nil {
		return "", err
	}
	tag := "termsuite-host:" + sum[:12]
	if r, err := d.run(ctx, nil, nil, "image", "inspect", "--format", "{{.Id}}", tag); err == nil && r.code == 0 {
		return tag, nil
	}
	if _, err := d.must(ctx, nil, "buildx", "build", "--load", "--tag", tag, "--file", file, filepath.Dir(file)); err != nil {
		return "", err
	}
	return tag, nil
}

// fileSHA256 returns the lowercase hex SHA-256 of the file at path.
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // G304: files of the repository or the temp dir
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}
