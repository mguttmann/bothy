// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// ObjectStore is the outgoing port of the files module to its object storage
// (docs/spec/12-files.md "4.1 Port", ADR 0010). It has exactly these four operations: no
// listing, copying, batch deletion, versioning, bucket policies, lifecycle or encryption headers,
// because the server knows every key from the database. The file system driver (adapters/fs) and
// the S3 driver (adapters/s3) implement it; both pass filestest.RunObjectStoreConformance. The
// multipart operations that only the S3 part strategy uses are the separate port MultipartStore.
//
// Every operation checks the key against domain.ValidateKey first and returns ErrInvalidKey
// before any storage call. Errors other than the sentinel errors below are storage failures that
// callers answer with 503 unavailable (02 "14.4"); they wrap their cause, name the operation and
// the key, and never contain object bytes. No operation panics on a failing storage.
// Implementations are safe for concurrent use. Nothing in the port reads a whole object into
// memory. The reader of a whole object yields as many bytes as the storage answers: a use case
// that keeps what it reads in memory bounds the read itself (limitedObject), a use case that
// streams needs no bound (docs/decisions.d/FIX-S3-READ-BOUND-1111.md).
type ObjectStore interface {
	// Put writes the whole object key from exactly size bytes of body. A body that yields fewer
	// or more bytes, or size < 0, gives ErrSizeMismatch and stores nothing; a cancelled context
	// gives the context error and stores nothing. Put on an existing key replaces it atomically:
	// a reader sees the old or the new object, never a mix. The module never overwrites an
	// object (12 "4.4"); the driver does not enforce that.
	Put(ctx context.Context, key string, body io.Reader, size int64) error

	// Get opens the object key for reading; rng nil means the whole object, otherwise exactly
	// one byte range (12 "9.3"): the reader starts at rng.Offset and never yields more than
	// rng.Length bytes. A range whose end lies beyond the object ends at the last byte.
	// Errors: ErrNotFound for a missing object, ErrInvalidRange for a range outside the rules of
	// ByteRange. The caller closes the reader.
	Get(ctx context.Context, key string, rng *ByteRange) (io.ReadCloser, error)

	// Head returns the size of the object key, or ErrNotFound.
	Head(ctx context.Context, key string) (ObjectInfo, error)

	// Delete removes the object key. A missing object is not an error: it counts as deleted
	// (12 "12").
	Delete(ctx context.Context, key string) error
}

// MultipartStore is the outgoing port of the multipart upload of the S3 driver (12 "3.2", "4.1"):
// the S3 part strategy (NewS3PartStore) keeps part n as part n of a multipart upload of the final
// key, and the scan pass completes it. Only adapters/s3 implements it; the file system driver
// writes parts with ObjectStore.Put to uploads/<file_id>/<n> (D-M3-03-2). Keys and errors follow
// ObjectStore; every operation accepts only an original key (domain.ParseOriginalKey), else
// ErrInvalidKey. filestest.RunMultipartConformance holds the contract.
type MultipartStore interface {
	// CreateMultipart opens a multipart upload whose result becomes the object key and returns
	// its upload ID.
	CreateMultipart(ctx context.Context, key string) (uploadID string, err error)

	// UploadPart stores part n (from 1) of the upload uploadID from exactly size bytes of body
	// and returns the part with its ETag. Errors: ErrUploadNotFound for an upload that was never
	// created, was aborted or was completed; ErrInvalidPart for n < 1; ErrSizeMismatch as in
	// Put. A part with the same number replaces the earlier one.
	UploadPart(ctx context.Context, key, uploadID string, n int, body io.Reader, size int64) (Part, error)

	// CompleteMultipart joins the listed parts in order into the object key and ends the
	// upload; unlisted parts are discarded. Errors: ErrUploadNotFound as in UploadPart;
	// ErrInvalidPart for an empty list, numbers that are not strictly ascending, a number
	// without an uploaded part or an ETag that does not match. After ErrInvalidPart the upload
	// stays open and a later correct call succeeds.
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) error

	// AbortMultipart ends the upload and discards its parts. An unknown, aborted or completed
	// upload is not an error.
	AbortMultipart(ctx context.Context, key, uploadID string) error
}

// ByteRange is one byte range of Get: Offset >= 0, Length >= 1, Offset below the object size.
// A range whose end lies beyond the object ends at the last byte, as one HTTP range does.
type ByteRange struct {
	Offset int64
	Length int64
}

// ObjectInfo is what Head reports about an object.
type ObjectInfo struct {
	// Size is the length of the object in bytes.
	Size int64
}

// Part is one uploaded part of a multipart upload, as file_upload_parts stores it
// (03 "4.7").
type Part struct {
	// Number is the part number from 1, as file_upload_parts.part_number.
	Number int
	// ETag is exactly what UploadPart returned, quotes included.
	ETag string
}

// The sentinel errors of the port (docs/DECISIONS.md M3-01/2). Every other error of an
// ObjectStore is a storage failure.
var (
	// ErrInvalidKey is domain.ErrInvalidKey: the key is outside the key grammar, or a
	// multipart operation got a key that is not an original key on a driver that needs one.
	ErrInvalidKey = domain.ErrInvalidKey
	// ErrNotFound: Get or Head of a missing object.
	ErrNotFound = errors.New("files: object not found")
	// ErrInvalidRange: Offset < 0, Length < 1, or Offset at or beyond the object size.
	ErrInvalidRange = errors.New("files: invalid byte range")
	// ErrSizeMismatch: size < 0, or the body yields fewer or more bytes than size.
	ErrSizeMismatch = errors.New("files: body size does not match")
	// ErrUploadNotFound: the multipart upload was never created, was aborted or was completed.
	ErrUploadNotFound = errors.New("files: upload not found")
	// ErrInvalidPart: a part number below 1, or a part list CompleteMultipart refuses.
	ErrInvalidPart = errors.New("files: invalid part")
)
