// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"strconv"
	"strings"
)

// FormatETag returns the strong ETag of a resource version: the decimal in double quotes, for
// example "7" for version 7 (06 "Nebenläufigkeit" item 1).
func FormatETag(version int64) string {
	return `"` + strconv.FormatInt(version, 10) + `"`
}

// IfMatch is the parsed If-Match header of a request.
type IfMatch struct {
	// Present is true when the header was sent with a non-empty value.
	Present bool
	// Version is the version the strong ETag names; meaningful only when Valid.
	Version int64
	// Valid is true for exactly one strong ETag holding a non-negative decimal. A weak tag (W/),
	// a list, the wildcard or any other value is not valid; the use case then answers
	// 412 version_conflict with current, because a strong comparison fails (RFC 9110 13.1.1).
	Valid bool
}

// ParseIfMatch reads If-Match from h. StagePreconditions has already refused a missing header or
// the wildcard on operations that require one; this helper serves the use case.
//
// Example:
//
//	im := httpserver.ParseIfMatch(r.Header)
//	if im.Present && (!im.Valid || im.Version != current.Version) {
//		return errs.VersionConflict(current)
//	}
func ParseIfMatch(h http.Header) IfMatch {
	v := strings.TrimSpace(strings.Join(h.Values(headerIfMatch), ","))
	if v == "" {
		return IfMatch{}
	}
	im := IfMatch{Present: true}
	if len(v) < 3 || v[0] != '"' || v[len(v)-1] != '"' {
		return im
	}
	digits := v[1 : len(v)-1]
	if digits == "" || strings.Trim(digits, "0123456789") != "" || len(digits) > 1 && digits[0] == '0' {
		return im
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return im
	}
	im.Version, im.Valid = n, true
	return im
}
