// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL adapter of the instance module: the sqlc queries under
// queries/ (generated code in sqlcgen/) and Repository, the implementation of the outgoing port
// app.Repository over the tables instance and sequence_counters. Statements run in the ambient
// transaction of a unit of work of platform/pg when there is one, otherwise on the pool.
//
// Only AdvanceCounter locks a counter row (UPDATE ... RETURNING); nothing in this module runs
// SELECT ... FOR UPDATE on the counters (04 "Pattern with a counter row").
package postgres
