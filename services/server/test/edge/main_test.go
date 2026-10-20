// SPDX-License-Identifier: Apache-2.0

// Package edge proves the example edge and the Compose overlay for several api nodes (brief
// M2-47, design notes 11 and 12; docs/spec/16-betrieb.md 4.4): infra/compose/edge/nginx.conf in
// the pinned image nginxinc/nginx-unprivileged routes both stream paths of a session to the same
// node, carries the timeouts of the configuration defaults and runs without root; and
// infra/compose/compose.multi-node.yaml renders as the brief describes.
//
// The tests need a Docker-compatible runtime and the Docker CLI with Compose; without them the
// test binary fails, it never skips (plan P-09). Every container carries the label
// org.bothy.test=edge, publishes only on 127.0.0.1 with a random host port and is removed in
// t.Cleanup; `docker ps --filter label=org.bothy.test=edge` finds one a killed binary left behind.
package edge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
)

// The label of every container and network of this package.
const (
	label      = "org.bothy.test"
	labelValue = "edge"
)

// nginxRepository is the image of the Compose service edge; its tag is BOTHY_NGINX_VERSION of
// infra/compose/.env.example, so the tests prove the version the overlay runs.
const nginxRepository = "nginxinc/nginx-unprivileged"

// runtimeWait bounds the check for a container runtime before the tests run.
const runtimeWait = 30 * time.Second

// TestMain disables Ryuk, whose reaper would publish its port beyond 127.0.0.1, and fails the
// binary with the reason when no container runtime answers.
func TestMain(m *testing.M) { os.Exit(edgeMain(m)) }

// edgeMain is TestMain without os.Exit, so deferred work of a future change still runs.
func edgeMain(m *testing.M) int {
	// testcontainers reads its configuration once; only the exact value "true" disables Ryuk.
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "edge tests: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimeWait)
	err := runtimeReachable(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "edge tests: no Docker-compatible runtime reachable (%v); the edge tests need one (plan P-09)\n", err)
		return 1
	}
	return m.Run()
}

// runtimeReachable asks the container runtime for its info; a panic of testcontainers on a
// broken setup counts as unreachable.
func runtimeReachable(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p, err := testcontainers.NewDockerProvider(testcontainers.WithLogger(tclog.NewNoopLogger()))
	if err != nil {
		return err
	}
	return p.Health(ctx)
}

// repoRoot walks up from the package directory to the repository root, the first directory
// with a .git entry (a directory in a clone, a file in a worktree), so no path is hardcoded.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no repository root (.git) above the package directory")
		}
		dir = parent
	}
}

// readRepoFile reads a file relative to the repository root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel)) // #nosec G304 -- a fixed file of the repository
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s not found in the repository", rel)
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// envExampleValue returns the value of name in infra/compose/.env.example; a missing or repeated
// variable fails the test.
func envExampleValue(t *testing.T, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `=(.*)$`)
	m := re.FindAllStringSubmatch(readRepoFile(t, "infra/compose/.env.example"), -1)
	if len(m) != 1 {
		t.Fatalf("infra/compose/.env.example sets %s %d times, want once", name, len(m))
	}
	return strings.TrimSpace(m[0][1])
}

// nginxImage is the pinned image of the Compose service edge.
func nginxImage(t *testing.T) string {
	t.Helper()
	return nginxRepository + ":" + envExampleValue(t, "BOTHY_NGINX_VERSION")
}

// tableCell returns the cell at index col (0 is the first cell) of the only Markdown table row
// in the file rel whose first cell is the code span first; zero or several such rows fail the
// test.
func tableCell(t *testing.T, rel, first string, col int) string {
	t.Helper()
	prefix := "| `" + first + "` |"
	var rows []string
	for line := range strings.Lines(readRepoFile(t, rel)) {
		if strings.HasPrefix(line, prefix) {
			rows = append(rows, line)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("%s has %d table rows for %s, want one", rel, len(rows), first)
	}
	cells := strings.Split(strings.TrimSpace(rows[0]), "|")
	// cells[0] is the empty text before the leading pipe.
	if len(cells) < col+2 {
		t.Fatalf("%s: row of %s has no column %d", rel, first, col)
	}
	return strings.Trim(strings.TrimSpace(cells[col+1]), "`")
}

// configDefault returns the default of the configuration key from the generated table of
// docs/operations/configuration.md section 4 (columns key, variable, place, type, default).
func configDefault(t *testing.T, key string) string {
	t.Helper()
	return tableCell(t, "docs/operations/configuration.md", key, 4)
}

// configDuration returns the default of key as a duration.
func configDuration(t *testing.T, key string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(configDefault(t, key))
	if err != nil {
		t.Fatalf("default of %s: %v", key, err)
	}
	return d
}
