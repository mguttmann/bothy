// SPDX-License-Identifier: Apache-2.0

// Package filestest is the test helper of the files module: the conformance suite
// RunObjectStoreConformance that every driver of the ObjectStore port runs
// (docs/spec/12-files.md "15. Tests", ADR 0010). Only _test.go files import it
// (02 "6. Code-Layout"); TestFilestestNotInBinary in internal/wire holds it out of the binary.
//
// A new driver runs the suite from its own test with a constructor of a fresh, empty store per
// case and the smallest part size it accepts:
//
//	filestest.RunObjectStoreConformance(t, newStore, filestest.Options{PartSize: 5 << 20})
package filestest
