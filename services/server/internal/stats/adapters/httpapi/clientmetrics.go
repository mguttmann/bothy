// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the stats module as part of
// rest.StrictServerInterface: submitClientMetrics (M4-31) of docs/api/openapi.yaml. The use case
// authorizes; the handler only converts and passes errors to the problem writer of the chain
// unchanged. The request validator of the chain has already refused a body outside
// ClientMetricsBatch with 400 (SEC-IN-1).
//
// Example:
//
//	h := httpapi.New(clientMetrics)
//	built, err := routes.Build(wire.Handlers{ClientMetrics: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/stats"
)

// ClientMetrics implements the REST operation of the app metrics.
type ClientMetrics struct {
	cm stats.ClientMetrics
}

// New returns the handler over the port cm.
func New(cm stats.ClientMetrics) *ClientMetrics { return &ClientMetrics{cm: cm} }

// SubmitClientMetrics implements rest.StrictServerInterface (POST /client-metrics): 202 without a
// body once the use case accepted the batch, also when it discarded entries.
func (h *ClientMetrics) SubmitClientMetrics(ctx context.Context, req rest.SubmitClientMetricsRequestObject) (rest.SubmitClientMetricsResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "the body is missing")
	}
	if err := h.cm.Submit(ctx, batch(*req.Body)); err != nil {
		return nil, err
	}
	return rest.SubmitClientMetrics202Response{}, nil
}

// batch converts the request body; absent density and theme become "".
func batch(b rest.ClientMetricsBatch) stats.ClientMetricsBatch {
	out := stats.ClientMetricsBatch{Platform: string(b.Platform), AppVersion: b.AppVersion, Metrics: make([]stats.ClientMetric, len(b.Metrics))}
	if b.Density != nil {
		out.Density = string(*b.Density)
	}
	if b.Theme != nil {
		out.Theme = string(*b.Theme)
	}
	for i, m := range b.Metrics {
		c := stats.ClientMetric{Name: m.Name, Kind: string(m.Kind), CloseCode: m.CloseCode, Value: m.Value}
		if m.Buckets != nil {
			for _, bk := range *m.Buckets {
				c.Buckets = append(c.Buckets, stats.ClientMetricBucket{LE: bk.Le, Count: bk.Count})
			}
		}
		out.Metrics[i] = c
	}
	return out
}
