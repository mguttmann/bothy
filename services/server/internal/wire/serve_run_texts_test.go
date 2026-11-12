// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The defaults of run.status_line_max_chars and run.summary_max_chars (03 section 9), which the
// world of TestRunTextsNeverBlockTheEndOfARun runs with.
const (
	runStatusLineMaxChars = 500
	runSummaryMaxChars    = 2000
)

// longRunText is a text of an adapter that is longer than sync.event_payload_max_bytes (65536) in
// characters and in bytes. U+2028 passes the display text rules and takes six bytes in JSON, the
// most a character of a display text can take, so the cut texts are the largest a run can carry.
var longRunText = strings.Repeat("\u2028", 70000)

// runStateEvent is the part of a run.state_changed envelope the tests of the run texts read.
type runStateEvent struct {
	Type      string `json:"type"`
	Truncated bool   `json:"truncated"`
	Resource  struct {
		ID string `json:"id"`
	} `json:"resource"`
	Payload struct {
		Run struct {
			State      string  `json:"state"`
			StatusLine *string `json:"status_line"`
			Summary    *string `json:"summary"`
		} `json:"run"`
	} `json:"payload"`
}

// nextRunState reads the frames of c until run.state_changed of run with state arrives.
func nextRunState(t *testing.T, c *wstest.Client, run, state string) runStateEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			t.Fatalf("no run.state_changed %s of run %s: %v", state, run, err)
		}
		if op != "dispatch" {
			continue
		}
		var ev runStateEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type == "run.state_changed" && ev.Resource.ID == run && ev.Payload.Run.State == state {
			return ev
		}
	}
}

// runTexts reads the stored status line and summary of run.
func (w *lbWorld) runTexts(t *testing.T, run string) (statusLine, summary string) {
	t.Helper()
	if err := w.queryRow(context.Background(), `SELECT coalesce(status_line, ''), coalesce(summary, '') FROM runs WHERE id = $1`, run).
		Scan(&statusLine, &summary); err != nil {
		t.Fatal(err)
	}
	return statusLine, summary
}

// wantCut requires got to be the first limit characters of longRunText. It does not end the
// test, so a subtest also shows what a longer stored text does to the end of its run.
func wantCut(t *testing.T, what, got string, limit int) {
	t.Helper()
	if n := utf8.RuneCountInString(got); n != limit || !strings.HasPrefix(longRunText, got) {
		t.Errorf("%s has %d characters, want the first %d of the text", what, n, limit)
	}
}

// TestRunTextsNeverBlockTheEndOfARun is the regression test of the audit finding srv-runs-core-1
// against a started api process and a worker on PostgreSQL: an adapter stores a status line
// longer than sync.event_payload_max_bytes, and the run must still end by every path whose
// run.state_changed carries the status line: finishRun (with a summary of the same length),
// awaitRunInput, agent.disable and the job runs.cancel_timeout. Before the fix the patch stored
// the whole text and each of these paths answered 500 or never ended the run, because the
// appender refuses a payload of that size. The texts are cut to run.status_line_max_chars and
// run.summary_max_chars, so the events arrive whole; a run whose row holds a longer text from
// before the limits ends with a reduced event (04 payload rule 2).
func TestRunTextsNeverBlockTheEndOfARun(t *testing.T) {
	t.Parallel()
	w := newLBWorld(t)
	worker := start(t, wire.RoleWorker, w.env, wire.Deps{})
	waitReadyz(t, worker.waitInternal(t), http.StatusOK)
	alex, _ := w.ready(t, w.params(w.owner))

	// longLine stores the long status line with patchRun and requires the stored cut.
	longLine := func(t *testing.T, r lbRun) {
		t.Helper()
		w.patchOK(t, r.agent, r.id, w.version(r.id), op("replace", "/status_line", longRunText))
		line, _ := w.runTexts(t, r.id)
		wantCut(t, "the stored status line", line, runStatusLineMaxChars)
	}

	t.Run("finishRun", func(t *testing.T) {
		r := w.running(t, "texts-finish")
		longLine(t, r)
		code, body := w.call(w.base, r.agent, http.MethodPost, "/api/v1/runs/"+r.id+"/finish",
			map[string]any{"state": "completed", "summary": longRunText})
		if code != http.StatusOK {
			t.Fatalf("finishRun = %d %.300s", code, body)
		}
		line, summary := w.runTexts(t, r.id)
		wantCut(t, "the stored status line", line, runStatusLineMaxChars)
		wantCut(t, "the stored summary", summary, runSummaryMaxChars)
		// The largest texts a run can carry fit the event: it arrives whole.
		ev := nextRunState(t, alex, r.id, "completed")
		if ev.Truncated || ev.Payload.Run.StatusLine == nil || ev.Payload.Run.Summary == nil ||
			*ev.Payload.Run.StatusLine != line || *ev.Payload.Run.Summary != summary {
			t.Fatalf("run.state_changed: truncated %v, status line %v, summary %v; want the stored texts in a whole event",
				ev.Truncated, ev.Payload.Run.StatusLine != nil, ev.Payload.Run.Summary != nil)
		}
	})

	t.Run("awaitRunInput", func(t *testing.T) {
		r := w.running(t, "texts-await")
		code, body := w.call(w.base, r.agent, http.MethodPost, "/api/v1/runs/"+r.id+"/await-input", map[string]any{"status_line": longRunText})
		if code != http.StatusOK {
			t.Fatalf("awaitRunInput = %d %.300s", code, body)
		}
		line, _ := w.runTexts(t, r.id)
		wantCut(t, "the stored status line", line, runStatusLineMaxChars)
		if got := w.runOfID(r.id); got.state != "waiting_input" {
			t.Fatalf("run %+v, want waiting_input", got)
		}
	})

	t.Run("disableAgent", func(t *testing.T) {
		r := w.running(t, "texts-disable")
		longLine(t, r)
		if code, body := w.rest(w.base, w.owner, http.MethodPost, "/api/v1/agents/"+r.agent.id.String()+"/disable",
			map[string]any{"reason": "test"}); code != http.StatusOK {
			t.Fatalf("disableAgent = %d %.300s", code, body)
		}
		if got := w.runOfID(r.id); got.state != "cancelled" || got.reason != "agent_disabled" {
			t.Fatalf("run %+v, want cancelled by agent_disabled", got)
		}
	})

	t.Run("runs.cancel_timeout", func(t *testing.T) {
		r := w.running(t, "texts-cancel")
		longLine(t, r)
		w.must(w.owner, http.MethodPost, "/api/v1/runs/"+r.id+"/cancel", map[string]any{})
		// The adapter does not answer the cancel; the job of the worker ends the run.
		testkit.Until(t, "the end of the run by runs.cancel_timeout", func() bool { return w.runOfID(r.id).state == "cancelled" })
		if got := w.runOfID(r.id); got.reason != "cancel_timeout" {
			t.Fatalf("run %+v, want cancelled by cancel_timeout", got)
		}
	})

	t.Run("a row from before the limits", func(t *testing.T) {
		r := w.running(t, "texts-stored")
		// A status line that a server before the limits stored whole.
		w.sql(`UPDATE runs SET status_line = $2 WHERE id = $1`, r.id, longRunText)
		code, body := w.call(w.base, r.agent, http.MethodPost, "/api/v1/runs/"+r.id+"/finish", map[string]any{"state": "completed", "summary": "Done"})
		if code != http.StatusOK {
			t.Fatalf("finishRun = %d %.300s", code, body)
		}
		// The event does not fit, so it goes out reduced: without the two texts and marked, and
		// the run document still answers them.
		ev := nextRunState(t, alex, r.id, "completed")
		if !ev.Truncated || ev.Payload.Run.StatusLine != nil || ev.Payload.Run.Summary != nil {
			t.Fatalf("run.state_changed: truncated %v, status line %v, summary %v; want a reduced event without the texts",
				ev.Truncated, ev.Payload.Run.StatusLine != nil, ev.Payload.Run.Summary != nil)
		}
		_, raw := w.getRun(t, r.id)
		var doc struct {
			State      string `json:"state"`
			StatusLine string `json:"status_line"`
			Summary    string `json:"summary"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if doc.State != "completed" || doc.StatusLine != longRunText || doc.Summary != "Done" {
			t.Fatalf("getRun: state %s, status line of %d characters, summary %q", doc.State, utf8.RuneCountInString(doc.StatusLine), doc.Summary)
		}
	})

	// No log line carries a run text, neither raw nor in the escaped form of a JSON log line.
	logs := w.p.logs.String() + worker.logs.String()
	if strings.Contains(logs, longRunText[:6]) || strings.Contains(logs, `\u2028\u2028`) {
		t.Error("a log line carries a run text")
	}
}
