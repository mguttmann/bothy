// SPDX-License-Identifier: Apache-2.0

// Command perfreport is the last step of make perf and make bench-search: it reads
// .cache/perf/results.json, prints the profile mode and the table of every target, and exits 1
// when the run does not end green (report.Exit: a missed or missing target, mode below, or a
// database without fsync or synchronous_commit). The flag -required names the targets that must
// have a result, comma-separated; without it every target of report.Required must (make perf).
// make bench-search passes the two search targets, because it runs only perf/search.
//
// Example:
//
//	cd services/server && go run ./perf/internal/report/perfreport
//	cd services/server && go run ./perf/internal/report/perfreport -required search.p95_target,search.prefix_p95_target
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

func main() {
	only := flag.String("required", "", "comma-separated targets that must have a result (default: every target of make perf)")
	flag.Parse()
	required := report.Required
	if *only != "" {
		required = strings.Split(*only, ",")
	}
	f, err := report.Read(report.ResultsPath())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "perf: no results: %v\n", err)
		os.Exit(1)
	}
	os.Exit(report.Exit(f, required, os.Stdout))
}
