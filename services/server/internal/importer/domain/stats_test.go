// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
)

// TestStats checks the namespaces of design note 10: the server's counters under server, the
// tool's under tool, neither overwriting the other.
func TestStats(t *testing.T) {
	t.Parallel()
	var c domain.Counts
	if !c.Zero() {
		t.Error("new counts are zero")
	}
	c.Add(domain.KindChannel, "posted")
	c.Add(domain.KindChannel, "posted")
	c.Add(domain.KindUser, "skipped")
	c.SourceChanged()
	s, err := domain.AddCounts(json.RawMessage(`{"tool":{"messages":5}}`), c)
	if err != nil {
		t.Fatal(err)
	}
	s, err = domain.AddCounts(s, c)
	if err != nil {
		t.Fatal(err)
	}
	s, err = domain.MergeTool(s, json.RawMessage(`{"messages": 7, "server": {"channel": 99}}`))
	if err != nil {
		t.Fatal(err)
	}
	s, err = domain.MarkCancelled(s)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(s, &got); err != nil {
		t.Fatal(err)
	}
	want := `{"server":{"cancelled":true,"channel":{"posted":4},"source_changed":2,"user":{"skipped":2}},"tool":{"messages":7,"server":{"channel":99}}}`
	if string(s) != want {
		t.Errorf("stats\n got %s\nwant %s", s, want)
	}
	if _, err := domain.MergeTool(s, json.RawMessage(`[1]`)); err == nil {
		t.Error("tool stats must be an object")
	}
}
