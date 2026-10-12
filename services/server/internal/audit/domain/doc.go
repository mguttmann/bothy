// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the audit module (ADR 0021): the row of audit_events,
// its canonical form after RFC 8785 (SEC-AUD-4) and the hash chain, the closed schema
// audit_details.v1 (SEC-AUD-5), the IP pseudonyms with their keys per period (20 "Audit-Log und
// IP-Adressen") and the aggregation of identical denials (SEC-AUD-6, R-AZ-04). It uses the
// standard library and the kernel-core packages only; time is always passed in, never read.
package domain
