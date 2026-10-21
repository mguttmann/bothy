// SPDX-License-Identifier: Apache-2.0

// Package chaos holds the chaos tests of the event path (brief M1-72, ADR 0063): losing the LISTEN
// connection of the log tail, restarting the database during the tail, and killing the server
// during commits, each under load from several writers with gateway receivers, followed by the
// invariants of invariants.go. Everything but this file carries the build tag perf; make perf runs
// the tests, make check only compiles and vets them. README.md describes them.
package chaos
