// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"context"
	"errors"
	"strconv"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
)

// The operation names of the error texts.
const (
	opPut      = "put"
	opGet      = "get"
	opHead     = "head"
	opDelete   = "delete"
	opCreate   = "create multipart"
	opPart     = "upload part"
	opComplete = "complete multipart"
	opAbort    = "abort multipart"
	opPresign  = "presign upload part"
)

// opError is every error of the driver (docs/DECISIONS.md M3-02/5). Its text is
// "files/s3: <operation> <key>: <detail>", where detail is a port sentinel, the S3 error code,
// the HTTP status, a fixed text for a known cause or "transport"; it never contains a request
// URL, a query, a presigned URL, the endpoint or credentials. net/http errors carry the full
// request URL, and callers that wrap a port error into kernel/errs put its text into the log
// (SEC-LOG-3, SEC-SEC-3). Unwrap keeps errors.Is working for the port sentinel and the cause
// (egress.ErrBlocked, egress.ErrResponseTooLarge, context errors, secret provider errors).
type opError struct {
	op       string
	key      string // "" when the key was refused: it may carry anything a caller passed
	detail   string
	sentinel error // a port sentinel, or nil for a storage failure
	cause    error
}

func (e *opError) Error() string {
	s := "files/s3: " + e.op
	if e.key != "" {
		s += " " + e.key
	}
	return s + ": " + e.detail
}

// Unwrap returns the port sentinel and the cause, whichever are set.
func (e *opError) Unwrap() []error {
	var out []error
	if e.sentinel != nil {
		out = append(out, e.sentinel)
	}
	if e.cause != nil {
		out = append(out, e.cause)
	}
	return out
}

// refused is a check of the port that fails before any request: sentinel with its own text.
func refused(op, key string, sentinel error) error {
	return &opError{op: op, key: key, detail: sentinel.Error(), sentinel: sentinel}
}

// invalidKey is a key or prefix outside the grammar; the key is not repeated.
func invalidKey(op string, cause error) error {
	return &opError{op: op, detail: cause.Error(), sentinel: app.ErrInvalidKey, cause: cause}
}

// mapping says which S3 answers of one operation are port sentinels.
type mapping struct {
	notFound      bool // NoSuchKey, NotFound or 404 is ErrNotFound (Get, Head)
	invalidRange  bool // InvalidRange or 416 is ErrInvalidRange (Get)
	uploadMissing bool // NoSuchUpload is ErrUploadNotFound (UploadPart, CompleteMultipart)
	invalidPart   bool // InvalidPart, InvalidPartOrder, EntityTooSmall are ErrInvalidPart
}

// apiStatus returns the S3 error code and the HTTP status of err; either may be empty or 0.
func apiStatus(err error) (string, int) {
	code, status := "", 0
	var ae smithy.APIError
	if errors.As(err, &ae) {
		code = ae.ErrorCode()
	}
	var re *awshttp.ResponseError
	if errors.As(err, &re) {
		status = re.HTTPStatusCode()
	}
	return code, status
}

// isNoSuchUpload reports the S3 answer to an unknown, aborted or completed upload.
func isNoSuchUpload(err error) bool {
	code, _ := apiStatus(err)
	return code == "NoSuchUpload"
}

// fail maps an SDK error of op on key to the port contract (docs/DECISIONS.md M3-02/5): the
// answers m names become port sentinels, everything else is a storage failure without sentinel,
// which callers answer with 503 unavailable (02 "14.1", "14.4").
func fail(op, key string, err error, m mapping) error {
	code, status := apiStatus(err)
	e := &opError{op: op, key: key, cause: err, detail: detail(err, code, status)}
	switch {
	case code == "NoSuchBucket":
		// A missing bucket is a wrong configuration, not a missing object.
	case m.notFound && (code == "NoSuchKey" || code == "NotFound" || status == 404):
		e.sentinel = app.ErrNotFound
	case m.invalidRange && (code == "InvalidRange" || status == 416):
		e.sentinel = app.ErrInvalidRange
	case m.uploadMissing && code == "NoSuchUpload":
		e.sentinel = app.ErrUploadNotFound
	case m.invalidPart && (code == "InvalidPart" || code == "InvalidPartOrder" || code == "EntityTooSmall"):
		e.sentinel = app.ErrInvalidPart
	}
	return e
}

// detail is the text of a failure: the S3 code, the status, or a fixed text for a known cause.
// Neither code nor status can carry a URL; a code that is not a plain identifier is replaced.
func detail(err error, code string, status int) string {
	var blocked *egress.BlockedError
	var cred *credentialError
	switch {
	case code != "" && plainCode(code):
		if status != 0 {
			return code + " (status " + strconv.Itoa(status) + ")"
		}
		return code
	case status != 0:
		return "status " + strconv.Itoa(status)
	case errors.As(err, &blocked):
		return "blocked by the egress client (" + string(blocked.Reason) + ")"
	case errors.As(err, &cred):
		return "credentials: " + cred.Error()
	case errors.Is(err, egress.ErrResponseTooLarge):
		return "response too large"
	case errors.Is(err, context.Canceled):
		return "transport: context canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "transport: deadline exceeded"
	}
	return "transport"
}

// plainCode reports an S3 error code of letters, digits and ".", at most 64 bytes.
func plainCode(code string) bool {
	if len(code) > 64 {
		return false
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}

// readError is an error while the caller reads a Get body. It keeps wrapping its cause
// (egress.ErrResponseTooLarge, a broken connection) and follows the text rule of opError.
func readError(key string, err error) error {
	d := "read: transport"
	switch {
	case errors.Is(err, egress.ErrResponseTooLarge):
		d = "read: response too large"
	case errors.Is(err, context.Canceled):
		d = "read: context canceled"
	case errors.Is(err, context.DeadlineExceeded):
		d = "read: deadline exceeded"
	}
	return &opError{op: opGet, key: key, detail: d, cause: err}
}
