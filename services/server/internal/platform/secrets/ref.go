// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"path"
	"regexp"
	"strings"
)

// Schemes of references. file and env have drivers in this server version; azkv and awssm are
// the later drivers named in docs/spec/16-operations.md 5.2.
const (
	schemeFile  = "file"
	schemeEnv   = "env"
	schemeAzkv  = "azkv"
	schemeAwssm = "awssm"
)

// dirPlaceholder is the one placeholder a reference may contain; ParseRef replaces it with
// the effective secrets.dir.
const dirPlaceholder = "<secrets.dir>"

// envNamePattern is the grammar of the variable an env: reference names, the same as in the
// configuration loader.
var envNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// Details of invalid references. They state the rule and never repeat the reference.
const (
	detailForm         = "must have the form <scheme>:<path>"
	detailScheme       = "unknown scheme; this server version supports file and env"
	detailFileAbsolute = "file reference must be an absolute path"
	detailFileClean    = "file reference must be a clean path without . or .. elements, repeated or trailing slashes"
	detailFileNUL      = "file reference must not contain a NUL character"
	detailEnvName      = "env reference must name a variable of upper-case letters, digits and underscores"
	detailSecretsDir   = dirPlaceholder + " must be an absolute clean path"
)

// Ref is a parsed secret reference "<scheme>:<path>" (docs/spec/16-operations.md 5.2): for file the
// absolute path of the file, for env the name of the variable.
type Ref struct {
	// Scheme selects the driver: "file" or "env" in this server version.
	Scheme string
	// Path is what the driver reads: an absolute clean file path or a variable name.
	Path string
}

// ParseRef parses raw after replacing every "<secrets.dir>" in it with secretsDir, the effective
// value of secrets.dir.
//
// Grammar: "file:<path>" with a path that is absolute, clean (no "." or ".." element, no repeated
// or trailing slash) and free of NUL; "env:<NAME>" with NAME matching ^[A-Z_][A-Z0-9_]*$. The
// schemes azkv and awssm are recognised but return ErrUnsupportedScheme; anything else, a missing
// colon or an empty scheme return ErrInvalidRef. When raw uses the placeholder, secretsDir must
// be absolute and clean itself, so a missing directory cannot turn a reference into a path below
// the root.
//
// Every error is a *RefError with an empty Ref and the zero Ref as result: the message names
// only a known scheme and the rule, never the text of raw, because a secret value may stand
// where a reference belongs.
//
// Example:
//
//	ref, err := secrets.ParseRef("file:<secrets.dir>/db_app_password", "/run/bothy/secrets")
//	// ref.Path == "/run/bothy/secrets/db_app_password", err == nil
func ParseRef(raw, secretsDir string) (Ref, error) {
	if strings.Contains(raw, dirPlaceholder) {
		if !isAbsClean(secretsDir) {
			return Ref{}, invalid(detailSecretsDir)
		}
		raw = strings.ReplaceAll(raw, dirPlaceholder, secretsDir)
	}
	scheme, rest, ok := strings.Cut(raw, ":")
	if !ok || scheme == "" {
		return Ref{}, invalid(detailForm)
	}
	switch scheme {
	case schemeFile, schemeEnv:
		ref := Ref{Scheme: scheme, Path: rest}
		if err := checkRef(ref); err != nil {
			return Ref{}, err
		}
		return ref, nil
	case schemeAzkv, schemeAwssm:
		return Ref{}, &RefError{Err: ErrUnsupportedScheme, Detail: scheme + " has no driver in this server version"}
	default:
		return Ref{}, invalid(detailScheme)
	}
}

// String returns the reference as "<scheme>:<path>"; for a reference from ParseRef without
// placeholder it is the parsed text.
func (r Ref) String() string { return r.Scheme + ":" + r.Path }

// checkRef checks the grammar of a reference with scheme file or env and of an empty scheme;
// other schemes belong to their driver. It returns a *RefError without Ref.
func checkRef(r Ref) error {
	switch r.Scheme {
	case "":
		return invalid(detailForm)
	case schemeFile:
		switch {
		case strings.IndexByte(r.Path, 0) >= 0:
			return invalid(detailFileNUL)
		case !path.IsAbs(r.Path):
			return invalid(detailFileAbsolute)
		case path.Clean(r.Path) != r.Path:
			return invalid(detailFileClean)
		}
	case schemeEnv:
		if !envNamePattern.MatchString(r.Path) {
			return invalid(detailEnvName)
		}
	}
	return nil
}

// isAbsClean reports whether p is an absolute slash path that path.Clean leaves unchanged.
func isAbsClean(p string) bool {
	return path.IsAbs(p) && path.Clean(p) == p && strings.IndexByte(p, 0) < 0
}

func invalid(detail string) *RefError {
	return &RefError{Err: ErrInvalidRef, Detail: detail}
}

// displayRef returns the text of ref for an error message when it is a file or env reference with
// a valid grammar, and "" otherwise, so an invalid hand-built reference is never repeated. Other
// schemes, a caller-supplied driver's included, have no grammar this package checks, so their
// path may be a secret in the wrong place (docs/DECISIONS.md M1-09/6, like ParseRef).
func displayRef(r Ref) string {
	if (r.Scheme != schemeFile && r.Scheme != schemeEnv) || checkRef(r) != nil {
		return ""
	}
	return r.String()
}
