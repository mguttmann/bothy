// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// LogCSPViolation is the message of the log record of one accepted violation report.
const LogCSPViolation = "csp violation reported"

// The attribute keys of that record (docs/observability/signals.md 5.3, the constants of
// platform/otel/attr, which app may not import); the log handler drops every other key.
const (
	keyLogCSPDirective     = "bothy.csp.directive"
	keyLogCSPDisposition   = "bothy.csp.disposition"
	keyLogCSPBlockedOrigin = "bothy.csp.blocked_origin"
)

// detailNotCSPReport is the detail of a body that is not of its media type; it names the rule,
// never the content.
const detailNotCSPReport = "Body is not a CSP violation report of its media type."

// CSPViolationCounter is the outgoing port of the metric bothy.console.csp_violations
// (adapters/otelmetrics).
type CSPViolationCounter interface {
	// Count adds n reports with the directive and the disposition of a domain.CSPViolation.
	Count(ctx context.Context, directive, disposition string, n int64)
}

// CSPReports implements instance.CSPReports. It is safe for concurrent use.
type CSPReports struct {
	az         authorize.Authorizer
	log        *slog.Logger
	counter    CSPViolationCounter
	maxRecords int
}

// Compile-time check: CSPReports is the incoming port.
var _ instance.CSPReports = (*CSPReports)(nil)

// NewCSPReports returns the use case over the Authorizer az, the logger log (nil discards), the
// counter and maxRecords, console.csp_report_max_records: the most log records one request
// writes. Errors: a nil az or counter, or maxRecords below 1.
//
// Example:
//
//	reports, err := app.NewCSPReports(az, logger, counter, maxRecords)
func NewCSPReports(az authorize.Authorizer, log *slog.Logger, counter CSPViolationCounter, maxRecords int) (*CSPReports, error) {
	if az == nil || counter == nil {
		return nil, errors.New("app: the CSP reports need an Authorizer and a counter")
	}
	if maxRecords < 1 {
		return nil, errors.New("app: console.csp_report_max_records must be at least 1")
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &CSPReports{az: az, log: log, counter: counter, maxRecords: maxRecords}, nil
}

// Accept implements instance.CSPReports: the counter counts every violation; identical violations
// share one Info record, and one request writes at most console.csp_report_max_records records, so
// a large body within the rate limit raises the counter, not the log volume (brief M6-03 Design
// note 4). A record carries the directive, the disposition and the blocked origin only (R-AZ-33);
// the document URL, the sample and every other member of the report are never read into one.
func (c *CSPReports) Accept(ctx context.Context, legacy bool, body []byte) error {
	a, ok := actor.FromContext(ctx)
	if !ok {
		a = actor.Anonymous()
	}
	d, err := c.az.Authorize(ctx, a, authorize.ActionConsoleCspReport, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return err
	}
	if err := d.Err(); err != nil {
		return err
	}
	format := domain.CSPReportingAPI
	if legacy {
		format = domain.CSPLegacy
	}
	violations, err := domain.ParseCSPReports(format, body)
	if err != nil {
		return errs.New(errs.CodeValidationFailed, detailNotCSPReport)
	}
	for i, v := range domain.SummarizeCSPViolations(violations) {
		c.counter.Count(ctx, v.Directive, v.Disposition, v.Count)
		if i >= c.maxRecords {
			continue
		}
		c.log.LogAttrs(ctx, slog.LevelInfo, LogCSPViolation,
			slog.String(keyLogCSPDirective, v.Directive),
			slog.String(keyLogCSPDisposition, v.Disposition),
			slog.String(keyLogCSPBlockedOrigin, v.BlockedOrigin))
	}
	return nil
}
