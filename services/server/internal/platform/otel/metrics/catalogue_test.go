// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// docRow is one metric row of signals.md tables 4.1 to 4.6.
type docRow struct {
	name, kind, unit string
	// attrs are the attribute keys with the values the catalogue lists for them (nil: none
	// listed).
	attrs map[string][]string
	// keys are the keys in order.
	keys []string
}

// docCatalogue is what the test reads from signals.md.
type docCatalogue struct {
	rows    map[string]docRow
	buckets map[string][]float64
}

// kindWords maps the kind column of the catalogue to Kind.
var kindWords = map[string]Kind{
	"Histogram": KindHistogram, "Counter": KindCounter,
	"UpDownCounter": KindUpDownCounter, "Gauge": KindGauge,
}

// noAttributes are the words of the attribute column for "no attribute".
var noAttributes = map[string]bool{"none": true}

var (
	backticked = regexp.MustCompile("`([^`]+)`")
	section4x  = regexp.MustCompile(`^### 4\.(\d)([a-z]?)\b`)
)

// signalsDoc finds docs/observability/signals.md by walking up from the working directory.
func signalsDoc(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "docs", "observability", "signals.md")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("docs/observability/signals.md not found above the working directory")
		}
		dir = parent
	}
}

// readCatalogue parses tables 4.1 to 4.6 (metric rows) and 4.7 (bucket rows) of signals.md.
func readCatalogue(t *testing.T) docCatalogue {
	t.Helper()
	f, err := os.Open(signalsDoc(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cat := docCatalogue{rows: map[string]docRow{}, buckets: map[string][]float64{}}
	section := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			section = ""
		}
		if m := section4x.FindStringSubmatch(line); m != nil {
			section = m[1] + m[2]
			continue
		}
		if section == "" || !strings.HasPrefix(line, "| `") && !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := splitRow(line)
		switch {
		case section == "7" && len(cells) == 2:
			addBuckets(t, cat.buckets, cells)
		case section == "6a" && len(cells) == 5:
			if r, ok := parseAppRow(cells); ok {
				cat.rows[r.name] = r
			}
		case section >= "1" && section <= "6" && section != "6a" && len(cells) == 6:
			if r, ok := parseRow(cells); ok {
				cat.rows[r.name] = r
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return cat
}

// splitRow splits a markdown table row into trimmed cells; a | inside backticks is no separator.
func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(strings.TrimSuffix(line, "|"), "|")
	var cells []string
	var cur strings.Builder
	inTick := false
	for _, r := range line {
		switch {
		case r == '`':
			inTick = !inTick
			cur.WriteRune(r)
		case r == '|' && !inTick:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

// parseRow reads a metric row: name, kind, unit, attributes, source, Prometheus.
func parseRow(cells []string) (docRow, bool) {
	names := backticked.FindAllStringSubmatch(cells[0], -1)
	kind, ok := kindWords[cells[1]]
	if len(names) != 1 || !ok {
		return docRow{}, false
	}
	r := docRow{name: names[0][1], kind: kind.String(), unit: strings.Trim(cells[2], "`"), attrs: map[string][]string{}}
	if noAttributes[cells[3]] {
		return r, true
	}
	// Top-level backticked words are keys; backticked words inside the parentheses after a key
	// are its listed values.
	depth := 0
	var key string
	for i := 0; i < len(cells[3]); i++ {
		switch c := cells[3][i]; c {
		case '(':
			depth++
		case ')':
			depth--
		case '`':
			end := strings.IndexByte(cells[3][i+1:], '`')
			word := cells[3][i+1 : i+1+end]
			i += end + 1
			if depth == 0 {
				key = word
				r.keys = append(r.keys, key)
				r.attrs[key] = nil
			} else if key != "" {
				r.attrs[key] = append(r.attrs[key], word)
			}
		}
	}
	return r, true
}

// appCommonKeys are the attributes of every app metric, which the prose of 4.6a names instead of
// the table.
var appCommonKeys = []string{"bothy.client.platform", "bothy.client.density", "bothy.client.theme", "bothy.client.app_major"}

// parseAppRow reads a row of 4.6a: name in the app, type, unit, further attributes, server name
// and Prometheus name. The row is keyed by the server name and carries the common attributes of
// the section plus its further ones.
func parseAppRow(cells []string) (docRow, bool) {
	server := backticked.FindAllStringSubmatch(cells[4], -1)
	if len(server) == 0 {
		return docRow{}, false
	}
	r, ok := parseRow([]string{"`" + server[0][1] + "`", cells[1], cells[2], cells[3], "", ""})
	if !ok {
		return docRow{}, false
	}
	for _, k := range appCommonKeys {
		r.keys = append(r.keys, k)
		r.attrs[k] = nil
	}
	return r, true
}

// addBuckets reads a 4.7 row: the histogram names in the first cell, the bounds in the second.
func addBuckets(t *testing.T, out map[string][]float64, cells []string) {
	t.Helper()
	var bounds []float64
	for _, f := range strings.Split(cells[1], ",") {
		v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
		if err != nil {
			return // the header row
		}
		bounds = append(bounds, v)
	}
	for _, m := range backticked.FindAllStringSubmatch(cells[0], -1) {
		out[m[1]] = bounds
	}
}

// TestCatalogue_RegistryMatchesSignalsDoc: every registered spec is a row of signals.md with the
// same kind, unit and attribute keys, the listed values where the catalogue lists them, and the
// bucket row of 4.7 (signals.md header; 17 "Tests that hold these rules" item 4).
func TestCatalogue_RegistryMatchesSignalsDoc(t *testing.T) {
	cat := readCatalogue(t)
	if len(cat.rows) < 50 || len(cat.buckets) < 10 {
		t.Fatalf("parsed only %d metric rows and %d bucket entries; the parser lost the tables", len(cat.rows), len(cat.buckets))
	}
	for _, s := range All() {
		row, ok := cat.rows[s.Name]
		if !ok {
			t.Errorf("%s: not in signals.md tables 4.1 to 4.6", s.Name)
			continue
		}
		if row.kind != s.Kind.String() || row.unit != s.Unit {
			t.Errorf("%s: kind %s unit %s, catalogue %s %s", s.Name, s.Kind, s.Unit, row.kind, row.unit)
		}
		if !slices.Equal(sortedCopy(s.Keys()), sortedCopy(row.keys)) {
			t.Errorf("%s: attributes %v, catalogue %v", s.Name, s.Keys(), row.keys)
		}
		for _, a := range s.Attrs {
			listed := row.attrs[a.Key]
			if listed != nil && a.Source == "" && !slices.Equal(sortedCopy(a.Values), sortedCopy(listed)) {
				t.Errorf("%s: values of %s %v, catalogue %v", s.Name, a.Key, a.Values, listed)
			}
		}
		bounds, has := cat.buckets[s.Name]
		switch {
		case s.Kind == KindHistogram && !has:
			t.Errorf("%s: histogram without a row in 4.7", s.Name)
		case s.Kind == KindHistogram && !slices.Equal(bounds, s.Buckets):
			t.Errorf("%s: bounds %v, catalogue %v", s.Name, s.Buckets, bounds)
		case s.Kind != KindHistogram && has:
			t.Errorf("%s: 4.7 names it, but it is a %s", s.Name, s.Kind)
		}
	}
}

// TestCatalogue_M1InstrumentSet: the registry holds exactly the instruments this build records
// (brief M1-68 "Instruments", plus the gauges of M2-01 already on the base,
// bothy.file.upload.size of M3-03, the app metrics of M4-31, the scan instruments of M3-04, the
// relay counter of M2-44, the gauges of bothy-backupd of M7-02, the restore test instruments of
// M7-03, the counter bothy.console.csp_violations of M6-03, the four run instruments of M2-05 and
// the four push instruments of M3-19).
func TestCatalogue_M1InstrumentSet(t *testing.T) {
	want := []string{
		"bothy.audit.verify.failures", "bothy.audit.verify.last_success.timestamp", "bothy.authz.decisions",
		"bothy.authz.errors", "bothy.backup.duration", "bothy.backup.last_success.timestamp", "bothy.backup.size",
		"bothy.backup.restore_test.duration", "bothy.backup.restore_test.failures", "bothy.backup.restore_test.last_success.timestamp",
		"bothy.backup.wal.last_archived.timestamp", "bothy.console.csp_violations", "bothy.db.event_seq.lock_wait", "bothy.db.pool.connections", "bothy.delivery.duration",
		"bothy.egress.blocked", "bothy.event.appended", "bothy.event.consumer.lag", "bothy.event.consumer.pending_age",
		"bothy.event.fanout.lag", "bothy.file.scan", "bothy.file.scan.duration", "bothy.file.scan.queue", "bothy.file.upload.size", "bothy.gateway.catchup.duration", "bothy.gateway.close", "bothy.gateway.connect.duration",
		"bothy.gateway.connections.active", "bothy.gateway.outbound.dropped", "bothy.gateway.resync", "bothy.job.duration",
		"bothy.job.queue", "bothy.message.created", "bothy.ratelimit.rejected",
		// M2-05: the run instruments of 4.3.
		"bothy.run.active", "bothy.run.duration", "bothy.run.finished", "bothy.run.start.latency",
		// M2-08: the denial counter of the delivery check.
		"bothy.agent.invocation.denied",
		// M2-09: the counter of I3 mismatches.
		"bothy.agent.billing_mode_mismatch",
		"bothy.secrets.refresh.failures",
		"bothy.storage.volume.free", "bothy.storage.volume.size", "bothy.telemetry.dropped", "bothy.terminal.hosts.connected", "bothy.terminal.relay.unpaired",
		"bothy.terminal.sessions.active", "bothy.terminal.sessions.closed", "db.client.operation.duration", "http.server.request.duration",
		// M4-31: the app metrics of 4.6a and the discard counter of their endpoint.
		"bothy.client.catchup.duration", "bothy.client.catchup.events", "bothy.client.cold_start.duration",
		"bothy.client.frames", "bothy.client.frames.over_budget", "bothy.client.outbox.depth",
		"bothy.client.push.placeholder", "bothy.client.push_display.duration", "bothy.client.reconnects",
		"bothy.client.resync", "bothy.client.send_local.duration", "bothy.client.thread_open.duration",
		"bothy.client_metrics.discarded",
		// M2-20: the presence gauges of 4.3.
		"bothy.agent.heartbeat.age", "bothy.agent.billing_mode.mismatch_age",
		// M2-10: the steps adapters add through live-box patches (4.3).
		"bothy.run.steps",
		// M3-19: the push instruments of 4.4 (bothy.push.fetch comes with M3-22).
		"bothy.push.registrations.active", "bothy.push.send.duration", "bothy.push.sent", "bothy.push.skipped",
	}
	slices.Sort(want)
	var got []string
	for _, s := range All() {
		got = append(got, s.Name)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("registry %v,\nwant %v", got, want)
	}
	if problems := Check(All()); len(problems) > 0 {
		t.Errorf("the registry fails Check: %v", problems)
	}
}

func sortedCopy(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

// TestCatalogue_PushInstruments holds the push instruments of signals.md 4.4 to brief M3-19
// design notes 13 and 15: the closed sets of bothy.push.provider (signals.md 5.3), bothy.outcome
// (the outcomes of the result classes) and bothy.reason (the seven skip reasons; quiet hours
// never drop a hint, R-PN-05), the seconds buckets of the send duration and a gauge per provider.
func TestCatalogue_PushInstruments(t *testing.T) {
	providers := []string{"apns", "fcm", "unifiedpush", "wns"}
	cases := []struct {
		spec  Spec
		kind  Kind
		unit  string
		attrs map[string][]string
	}{
		{PushSent, KindCounter, "{notification}", map[string][]string{
			"bothy.push.provider": providers,
			"bothy.outcome":       {"accepted", "rejected_token", "rejected_other", "throttled", "provider_error"},
		}},
		{PushSendDuration, KindHistogram, "s", map[string][]string{"bothy.push.provider": providers}},
		{PushSkipped, KindCounter, "{notification}", map[string][]string{
			"bothy.reason": {"no_access", "preference", "active_in_thread", "no_registration", "read", "expired", "no_provider"},
		}},
		{PushRegistrationsActive, KindGauge, "{registration}", map[string][]string{"bothy.push.provider": providers}},
	}
	for _, c := range cases {
		if c.spec.Kind != c.kind || c.spec.Unit != c.unit || c.spec.Description == "" {
			t.Errorf("%s: kind %s unit %s description %q, want %s %s and a description", c.spec.Name, c.spec.Kind, c.spec.Unit, c.spec.Description, c.kind, c.unit)
		}
		if len(c.spec.Attrs) != len(c.attrs) {
			t.Errorf("%s: attributes %v, want %v", c.spec.Name, c.spec.Keys(), c.attrs)
		}
		for _, a := range c.spec.Attrs {
			if want, ok := c.attrs[a.Key]; !ok || a.Source != "" || a.Optional || !slices.Equal(sortedCopy(a.Values), sortedCopy(want)) {
				t.Errorf("%s: %s values %v (source %q, optional %v), want %v", c.spec.Name, a.Key, a.Values, a.Source, a.Optional, want)
			}
		}
	}
	if !slices.Equal(PushSendDuration.Buckets, BucketsSeconds) {
		t.Errorf("bothy.push.send.duration bounds %v, want BucketsSeconds", PushSendDuration.Buckets)
	}
	if problems := Check([]Spec{PushSent, PushSendDuration, PushSkipped, PushRegistrationsActive}); len(problems) > 0 {
		t.Errorf("the push instruments fail Check: %v", problems)
	}
}
