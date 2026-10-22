// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the importer module (M8-03): the sources with their state
// machine and mapping, the ownership list, the batches of structure items with origin keys and the
// cursor (docs/spec/22-import-migration.md sections 5, 7, 8 and 12). Service implements
// importer.Imports; Guard implements the source port chat.ReadOnlyGuard. Store is the outgoing
// port over import_sources and import_items, implemented by adapters/postgres.
package app
