// SPDX-License-Identifier: Apache-2.0

// Package importer is the port of the importer module: import sources, batches with provenance
// keys (import.apply), the ownership list and the read-only state of mirrored channels; the dry
// run lives in the import tool. Other modules import only this package
// (docs/spec/02-architektur.md, "Schichten im Modul").
package importer
