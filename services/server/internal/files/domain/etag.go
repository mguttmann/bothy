// SPDX-License-Identifier: Apache-2.0

package domain

import "encoding/hex"

// FSPartETag returns the ETag of a part on the file system driver: the lowercase hex SHA-256 of
// the part in double quotes, derived from file_upload_parts.sha256, whose etag column stays NULL
// there (docs/DECISIONS.md D-M3-03-3). It equals the ETag the file system driver computes for a
// multipart part (M3-01/6) and the form of the ETag header of 12 "9.3".
//
// Example:
//
//	etag := domain.FSPartETag(sha256.Sum256(part)) // "\"<64 hex digits>\""
func FSPartETag(sum [32]byte) string {
	return `"` + hex.EncodeToString(sum[:]) + `"`
}
