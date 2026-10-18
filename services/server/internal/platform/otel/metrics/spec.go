// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"slices"
	"strings"
)

// Kind is the instrument kind of a Spec, the column "Typ" or "Type" of signals.md.
type Kind uint8

// The instrument kinds of the catalogue.
const (
	// KindCounter is a monotonic counter (Prometheus suffix _total).
	KindCounter Kind = iota + 1
	// KindUpDownCounter is a counter that also decreases.
	KindUpDownCounter
	// KindHistogram is a histogram with the explicit bounds of signals.md 4.7.
	KindHistogram
	// KindGauge is a gauge, synchronous or observable.
	KindGauge
)

// String returns the English name of the kind as the catalogue writes it.
func (k Kind) String() string {
	switch k {
	case KindCounter:
		return "Counter"
	case KindUpDownCounter:
		return "UpDownCounter"
	case KindHistogram:
		return "Histogram"
	case KindGauge:
		return "Gauge"
	default:
		return fmt.Sprintf("Kind(%d)", uint8(k))
	}
}

// Source names a closed value set that comes from generated or registered code instead of a
// literal list: the instrument's owner passes its values at construction (Values). A Source
// outside the constants below is an open set and fails Check.
type Source string

// The closed sources of attribute values (signals.md "Cardinality" item 1).
const (
	// SourceActions are the action ids of the action catalogue (authorize.AllActions).
	SourceActions Source = "actions"
	// SourceRateClasses are the rate-limit classes of 05 (ratelimit.Classes).
	SourceRateClasses Source = "rate_classes"
	// SourceEventTypes are the durable event types of 05 section 6 (authorize.AllEventTypes).
	SourceEventTypes Source = "event_types"
	// SourceRoutes are the route templates /api/v1 plus the path of each enabled operation,
	// /api/v1/gateway and "unmatched" (api/routes).
	SourceRoutes Source = "routes"
	// SourceHTTPStatus are the three-digit HTTP status codes 100 to 599.
	SourceHTTPStatus Source = "http_status"
	// SourceCloseCodes are the close codes of 04 "Close codes" as decimal strings.
	SourceCloseCodes Source = "close_codes"
	// SourceJobKinds are the job kinds registered at startup (platform/jobs registry).
	SourceJobKinds Source = "job_kinds"
	// SourceTables are the table names of the module register, read from the embedded
	// migrations (platform/pg).
	SourceTables Source = "tables"
)

// knownSources is the set of Source constants.
var knownSources = map[Source]bool{
	SourceActions: true, SourceRateClasses: true, SourceEventTypes: true, SourceRoutes: true,
	SourceHTTPStatus: true, SourceCloseCodes: true, SourceJobKinds: true, SourceTables: true,
}

// Other is the value an attribute takes when the recorded value lies outside its closed set; it
// is implicitly part of every set (signals.md "Cardinality").
const Other = "other"

// Attr is one attribute of an instrument: its key and its closed value set, either the literal
// Values or a Source. Optional attributes may be recorded as "", which leaves the key out of the
// data point (for example db.collection.name of a statement without a table).
type Attr struct {
	// Key is the attribute key as the catalogue names it.
	Key string
	// Values is the literal closed set; empty when Source names the set.
	Values []string
	// Source names a closed set the owner passes at construction; "" for a literal set.
	Source Source
	// Optional allows the empty value, recorded as an absent key.
	Optional bool
}

// Spec declares one instrument of the catalogue.
type Spec struct {
	// Name is the instrument name, for example bothy.gateway.close.
	Name string
	// Kind is the instrument kind.
	Kind Kind
	// Unit is the UCUM unit of the catalogue, for example s or {connection}.
	Unit string
	// Description is the English description of the instrument.
	Description string
	// Float makes counters and gauges float64 instruments; histograms are always float64.
	Float bool
	// Attrs are the attributes in recording order.
	Attrs []Attr
	// Buckets are the explicit bucket bounds of the histogram, a row of signals.md 4.7.
	Buckets []float64
	// Library is the instrumentation scope that creates the instrument when it is not created
	// through this package (otelhttp); "" for the server's own instruments.
	Library string
}

// Keys returns the attribute keys of s in order.
func (s Spec) Keys() []string {
	keys := make([]string, len(s.Attrs))
	for i, a := range s.Attrs {
		keys[i] = a.Key
	}
	return keys
}

// personKeys are attribute keys that identify a person or carry an ID (17 rule 8, signals.md 5.3
// "never on metrics").
var personKeys = map[string]bool{
	"bothy.principal.pseudo_id": true, "bothy.correlation_id": true, "bothy.event.seq": true,
	"client.address": true, "session.id": true,
}

// personPrefixes are key prefixes of person attributes of the semantic conventions.
var personPrefixes = []string{"user.", "enduser."}

// PersonOrIDKey reports whether key names a person or an ID and therefore never appears on a
// metric: keys ending in .id or _id, bothy.principal.pseudo_id, bothy.correlation_id,
// bothy.event.seq, user.*, enduser.*, client.address and session.id.
func PersonOrIDKey(key string) bool {
	if personKeys[key] || strings.HasSuffix(key, ".id") || strings.HasSuffix(key, "_id") {
		return true
	}
	for _, p := range personPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

// Check returns every structural problem of specs, nil when there is none: an empty or repeated
// name, an unknown kind, an empty unit or description, an attribute without a closed set (no
// literal values and no known Source), an attribute that names a person or an ID, a repeated
// attribute key, a histogram without ascending bounds and bounds on an instrument that is no
// histogram.
func Check(specs []Spec) []error {
	var problems []error
	seen := map[string]bool{}
	for _, s := range specs {
		if s.Name == "" {
			problems = append(problems, fmt.Errorf("an instrument has no name"))
			continue
		}
		if seen[s.Name] {
			problems = append(problems, fmt.Errorf("%s: declared twice", s.Name))
		}
		seen[s.Name] = true
		problems = append(problems, checkSpec(s)...)
	}
	return problems
}

// checkSpec returns the problems of one spec.
func checkSpec(s Spec) []error {
	var problems []error
	if s.Kind < KindCounter || s.Kind > KindGauge {
		problems = append(problems, fmt.Errorf("%s: unknown kind %d", s.Name, s.Kind))
	}
	if s.Unit == "" || s.Description == "" {
		problems = append(problems, fmt.Errorf("%s: unit and description are required", s.Name))
	}
	keys := map[string]bool{}
	for _, a := range s.Attrs {
		switch {
		case a.Key == "":
			problems = append(problems, fmt.Errorf("%s: an attribute has no key", s.Name))
		case keys[a.Key]:
			problems = append(problems, fmt.Errorf("%s: attribute %s declared twice", s.Name, a.Key))
		case PersonOrIDKey(a.Key):
			problems = append(problems, fmt.Errorf("%s: attribute %s identifies a person or carries an ID", s.Name, a.Key))
		case len(a.Values) == 0 && a.Source == "":
			problems = append(problems, fmt.Errorf("%s: attribute %s is open: it has neither literal values nor a closed source", s.Name, a.Key))
		case a.Source != "" && !knownSources[a.Source]:
			problems = append(problems, fmt.Errorf("%s: attribute %s names the unknown source %q", s.Name, a.Key, a.Source))
		}
		keys[a.Key] = true
	}
	switch {
	case s.Kind == KindHistogram && len(s.Buckets) == 0:
		problems = append(problems, fmt.Errorf("%s: histogram without bucket bounds", s.Name))
	case s.Kind == KindHistogram && !slices.IsSorted(s.Buckets):
		problems = append(problems, fmt.Errorf("%s: bucket bounds are not ascending", s.Name))
	case s.Kind != KindHistogram && len(s.Buckets) > 0:
		problems = append(problems, fmt.Errorf("%s: bucket bounds on a %s", s.Name, s.Kind))
	}
	return problems
}
