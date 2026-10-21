// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// View is the SDK view of the server's meter provider over All (signals.md "Cardinality" item 5).
// platform/otel installs it in Setup.
func View(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
	return NewView(All())(i)
}

// NewView returns the view over specs: an instrument of a dropped library scope (otelpgx) and
// every instrument whose name specs do not declare is dropped, because what the catalogue does
// not name is not sent (signals.md, header). A declared instrument keeps its name, unit and
// description, passes only the attribute keys of its spec (otelhttp, for example, also sets
// server.address from the client's Host header, an open set) and, for a histogram, aggregates
// with the explicit bucket bounds of its spec, which overrides any bounds the library chose.
//
// Example:
//
//	mp := sdkmetric.NewMeterProvider(sdkmetric.WithView(metrics.NewView(metrics.All())))
func NewView(specs []Spec) sdkmetric.View {
	streams := make(map[string]sdkmetric.Stream, len(specs))
	for _, s := range specs {
		keys := make([]attribute.Key, len(s.Attrs))
		for i, a := range s.Attrs {
			keys[i] = attribute.Key(a.Key)
		}
		st := sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter(keys...)}
		if s.Kind == KindHistogram {
			st.Aggregation = sdkmetric.AggregationExplicitBucketHistogram{Boundaries: append([]float64(nil), s.Buckets...)}
		}
		streams[s.Name] = st
	}
	return func(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
		st, ok := streams[i.Name]
		if !ok || droppedScopes[i.Scope.Name] {
			return sdkmetric.Stream{Aggregation: sdkmetric.AggregationDrop{}}, true
		}
		st.Name, st.Description, st.Unit = i.Name, i.Description, i.Unit
		return st, true
	}
}
