// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// Level is a level of a grant (07 section 2.3). Levels are ordered and each includes the lower
// ones: LevelNone < LevelRead < LevelWrite < LevelTrigger, so plain comparison decides
// "at least". LevelNone is the absence of a level: the zero value, and the level of a grant on
// a host, where levels do not apply (07 section 2.12, column grants.level NULL).
type Level uint8

const (
	// LevelNone means no level.
	LevelNone Level = iota
	// LevelRead allows seeing (07 section 2.3).
	LevelRead
	// LevelWrite adds writing; it never triggers an agent.
	LevelWrite
	// LevelTrigger adds triggering agents, components and commands.
	LevelTrigger
)

// levelNames are the column values of grants.level, indexed by Level.
//
//nolint:goconst // "read" is a grant level, not the action effect EffectRead; the count includes the generated catalogue of M1-25 (docs/DECISIONS.md M1-27/10)
var levelNames = [...]string{LevelNone: "", LevelRead: "read", LevelWrite: "write", LevelTrigger: "trigger"}

// ParseLevel returns the level of a column or API value: "read", "write" or "trigger".
//
// Errors: validation_failed on /level for any other text, including the empty string and
// other cases of the letters.
//
// Example: ParseLevel("write") returns LevelWrite.
func ParseLevel(s string) (Level, error) {
	for l := LevelRead; l <= LevelTrigger; l++ {
		if levelNames[l] == s {
			return l, nil
		}
	}
	return LevelNone, fieldError(pointerLevel, fieldInvalid, "the level must be read, write or trigger")
}

// String returns the column value of l; LevelNone and values out of range give "".
func (l Level) String() string {
	if l > LevelTrigger {
		return ""
	}
	return levelNames[l]
}

// valid reports whether l is one of the four defined levels.
func (l Level) valid() bool { return l <= LevelTrigger }

// MaxLevel returns the higher of a and b, for example the effective level over several grants
// (07 section 2.2).
func MaxLevel(a, b Level) Level { return max(a, b) }

// MinLevel returns the lower of a and b, for example the cap a principal kind puts on a level.
func MinLevel(a, b Level) Level { return min(a, b) }

// fieldError is the validation_failed error with one field error on pointer.
func fieldError(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, detail).
		WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// internalError is the error for a caller's programming error, such as a zero id.
func internalError(detail string) *errs.Error {
	return errs.New(errs.CodeInternalError, "authz: "+detail)
}
