// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/test/canary"
)

// The sinks of TM-T06 (Design note 9): what the relay passed, the container logs, the database
// dump, the collector export and the journal of the host service.
const (
	SinkRelayView    = "relay"
	SinkContainerLog = "container_logs"
	SinkDatabaseDump = "database_dump"
	SinkCollector    = "collector_export"
	SinkHostJournal  = "host_journal"
)

// CanarySinks returns the records of TM-T06: every binary message of relay as one record of the
// relay view, and every line of each text source as one record of its sink. The sources are keyed
// "<sink>:<name>".
func CanarySinks(sources map[string][]byte, relay [][]byte) *canary.Sinks {
	s := canary.NewSinks()
	for i, m := range relay {
		s.Add(canary.Record{Sink: SinkRelayView, Source: fmt.Sprintf("message %d", i), Fields: []canary.Field{{Location: "bytes", Text: string(m)}}})
	}
	for _, key := range slices.Sorted(func(yield func(string) bool) {
		for k := range sources {
			if !yield(k) {
				return
			}
		}
	}) {
		sink, name, _ := strings.Cut(key, ":")
		sc := bufio.NewScanner(bytes.NewReader(sources[key]))
		sc.Buffer(make([]byte, 0, 64*1024), 64<<20)
		line := 0
		for sc.Scan() {
			line++
			s.Add(canary.Record{Sink: sink, Source: fmt.Sprintf("%s line %d", name, line), Fields: []canary.Field{{Location: "text", Text: sc.Text()}}})
		}
	}
	return s
}

// ScanCanaries scans the sinks for every value of reg with the allow rules of TM-T06: no terminal
// canary and no session token is allowed in any sink. The findings name sink, place, class and
// label, never the value.
func ScanCanaries(reg *canary.Registry, sinks *canary.Sinks) []string {
	var out []string
	for _, f := range canary.Scan(reg, sinks, canary.AllowRules{}) {
		out = append(out, f.String())
	}
	return out
}

// canaryValue returns the canary of a name, creating it on first use (class content).
func (w *instance) canaryValue(name string) string {
	if w.canaries == nil {
		w.canaries = canary.NewRegistry()
		w.canaryNames = map[string]string{}
	}
	if v, ok := w.canaryNames[name]; ok {
		return v
	}
	v := w.canaries.New(canary.ClassContent, "", "terminal content "+name)
	w.canaryNames[name] = v
	return v
}

// checkCanary collects the sinks of TM-T06 and scans them: the positive control is that the
// output of a session holds the concatenation of the canaries the step names in expect.output.
func (w *instance) checkCanary(s Step, exp Expect) []string {
	w.mu.Lock()
	reg := w.canaries
	w.mu.Unlock()
	if reg == nil {
		return []string{"the case created no canary"}
	}
	concat := w.expand(exp.Output)
	reg.Register(concat, canary.ClassContent, "", "terminal output")
	var relay [][]byte
	control := false
	w.mu.Lock()
	for _, ses := range w.sessions {
		for _, c := range ses.conns {
			reg.Register(c.Token, canary.ClassSecret, "", "session token")
			if bytes.Contains(c.Output(), []byte(concat)) {
				control = true
			}
			if c.Stream != nil {
				for _, m := range c.Stream.Wire() {
					relay = append(relay, m.Bytes)
				}
			}
		}
	}
	w.mu.Unlock()
	sources := map[string][]byte{}
	logs, err := w.env.Stack.Logs(w.ctx)
	if err != nil {
		return []string{err.Error()}
	}
	sources[SinkContainerLog+":compose logs"] = logs
	dump, err := w.env.Stack.Exec(w.ctx, "postgres", "pg_dump", "-U", "postgres", "--data-only", "bothy")
	if err != nil {
		return []string{err.Error()}
	}
	sources[SinkDatabaseDump+":pg_dump"] = dump
	dir := w.t.TempDir()
	if err := w.env.Stack.CopyFrom(w.ctx, "otel-collector", "/var/lib/otelcol-smoke/.", dir); err != nil {
		return []string{err.Error()}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{err.Error()}
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // G304: the copied export in the test's temp dir
		if err != nil {
			return []string{err.Error()}
		}
		sources[SinkCollector+":"+e.Name()] = b
	}
	for name := range w.hosts {
		h, err := w.container(name)
		if err != nil {
			return []string{err.Error()}
		}
		j, err := h.Journal(w.ctx)
		if err != nil {
			return []string{err.Error()}
		}
		sources[SinkHostJournal+":"+name] = j
	}
	return CompareCanary(exp, ScanCanaries(reg, CanarySinks(sources, relay)), control)
}
