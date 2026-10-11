// SPDX-License-Identifier: Apache-2.0

// Package eventlog declares the Appender for durable events, the Redactor that blanks the payloads
// of a deleted message and the EphemeralPublisher for short-lived events. sync implements Appender
// and Redactor, gateway implements EphemeralPublisher; every module appends, only messaging (and
// files later) redacts.
package eventlog
