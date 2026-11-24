// SPDX-License-Identifier: Apache-2.0

// Package app holds the application layer of the files module: the use cases of uploads, scan,
// images, attachments, downloads and tickets, transcripts, text extraction, cleanup and purge,
// import, the personal data participant and the export packages, and the outgoing ports they
// need. One of them is ObjectStore with its value types and sentinel errors
// (docs/spec/12-files.md "4.1 Port"): adapters/fs and adapters/s3 implement it, and only the files
// module sees it; other modules reach files through the root package internal/files (02 "4.3").
//
// Example:
//
//	rc, err := store.Get(ctx, key, &app.ByteRange{Offset: 0, Length: 1024})
//	if err != nil {
//		return err // errors.Is(err, app.ErrNotFound) for a missing object
//	}
//	defer rc.Close()
package app
