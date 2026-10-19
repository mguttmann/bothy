// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Sink names one output of the server that Scan reads.
type Sink = string

// The sinks of the brief, numbered as in its design notes.
const (
	// SinkLog is 1: the structured log stream (stdout JSON lines and OTLP log records) and
	// whatever libraries write through the standard logger.
	SinkLog Sink = "log"
	// SinkTrace is 2: span names, attributes, events, status, links and resources.
	SinkTrace Sink = "trace"
	// SinkMetric is 3: metric names, attribute sets and exemplars.
	SinkMetric Sink = "metric"
	// SinkAudit is 4: every column of every audit_events row.
	SinkAudit Sink = "audit"
	// SinkError is 5: body and headers of every response with status 400 or higher.
	SinkError Sink = "error"
	// SinkForeign is 6: frames, event pages and snapshots of observers without read permission
	// on the scenario's scopes.
	SinkForeign Sink = "foreign"
)

// Field is one text of a record at a location: an attribute key, a JSON pointer or a header
// name.
type Field struct {
	Location string
	Text     string
	// Echo marks a field of an error response that returns to the caller what it sent or may
	// read, as the contract defines it: current of version_conflict and base_version_conflict
	// (06 "Errors" item 3) and the pointer of an unknown_member error, which names the member the
	// caller sent (W7, M1-18/3). Content there counts as a success response (ruling M1-69/2).
	Echo bool
}

// Record is one captured output: a log line, a span, a metric point, an audit row, an error
// response or an observer frame, split into its fields.
type Record struct {
	// Sink is the output the record came from.
	Sink Sink
	// Source names the record within its sink (for example "line 12", "span http.server",
	// "listEvents page 3"); it starts the location of a finding.
	Source string
	// Viewer is the observer of a foreign record (Sam, Robin); empty for other sinks.
	Viewer string
	// Fields are the texts of the record.
	Fields []Field
}

// Sinks collects the records of all outputs during a run. It is safe for concurrent use.
type Sinks struct {
	mu      sync.Mutex
	records []Record
}

// NewSinks returns an empty collection.
func NewSinks() *Sinks { return &Sinks{} }

// Add appends r.
func (s *Sinks) Add(r Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
}

// Records returns a copy of the collected records.
func (s *Sinks) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.records...)
}

// Count returns the number of records of sink.
func (s *Sinks) Count(sink Sink) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.records {
		if r.Sink == sink {
			n++
		}
	}
	return n
}

// AddJSON adds one record of sink whose fields are the leaves of the JSON document b (JSON
// pointers below prefix); a text that is no JSON becomes one field at prefix.
func (s *Sinks) AddJSON(sink Sink, source, viewer, prefix string, b []byte) {
	s.Add(Record{Sink: sink, Source: source, Viewer: viewer, Fields: JSONFields(prefix, b)})
}

// JSONFields returns the leaves of the JSON document b as fields with their JSON pointer below
// prefix: object keys are leaves too, so a canary used as a member name is found. Text that is
// not exactly one JSON value is one field at prefix.
func JSONFields(prefix string, b []byte) []Field {
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil || len(bytes.TrimSpace(b[dec.InputOffset():])) > 0 {
		// No JSON, or text after the first value (a standard logger line with its date prefix):
		// the whole text is one field, so nothing after a leading value goes unscanned.
		return []Field{{Location: prefix, Text: string(b)}}
	}
	var out []Field
	flatten(prefix, v, &out)
	return out
}

// flatten appends the leaves of v below pointer p.
func flatten(p string, v any, out *[]Field) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := p + "/" + escapePointer(k)
			*out = append(*out, Field{Location: child + "#key", Text: k})
			flatten(child, x[k], out)
		}
	case []any:
		for i, e := range x {
			flatten(p+"/"+strconv.Itoa(i), e, out)
		}
	case string:
		*out = append(*out, Field{Location: p, Text: x})
	case json.Number:
		*out = append(*out, Field{Location: p, Text: x.String()})
	case nil, bool:
	default:
		*out = append(*out, Field{Location: p, Text: fmt.Sprint(x)})
	}
}

// escapePointer escapes a member name for a JSON pointer (RFC 6901).
func escapePointer(k string) string {
	return strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
}

// LineWriter is an io.Writer that turns every line written to it into a record of sink (JSON
// lines are split into their fields). Partial lines wait for their newline; Flush adds the rest.
type LineWriter struct {
	sinks  *Sinks
	sink   Sink
	source string

	mu   sync.Mutex
	buf  bytes.Buffer
	line int
}

// NewLineWriter returns a LineWriter that adds to sinks as sink; source prefixes the line
// numbers in finding locations.
func NewLineWriter(sinks *Sinks, sink Sink, source string) *LineWriter {
	return &LineWriter{sinks: sinks, sink: sink, source: source}
}

// Write implements io.Writer.
func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		i := bytes.IndexByte(w.buf.Bytes(), '\n')
		if i < 0 {
			return len(p), nil
		}
		line := append([]byte(nil), w.buf.Next(i + 1)[:i]...)
		w.add(line)
	}
}

// Flush adds an unterminated last line.
func (w *LineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		w.add(append([]byte(nil), w.buf.Bytes()...))
		w.buf.Reset()
	}
}

// Lines returns the number of lines added.
func (w *LineWriter) Lines() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.line
}

func (w *LineWriter) add(line []byte) {
	w.line++
	w.sinks.AddJSON(w.sink, w.source+" line "+strconv.Itoa(w.line), "", "", line)
}
