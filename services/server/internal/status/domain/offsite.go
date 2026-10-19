// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ErrInvalidOffsiteTarget reports a backup.offsite.target this build cannot use. The message
// names the rule, never the value.
var ErrInvalidOffsiteTarget = errors.New("backup.offsite.target must be s3:https://<host>[:<port>]/<bucket>[/<prefix>]")

// OffsiteTarget is a parsed backup.offsite.target: an S3-compatible bucket with versioning and
// Object Lock (docs/operations/backup-restore.md 2, docs/DECISIONS.md D-M7-02-4). Inside the
// bucket, pgBackRest repo2 lives below <prefix>/pgbackrest, the restic repository below
// <prefix>/restic and the object mirror of the driver s3 below <prefix>/objects.
type OffsiteTarget struct {
	// Host is the host name or address of the endpoint, without port.
	Host string
	// Port is the TCP port, 443 when the target names none.
	Port int
	// Bucket is the bucket name.
	Bucket string
	// Prefix is the key prefix without leading or trailing slash; "" for the bucket root.
	Prefix string
}

// bucketPattern is the S3 bucket naming rule (3 to 63 lowercase letters, digits, dots, hyphens).
var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

// prefixSegment is one segment of the prefix: letters, digits, dot, underscore and hyphen.
var prefixSegment = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ParseOffsiteTarget parses s in the restic form s3:https://<host>[:<port>]/<bucket>[/<prefix>].
// Only HTTPS is accepted: pgBackRest talks to S3 only over TLS. The empty string is no target
// (local backup only); callers check it before. Errors wrap ErrInvalidOffsiteTarget.
//
// Example:
//
//	t, err := domain.ParseOffsiteTarget("s3:https://s3.example.com/bothy-backup/instance-1")
//	// t.Host "s3.example.com", t.Port 443, t.Bucket "bothy-backup", t.Prefix "instance-1"
func ParseOffsiteTarget(s string) (OffsiteTarget, error) {
	rest, ok := strings.CutPrefix(s, "s3:")
	if !ok {
		return OffsiteTarget{}, ErrInvalidOffsiteTarget
	}
	u, err := url.Parse(rest)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return OffsiteTarget{}, ErrInvalidOffsiteTarget
	}
	host := u.Hostname()
	if host == "" || strings.ContainsAny(host, " /") {
		return OffsiteTarget{}, ErrInvalidOffsiteTarget
	}
	port := 443
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return OffsiteTarget{}, ErrInvalidOffsiteTarget
		}
		port = n
	}
	if ip := net.ParseIP(host); ip == nil && !hostPattern.MatchString(host) {
		return OffsiteTarget{}, ErrInvalidOffsiteTarget
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) == 0 || !bucketPattern.MatchString(segments[0]) || strings.HasSuffix(u.Path, "//") {
		return OffsiteTarget{}, ErrInvalidOffsiteTarget
	}
	for _, seg := range segments[1:] {
		if !prefixSegment.MatchString(seg) || seg == "." || seg == ".." {
			return OffsiteTarget{}, ErrInvalidOffsiteTarget
		}
	}
	return OffsiteTarget{Host: host, Port: port, Bucket: segments[0], Prefix: strings.Join(segments[1:], "/")}, nil
}

// hostPattern is a DNS host name.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// Key returns the key prefix of part inside the bucket: "<prefix>/<part>" or "<part>".
func (t OffsiteTarget) Key(part string) string {
	if t.Prefix == "" {
		return part
	}
	return t.Prefix + "/" + part
}

// Endpoint returns "https://<host>:<port>".
func (t OffsiteTarget) Endpoint() string {
	return "https://" + net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
}
