// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"

	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PartStore keeps the parts of an open upload in the object storage (docs/DECISIONS.md
// D-M3-03-2). internal/wire chooses the strategy from file.storage.driver: NewFSPartStore for fs,
// NewS3PartStore for s3. No method may run inside a unit of work (02 8.5); a store over
// ObjectStore never does, because the use cases call it only outside one.
type PartStore interface {
	// Put stores part n of the session from exactly size bytes of body and returns the ETag to
	// keep in file_upload_parts.etag ("" when the ETag derives from the digest). A body error
	// leaves nothing stored.
	Put(ctx context.Context, s SessionRecord, n int, body io.Reader, size int64) (string, error)
	// Discard removes every part of the session; a missing part is no error.
	Discard(ctx context.Context, s SessionRecord) error
	// DiscardUnconfirmed removes part n that was stored for a session that ended meanwhile.
	DiscardUnconfirmed(ctx context.Context, s SessionRecord, n int) error
}

// fsPartStore keeps part n as the object uploads/<file_id>/<n> (12 3.2, 4.4); completion touches
// no object, the verify pass of M3-04 joins the parts into orig-1.
type fsPartStore struct {
	store ObjectStore
}

// NewFSPartStore returns the part strategy of the file system driver over store.
func NewFSPartStore(store ObjectStore) PartStore { return fsPartStore{store: store} }

func (p fsPartStore) Put(ctx context.Context, s SessionRecord, n int, body io.Reader, size int64) (string, error) {
	key, err := domain.UploadPartKey(s.FileID, n)
	if err != nil {
		return "", err
	}
	return "", p.store.Put(ctx, key, body, size)
}

func (p fsPartStore) Discard(ctx context.Context, s SessionRecord) error {
	var errsOut []error
	for n := 1; n <= s.PartCount; n++ {
		errsOut = append(errsOut, p.DiscardUnconfirmed(ctx, s, n))
	}
	return errors.Join(errsOut...)
}

func (p fsPartStore) DiscardUnconfirmed(ctx context.Context, s SessionRecord, n int) error {
	key, err := domain.UploadPartKey(s.FileID, n)
	if err != nil {
		return err
	}
	return p.store.Delete(ctx, key)
}

// s3PartStore keeps part n as part n of a multipart upload of the final object key (12 3.2). The
// upload is created at the first part, outside any transaction; its id is stored in
// file_upload_sessions.storage_upload_id, and a request that loses the race aborts its own upload
// and uses the stored one. CompleteMultipart runs in the verify pass of M3-04.
type s3PartStore struct {
	store ObjectStore
	repo  Repository
	uow   txn.UnitOfWork
}

// NewS3PartStore returns the part strategy of the S3 driver over store, with the repository and
// unit of work that store the multipart upload id.
func NewS3PartStore(store ObjectStore, repo Repository, uow txn.UnitOfWork) PartStore {
	return s3PartStore{store: store, repo: repo, uow: uow}
}

func (p s3PartStore) Put(ctx context.Context, s SessionRecord, n int, body io.Reader, size int64) (string, error) {
	uploadID, err := p.uploadID(ctx, s)
	if err != nil {
		return "", err
	}
	part, err := p.store.UploadPart(ctx, s.File.ObjectKey, uploadID, n, body, size)
	if err != nil {
		return "", err
	}
	return part.ETag, nil
}

// uploadID returns the stored multipart upload of s or creates one.
func (p s3PartStore) uploadID(ctx context.Context, s SessionRecord) (string, error) {
	if s.StorageUploadID != "" {
		return s.StorageUploadID, nil
	}
	created, err := p.store.CreateMultipart(ctx, s.File.ObjectKey)
	if err != nil {
		return "", err
	}
	var stored bool
	var current SessionRecord
	err = p.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if stored, err = p.repo.SetStorageUploadID(ctx, s.FileID, created); err != nil || stored {
			return err
		}
		var found bool
		current, found, err = p.repo.Session(ctx, s.FileID, s.File.UploadedBy)
		if err == nil && !found {
			err = errs.New(errs.CodeNotFound, "")
		}
		return err
	})
	if err != nil || stored {
		return created, err
	}
	// Another request stored its upload first, or the session ended: drop the own upload.
	if err := p.store.AbortMultipart(ctx, s.File.ObjectKey, created); err != nil {
		return "", err
	}
	if current.StorageUploadID == "" {
		return "", errs.New(errs.CodeGone, "")
	}
	return current.StorageUploadID, nil
}

func (p s3PartStore) Discard(ctx context.Context, s SessionRecord) error {
	if s.StorageUploadID == "" {
		return nil
	}
	return p.store.AbortMultipart(ctx, s.File.ObjectKey, s.StorageUploadID)
}

// DiscardUnconfirmed does nothing: the part belongs to the multipart upload, which the abort or
// expiry of the session ends as a whole.
func (p s3PartStore) DiscardUnconfirmed(context.Context, SessionRecord, int) error { return nil }
