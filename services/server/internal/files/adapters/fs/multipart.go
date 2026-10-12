// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	iofs "io/fs"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// uploadsDir is the parent of the part directories uploads/<file_id> (12 "4.4").
const uploadsDir = "uploads"

// uploadDir returns uploads/<file_id>, the directory of the parts of one upload.
func uploadDir(id ids.ID) string { return uploadsDir + "/" + id.String() }

// CreateMultipart opens the upload of an original key on the file system
// (docs/DECISIONS.md M3-01/5): it creates uploads/<file_id>/ with mode 0700 if missing, is
// idempotent, and returns the upload ID "", because the place of the parts follows from the key
// and 03 file_upload_sessions.storage_upload_id holds an ID only for the S3 driver.
//
// Errors: app.ErrInvalidKey for a key outside the grammar or one that is not an original key
// (clients upload only originals in parts, 12 "3.1"); a symbolic link in the path of the key or of
// the upload directory is a storage failure.
func (s *Store) CreateMultipart(ctx context.Context, key string) (string, error) {
	id, err := s.originalID("create multipart", key)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", opErr("create multipart", key, err)
	}
	if _, err := s.inspect(key); err != nil {
		return "", opErr("create multipart", key, err)
	}
	dir := uploadDir(id)
	fi, err := s.inspect(dir)
	switch {
	case err != nil:
		return "", opErr("create multipart", key, err)
	case fi != nil && !fi.IsDir():
		return "", opErr("create multipart", key, errors.New("the upload path is not a directory"))
	case fi == nil:
		// mkdirs syncs uploads/ when it creates the directory of the upload.
		if err := s.mkdirs(ctx, dir); err != nil {
			return "", opErr("create multipart", key, err)
		}
	}
	return "", nil
}

// originalID validates key and returns the file ID of an original key; any other key is
// app.ErrInvalidKey.
func (s *Store) originalID(op, key string) (ids.ID, error) {
	if err := domain.ValidateKey(key); err != nil {
		return ids.ID{}, opErr(op, "", err)
	}
	id, _, err := domain.ParseOriginalKey(key)
	if err != nil {
		return ids.ID{}, opErr(op, key, err)
	}
	return id, nil
}

// openUpload checks key and uploadID of an open upload and returns the file ID. A link in the
// path of the key or of the upload directory is a storage failure; an upload ID other than "" or
// a missing upload directory is app.ErrUploadNotFound.
func (s *Store) openUpload(ctx context.Context, op, key, uploadID string) (ids.ID, error) {
	id, err := s.originalID(op, key)
	if err != nil {
		return ids.ID{}, err
	}
	if err := ctx.Err(); err != nil {
		return ids.ID{}, opErr(op, key, err)
	}
	if _, err := s.inspect(key); err != nil {
		return ids.ID{}, opErr(op, key, err)
	}
	fi, err := s.inspect(uploadDir(id))
	if err != nil {
		return ids.ID{}, opErr(op, key, err)
	}
	if uploadID != "" || fi == nil || !fi.IsDir() {
		return ids.ID{}, opErr(op, key, app.ErrUploadNotFound)
	}
	return id, nil
}

// quotedHex is the ETag form of a part: the lower-case hex SHA-256 in double quotes, as the ETag
// header of 12 "9.3" (docs/DECISIONS.md M3-01/6).
func quotedHex(h hash.Hash) string { return `"` + hex.EncodeToString(h.Sum(nil)) + `"` }

// UploadPart writes part n of the open upload of key to uploads/<file_id>/<n> as Put writes an
// object, and returns the part with the quoted SHA-256 of its bytes as ETag, computed while
// writing. A part with the same number is replaced atomically, as S3 does. While the upload is
// open, the part is a regular object under its key, so Head and Get work on it.
//
// Errors: app.ErrInvalidKey as in CreateMultipart; app.ErrUploadNotFound for an upload ID other
// than "" or an upload that was never created, was aborted or was completed; app.ErrInvalidPart
// for n < 1; app.ErrSizeMismatch and the context error as in Put.
func (s *Store) UploadPart(ctx context.Context, key, uploadID string, n int, body io.Reader, size int64) (app.Part, error) {
	id, err := s.openUpload(ctx, "upload part", key, uploadID)
	if err != nil {
		return app.Part{}, err
	}
	if n < 1 {
		return app.Part{}, opErr("upload part", key, app.ErrInvalidPart)
	}
	if size < 0 {
		return app.Part{}, opErr("upload part", key, app.ErrSizeMismatch)
	}
	pk, err := domain.UploadPartKey(id, n)
	if err != nil {
		return app.Part{}, opErr("upload part", key, err)
	}
	h := sha256.New()
	err = s.commit(ctx, pk, false, func(w io.Writer) error {
		return copyExactly(ctx, io.MultiWriter(w, h), body, size)
	})
	if errors.Is(err, errParentGone) {
		err = app.ErrUploadNotFound
	}
	if err != nil {
		return app.Part{}, opErr("upload part", key, err)
	}
	return app.Part{Number: n, ETag: quotedHex(h)}, nil
}

// CompleteMultipart joins the listed parts in order into the object key: it writes them into one
// temporary file while hashing each part, so each part is read exactly once (R-FI-04), then syncs,
// renames, syncs the directory and removes uploads/<file_id>/ with all parts, listed or not.
//
// Errors: app.ErrInvalidKey and app.ErrUploadNotFound as in UploadPart; app.ErrInvalidPart for an
// empty list, numbers that are not strictly ascending or below 1, a listed part that is missing
// or whose bytes do not match its ETag. After app.ErrInvalidPart the temporary file is gone and
// the upload stays open.
func (s *Store) CompleteMultipart(ctx context.Context, key, uploadID string, parts []app.Part) error {
	if _, err := s.originalID("complete multipart", key); err != nil {
		return err
	}
	// The list shape comes before any file system call, as on every driver.
	if err := checkPartList(parts); err != nil {
		return opErr("complete multipart", key, err)
	}
	id, err := s.openUpload(ctx, "complete multipart", key, uploadID)
	if err != nil {
		return err
	}
	err = s.commit(ctx, key, true, func(w io.Writer) error {
		for _, p := range parts {
			if err := s.appendPart(ctx, w, id, p); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return opErr("complete multipart", key, err)
	}
	// The object is complete and durable now. A failure to remove the parts is not a failure of
	// the completion: a retry would find parts missing and refuse a finished object. Parts left
	// behind go with the Delete of the original (M3-01/18) or a later AbortMultipart.
	_ = s.dropUpload(id)
	return nil
}

// checkPartList refuses an empty list and numbers that are not strictly ascending from 1,
// before any part is read.
func checkPartList(parts []app.Part) error {
	if len(parts) == 0 {
		return fmt.Errorf("%w: empty part list", app.ErrInvalidPart)
	}
	for i, p := range parts {
		if p.Number < 1 || (i > 0 && p.Number <= parts[i-1].Number) {
			return fmt.Errorf("%w: part numbers must ascend strictly from 1", app.ErrInvalidPart)
		}
	}
	return nil
}

// appendPart copies part p of the upload of id to w while hashing it and refuses with
// app.ErrInvalidPart when the part is missing or its hash does not match its ETag.
func (s *Store) appendPart(ctx context.Context, w io.Writer, id ids.ID, p app.Part) error {
	pk, err := domain.UploadPartKey(id, p.Number)
	if err != nil {
		return err
	}
	f, size, err := s.openRegular(pk)
	if errors.Is(err, app.ErrNotFound) {
		return fmt.Errorf("%w: part %d was not uploaded", app.ErrInvalidPart, p.Number)
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if err := copyExactly(ctx, io.MultiWriter(w, h), f, size); err != nil {
		return err
	}
	if quotedHex(h) != p.ETag {
		return fmt.Errorf("%w: part %d does not match its ETag", app.ErrInvalidPart, p.Number)
	}
	return nil
}

// AbortMultipart removes uploads/<file_id>/ with its parts. An unknown or already removed
// upload, or an upload ID other than "", is no error and leaves an open upload untouched.
//
// Errors: app.ErrInvalidKey as in CreateMultipart; a symbolic link at the upload directory is a
// storage failure and nothing is removed.
func (s *Store) AbortMultipart(ctx context.Context, key, uploadID string) error {
	id, err := s.originalID("abort multipart", key)
	if err != nil {
		return err
	}
	if uploadID != "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return opErr("abort multipart", key, err)
	}
	if err := s.dropUpload(id); err != nil {
		return opErr("abort multipart", key, err)
	}
	return nil
}

// dropUpload removes uploads/<file_id>/ with its parts and syncs uploads/; a missing directory,
// or another entry than a directory there, is nothing to remove.
func (s *Store) dropUpload(id ids.ID) error {
	dir := uploadDir(id)
	fi, err := s.inspect(dir)
	if err != nil {
		return err
	}
	if fi == nil || !fi.IsDir() {
		return nil
	}
	if err := s.root.RemoveAll(osPath(dir)); err != nil {
		return err
	}
	if err := s.syncDir(uploadsDir); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return nil
}
