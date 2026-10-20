// SPDX-License-Identifier: Apache-2.0

//go:build perf

package authorizecached_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// benchtime is how long the benchmark runs (go test -benchtime).
const benchtime = "3s"

// TestPerf_AuthorizeCachedP99 runs BenchmarkAuthorizeCached of internal/authz/app with
// -run '^$' -bench '^BenchmarkAuthorizeCached$' and compares its p99-ns metric with
// perf.authorize_cached_p99; in emulated mode the benchmark runs with -cpu set to the CPUs of the
// profile.
func TestPerf_AuthorizeCachedP99(t *testing.T) {
	env := perfkit.Setup(t)
	th := env.Threshold(t, "perf.authorize_cached_p99")
	_, file, _, _ := runtime.Caller(0)
	module := filepath.Join(filepath.Dir(file), "..", "..")
	args := []string{"test", "-count=1", "-run", "^$", "-bench", "^BenchmarkAuthorizeCached$", "-benchtime", benchtime}
	if env.GOMAXPROCS > 0 {
		args = append(args, "-cpu", strconv.Itoa(env.GOMAXPROCS))
	}
	args = append(args, "./internal/authz/app/")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute) // nosemgrep: bothy-go-test-wall-clock-wait -- the time limit of a load run, longer than testkit.Patience
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...) // #nosec G204 -- fixed arguments
	cmd.Dir = module
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	p99, iterations, err := parse(out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	env.Record(t, []report.Result{th.Result(t.Name(), p99, iterations)},
		map[string]any{"authorize_cached.params": map[string]any{"benchtime": benchtime}})
}

// parse reads the iterations and the p99-ns metric of the BenchmarkAuthorizeCached line.
func parse(out []byte) (p99 float64, iterations int, err error) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 4 || !strings.HasPrefix(f[0], "BenchmarkAuthorizeCached") {
			continue
		}
		iterations, err = strconv.Atoi(f[1])
		if err != nil {
			return 0, 0, err
		}
		for i := 3; i < len(f); i++ {
			if f[i] == "p99-ns" {
				p99, err = strconv.ParseFloat(f[i-1], 64)
				return p99, iterations, err
			}
		}
	}
	return 0, 0, errNoMetric
}

// errNoMetric reports a benchmark output without p99-ns.
var errNoMetric = errors.New("no BenchmarkAuthorizeCached line with p99-ns")
