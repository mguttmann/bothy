// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"maps"
)

// Namespaces of import_sources.stats (M8-03 design note 10): the server's counters under server,
// the tool's under tool, so neither overwrites the other.
const (
	statsServer       = "server"
	statsTool         = "tool"
	statSourceChanged = "source_changed"
	statCancelled     = "cancelled"
)

// Counts are the new transitions of one batch: item kind, then state, then the number; and the
// outcomes of the earlier sessions of thread items (M8-06 design note 3).
type Counts struct {
	byKind        map[string]map[string]int64
	sourceChanged int64
	sessions      map[string]int64
}

// statSessionsPrefix starts the counters of the earlier sessions: server.sessions_taken_over,
// server.sessions_already_present and server.sessions_dropped_late.
const statSessionsPrefix = "sessions_"

// Session counts one earlier session of a thread item with its outcome (taken_over,
// already_present or dropped_late).
func (c *Counts) Session(outcome string) {
	if c.sessions == nil {
		c.sessions = map[string]int64{}
	}
	c.sessions[outcome]++
}

// Add counts one item of kind that made a new transition into state in this batch.
func (c *Counts) Add(kind, state string) {
	if c.byKind == nil {
		c.byKind = map[string]map[string]int64{}
	}
	if c.byKind[kind] == nil {
		c.byKind[kind] = map[string]int64{}
	}
	c.byKind[kind][state]++
}

// SourceChanged counts one item answered with source_changed.
func (c *Counts) SourceChanged() { c.sourceChanged++ }

// Zero reports that nothing was counted.
func (c Counts) Zero() bool {
	return len(c.byKind) == 0 && c.sourceChanged == 0 && len(c.sessions) == 0
}

// decodeStats reads stats as an object; an empty or invalid value starts from {}.
func decodeStats(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

// namespace returns the object member name of m, created when missing or not an object.
func namespace(m map[string]any, name string) map[string]any {
	ns, ok := m[name].(map[string]any)
	if !ok {
		ns = map[string]any{}
		m[name] = ns
	}
	return ns
}

// number reads a counter, 0 when missing or not a number.
func number(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

// AddCounts adds c to server.<kind>.<state>, server.source_changed and server.sessions_<outcome>
// of stats.
func AddCounts(stats json.RawMessage, c Counts) (json.RawMessage, error) {
	m := decodeStats(stats)
	srv := namespace(m, statsServer)
	for kind, states := range c.byKind {
		k := namespace(srv, kind)
		for state, n := range states {
			k[state] = float64(number(k[state]) + n)
		}
	}
	if c.sourceChanged > 0 {
		srv[statSourceChanged] = float64(number(srv[statSourceChanged]) + c.sourceChanged)
	}
	for outcome, n := range c.sessions {
		k := statSessionsPrefix + outcome
		srv[k] = float64(number(srv[k]) + n)
	}
	return json.Marshal(m)
}

// MarkCancelled sets server.cancelled = true: a cancelled source is failed (the enum has no
// cancelled state, M8-03 design note 3).
func MarkCancelled(stats json.RawMessage) (json.RawMessage, error) {
	m := decodeStats(stats)
	namespace(m, statsServer)[statCancelled] = true
	return json.Marshal(m)
}

// MergeTool merges the top-level members of tool into stats.tool (setImportCursor): a member
// replaces the member of the same name, other members stay. tool must be a JSON object.
func MergeTool(stats, tool json.RawMessage) (json.RawMessage, error) {
	var in map[string]any
	if err := json.Unmarshal(tool, &in); err != nil {
		return nil, err
	}
	m := decodeStats(stats)
	maps.Copy(namespace(m, statsTool), in)
	return json.Marshal(m)
}
