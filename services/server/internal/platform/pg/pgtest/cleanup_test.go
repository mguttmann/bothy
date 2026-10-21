// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"

	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// childMode makes TestPgtestChild act: the test binary runs itself as a child with the flag set.
// childTempDir is the temporary directory of the child (its secrets directory goes there).
// Flags, not environment variables: only cmd/bothy reads the environment.
var (
	childMode    = flag.String("pgtest.child", "", "internal: the mode of the child of TestNoContainerSurvivesTheRun")
	childTempDir = flag.String("pgtest.child-tmpdir", "", "internal: TMPDIR of the child of TestNoContainerSurvivesTheRun")
	childRun     = flag.String("pgtest.child-run", "", "internal: the run label value of the child of TestNoContainerSurvivesTheRun")
)

// childContainerPrefix starts the line on which the child reports its container.
const childContainerPrefix = "pgtest-child-container="

// survivalDeadline bounds how long a container may outlive its test binary: the watchdog
// notices the lost lease within leaseMisses seconds, then PostgreSQL shuts down and Docker
// removes the container.
const survivalDeadline = 2 * time.Minute

// liveWindow is how long a hanging child runs before the parent checks that its container is
// still there: longer than the leaseMisses seconds after which a lost lease stops it.
const liveWindow = (leaseMisses + 5) * time.Second

// childTimeout is the -test.timeout of the timed-out child: its container may take up to
// startTimeout to start, and it must then run for liveWindow before the alarm ends it.
const childTimeout = startTimeout + liveWindow + 15*time.Second

// TestPgtestChild is the child of TestNoContainerSurvivesTheRun. Without -pgtest.child it does
// nothing; with it, it starts the container, reports its ID, waits until the parent closes its
// standard input (the parent has inspected the container by then) and then passes, fails,
// panics or hangs until the parent kills it or the test timeout ends it.
func TestPgtestChild(t *testing.T) {
	mode := *childMode
	if mode == "" {
		return // only meaningful as the child of TestNoContainerSurvivesTheRun
	}
	if *childTempDir != "" {
		t.Setenv("TMPDIR", *childTempDir)
	}
	runLabelValue = *childRun
	db := New(t, WithoutMigrations())
	_, _ = fmt.Fprintf(os.Stdout, "%s%s\n", childContainerPrefix, db.c.ctr.GetContainerID())
	_, _ = io.Copy(io.Discard, os.Stdin)
	switch mode {
	case "pass":
	case "fail":
		t.Fatal("deliberate failure of the child")
	case "panic":
		panic("deliberate panic of the child")
	case "hang":
		time.Sleep(10 * time.Minute)
	default:
		t.Fatalf("unknown -pgtest.child %q", mode)
	}
}

// TestNoContainerSurvivesTheRun runs this test binary as a child that starts its container and
// then ends in every way a run can end: passing, with a failed test, with a panic, by the
// -test.timeout alarm and by SIGKILL (which no code in the process can intercept, so it stands
// for every signal). After each, the container and its anonymous data volume must be gone.
func TestNoContainerSurvivesTheRun(t *testing.T) {
	testkit.Integration(t)
	if *childMode != "" {
		return // a child runs only TestPgtestChild
	}
	t.Parallel()
	ctx := context.Background()
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	cases := []struct {
		name string
		mode string
		args []string
		kill bool
	}{
		{name: "passing run", mode: "pass"},
		{name: "failing test", mode: "fail"},
		{name: "panicking test", mode: "panic"},
		// Long enough for the slowest container start plus liveWindow, short enough for make check.
		{name: "test timeout", mode: "hang", args: []string{"-test.timeout=" + childTimeout.String()}},
		{name: "killed run", mode: "hang", kill: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// A bound for every child, so none outlives a parent that dies without its cleanup;
			// a later -test.timeout in tc.args wins.
			// The child's TMPDIR is a temporary directory of the parent's test: a child that never
			// reaches the end of Main leaves its secrets directory there, and the parent's cleanup
			// removes it.
			run := randomHex(8)
			args := append([]string{
				"-test.run=^TestPgtestChild$", "-test.count=1", "-test.timeout=5m",
				"-pgtest.child=" + tc.mode, "-pgtest.child-tmpdir=" + t.TempDir(), "-pgtest.child-run=" + run,
			}, tc.args...)
			cmd := exec.CommandContext(ctx, os.Args[0], args...) // #nosec G204 G702 -- this test binary
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			report := readChild(stdout)
			waited := false
			t.Cleanup(func() { // a parent that stops early must not leave its child running
				if !waited {
					_ = cmd.Process.Kill()
					<-report.done
					_ = cmd.Wait()
				}
				// Whatever the child left of its run goes, also when it never reported.
				removeRun(ctx, t, cli.Client, run)
			})
			// The child reports its container once New returned, which takes longer on a loaded
			// host; a child that ends first, or hangs past Patience, fails with all its output.
			id, reported := "", false
			select {
			case id = <-report.id:
				reported = true
			case <-report.done:
				select { // the report line may come right before the end of the output
				case id = <-report.id:
					reported = true
				default:
				}
			case <-time.After(testkit.Patience):
			}
			if !reported {
				_ = cmd.Process.Kill()
				<-report.done
				_ = cmd.Wait() // stderr is complete only after Wait
				waited = true
				t.Fatalf("the child reported no container within %s; stdout:\n%s\nstderr:\n%s", testkit.Patience, report.output(), stderr.String())
			}
			if !runContainers(ctx, t, cli.Client, run)[id] {
				t.Fatalf("the reported container %s does not carry the run label %s=%s", id, runLabel, run)
			}
			volumes := anonymousVolumes(ctx, t, cli.Client, id)
			_ = stdin.Close() // lets the child go on
			if tc.mode == "hang" {
				// The container of a live run must outlast the watchdog's verdict window.
				time.Sleep(liveWindow)
				if gone(ctx, t, cli.Client, id, 0) {
					t.Fatalf("container %s of a live run vanished within %s (%s)", id, liveWindow, tc.name)
				}
			}
			if tc.kill {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			}
			<-report.done // the child's output up to its exit; it never blocks on a full pipe
			waitErr := cmd.Wait()
			waited = true
			if tc.mode == "pass" && waitErr != nil {
				t.Errorf("the passing child exited with %v; stdout:\n%s\nstderr:\n%s", waitErr, report.output(), stderr.String())
			}
			if tc.mode != "pass" && waitErr == nil {
				t.Errorf("the %s child exited with 0", tc.mode)
			}
			if !gone(ctx, t, cli.Client, id, survivalDeadline) {
				_, _ = cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
				t.Fatalf("container %s still exists %s after its test binary ended (%s)", id, survivalDeadline, tc.name)
			}
			// Docker removes the volumes right after the container, so poll for them as well.
			for _, v := range volumes {
				if !volumeGone(ctx, t, cli.Client, v, testkit.Patience) {
					t.Errorf("volume %s of container %s still exists (%s)", v, id, tc.name)
				}
			}
		})
	}
}

// childReport is the standard output of a child: every line in out, the container ID of the
// report line on id, and done closed at the end of the output.
type childReport struct {
	mu   sync.Mutex
	out  strings.Builder
	id   chan string
	done chan struct{}
}

// readChild reads the standard output of a child until it ends.
func readChild(stdout io.Reader) *childReport {
	r := &childReport{id: make(chan string, 1), done: make(chan struct{})}
	go func() {
		defer close(r.done)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Text()
			r.mu.Lock()
			r.out.WriteString(line + "\n")
			r.mu.Unlock()
			if rest, ok := strings.CutPrefix(line, childContainerPrefix); ok {
				select {
				case r.id <- rest:
				default:
				}
			}
		}
		// The rest of a line longer than the scanner's buffer is not needed; drain it, so the
		// child never blocks on a full pipe.
		_, _ = io.Copy(io.Discard, stdout)
	}()
	return r
}

// output returns the standard output of the child so far.
func (r *childReport) output() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out.String()
}

// runContainers returns the IDs of the containers that carry runLabel with the value run.
func runContainers(ctx context.Context, t *testing.T, cli *client.Client, run string) map[string]bool {
	t.Helper()
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{
		All: true, Filters: client.Filters{}.Add("label", runLabel+"="+run),
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range res.Items {
		ids[c.ID] = true
	}
	return ids
}

// removeRun removes, by ID, every container that carries runLabel with the value run, with its
// anonymous volumes.
func removeRun(ctx context.Context, t *testing.T, cli *client.Client, run string) {
	t.Helper()
	for id := range runContainers(ctx, t, cli, run) {
		_, _ = cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	}
}

// anonymousVolumes returns the names of the anonymous volumes of container id.
func anonymousVolumes(ctx context.Context, t *testing.T, cli *client.Client, id string) []string {
	t.Helper()
	res, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range res.Container.Mounts {
		if m.Type == mount.TypeVolume {
			names = append(names, m.Name)
		}
	}
	return names
}

// volumeGone polls until volume name no longer exists, at most for limit.
func volumeGone(ctx context.Context, t *testing.T, cli *client.Client, name string, limit time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(limit) // nosemgrep: bothy-go-test-wall-clock-wait -- the bound under test (survivalDeadline), testkit.Patience or a single probe (0)
	for {
		res, err := cli.VolumeList(ctx, client.VolumeListOptions{Filters: client.Filters{}.Add("name", name)})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, v := range res.Items {
			found = found || v.Name == name
		}
		if !found {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Second)
	}
}

// gone polls until container id no longer exists, at most for limit.
func gone(ctx context.Context, t *testing.T, cli *client.Client, id string, limit time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(limit) // nosemgrep: bothy-go-test-wall-clock-wait -- the bound under test (survivalDeadline), testkit.Patience or a single probe (0)
	for {
		res, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: client.Filters{}.Add("id", id)})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Items) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Second)
	}
}

// TestReapLegacyRemovesOnlyOldUnwatchedContainers starts idle containers under a label key of its
// own: two without the watchdog label (one with the value pgtest, one with seaweedfs, as older
// pgtest versions and the SeaweedFS helper of the files tests leave them) and one with it. It
// checks that reapLegacy removes the unwatched ones whatever their value, and only once they are
// older than legacyReapAge.
func TestReapLegacyRemovesOnlyOldUnwatchedContainers(t *testing.T) {
	testkit.Integration(t)
	if *childMode != "" {
		return
	}
	t.Parallel()
	ctx := context.Background()
	key := label + ".reap-" + randomHex(6)
	start := func(labels map[string]string) string {
		// testcontainers publishes the port the image exposes; it goes on 127.0.0.1 like the
		// port of pgtest, and runRetrying starts again when the host port Docker picked is taken.
		ctr, err := runRetrying(ctx, func(ctx context.Context) (*testcontainers.DockerContainer, error) {
			return testcontainers.Run(ctx, Image,
				testcontainers.WithName("bothy-pgtest-reap-"+randomHex(6)),
				testcontainers.WithLabels(labels),
				// A bounded sleep and AutoRemove: should this test die before its cleanup, the
				// container still goes away on its own, as it has no watchdog.
				testcontainers.WithEntrypoint("sleep", "600"),
				testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
					hc.AutoRemove = true
					hc.PortBindings = network.PortMap{network.MustParsePort(containerPort): {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
				}),
			)
		})
		if ctr != nil {
			t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
		}
		if err != nil {
			t.Fatal(err)
		}
		return ctr.GetContainerID()
	}
	unwatched := []string{start(map[string]string{key: labelValue}), start(map[string]string{key: "seaweedfs"})}
	watched := start(map[string]string{key: labelValue, watchdogLabel: watchdogLabelValue})
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	exists := func(id string) bool { return !gone(ctx, t, cli.Client, id, 0) }

	if err := reapLegacy(ctx, key, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, id := range append([]string{watched}, unwatched...) {
		if !exists(id) {
			t.Fatalf("reapLegacy removed container %s younger than legacyReapAge", id)
		}
	}
	if err := reapLegacy(ctx, key, time.Now().Add(legacyReapAge+time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, id := range unwatched {
		if exists(id) {
			t.Errorf("reapLegacy kept unwatched container %s older than legacyReapAge", id)
		}
	}
	if !exists(watched) {
		t.Error("reapLegacy removed a container with the watchdog label")
	}
}
