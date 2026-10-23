// SPDX-License-Identifier: Apache-2.0

// Package search is the port of the search module: full-text search over messages, thread titles
// and files with permission filter and ranking, and the prefix search over thread titles of the
// quick switcher (docs/spec/14-search.md). The module owns no tables; its coordinator reads
// through the read operations of messaging, chat and files. Other modules import only this
// package (docs/spec/02-architecture.md, "Layers within a module").
package search
