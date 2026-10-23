// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"errors"
	"io"
	"mime/multipart"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// ImagePartName is the name of the one part of the schema ImageUpload (docs/api/openapi.yaml).
const ImagePartName = "file"

// pointerImagePart is the JSON pointer of the part "file" in a field error.
const pointerImagePart = "/" + ImagePartName

// ImagePart opens the first part of the multipart body of an image upload (schema ImageUpload:
// exactly one part named "file", brief M3-10 design note 2). It returns the declared Content-Type
// of the part and a reader of its bytes. The reader ends with io.EOF only when the body has no
// further part; a further part ends it with 400 validation_failed (pointer "/<name>", code
// unknown_member, as the chain reports a member outside a schema, 06 2 "Unknown values"), so a
// caller that reads to io.EOF has checked the whole body. It reads nothing before it is called,
// so a handler calls it only after authorize (design note 5). It parses no image: only the MIME
// framing of the request (SEC-UP-5).
//
// Errors: 400 validation_failed with pointer /file and code required when the body has no part
// or its first part is not named "file", with code invalid when the framing is broken.
//
// Example:
//
//	declared, body, err := httpserver.ImagePart(req.Body)
func ImagePart(mr *multipart.Reader) (string, io.Reader, error) {
	if mr == nil {
		return "", nil, imagePartError(pointerImagePart, fieldCodeRequired)
	}
	p, err := mr.NextPart()
	switch {
	case errors.Is(err, io.EOF):
		return "", nil, imagePartError(pointerImagePart, fieldCodeRequired)
	case err != nil:
		return "", nil, imagePartError(pointerImagePart, fieldCodeInvalid)
	case p.FormName() != ImagePartName:
		_ = p.Close()
		return "", nil, imagePartError(pointerImagePart, fieldCodeRequired)
	}
	return p.Header.Get(headerContentType), &imagePartReader{mr: mr, part: p}, nil
}

// imagePartReader reads the part "file" and, at its end, checks that no further part follows.
type imagePartReader struct {
	mr   *multipart.Reader
	part *multipart.Part
	err  error
}

func (r *imagePartReader) Read(b []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.part.Read(b)
	switch {
	case errors.Is(err, io.EOF):
		r.err = r.end()
		return n, r.err
	case err != nil:
		// A broken framing inside the part is the client's error, never a server error.
		r.err = imagePartError(pointerImagePart, fieldCodeInvalid)
		return n, r.err
	}
	return n, nil
}

// end is the check after the part: io.EOF when the body ends, else the error of a further part.
func (r *imagePartReader) end() error {
	next, err := r.mr.NextPart()
	switch {
	case errors.Is(err, io.EOF):
		return io.EOF
	case err != nil:
		return imagePartError(pointerImagePart, fieldCodeInvalid)
	}
	name := next.FormName()
	_ = next.Close()
	return imagePartError("/"+escapePointer(name), fieldCodeUnknownMember)
}

// imagePartError is the 400 validation_failed with one field error of code at pointer.
func imagePartError(pointer, code string) error {
	detail := fieldDetails[code]
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}
