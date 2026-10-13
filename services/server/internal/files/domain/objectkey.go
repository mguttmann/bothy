// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ErrInvalidKey reports a key outside the grammar of [ValidateKey] or arguments a key builder
// refuses (the nil ID, a revision or part number below 1, an unknown variant). Errors wrapping it
// never repeat the key, because a caller may log them.
var ErrInvalidKey = errors.New("files: invalid object key")

// Variant names one stored thumbnail variant of a file (docs/spec/03-data-model.md
// file_thumbnails.variant). The type is closed: VariantKey refuses every value except the four
// constants, so a name converted to Variant can never reach a key.
type Variant string

// The variants of docs/spec/12-files.md "4.4 Key schema".
const (
	VariantSmall  Variant = "small"
	VariantMedium Variant = "medium"
	VariantLarge  Variant = "large"
	VariantPoster Variant = "poster"
)

const (
	filePrefix    = "f/"
	uploadsPrefix = "uploads/"
	origPrefix    = "orig-"
)

// fileDir returns f/<aa>/<bb>/<file_id>: <aa> and <bb> are the last four hex characters of the
// canonical file ID, the random bits of a UUIDv7, so files spread evenly over directories.
func fileDir(fileID ids.ID) (string, error) {
	if fileID.IsZero() {
		return "", fmt.Errorf("%w: nil file ID", ErrInvalidKey)
	}
	s := fileID.String()
	return filePrefix + s[len(s)-4:len(s)-2] + "/" + s[len(s)-2:] + "/" + s, nil
}

// OriginalKey returns the key of revision revision of the original of a file,
// f/<aa>/<bb>/<file_id>/orig-<revision>. Revision 1 is the original as uploaded; a changed
// original gets the next revision, because objects are never overwritten (12 "4.4").
//
// The error wraps ErrInvalidKey for the nil ID or a revision below 1.
//
// Example:
//
//	k, _ := domain.OriginalKey(ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"), 1)
//	// k == "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1"
func OriginalKey(fileID ids.ID, revision int) (string, error) {
	dir, err := fileDir(fileID)
	if err != nil {
		return "", err
	}
	if revision < 1 {
		return "", fmt.Errorf("%w: revision below 1", ErrInvalidKey)
	}
	return dir + "/" + origPrefix + strconv.Itoa(revision), nil
}

// VariantKey returns the key of a thumbnail variant of a file, f/<aa>/<bb>/<file_id>/<variant>.
//
// The error wraps ErrInvalidKey for the nil ID or a variant other than the four constants.
//
// Example:
//
//	k, _ := domain.VariantKey(ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"), domain.VariantSmall)
//	// k == "f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/small"
func VariantKey(fileID ids.ID, v Variant) (string, error) {
	switch v {
	case VariantSmall, VariantMedium, VariantLarge, VariantPoster:
	default:
		return "", fmt.Errorf("%w: unknown variant", ErrInvalidKey)
	}
	dir, err := fileDir(fileID)
	if err != nil {
		return "", err
	}
	return dir + "/" + string(v), nil
}

// UploadPartKey returns the key of part n of an open upload on the file system driver,
// uploads/<file_id>/<n> (12 "3.2", "4.4"). The S3 driver keeps parts in the multipart upload of
// the service and never uses these keys.
//
// The error wraps ErrInvalidKey for the nil ID or a part number below 1.
//
// Example:
//
//	k, _ := domain.UploadPartKey(ids.MustParse("0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b"), 1)
//	// k == "uploads/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/1"
func UploadPartKey(fileID ids.ID, n int) (string, error) {
	if fileID.IsZero() {
		return "", fmt.Errorf("%w: nil file ID", ErrInvalidKey)
	}
	if n < 1 {
		return "", fmt.Errorf("%w: part number below 1", ErrInvalidKey)
	}
	return uploadsPrefix + fileID.String() + "/" + strconv.Itoa(n), nil
}

// ParseOriginalKey returns the file ID and the revision of an original key. It accepts exactly
// the output of OriginalKey: the canonical lower-case file ID, the fan-out <aa>/<bb> of that ID
// and a decimal revision of at least 1 without leading zeros.
//
// The error wraps ErrInvalidKey for every other string, variant and part keys included.
//
// Example:
//
//	id, rev, err := domain.ParseOriginalKey("f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-2")
//	// err == nil, rev == 2
func ParseOriginalKey(key string) (ids.ID, int, error) {
	invalid := fmt.Errorf("%w: not an original key", ErrInvalidKey)
	segs := strings.Split(key, "/")
	if len(segs) != 5 || segs[0] != "f" || !strings.HasPrefix(segs[4], origPrefix) {
		return ids.ID{}, 0, invalid
	}
	id, err := ids.Parse(segs[3])
	if err != nil || id.String() != segs[3] {
		return ids.ID{}, 0, invalid
	}
	digits := strings.TrimPrefix(segs[4], origPrefix)
	if digits == "" || digits[0] == '0' || strings.Trim(digits, "0123456789") != "" {
		return ids.ID{}, 0, invalid
	}
	rev, err := strconv.Atoi(digits)
	if err != nil {
		return ids.ID{}, 0, invalid
	}
	if want, err := OriginalKey(id, rev); err != nil || want != key {
		return ids.ID{}, 0, invalid
	}
	return id, rev, nil
}

// ValidateKey checks the one key grammar of all drivers: one or more segments of [a-z0-9-],
// joined by single "/", without a leading or trailing "/". It therefore refuses ".", "..",
// absolute paths, empty segments, backslashes, upper case, percent signs, spaces, control and
// non-ASCII characters, and filepath.Clean(key) == key holds for every accepted key. Both drivers
// refuse the same keys, so a key that works on one works on the other (ADR 0010).
//
// The error wraps ErrInvalidKey and names the first defect, never the key.
//
// Example:
//
//	domain.ValidateKey("f/5a/6b/0192a3b4-c5d6-7e8f-9a0b-1c2d3e4f5a6b/orig-1") // nil
//	domain.ValidateKey("../etc/passwd")                                     // wraps ErrInvalidKey
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty key", ErrInvalidKey)
	}
	segStart := true
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c == '/':
			if segStart {
				return fmt.Errorf("%w: empty segment at byte %d", ErrInvalidKey, i)
			}
			segStart = true
		case 'a' <= c && c <= 'z', '0' <= c && c <= '9', c == '-':
			segStart = false
		default:
			return fmt.Errorf("%w: character outside [a-z0-9/-] at byte %d", ErrInvalidKey, i)
		}
	}
	if segStart {
		return fmt.Errorf("%w: trailing slash", ErrInvalidKey)
	}
	return nil
}
