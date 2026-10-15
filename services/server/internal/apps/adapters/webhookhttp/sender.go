// SPDX-License-Identifier: Apache-2.0

// Package webhookhttp sends the outgoing webhooks of the apps module (M2-26): one Standard
// Webhooks POST per attempt through one egress client built with purpose webhook, the hosts of
// webhook.allowed_hosts, no redirects and the time limit webhook.delivery_timeout (SEC-NET-1 to
// SEC-NET-5). It reads only the status and Retry-After, discards the body, and opens the span
// webhook.deliver with server.address, http.response.status_code and bothy.outcome. Logs, spans
// and errors carry the host only, never path, query, headers or body (SEC-LOG-1 to SEC-LOG-4).
//
// Example:
//
//	s, err := webhookhttp.New(egressCfg, hosts, cfg.DeliveryTimeout, egress.Deps{Logger: log, Metrics: m}, nil)
//	res, err := s.Send(ctx, req)
package webhookhttp

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Purpose is the egress purpose of the webhook client (bothy.egress.purpose).
const Purpose = "webhook"

// SpanName is the client span of one attempt (docs/observability/signals.md 5.2).
const SpanName = "webhook.deliver"

// instrumentationName names the tracer of this package.
const instrumentationName = "github.com/OWNER/bothy/services/server/internal/apps/adapters/webhookhttp"

// The closed list of bothy.outcome on webhook.deliver (D-M2-26-17).
const (
	OutcomeSucceeded = "succeeded"
	OutcomeThrottled = "throttled"
	OutcomeGone      = "gone"
	OutcomeFailed    = "failed"
	OutcomeBlocked   = "blocked"
)

// The headers of Standard Webhooks (webhooks.md section 2).
const (
	headerID        = "webhook-id"
	headerTimestamp = "webhook-timestamp"
	headerSignature = "webhook-signature"
)

// Sender implements app.WebhookSender over one egress client. It is safe for concurrent use.
type Sender struct {
	client *egress.Client
	tp     func() trace.TracerProvider
	ua     string
}

// Compile-time check: Sender is the outgoing port of the deliveries.
var _ app.WebhookSender = (*Sender)(nil)

// New builds the egress client of the webhooks: purpose webhook, the host policy of hosts (the
// rule of domain.ValidateTarget), RedirectNever and timeout webhook.delivery_timeout; tp is the
// tracer provider, nil for the global one. Errors: those of egress.New.
func New(cfg egress.Config, hosts []domain.AllowedHost, timeout time.Duration, deps egress.Deps, tp trace.TracerProvider) (*Sender, error) {
	allows := domain.AllowsHost(hosts)
	client, err := egress.New(cfg, egress.Options{
		Purpose: Purpose, Redirects: egress.RedirectNever, Timeout: timeout,
		Hosts: func(_ context.Context, scheme, host string, port uint16) bool { return allows(scheme, host, port) },
	}, deps)
	if err != nil {
		return nil, err
	}
	s := &Sender{client: client, tp: otel.GetTracerProvider, ua: "chat-server-webhooks/" + buildinfo.Version()}
	if tp != nil {
		s.tp = func() trace.TracerProvider { return tp }
	}
	return s, nil
}

// Send implements app.WebhookSender: one POST of req with Content-Type application/json, the
// User-Agent of the server and the three headers of Standard Webhooks. A response is returned with
// its status and Retry-After, its body discarded; no response is a transport error, a refusal of
// the egress client converted to *domain.BlockedError.
func (s *Sender) Send(ctx context.Context, req app.SignedRequest) (app.SendResult, error) {
	ctx, span := s.tp().Tracer(instrumentationName).Start(ctx, SpanName, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String(attr.ServerAddress, req.Host)))
	defer span.End()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		// The address passed ValidateTarget; a request that cannot be built is refused like a
		// URL the egress client does not accept.
		return s.fail(span, &domain.BlockedError{Reason: string(egress.ReasonURLNotAllowed)})
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", s.ua)
	r.Header.Set(headerID, req.WebhookID)
	r.Header.Set(headerTimestamp, strconv.FormatInt(req.Timestamp, 10))
	r.Header.Set(headerSignature, req.Signature)
	resp, err := s.client.Do(r)
	if err != nil {
		var blocked *egress.BlockedError
		if errors.As(err, &blocked) {
			return s.fail(span, &domain.BlockedError{Reason: string(blocked.Reason)})
		}
		return s.fail(span, err)
	}
	// The body is never read or stored (webhooks.md section 4 point 5).
	_ = resp.Body.Close()
	o := domain.Classify(resp.StatusCode, nil)
	span.SetAttributes(attribute.Int(attr.HTTPResponseStatusCode, resp.StatusCode), attribute.String(attr.Outcome, outcomeOf(o)))
	if o.Kind != domain.OutcomeSucceeded {
		span.SetStatus(codes.Error, "")
	}
	return app.SendResult{StatusCode: resp.StatusCode, RetryAfter: resp.Header.Get("Retry-After")}, nil
}

// fail ends an attempt without response: outcome blocked for a refusal, failed otherwise.
func (s *Sender) fail(span trace.Span, err error) (app.SendResult, error) {
	outcome := OutcomeFailed
	var blocked *domain.BlockedError
	if errors.As(err, &blocked) {
		outcome = OutcomeBlocked
	}
	span.SetAttributes(attribute.String(attr.Outcome, outcome))
	span.SetStatus(codes.Error, "")
	return app.SendResult{}, err
}

// outcomeOf maps the outcome of an answer to bothy.outcome.
func outcomeOf(o domain.Outcome) string {
	switch o.Kind {
	case domain.OutcomeSucceeded:
		return OutcomeSucceeded
	case domain.OutcomeThrottled:
		return OutcomeThrottled
	case domain.OutcomeGone:
		return OutcomeGone
	case domain.OutcomeFailed:
		return OutcomeFailed
	default:
		return OutcomeFailed
	}
}
