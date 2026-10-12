// SPDX-License-Identifier: Apache-2.0

// Package app holds the application layer of the files module. So far it holds the outgoing
// port ObjectStore with its value types and sentinel errors (docs/spec/12-dateien.md "4.1 Port"):
// adapters implement it (adapters/fs, later adapters/s3), and only the files module sees it; other
// modules reach files through the root package internal/files (02 "4.3").
//
// Example:
//
//	rc, err := store.Get(ctx, key, &app.ByteRange{Offset: 0, Length: 1024})
//	if errors.Is(err, app.ErrNotFound) { ... }
//	defer rc.Close()
package app
