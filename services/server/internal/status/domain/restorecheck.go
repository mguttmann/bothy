// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// RestoreChecksSchema is the schema name of backup_runs.checks of a restore test
// (docs/spec/03-datenmodell.md 4.12).
const RestoreChecksSchema = "restore_test_checks.v1"

// CheckName names one of the seven checks of the restore test (docs/spec/16-betrieb.md 8.3).
type CheckName string

// The checks of 16 8.3, in their order.
const (
	// CheckPgBackRest is item 1: the restore with its checksums, pgbackrest verify and the start
	// of the scratch database.
	CheckPgBackRest CheckName = "pgbackrest"
	// CheckManifest is item 2: schema version and row counts against the manifest.
	CheckManifest CheckName = "manifest"
	// CheckEventLog is item 3: event_log without gaps per partition and sequence_counters.
	CheckEventLog CheckName = "event_log"
	// CheckAuditChain is item 4: the audit chain with the verification of audit.verify.
	CheckAuditChain CheckName = "audit_chain"
	// CheckThreadSample is item 5: thread_seq without gaps in a sample of threads.
	CheckThreadSample CheckName = "thread_sample"
	// CheckFileSample is item 6: SHA-256 of a sample of objects from the object backup.
	CheckFileSample CheckName = "file_sample"
	// CheckDuration is item 7: the duration of the whole test against dr.rto_target.
	CheckDuration CheckName = "duration"
)

// RestoreCheckNames returns the seven checks in the order of 16 8.3, the order of
// RestoreChecks.Checks.
func RestoreCheckNames() []CheckName {
	return []CheckName{CheckPgBackRest, CheckManifest, CheckEventLog, CheckAuditChain, CheckThreadSample, CheckFileSample, CheckDuration}
}

// CheckOutcome is the result of one check.
type CheckOutcome string

// The results of a check (brief M7-03 rule 13).
const (
	ResultPassed  CheckOutcome = "passed"
	ResultFailed  CheckOutcome = "failed"
	ResultSkipped CheckOutcome = "skipped"
)

// ReasonRestoreFailed is the reason of a check skipped because the copy could not be restored
// or started (brief M7-03 design note 1).
const ReasonRestoreFailed = "restore_failed"

// ErrInvalidRestoreChecks reports a value that is not restore_test_checks.v1. The message names
// the field, never its value.
var ErrInvalidRestoreChecks = errors.New("status: invalid restore test checks")

// CheckResult is the result of one check: its name, outcome, duration and, for failed and
// skipped, a reason code (ValidErrorCode, never text).
type CheckResult struct {
	// Name is the check.
	Name CheckName `json:"name"`
	// Result is passed, failed or skipped.
	Result CheckOutcome `json:"result"`
	// DurationMS is the time the check took, in milliseconds; 0 for a skipped check.
	DurationMS int64 `json:"duration_ms"`
	// Reason is the code of a failed or skipped check, for example event_log_gap or
	// restore_failed; empty for a passed check.
	Reason string `json:"reason,omitempty"`
	// Items is how many partitions, threads, files or rows the check looked at; 0 where it
	// counts nothing.
	Items int64 `json:"items,omitempty"`
}

// RestoreChecks is restore_test_checks.v1, the value of backup_runs.checks of a restore test:
// the backup it restored and one result per check of 16 8.3, in order.
type RestoreChecks struct {
	// Schema is RestoreChecksSchema.
	Schema string `json:"schema"`
	// BackupID is the backup_runs id of the restored backup; empty when there was none.
	BackupID string `json:"backup_id,omitempty"`
	// Label is the pgBackRest label of the restored backup; empty when there was none.
	Label string `json:"pgbackrest_label,omitempty"`
	// Checks are the seven results in the order of RestoreCheckNames.
	Checks []CheckResult `json:"checks"`
}

// invalidChecks returns ErrInvalidRestoreChecks for field with the rule it breaks.
func invalidChecks(field, rule string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidRestoreChecks, field, rule)
}

// Validate checks c against restore_test_checks.v1: the schema name; backup_id a UUID and
// pgbackrest_label a pgBackRest label when set; exactly the seven checks in the order of
// RestoreCheckNames; each result passed, failed or skipped; durations and items not negative;
// a reason code on every failed and skipped check and none on a passed one.
//
// It returns nil or an error wrapping ErrInvalidRestoreChecks that names the first failing field.
func (c RestoreChecks) Validate() error {
	switch {
	case c.Schema != RestoreChecksSchema:
		return invalidChecks("schema", "must be "+RestoreChecksSchema)
	case c.BackupID != "" && !uuidPattern.MatchString(c.BackupID):
		return invalidChecks("backup_id", "must be a UUID")
	case c.Label != "" && !labelPattern.MatchString(c.Label):
		return invalidChecks("pgbackrest_label", "must be a pgBackRest backup label")
	}
	names := RestoreCheckNames()
	if len(c.Checks) != len(names) {
		return invalidChecks("checks", "must hold the seven checks of 16 8.3")
	}
	for i, r := range c.Checks {
		field := "checks." + string(names[i])
		switch {
		case r.Name != names[i]:
			return invalidChecks(fmt.Sprintf("checks[%d].name", i), "must be "+string(names[i]))
		case r.Result != ResultPassed && r.Result != ResultFailed && r.Result != ResultSkipped:
			return invalidChecks(field+".result", "must be passed, failed or skipped")
		case r.DurationMS < 0:
			return invalidChecks(field+".duration_ms", "must not be negative")
		case r.Items < 0:
			return invalidChecks(field+".items", "must not be negative")
		case r.Result == ResultPassed && r.Reason != "":
			return invalidChecks(field+".reason", "must be empty for a passed check")
		case r.Result != ResultPassed && !ValidErrorCode(r.Reason):
			return invalidChecks(field+".reason", "must be a code for a failed or skipped check")
		}
	}
	return nil
}

// Passed reports whether no check failed. A skipped check does not fail the test: only checks
// 2 to 6 are ever skipped, and only after check 1 failed.
func (c RestoreChecks) Passed() bool { return len(c.Failed()) == 0 }

// Failed returns the names of the failed checks in order.
func (c RestoreChecks) Failed() []CheckName {
	var out []CheckName
	for _, r := range c.Checks {
		if r.Result == ResultFailed {
			out = append(out, r.Name)
		}
	}
	return out
}

// Marshal validates c and returns its JSON, the stored form of backup_runs.checks.
//
// It returns nil and an error wrapping ErrInvalidRestoreChecks when Validate fails.
func (c RestoreChecks) Marshal() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("%w: encoding failed", ErrInvalidRestoreChecks)
	}
	return b, nil
}

// ParseRestoreChecks decodes b strictly (an unknown field, a second value or a wrong type fails)
// and validates the result.
//
// Example:
//
//	c, err := domain.ParseRestoreChecks(row.Checks)
func ParseRestoreChecks(b []byte) (RestoreChecks, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c RestoreChecks
	if err := dec.Decode(&c); err != nil {
		return RestoreChecks{}, fmt.Errorf("%w: not a JSON object of %s", ErrInvalidRestoreChecks, RestoreChecksSchema)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return RestoreChecks{}, fmt.Errorf("%w: trailing data after the object", ErrInvalidRestoreChecks)
	}
	if err := c.Validate(); err != nil {
		return RestoreChecks{}, err
	}
	return c, nil
}
