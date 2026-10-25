// SPDX-License-Identifier: Apache-2.0

package checks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/stats"
	"github.com/OWNER/bothy/services/server/internal/status/adapters/checks"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// fakeSLI returns the totals of one metric or fails.
type fakeSLI struct {
	totals map[string][]stats.SLITotal
	err    error
	asked  []time.Time
}

func (f *fakeSLI) SLITotals(_ context.Context, metric string, from, to time.Time) ([]stats.SLITotal, error) {
	f.asked = append(f.asked, from, to)
	return f.totals[metric], f.err
}

type fakeScanner struct{ status string }

func (f fakeScanner) ScannerState() files.ScannerState { return files.ScannerState{Status: f.status} }

type fakeProbe struct{ err error }

func (f fakeProbe) ProbeStorage(context.Context) error { return f.err }
func (f fakeProbe) Probe(context.Context) error        { return f.err }

type fakeAgents struct {
	list []agents.StatusAgent
	err  error
}

func (f fakeAgents) ActiveAgents(context.Context) ([]agents.StatusAgent, error) { return f.list, f.err }

var cfg = checks.Config{Thresholds: domain.Thresholds{Degraded: 0.05, Outage: 0.5}, FanoutLag: 5 * time.Second}

// check runs the check key of src once.
func check(t *testing.T, src app.Sources, key string) domain.Outcome {
	t.Helper()
	s, ok := src.Checks[key]
	if !ok {
		t.Fatalf("no check %s", key)
	}
	return s.Check(context.Background(), app.Window{From: time.Unix(60, 0), To: time.Unix(120, 0)})
}

// TestMeasuredChecks (design note 1): api.http and push.* are the error ratio 1 - good/count of
// their dimension, sync.fanout the mean latency sum/count against status.fanout_lag_degraded; no
// traffic is no evidence, a failed read no answer; the window of the round is what they read.
func TestMeasuredChecks(t *testing.T) {
	t.Parallel()
	sli := &fakeSLI{totals: map[string][]stats.SLITotal{
		"sli.api_availability": {{Count: 100, Good: 40}},
		"sli.push_acceptance":  {{Dimension: "apns", Count: 100, Good: 90}, {Dimension: "fcm", Count: 10, Good: 10}},
		"sli.delivery_latency": {{Count: 4, Sum: 24000, Good: 0}},
	}}
	src := checks.Sources(cfg, checks.Deps{SLI: sli})
	for key, want := range map[string]domain.Outcome{
		domain.SourceAPIHTTP: domain.Outage, domain.SourcePushAPNs: domain.Fail, domain.SourcePushFCM: domain.Pass,
		domain.SourceSyncFanout: domain.Fail,
	} {
		if got := check(t, src, key); got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if !sli.asked[0].Equal(time.Unix(60, 0)) || !sli.asked[1].Equal(time.Unix(120, 0)) {
		t.Errorf("window read %v", sli.asked[:2])
	}
	quiet := checks.Sources(cfg, checks.Deps{SLI: &fakeSLI{totals: map[string][]stats.SLITotal{
		"sli.delivery_latency": {{Count: 4, Sum: 4000}},
	}}})
	for key, want := range map[string]domain.Outcome{
		domain.SourceAPIHTTP: domain.NoEvidence, domain.SourceSyncFanout: domain.Pass, domain.SourcePushAPNs: domain.NoEvidence,
	} {
		if got := check(t, quiet, key); got != want {
			t.Errorf("quiet %s = %v, want %v", key, got, want)
		}
	}
	broken := checks.Sources(cfg, checks.Deps{SLI: &fakeSLI{err: errors.New("down")}})
	for _, key := range []string{domain.SourceAPIHTTP, domain.SourceSyncFanout, domain.SourcePushFCM} {
		if got := check(t, broken, key); got != domain.NoAnswer {
			t.Errorf("failed read %s = %v", key, got)
		}
	}
	if got := check(t, checks.Sources(cfg, checks.Deps{SLI: &fakeSLI{}}), domain.SourceSyncFanout); got != domain.NoEvidence {
		t.Errorf("no delivery = %v", got)
	}
}

// TestProbedChecks: files.storage and search.query pass or give no answer; files.scan follows
// the last PING (off and not checked yet are no evidence); a missing port leaves its check out;
// the agents keep their presence.
func TestProbedChecks(t *testing.T) {
	t.Parallel()
	ok := checks.Sources(cfg, checks.Deps{Storage: fakeProbe{}, Search: fakeProbe{}, Scanner: fakeScanner{files.ScannerOK}})
	bad := checks.Sources(cfg, checks.Deps{Storage: fakeProbe{errors.New("x")}, Search: fakeProbe{errors.New("x")}, Scanner: fakeScanner{files.ScannerUnreachable}})
	for _, key := range []string{domain.SourceFilesStorage, domain.SourceSearchQuery, domain.SourceFilesScan} {
		if got := check(t, ok, key); got != domain.Pass {
			t.Errorf("%s = %v", key, got)
		}
		if got := check(t, bad, key); got != domain.NoAnswer {
			t.Errorf("failing %s = %v", key, got)
		}
	}
	for _, st := range []string{files.ScannerOff, files.ScannerUnknown} {
		if got := check(t, checks.Sources(cfg, checks.Deps{Scanner: fakeScanner{st}}), domain.SourceFilesScan); got != domain.NoEvidence {
			t.Errorf("scanner %s = %v", st, got)
		}
	}
	none := checks.Sources(cfg, checks.Deps{})
	if len(none.Checks) != 0 || none.Agents != nil {
		t.Errorf("checks without ports: %v", none)
	}
	id := ids.New()
	src := checks.Sources(cfg, checks.Deps{Agents: fakeAgents{list: []agents.StatusAgent{{ID: id, Handle: "helper", DisplayName: "Helper", State: agents.StateOnline}}}})
	got, err := src.Agents.ActiveAgents(context.Background())
	if err != nil || len(got) != 1 || got[0].ID != id || got[0].State != app.AgentOnline || got[0].Handle != "helper" {
		t.Errorf("agents %+v %v", got, err)
	}
	if _, err := checks.Sources(cfg, checks.Deps{Agents: fakeAgents{err: errors.New("x")}}).Agents.ActiveAgents(context.Background()); err == nil {
		t.Error("the error of the agents was lost")
	}
}
