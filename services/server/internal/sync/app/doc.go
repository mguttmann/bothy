// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the sync module for the event log (M1-16): the Appender of
// kernel/eventlog, which validates durable events and buffers them in the unit of work; the
// PhaseHook of phase sync, which numbers them with event_seq, writes them and notifies the log
// tails before the commit (docs/spec/02-architecture.md "Flow" step 3.2); and the partition
// maintenance of event_log. The outgoing ports (Writer, Partitions, Correlator) are implemented
// in adapters/postgres and adapters/telemetry.
package app
