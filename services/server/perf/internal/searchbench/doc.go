// SPDX-License-Identifier: Apache-2.0

// Package searchbench is the library of the search load test perf/search (brief M3-16,
// docs/spec/14-search.md section 7): the named parameters of the run (Params, DefaultParams), the
// plan derived from them, the configuration defaults and the values of 14 section 15 (Plan), the
// deterministic synthetic vocabulary and corpus (Vocabulary, Corpus), the queries of the phases
// (Queries), the bulk loader of the content rows with its consistency checks (Load), the rewrite
// and size measurement of the search vector (MeasureRewrite), the samples and statistics of a
// phase (Samples) and the response checks that decide whether a measurement counts (CheckSearch,
// CheckPrefix, CheckProbe).
//
// The package is untagged so make test runs its unit tests without a container; the database
// parts are tested by the small-corpus tests of perf/internal/searchworld. The constants are
// parameters of a test, not limits of the product: every product limit they depend on is read by
// its key name, and perf/README.md lists their values.
//
// Example:
//
//	d, err := searchbench.LoadDefaults()
//	plan, err := searchbench.NewPlan(searchbench.DefaultParams(), n, d)
//	corpus, err := searchbench.NewCorpus(plan, start)
//	msg := corpus.Message(42)
package searchbench
