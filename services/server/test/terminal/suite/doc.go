// SPDX-License-Identifier: Apache-2.0

// Package suite is the data-driven conformance suite of the terminal (brief M2-48 Design notes 2,
// 4, 7 and 8): it loads the catalogue (catalogue.yaml), the profiles (profiles.yaml) and the
// scenario files (scenarios/<id>.yaml) of services/server/test/terminal and runs every case of a
// scenario in its own world against the Compose stack of package stack, real host services in
// Linux containers and the Go test device of package device. Every expectation is literal data of
// the scenario files; the suite never calls server or host code to compute one. A failed
// expectation names scenario, case, step index, expected and observed codes, states and counts,
// never a body, a token or terminal output. It belongs to the architecture component testsupport
// and is never imported by production code.
package suite
