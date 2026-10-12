// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// errInvalidSemVer is wrapped by every parse error of CompareSemVer; the message names the
// defect, never the input, which may come from the database.
var errInvalidSemVer = errors.New("not a SemVer 2.0.0 version")

// semver is a parsed version: the three numbers as digit strings without leading zeros, so
// values beyond 64 bits compare correctly, and the dot-separated pre-release identifiers.
type semver struct {
	core [3]string
	pre  []string
}

// CompareSemVer compares a and b by SemVer 2.0.0 precedence (https://semver.org/spec/v2.0.0.html,
// section 11) and returns -1 when a < b, 0 when they have the same precedence and 1 when a > b.
// Major, minor and patch compare numerically; a version with a pre-release sorts before the same
// version without; pre-release identifiers compare numerically when both are numeric, numeric
// before alphanumeric, otherwise in ASCII order, and a longer set wins when all preceding
// identifiers are equal. Build metadata ("+...") is validated but ignored. A leading "v" is not
// part of SemVer and is refused (18-release.md "Versionen").
//
// The error reports which argument is not a valid SemVer 2.0.0 version.
//
// Example:
//
//	c, err := domain.CompareSemVer("1.0.0-rc.1", "1.0.0") // c == -1, err == nil
func CompareSemVer(a, b string) (int, error) {
	va, err := parseSemVer(a)
	if err != nil {
		return 0, fmt.Errorf("first version: %w", err)
	}
	vb, err := parseSemVer(b)
	if err != nil {
		return 0, fmt.Errorf("second version: %w", err)
	}
	for i := range va.core {
		if c := compareNumeric(va.core[i], vb.core[i]); c != 0 {
			return c, nil
		}
	}
	return comparePre(va.pre, vb.pre), nil
}

// ValidateSemVer reports whether s is a SemVer 2.0.0 version without a leading "v"; the error
// names the defect, never the input.
func ValidateSemVer(s string) error {
	_, err := parseSemVer(s)
	return err
}

// CheckServerVersion reports whether a server of version own may run against a schema that
// requires at least minimum (instance.min_server_version). An empty minimum is no lock: a newer,
// compatible schema without it is allowed (upgrade.md "Rückweg"). A minimum above own, an own
// version or a minimum that is not valid SemVer fail closed.
//
// The error has code unavailable: the server keeps running but is never ready
// (readiness check schema_requires_newer_server). Its detail names both versions for the log
// line at start; /readyz shows only the check name.
func CheckServerVersion(own, minimum string) error {
	if minimum == "" {
		if _, err := parseSemVer(own); err != nil {
			return errs.Wrap(errs.CodeUnavailable, err, "the server version is not valid SemVer")
		}
		return nil
	}
	c, err := CompareSemVer(own, minimum)
	if err != nil {
		return errs.Wrap(errs.CodeUnavailable, err, fmt.Sprintf("cannot compare the server version with instance.min_server_version %q", printable(minimum)))
	}
	if c < 0 {
		return errs.New(errs.CodeUnavailable, fmt.Sprintf("server version %s is below instance.min_server_version %s; deploy a newer server", own, minimum))
	}
	return nil
}

// printable bounds a value from the database for an error text: at most 64 bytes, control
// characters replaced.
func printable(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, s)
}

// parseSemVer parses s after the grammar of SemVer 2.0.0.
func parseSemVer(s string) (semver, error) {
	rest, build, hasBuild := strings.Cut(s, "+")
	if hasBuild {
		if err := identifiers(build, false); err != nil {
			return semver{}, fmt.Errorf("%w: build metadata: %w", errInvalidSemVer, err)
		}
	}
	core, pre, hasPre := strings.Cut(rest, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("%w: want MAJOR.MINOR.PATCH", errInvalidSemVer)
	}
	var v semver
	for i, p := range parts {
		if !numericIdentifier(p) {
			return semver{}, fmt.Errorf("%w: version core part %d is not a number without leading zeros", errInvalidSemVer, i+1)
		}
		v.core[i] = p
	}
	if hasPre {
		if err := identifiers(pre, true); err != nil {
			return semver{}, fmt.Errorf("%w: pre-release: %w", errInvalidSemVer, err)
		}
		v.pre = strings.Split(pre, ".")
	}
	return v, nil
}

// identifiers checks dot-separated identifiers: non-empty, only [0-9A-Za-z-]; in a pre-release
// a numeric identifier must not have leading zeros.
func identifiers(s string, pre bool) error {
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return errors.New("empty identifier")
		}
		for i := range len(id) {
			if !identChar(id[i]) {
				return errors.New("identifier with a character outside [0-9A-Za-z-]")
			}
		}
		if pre && allDigits(id) && len(id) > 1 && id[0] == '0' {
			return errors.New("numeric identifier with a leading zero")
		}
	}
	return nil
}

func identChar(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-'
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// numericIdentifier reports "0" or digits without a leading zero.
func numericIdentifier(s string) bool {
	return allDigits(s) && (s == "0" || s[0] != '0')
}

// compareNumeric compares two digit strings without leading zeros as numbers of any size.
func compareNumeric(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// comparePre compares pre-release identifier lists; an empty list is a release and sorts last.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdentifier(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

// compareIdentifier compares one pre-release identifier pair (section 11 item 4.1).
func compareIdentifier(a, b string) int {
	na, nb := allDigits(a), allDigits(b)
	switch {
	case na && nb:
		return compareNumeric(a, b)
	case na:
		return -1
	case nb:
		return 1
	default:
		return strings.Compare(a, b)
	}
}
