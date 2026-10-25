// SPDX-License-Identifier: Apache-2.0

package measure_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
)

// metricCheck finds the list of the CHECK on stats_minutes.metric in the migration.
var metricCheck = regexp.MustCompile(`CHECK \(metric IN \(([^)]*)\)\)`)

// TestClosedMetricsAndDimensions holds the 18 metrics equal to the CHECK of the migration of
// stats_minutes (03 4.15) and every dimension list and scope equal to the table of
// docs/spec/25-visualization.md 3.2, reproduced here row by row.
func TestClosedMetricsAndDimensions(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*_stats_minutes.sql"))
	if err != nil || len(files) != 1 {
		t.Fatalf("migration of stats_minutes: %v, %v", files, err)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	m := metricCheck.FindSubmatch(b)
	if m == nil {
		t.Fatal("no CHECK (metric IN (...)) in the migration")
	}
	var check []measure.Metric
	for _, v := range strings.Split(string(m[1]), ",") {
		check = append(check, measure.Metric(strings.Trim(strings.TrimSpace(v), "'")))
	}
	if got := measure.Metrics(); !slices.Equal(got, check) {
		t.Fatalf("Metrics() = %q\nCHECK      %q", got, check)
	}
	empty := []string{""}
	modes := []string{"personal", "organization"}
	want := map[measure.Metric]struct {
		dims  []string
		scope measure.Scope
	}{
		"messages_created":      {[]string{"human", "agent", "app", "import"}, measure.ScopeChannel},
		"threads_created":       {empty, measure.ScopeChannel},
		"runs_started":          {modes, measure.ScopeChannelAgent},
		"runs_finished":         {[]string{"completed", "failed", "cancelled"}, measure.ScopeChannelAgent},
		"tool_calls":            {[]string{"completed", "failed"}, measure.ScopeChannelAgent},
		"approval_cards":        {[]string{"requested", "approved", "denied", "expired"}, measure.ScopeChannelAgent},
		"cost":                  {modes, measure.ScopeChannelAgent},
		"invocations_denied":    {[]string{"i2_denied", "billing_mode_mismatch"}, measure.ScopeChannelAgent},
		"billing_mode_switches": {modes, measure.ScopeAgent},
		"catchups":              {empty, measure.ScopeNone},
		"resyncs":               {[]string{"epoch_changed", "retention", "too_far_behind", "cursor_ahead"}, measure.ScopeNone},
		"slow_consumer_closes":  {empty, measure.ScopeNone},
		"sli.api_availability":  {empty, measure.ScopeNone},
		"sli.delivery_latency":  {empty, measure.ScopeNone},
		"sli.reconnect":         {empty, measure.ScopeNone},
		"sli.push_acceptance":   {[]string{"apns", "fcm", "unifiedpush", "wns"}, measure.ScopeNone},
		"sli.run_start":         {empty, measure.ScopeChannelAgent},
		"sli.turn_duration":     {empty, measure.ScopeChannelAgent},
	}
	if len(want) != len(check) {
		t.Fatalf("the table has %d rows, the CHECK %d", len(want), len(check))
	}
	for _, metric := range check {
		w, ok := want[metric]
		if !ok {
			t.Errorf("%s: not in the table of 25 3.2", metric)
			continue
		}
		if got := measure.Dimensions(metric); !slices.Equal(got, w.dims) {
			t.Errorf("Dimensions(%s) = %q, want %q", metric, got, w.dims)
		}
		if got, ok := measure.ScopeOf(metric); !ok || got != w.scope {
			t.Errorf("ScopeOf(%s) = %d, %v, want %d", metric, got, ok, w.scope)
		}
		if got, want := metric.IsSLI(), strings.HasPrefix(string(metric), "sli."); got != want {
			t.Errorf("%s.IsSLI() = %v", metric, got)
		}
	}
	if measure.Dimensions("unknown") != nil {
		t.Error("Dimensions of an unknown metric is not nil")
	}
}

// TestUnknownDimension shows that Validate refuses every key outside the closed lists with
// ErrInvalid and accepts the valid ones; the stats recorder panics on such a key in a test binary
// (internal/stats/app TestRecordInvalidKeyPanicsInTests).
func TestUnknownDimension(t *testing.T) {
	t.Parallel()
	ch, ag := ids.New(), ids.New()
	bad := []struct {
		name string
		m    measure.Metric
		k    measure.Key
		v    measure.Value
	}{
		{"unknown metric", "messages_deleted", measure.Key{}, measure.One()},
		{"unknown dimension", measure.MessagesCreated, measure.Key{ChannelID: ch, Dimension: "system"}, measure.One()},
		{"free text in an empty dimension", measure.Catchups, measure.Key{Dimension: "x"}, measure.One()},
		{"tool name", measure.ToolCalls, measure.Key{ChannelID: ch, AgentID: ag, Dimension: "grep"}, measure.One()},
		{"channel on an instance metric", measure.SLIReconnect, measure.Key{ChannelID: ch}, measure.One()},
		{"agent on a channel metric", measure.ThreadsCreated, measure.Key{ChannelID: ch, AgentID: ag}, measure.One()},
		{"channel on an agent metric", measure.BillingModeSwitches, measure.Key{ChannelID: ch, AgentID: ag, Dimension: "personal"}, measure.One()},
		{"cost without currency", measure.Cost, measure.Key{ChannelID: ch, AgentID: ag, Dimension: "personal"}, measure.Amount(5)},
		{"cost with a malformed currency", measure.Cost, measure.Key{Dimension: "personal", Currency: "usd"}, measure.Amount(5)},
		{"currency outside cost", measure.RunsStarted, measure.Key{Dimension: "personal", Currency: "USD"}, measure.One()},
		{"zero count", measure.Catchups, measure.Key{}, measure.Value{}},
		{"negative sum", measure.Cost, measure.Key{Dimension: "personal", Currency: "EUR"}, measure.Amount(-1)},
	}
	for _, c := range bad {
		if err := measure.Validate(c.m, c.k, c.v); !errors.Is(err, measure.ErrInvalid) {
			t.Errorf("%s: Validate = %v, want ErrInvalid", c.name, err)
		}
	}
	good := []struct {
		m measure.Metric
		k measure.Key
		v measure.Value
	}{
		{measure.MessagesCreated, measure.Key{ChannelID: ch, Dimension: "import"}, measure.One()},
		{measure.Cost, measure.Key{ChannelID: ch, AgentID: ag, Dimension: "organization", Currency: "USD"}, measure.Amount(0)},
		{measure.BillingModeSwitches, measure.Key{AgentID: ag, Dimension: "organization"}, measure.One()},
		{measure.Resyncs, measure.Key{Dimension: "cursor_ahead"}, measure.One()},
		{measure.SLIRunStart, measure.Key{ChannelID: ch, AgentID: ag}, measure.SLI(-1, true)},
		{measure.RunsFinished, measure.Key{Dimension: "cancelled"}, measure.Timed(0)},
	}
	for _, c := range good {
		if err := measure.Validate(c.m, c.k, c.v); err != nil {
			t.Errorf("Validate(%s, %+v) = %v", c.m, c.k, err)
		}
	}
	measure.OrNop(nil).Record(measure.Catchups, measure.Key{}, measure.One())
}
