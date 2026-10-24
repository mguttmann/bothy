// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the billing module (M2-22): the booking of the cost reports
// of a finished run inside the unit of work of run.finish, the thread breakdown of
// getThreadCosts, the valid sums per level for the statistics (M6-05) and the price table with
// listPrices, createPrice and closePrice. Service implements billing.Costs and billing.Prices.
//
// Example:
//
//	svc, err := app.New(app.Config{ShowPersonalEstimates: false}, app.Deps{Repo: repo, UoW: uow,
//		Authorizer: az, Audit: rec, Clock: clock.System()})
package app
