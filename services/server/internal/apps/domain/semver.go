// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"strconv"
	"strings"
)

// SemVer is a parsed version of Semantic Versioning 2.0.0 (https://semver.org/spec/v2.0.0.html).
// Build metadata is kept but never compared (rule 10 of the specification).
type SemVer struct {
	Major, Minor, Patch uint64
	// Pre are the dot-separated identifiers of the pre-release; nil for a release.
	Pre []string
	// Build is the build metadata after "+"; "" for none.
	Build string
}

// ParseSemVer parses s as SemVer 2.0.0: three numeric parts without leading zeros, an optional
// pre-release of non-empty alphanumeric or hyphen identifiers (numeric ones without leading
// zeros) and optional build metadata. ok is false for any other text. The parse uses only the
// standard library and runs in linear time (SEC-IN-6).
//
// Example:
//
//	v, ok := domain.ParseSemVer("1.2.0-rc.1+build.5")
func ParseSemVer(s string) (SemVer, bool) {
	var v SemVer
	rest, build, hasBuild := strings.Cut(s, "+")
	if hasBuild {
		if !validIdentifiers(build, false) {
			return SemVer{}, false
		}
		v.Build = build
	}
	core, pre, hasPre := strings.Cut(rest, "-")
	if hasPre {
		if !validIdentifiers(pre, true) {
			return SemVer{}, false
		}
		v.Pre = strings.Split(pre, ".")
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return SemVer{}, false
	}
	nums := [3]*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, ok := numeric(p)
		if !ok {
			return SemVer{}, false
		}
		*nums[i] = n
	}
	return v, true
}

// numeric parses a numeric identifier: digits only, no leading zero except "0" itself.
func numeric(p string) (uint64, bool) {
	if p == "" || (len(p) > 1 && p[0] == '0') {
		return 0, false
	}
	for _, r := range p {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(p, 10, 64)
	return n, err == nil
}

// validIdentifiers checks dot-separated identifiers of [0-9A-Za-z-]; pre-release identifiers
// that are numeric must not have a leading zero.
func validIdentifiers(s string, pre bool) bool {
	for id := range strings.SplitSeq(s, ".") {
		if id == "" {
			return false
		}
		allDigits := true
		for _, r := range id {
			switch {
			case r >= '0' && r <= '9':
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '-':
				allDigits = false
			default:
				return false
			}
		}
		if pre && allDigits && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}

// Compare returns -1, 0 or +1 when v has lower, equal or higher precedence than w (rule 11 of
// SemVer 2.0.0): major, minor and patch numerically, a pre-release below its release, then the
// pre-release identifiers one by one (numeric below alphanumeric, numeric by value, others in
// ASCII order, a shorter list below a longer one with an equal prefix). Build metadata is ignored.
func (v SemVer) Compare(w SemVer) int {
	for _, p := range [][2]uint64{{v.Major, w.Major}, {v.Minor, w.Minor}, {v.Patch, w.Patch}} {
		if c := cmpUint(p[0], p[1]); c != 0 {
			return c
		}
	}
	switch {
	case len(v.Pre) == 0 && len(w.Pre) == 0:
		return 0
	case len(v.Pre) == 0:
		return 1
	case len(w.Pre) == 0:
		return -1
	}
	for i := 0; i < len(v.Pre) && i < len(w.Pre); i++ {
		if c := comparePre(v.Pre[i], w.Pre[i]); c != 0 {
			return c
		}
	}
	return cmpUint(uint64(len(v.Pre)), uint64(len(w.Pre)))
}

// comparePre compares two pre-release identifiers.
func comparePre(a, b string) int {
	na, aNum := numeric(a)
	nb, bNum := numeric(b)
	switch {
	case aNum && bNum:
		return cmpUint(na, nb)
	case aNum:
		return -1
	case bNum:
		return 1
	}
	return strings.Compare(a, b)
}

// cmpUint compares two unsigned integers.
func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
