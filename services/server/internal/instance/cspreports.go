// SPDX-License-Identifier: Apache-2.0

package instance

import "context"

// CSPReports is the incoming port of submitCspReport (R-AZ-33, docs/spec/19-sicherheit.md 5
// rule 6): it accepts the body of a Content Security Policy violation report of the web console,
// logs only the directive, the disposition and the origin of the blocked URL of each violation,
// counts bothy.console.csp_violations and stores nothing. app.CSPReports implements it.
type CSPReports interface {
	// Accept authorizes console.csp_report for the principal of ctx (anonymous without one) and
	// handles body: legacy is true for application/csp-report, false for
	// application/reports+json. Errors: the decision of the authorization, or 400
	// validation_failed for a body that is not of its format.
	Accept(ctx context.Context, legacy bool, body []byte) error
}
