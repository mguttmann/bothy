// SPDX-License-Identifier: Apache-2.0

package domain

import "regexp"

// errorCodePattern is the form of backup_runs.error: lowercase letters, digits and underscores,
// starting with a letter, at most 64 characters. Output text, paths and secrets cannot take this
// form (03 4.12: "code without content or secrets").
var errorCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ValidErrorCode reports whether code has the form of an error code, for example
// "pgbackrest_exit_82".
func ValidErrorCode(code string) bool { return errorCodePattern.MatchString(code) }
