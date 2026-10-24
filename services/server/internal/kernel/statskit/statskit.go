// SPDX-License-Identifier: Apache-2.0

// Package statskit holds the input rule and the read error the statistics sources of several modules share (M6-05,
// docs/decisions.d/M6-05.md D-M6-05-9): an internal library without state in the kernel core, like
// searchkit, so chat, messaging, runs, billing, files and apps check a statistics query the same
// way without importing each other.
package statskit

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// granularities are the bucket sizes date_trunc of the statistics reads accepts.
var granularities = map[string]bool{"hour": true, "day": true, "week": true, "month": true}

// CheckQuery refuses what the statistics reads cannot run: an unknown granularity or time zone
// and parallel lists of different lengths (validation_failed, a programming error of the caller).
func CheckQuery(granularity, tz string, keys, groups int) error {
	if !granularities[granularity] {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics granularity")
	}
	if _, err := time.LoadLocation(tz); err != nil || tz == "" || tz == "Local" {
		return errs.New(errs.CodeValidationFailed, ": unknown statistics time zone")
	}
	if keys != groups {
		return errs.New(errs.CodeValidationFailed, ": the statistics keys and groups differ in length")
	}
	return nil
}

// ReadError returns the error of a statistics read for its caller: an errs error unchanged, any
// other (a database error) wrapped as internal_error with "<what> failed".
func ReadError(err error, what string) error {
	if _, ok := errs.As(err); ok {
		return err
	}
	return errs.Wrap(errs.CodeInternalError, err, ": "+what+" failed")
}
