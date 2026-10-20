// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
)

// largeLimit is a rendering limit no fixture reaches.
const largeLimit = 1 << 20

// goldenDir holds the Alertmanager fixtures and their expected messages.
const goldenDir = "testdata/alertmanager"

// goldenCases are the fixture names TestRenderAlertmanager_Golden must find; a missing file fails.
var goldenCases = []string{
	"firing_two_alerts", // two alerts of one alertname with severity, summary, runbook and source
	"resolved_one",      // resolved with one alert
	"mixed_names",       // mixed alert names without commonLabels.alertname
	"truncated",         // truncatedAlerts above zero
	"unsafe_urls",       // a javascript: URL and relative URLs left out
	"escaped_label",     // a label value with *, [, a line break and @alex
	"description_only",  // no summary but description
	"labels_only",       // neither, so the remaining labels are listed; then no summary at all
}

// readPayload decodes one fixture.
func readPayload(t testing.TB, path string) incoming.Payload {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // G304: a fixture below testdata, not input.
	if err != nil {
		t.Fatal(err)
	}
	var p incoming.Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return p
}

// readGolden reads an expected message; the file ends with the one newline the repository's
// end-of-file rule demands, which is not part of the message.
func readGolden(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // G304: a golden file below testdata, not input.
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(string(raw), "\n")
}

func TestParseAlertmanager(t *testing.T) {
	valid := func() incoming.Payload {
		return incoming.Payload{
			Version: "4", Status: "firing", TruncatedAlerts: 2,
			CommonLabels: map[string]any{"alertname": "DiskFull"},
			ExternalURL:  "https://alertmanager.example.com",
			Alerts: []incoming.RawAlert{{
				Status: "firing", Labels: map[string]any{"alertname": "DiskFull", "severity": "warning"},
				GeneratorURL: "https://prometheus.example.com/graph?g0.expr=up",
			}},
		}
	}

	t.Run("valid payload gives string maps", func(t *testing.T) {
		n, err := incoming.ParseAlertmanager(valid())
		if err != nil {
			t.Fatal(err)
		}
		if n.Status != "firing" || n.TruncatedAlerts != 2 || n.CommonLabels["alertname"] != "DiskFull" ||
			n.ExternalURL != "https://alertmanager.example.com" || len(n.Alerts) != 1 {
			t.Fatalf("got %+v", n)
		}
		if n.GroupLabels == nil || n.CommonAnnotations == nil || n.Alerts[0].Annotations == nil {
			t.Fatal("absent maps must be empty, not nil")
		}
		a := n.Alerts[0]
		if a.Status != "firing" || a.Labels["severity"] != "warning" || a.GeneratorURL != "https://prometheus.example.com/graph?g0.expr=up" {
			t.Fatalf("alert %+v", a)
		}
	})

	for _, tc := range []struct {
		name          string
		mutate        func(p *incoming.Payload)
		pointer, code string
	}{
		{"missing labels", func(p *incoming.Payload) { p.Alerts[0].Labels = nil }, "/alerts/0/labels", incoming.CodeRequired},
		{"label value is a number", func(p *incoming.Payload) { p.Alerts[0].Labels["severity"] = 3.0 }, "/alerts/0/labels/severity", incoming.CodeInvalidType},
		{"annotation value is an object", func(p *incoming.Payload) {
			p.Alerts[0].Annotations = map[string]any{"summary": map[string]any{}}
		}, "/alerts/0/annotations/summary", incoming.CodeInvalidType},
		{"common label is a bool", func(p *incoming.Payload) { p.CommonLabels["team"] = true }, "/commonLabels/team", incoming.CodeInvalidType},
		{"member name escaped per RFC 6901", func(p *incoming.Payload) { p.GroupLabels = map[string]any{"a/b~c": nil} }, "/groupLabels/a~1b~0c", incoming.CodeInvalidType},
		{"common annotation is an array", func(p *incoming.Payload) { p.CommonAnnotations = map[string]any{"x": []any{}} }, "/commonAnnotations/x", incoming.CodeInvalidType},
		{"empty alerts list", func(p *incoming.Payload) { p.Alerts = []incoming.RawAlert{} }, "/alerts", "too_few_items"},
		{"version other than 4", func(p *incoming.Payload) { p.Version = "3" }, "/version", "invalid_enum"},
		{"unknown status", func(p *incoming.Payload) { p.Status = "pending" }, "/status", "invalid_enum"},
		{"alert without status", func(p *incoming.Payload) { p.Alerts[0].Status = "" }, "/alerts/0/status", incoming.CodeRequired},
		{"negative truncatedAlerts", func(p *incoming.Payload) { p.TruncatedAlerts = -1 }, "/truncatedAlerts", "too_small"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := valid()
			tc.mutate(&p)
			_, err := incoming.ParseAlertmanager(p)
			wantField(t, err, tc.pointer, tc.code)
		})
	}
}

func TestRenderAlertmanager_Golden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(goldenDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		found[name] = true
		t.Run(name, func(t *testing.T) {
			n, err := incoming.ParseAlertmanager(readPayload(t, f))
			if err != nil {
				t.Fatal(err)
			}
			want := readGolden(t, strings.TrimSuffix(f, ".json")+".md")
			if got := incoming.RenderAlertmanager(n, largeLimit); got != want {
				t.Fatalf("rendered message differs\n--- got\n%s\n--- want\n%s", got, want)
			}
		})
	}
	for _, c := range goldenCases {
		if !found[c] {
			t.Errorf("fixture %s.json missing", c)
		}
	}
}

func TestRenderAlertmanager_FitsMaxContentBytes(t *testing.T) {
	n, err := incoming.ParseAlertmanager(readPayload(t, filepath.Join(goldenDir, "firing_two_alerts.json")))
	if err != nil {
		t.Fatal(err)
	}
	const (
		header = "**FIRING** HighErrorRate (2 alerts)"
		item1  = "- **firing** HighErrorRate, severity critical: Error rate above 5 percent on api\\-1\n" +
			"  Runbook: <https://runbooks.example.com/high-error-rate>\n" +
			"  Source: <https://prometheus.example.com/graph?g0.expr=rate%28http_errors_total%5B5m%5D%29&g0.tab=1>"
		footer = "\n\nAlertmanager: <https://alertmanager.example.com>"
	)
	full := incoming.RenderAlertmanager(n, largeLimit)
	oneDropped := header + "\n\n" + item1 + "\n- and 1 more alerts" + footer
	allDropped := header + "\n\n- and 2 more alerts" + footer
	noFooter := header + "\n\n- and 2 more alerts"

	for _, tc := range []struct {
		name  string
		limit int
		want  string
	}{
		{"exactly the full length", len(full), full},
		{"one byte less drops the last alert and counts it", len(full) - 1, oneDropped},
		{"exactly one alert dropped", len(oneDropped), oneDropped},
		{"all alerts dropped", len(oneDropped) - 1, allDropped},
		{"then the footer", len(allDropped) - 1, noFooter},
		{"exactly without footer", len(noFooter), noFooter},
		{"then a cut ending in ...", len(noFooter) - 1, noFooter[:len(noFooter)-4] + "..."},
		{"a short cut", 13, "**FIRING**..."},
		{"room for ... only", 3, "..."},
		{"no room", 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := incoming.RenderAlertmanager(n, tc.limit); got != tc.want {
				t.Fatalf("limit %d\n--- got\n%s\n--- want\n%s", tc.limit, got, tc.want)
			}
		})
	}

	t.Run("cut at a UTF-8 boundary for every limit", func(t *testing.T) {
		m := n
		m.CommonLabels = map[string]string{"alertname": "Störung äöü 日本語"}
		full := incoming.RenderAlertmanager(m, largeLimit)
		mNoFooter := "**FIRING** Störung äöü 日本語 (2 alerts)\n\n- and 2 more alerts"
		for limit := 0; limit <= len(full)+1; limit++ {
			got := incoming.RenderAlertmanager(m, limit)
			if len(got) > limit || !utf8.ValidString(got) {
				t.Fatalf("limit %d: %d bytes, valid UTF-8 %v", limit, len(got), utf8.ValidString(got))
			}
			if limit < len(mNoFooter) && limit >= 3 && !strings.HasSuffix(got, "...") {
				t.Fatalf("limit %d: a cut must end in ...: %q", limit, got)
			}
		}
	})
}

// TestRenderAlertmanager_ManyAlertsLinear (code review M2-27, Major): a payload with tens of
// thousands of minimal alerts, as the body limit allows, renders within the limit with the
// dropped alerts counted; the choice of how many items fit costs one pass over the items, so the
// call stays fast (the quadratic version needed seconds of CPU for this payload).
func TestRenderAlertmanager_ManyAlertsLinear(t *testing.T) {
	t.Parallel()
	n := incoming.Notification{Status: "firing", CommonLabels: map[string]string{"alertname": "A"}}
	for range 40000 {
		n.Alerts = append(n.Alerts, incoming.Alert{Status: "firing", Labels: map[string]string{"alertname": "A"}, Annotations: map[string]string{}})
	}
	const limit = 4096
	got := incoming.RenderAlertmanager(n, limit)
	if len(got) > limit || !utf8.ValidString(got) {
		t.Fatalf("%d bytes, valid UTF-8 %v", len(got), utf8.ValidString(got))
	}
	kept := strings.Count(got, "\n- **firing**")
	if !strings.HasSuffix(got, "- and "+strconv.Itoa(40000-kept)+" more alerts") || kept == 0 {
		t.Fatalf("kept %d items, tail %q", kept, got[max(0, len(got)-60):])
	}
	// The kept items are the most that fit: one more item would exceed the limit.
	one := len("\n- **firing** A: no summary")
	if len(got)+one-len(strconv.Itoa(40000-kept))+len(strconv.Itoa(40000-kept-1)) <= limit {
		t.Fatalf("%d bytes with %d items: one more would have fit", len(got), kept)
	}
}
