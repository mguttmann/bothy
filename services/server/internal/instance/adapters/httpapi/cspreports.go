// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// CSPReports implements submitCspReport of rest.StrictServerInterface over instance.CSPReports
// (M6-03, R-AZ-33). The HTTP chain has checked the media type, the JSON guard and the limit
// http.max_json_body_bytes (body kind report); the handler hands the raw body to the use case and
// answers 204.
type CSPReports struct {
	reports instance.CSPReports
}

// NewCSPReports returns the handler over the use case.
func NewCSPReports(reports instance.CSPReports) *CSPReports { return &CSPReports{reports: reports} }

// SubmitCspReport implements rest.StrictServerInterface (POST /csp-reports). The generated server
// sets Body for application/csp-report and the decoded array for application/reports+json; the
// handler uses that only to learn the format. The use case reads the fields it logs from the bytes
// the chain checked (httpserver.RawJSONBody), for both formats alike.
func (h *CSPReports) SubmitCspReport(ctx context.Context, req rest.SubmitCspReportRequestObject) (rest.SubmitCspReportResponseObject, error) {
	body, ok := httpserver.RawJSONBody(ctx)
	if !ok {
		return nil, errs.New(errs.CodeValidationFailed, "Body is required.")
	}
	if err := h.reports.Accept(ctx, req.Body != nil, body); err != nil {
		return nil, err
	}
	return rest.SubmitCspReport204Response{}, nil
}
