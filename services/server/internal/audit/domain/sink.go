// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
)

// SinkKind is the kind of target audit.checkpoint_sink names.
type SinkKind string

// The kinds of audit.checkpoint_sink (19 section 16).
const (
	// SinkNone is the empty value: no sink, a permanent warning (19 SEC-AUD-2).
	SinkNone SinkKind = ""
	// SinkFile is file://<absolute path> (M1-50).
	SinkFile SinkKind = "file"
	// SinkS3 is s3://<bucket>[/<prefix>], an S3-compatible object store with a retention lock
	// (M7-22).
	SinkS3 SinkKind = "s3"
)

// Sink is a parsed value of audit.checkpoint_sink.
type Sink struct {
	// Kind is the kind of target.
	Kind SinkKind
	// Dir is the directory of a file sink, "" otherwise.
	Dir string
	// Bucket is the bucket of an S3 sink, "" otherwise.
	Bucket string
	// Prefix is the optional prefix of an S3 sink: segments of [a-z0-9-] joined by single "/",
	// without a leading or trailing "/"; "" for none.
	Prefix string
}

// errSink is the error of every value ParseSink refuses; it names the supported forms and never
// the value.
var errSink = errors.New("audit.checkpoint_sink must be empty, file://<absolute path> or s3://<bucket>[/<prefix>]; the supported schemes are file:// and s3://")

// prefixSegment is one segment of the prefix of an S3 sink.
var prefixSegment = regexp.MustCompile(`^[a-z0-9-]+$`)

// ParseSink reads the value of audit.checkpoint_sink (19 section 16):
//
//   - empty means no sink and returns the zero Sink;
//   - exactly "file://" and an absolute path without host, user, query, fragment, percent escapes
//     or control characters and with a clean path (no "..", no trailing slash) is a file sink;
//   - exactly "s3://" and a bucket that follows the S3 naming rules for general purpose buckets,
//     optionally "/" and a prefix of segments of [a-z0-9-] joined by single "/", without user,
//     port, query, fragment, percent escapes or control characters, is an S3 sink.
//
// Errors: every other value, with a message that names the supported forms and never the value.
//
// Example:
//
//	s, err := domain.ParseSink("s3://bothy-audit/prod") // Sink{Kind: SinkS3, Bucket: "bothy-audit", Prefix: "prod"}
func ParseSink(v string) (Sink, error) {
	if v == "" {
		return Sink{}, nil
	}
	// The exact prefix and no escapes: url.Parse lower-cases the scheme, takes "file:/x" and
	// decodes %00, all forms the documented value does not have.
	if strings.ContainsAny(v, "%\x00") || strings.ContainsFunc(v, unicode.IsControl) {
		return Sink{}, errSink
	}
	switch {
	case strings.HasPrefix(v, string(SinkFile)+"://"):
		return parseFileSink(v)
	case strings.HasPrefix(v, string(SinkS3)+"://"):
		return parseS3Sink(v)
	}
	return Sink{}, errSink
}

// parseFileSink parses file://<absolute path>.
func parseFileSink(v string) (Sink, error) {
	u, err := url.Parse(v)
	if err != nil || u.Scheme != string(SinkFile) || u.Opaque != "" || u.Host != "" || u.User != nil ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return Sink{}, errSink
	}
	p := u.Path
	if p == "" || !path.IsAbs(p) || path.Clean(p) != p || p == "/" {
		return Sink{}, errSink
	}
	return Sink{Kind: SinkFile, Dir: p}, nil
}

// parseS3Sink parses s3://<bucket>[/<prefix>] by hand: the bucket is the authority, so url.Parse
// would accept a port or user info that S3 has no place for.
func parseS3Sink(v string) (Sink, error) {
	rest := strings.TrimPrefix(v, string(SinkS3)+"://")
	bucket, prefix, hasPrefix := strings.Cut(rest, "/")
	if !ValidBucket(bucket) {
		return Sink{}, errSink
	}
	if hasPrefix {
		for _, seg := range strings.Split(prefix, "/") {
			if !prefixSegment.MatchString(seg) {
				return Sink{}, errSink
			}
		}
	}
	return Sink{Kind: SinkS3, Bucket: bucket, Prefix: prefix}, nil
}

// ValidBucket checks the S3 bucket naming rules for general purpose buckets: 3 to 63 bytes of
// lower-case letters, digits, "." and "-", beginning and ending with a letter or digit, no "..",
// not an IPv4 address, and none of the prefixes and suffixes S3 reserves. The same rules hold
// file.s3.bucket (internal/files/adapters/s3); the module boundary keeps a copy here.
func ValidBucket(b string) bool {
	if len(b) < 3 || len(b) > 63 || strings.Contains(b, "..") || !alnum(b[0]) || !alnum(b[len(b)-1]) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if c := b[i]; !alnum(c) && c != '.' && c != '-' {
			return false
		}
	}
	if a, err := netip.ParseAddr(b); err == nil && a.Is4() {
		return false
	}
	for _, p := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(b, p) {
			return false
		}
	}
	for _, s := range []string{"-s3alias", "--ol-s3", "--x-s3", "--table-s3", ".mrap"} {
		if strings.HasSuffix(b, s) {
			return false
		}
	}
	return true
}

// alnum reports a lower-case ASCII letter or a digit.
func alnum(c byte) bool { return ('a' <= c && c <= 'z') || ('0' <= c && c <= '9') }
