// SPDX-License-Identifier: Apache-2.0

//go:build perf

package startup_test

import (
	"os"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/perfkit"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// The parameters of the measurement (perf/README.md).
const (
	// starts is how many starts of bothy serve are measured.
	starts = 5
	// idleSettle is the idle time after ready before the RSS reading.
	idleSettle = 10 * time.Second
)

// TestPerf_StartupReadyAndIdleRSS starts bothy serve --role=all repeatedly on a migrated database
// and measures the time from exec to the first 200 of /readyz (perf.startup_ready_max) and the RSS
// after an idle settle period (perf.idle_rss_max); the maximum over the starts is compared.
func TestPerf_StartupReadyAndIdleRSS(t *testing.T) {
	env := perfkit.Setup(t)
	thReady := env.Threshold(t, "perf.startup_ready_max")
	thRSS := env.Threshold(t, "perf.idle_rss_max")
	p := harness.StartProcess(t, harness.ProcessOptions{GOMAXPROCS: env.GOMAXPROCS, DBCPUs: env.DBCPUs, DBMemoryBytes: env.DBMemoryBytes})
	env.ReadDurability(t, p.AdminDB())
	var ready []time.Duration
	var rss []float64
	for i := range starts {
		if i > 0 {
			if err := p.Restart(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		ready = append(ready, p.ReadyAfter())
		time.Sleep(idleSettle)
		b, err := p.RSSBytes()
		if err != nil {
			t.Fatal(err)
		}
		rss = append(rss, float64(b))
	}
	maxReady, err := report.DurationsP(ready, 100)
	if err != nil {
		t.Fatal(err)
	}
	maxRSS, err := report.NearestRank(rss, 100)
	if err != nil {
		t.Fatal(err)
	}
	env.Record(t, []report.Result{thReady.Result(t.Name(), maxReady, starts), thRSS.Result(t.Name(), maxRSS, starts)},
		map[string]any{"startup.params": map[string]any{"starts": starts, "idle_settle_s": idleSettle.Seconds()}})
}
