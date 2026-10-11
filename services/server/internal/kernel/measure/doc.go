// SPDX-License-Identifier: Apache-2.0

// Package measure declares the Recorder of the own measurement (Record(metric, key, value)):
// in-memory counters only, never blocking (docs/spec/25-visualisierung.md). The stats module
// implements it; messaging, runs, billing, agents, notify, gateway and platform/httpserver record
// through it.
package measure
