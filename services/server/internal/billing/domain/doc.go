// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the billing module (M2-22): the validation of the cost
// reports of a finished run, the price set of provider, model and time with the exact
// computation of an amount (docs/spec/10-backend-policy.md section 12.1, R-DM-23), the
// booking of one entry per report, the valid sums per billing mode and currency without
// conversion, the display rule of section 12.2, the validation of a price row and the decision of
// closePrice. It knows no clock, database or other module.
//
// Validation errors are kernel/errs errors of code validation_failed whose field errors name the
// field of the request body; they never repeat the rejected value.
//
// Example:
//
//	priced, err := domain.Compute(report, domain.PriceSet(rows, report.Provider, report.Model, now))
package domain
