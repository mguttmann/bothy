// SPDX-License-Identifier: Apache-2.0

package files

import (
	"context"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StoreExport is the input of ExportPackages.Store: one finished export package of
// docs/spec/20-privacy.md section 5.2.
type StoreExport struct {
	// SubjectID is the person the package belongs to; the file row names it as uploaded_by.
	SubjectID ids.ID
	// Name is the original name of the package, export-<user_id>-<date>.zip.
	Name string
	// SizeBytes is the exact length of Body.
	SizeBytes int64
	// SHA256 is the lower-case hex SHA-256 of Body.
	SHA256 string
	// Body streams the package; Store reads it once to the end.
	Body io.Reader
}

// ExportContent is an opened export package.
type ExportContent struct {
	// Name is the original name of the package.
	Name string
	// SizeBytes is its length; SHA256 its lower-case hex SHA-256.
	SizeBytes int64
	SHA256    string
	// Body streams the bytes; the caller closes it.
	Body io.ReadCloser
}

// ExportPackages is the inbound port through which the module privacy stores, opens and deletes
// the export packages of 20 section 5.2: files with purpose gdpr_export, never visible through
// the file operations (their only path is gdpr.export.download, 05 section 2.2). internal/wire
// binds the implementation of the module files. Errors are kernel/errs errors.
//
// Example:
//
//	id, err := pkgs.Store(ctx, files.StoreExport{SubjectID: alex, Name: name, SizeBytes: n, SHA256: sum, Body: f})
//	c, err := pkgs.Open(ctx, id)
//	defer c.Body.Close()
type ExportPackages interface {
	// Store checks Body against SizeBytes and SHA256 while it writes the object and inserts the
	// file row with purpose gdpr_export, complete and without a scan (the server wrote it). It
	// runs outside any unit of work, because it writes the object (02 8.5), and returns the ID
	// of the new file. Errors: a length or digest that differs (internal), 503 storage_full, the
	// storage.
	Store(ctx context.Context, cmd StoreExport) (ids.ID, error)
	// Open opens the bytes of the export package id for the download. Errors: not_found for an
	// unknown or deleted file and for a file that is no export package; the storage.
	Open(ctx context.Context, id ids.ID) (ExportContent, error)
	// Delete marks the export package id deleted inside the caller's unit of work and queues
	// files.purge_objects, which removes the object within privacy.object_delete_delay (20
	// section 4.3). An unknown or already deleted file, or one that is no export package, is
	// left alone.
	Delete(ctx context.Context, id ids.ID) error
}
