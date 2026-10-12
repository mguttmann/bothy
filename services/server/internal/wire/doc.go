// SPDX-License-Identifier: Apache-2.0

// Package wire builds the object graph of each process role (api, worker, all): it connects the
// module ports with their app and adapter packages and the platform packages, and registers the
// in-transaction subscribers in the allowed direction (docs/spec/02-architecture.md, "Code layout"
// and "In-Tx events"). Only cmd imports it; it is the one place that knows every module.
package wire
