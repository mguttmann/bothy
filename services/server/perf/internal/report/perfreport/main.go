// SPDX-License-Identifier: Apache-2.0

// Command perfreport is the last step of make perf: it reads .cache/perf/results.json, prints the
// profile mode and the table of every target, and exits 1 when the run does not end green
// (report.Exit: a missed or missing target, mode below, or a database without fsync or
// synchronous_commit). It takes no arguments.
//
// Example:
//
//	cd services/server && go run ./perf/internal/report/perfreport
package main

import (
	"fmt"
	"os"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

func main() {
	f, err := report.Read(report.ResultsPath())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "perf: no results: %v\n", err)
		os.Exit(1)
	}
	os.Exit(report.Exit(f, report.Required, os.Stdout))
}
