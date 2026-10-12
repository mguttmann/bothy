// SPDX-License-Identifier: Apache-2.0

// Package fs is the file system driver of the ObjectStore port of the files module, the default
// driver (docs/spec/12-dateien.md "4.2 Dateisystem-Treiber (Standard)", file.storage.driver = fs).
//
// Objects are regular files below file.fs.root at the path of their key; directories have mode
// 0700 and files 0600. Writes go through a temporary file in <root>/.tmp, Sync, rename and Sync
// of the directory, so a reader never sees half an object. No symbolic link is followed at any
// level, and every key is checked against the key grammar before any file system call. Parts of a
// multipart upload lie at uploads/<file_id>/<n>. Encryption at rest is a task of operations.
//
// Example:
//
//	store, err := fs.Open(fs.Config{Root: "/var/lib/bothy/files"})
//	if err != nil {
//		return err
//	}
//	defer store.Close()
//	err = store.Put(ctx, key, body, size)
package fs
