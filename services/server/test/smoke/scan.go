// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// minCanaryLen is the shortest value the scan accepts. A short value matches by chance in
// unrelated text (an empty one everywhere); every real value is longer: the canary has 27
// characters, a principal ID 36, tokens and the enrollment code more.
const minCanaryLen = 16

// LabelCanary is the label of the run canary; a canary list without it is invalid.
const LabelCanary = "canary"

// Canary is one value the scan looks for, with the label a hit reports instead of the value.
type Canary struct {
	Label, Value string
}

// Canaries is the list of the values of one run (canaries.txt).
type Canaries []Canary

// Marshal returns the list as canaries.txt: one "label<TAB>value" line per entry.
func (c Canaries) Marshal() []byte {
	var b bytes.Buffer
	for _, x := range c {
		b.WriteString(x.Label + "\t" + x.Value + "\n")
	}
	return b.Bytes()
}

// ParseCanaries reads canaries.txt. The error reports a non-empty line without a tab; it names
// the line number, never the content.
func ParseCanaries(b []byte) (Canaries, error) {
	var out Canaries
	for i, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		label, value, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("smoke: canaries line %d has no tab", i+1)
		}
		out = append(out, Canary{Label: label, Value: value})
	}
	return out, nil
}

// Hit is one occurrence of a canary value in a scanned file.
type Hit struct {
	File  string
	Line  int
	Label string
}

// String returns "<file>:<line>: <label>"; a hit never carries the value.
func (h Hit) String() string { return fmt.Sprintf("%s:%d: %s", h.File, h.Line, h.Label) }

// match reports whether value occurs in line. It is a variable so that a test can break it and
// prove that the positive control of Scan catches a broken matcher.
var match = bytes.Contains

// controlText is the built-in text of the positive control: the value inside a JSON string as a
// collector export or a log line carries it.
func controlText(value string) []byte {
	return []byte(`{"attributes":[{"key":"content","value":{"stringValue":"x ` + value + ` y"}}]}`)
}

// Roots returns the directories below the input directory in that the scan looks: the collector
// export otel/ and the container logs logs/. The probe's own output out/ (setup.out,
// canaries.txt) is never scanned (docs/observability/privacy.md section 6).
func Roots(in string) []string {
	return []string{filepath.Join(in, "otel"), filepath.Join(in, "logs")}
}

// Scan looks for every value of canaries in every regular file below roots and returns one Hit per
// file, line and label. It fails before it scans when the list is empty, lacks the label canary or
// holds a value shorter than minCanaryLen, and when the positive control (the first canary inside
// controlText) is not found, so a broken scanner cannot pass. A missing or unreadable root is an
// error, never an empty result.
func Scan(canaries Canaries, roots []string) ([]Hit, error) {
	if err := validate(canaries); err != nil {
		return nil, err
	}
	if !match(controlText(canaries[0].Value), []byte(canaries[0].Value)) {
		return nil, errors.New("smoke: the positive control failed: the scanner did not find a canary in its control text")
	}
	var hits []Hit
	for _, root := range roots {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("smoke: scan root %s is missing", root)
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil
			}
			found, err := scanFile(path, canaries)
			hits = append(hits, found...)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("smoke: scan %s: %w", root, err)
		}
	}
	return hits, nil
}

// validate checks the canary list as Scan documents; the label canary must come first, so the
// positive control uses the run canary.
func validate(canaries Canaries) error {
	if len(canaries) == 0 {
		return errors.New("smoke: the canary list is empty")
	}
	if canaries[0].Label != LabelCanary {
		return errors.New("smoke: the canary list does not start with the label canary")
	}
	for i, c := range canaries {
		if len(c.Value) < minCanaryLen {
			return fmt.Errorf("smoke: canary %d (%s) is shorter than %d characters", i+1, c.Label, minCanaryLen)
		}
	}
	return nil
}

// scanFile returns the hits of one file, line by line; a line may be long (one OTLP batch).
func scanFile(path string, canaries Canaries) ([]Hit, error) {
	f, err := os.Open(path) // #nosec G304 -- the files of the scan roots are the input
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	var hits []Hit
	for n := 1; ; n++ {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			for _, c := range canaries {
				if match(line, []byte(c.Value)) {
					hits = append(hits, Hit{File: path, Line: n, Label: c.Label})
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return hits, nil
		}
		if err != nil {
			return hits, err
		}
	}
}

// otlpValue, otlpAttr and otlpResource are the parts of OTLP JSON (the file exporter of the
// collector, one ExportRequest per line) that CheckTelemetry reads.
type otlpValue struct {
	StringValue string `json:"stringValue"`
}

type otlpAttr struct {
	Key   string    `json:"key"`
	Value otlpValue `json:"value"`
}

type otlpResource struct {
	Attributes []otlpAttr `json:"attributes"`
}

// attr returns the string value of key in attrs, or "".
func attr(attrs []otlpAttr, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.StringValue
		}
	}
	return ""
}

// otlpLine is one line of traces.jsonl, logs.jsonl or metrics.jsonl.
type otlpLine struct {
	ResourceSpans []struct {
		Resource   otlpResource `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				Attributes []otlpAttr `json:"attributes"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
	ResourceLogs []struct {
		Resource  otlpResource `json:"resource"`
		ScopeLogs []struct {
			LogRecords []json.RawMessage `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
	ResourceMetrics []struct {
		Resource     otlpResource `json:"resource"`
		ScopeMetrics []struct {
			Metrics []map[string]json.RawMessage `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}

// telemetryCount is what CheckTelemetry found of the service.
type telemetryCount struct {
	routeSpans, logRecords, metricPoints int
}

// CheckTelemetry fails unless the OTLP JSON export in otelDir (traces.jsonl, logs.jsonl,
// metrics.jsonl) holds, for resources with service.name = serviceName, at least one span whose
// http.route is route, at least one log record and at least one metric data point. An empty
// export cannot pass the scan this way. The error names what is missing.
func CheckTelemetry(otelDir, serviceName, route string) error {
	var n telemetryCount
	for _, name := range []string{"traces.jsonl", "logs.jsonl", "metrics.jsonl"} {
		b, err := os.ReadFile(filepath.Join(otelDir, name)) // #nosec G304 -- fixed names below the input
		if err != nil {
			return fmt.Errorf("smoke: collector export %s: %w", name, err)
		}
		for i, line := range bytes.Split(b, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var l otlpLine
			if err := json.Unmarshal(line, &l); err != nil {
				return fmt.Errorf("smoke: collector export %s line %d is no OTLP JSON", name, i+1)
			}
			n.add(l, serviceName, route)
		}
	}
	var missing []string
	if n.routeSpans == 0 {
		missing = append(missing, "a span with http.route "+route)
	}
	if n.logRecords == 0 {
		missing = append(missing, "a log record")
	}
	if n.metricPoints == 0 {
		missing = append(missing, "a metric data point")
	}
	if len(missing) > 0 {
		return fmt.Errorf("smoke: the collector export of %s lacks %s", serviceName, strings.Join(missing, ", "))
	}
	return nil
}

// metricKinds are the members of an OTLP metric that hold data points.
var metricKinds = []string{"sum", "gauge", "histogram", "exponentialHistogram", "summary"}

// add counts the spans, log records and metric points of l that belong to serviceName.
func (n *telemetryCount) add(l otlpLine, serviceName, route string) {
	for _, rs := range l.ResourceSpans {
		if attr(rs.Resource.Attributes, "service.name") != serviceName {
			continue
		}
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				if attr(s.Attributes, "http.route") == route {
					n.routeSpans++
				}
			}
		}
	}
	for _, rl := range l.ResourceLogs {
		if attr(rl.Resource.Attributes, "service.name") != serviceName {
			continue
		}
		for _, sl := range rl.ScopeLogs {
			n.logRecords += len(sl.LogRecords)
		}
	}
	for _, rm := range l.ResourceMetrics {
		if attr(rm.Resource.Attributes, "service.name") != serviceName {
			continue
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				n.metricPoints += dataPoints(m)
			}
		}
	}
}

// dataPoints returns the number of data points of one OTLP metric.
func dataPoints(m map[string]json.RawMessage) int {
	for _, kind := range metricKinds {
		raw, ok := m[kind]
		if !ok {
			continue
		}
		var body struct {
			DataPoints []json.RawMessage `json:"dataPoints"`
		}
		if json.Unmarshal(raw, &body) == nil {
			return len(body.DataPoints)
		}
	}
	return 0
}
