// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ErrInvalidManifest reports a manifest that is not backup_manifest.v1. The message names the
// field, never its value: a manifest that fails may carry exactly the text it must not hold.
var ErrInvalidManifest = errors.New("status: invalid backup manifest")

// The closed forms of the manifest strings. Each admits ids, versions, labels, hashes or table
// names only, so message text, paths and secrets cannot pass (03 4.12: "never content").
var (
	uuidPattern          = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	serverVersionPattern = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,64}$`)
	hashPattern          = regexp.MustCompile(`^(?:[0-9a-f]{2})+$`)
	tableNamePattern     = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)
	labelPattern         = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}F(_[0-9]{8}-[0-9]{6}[DI])?$`)
	snapshotPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// fieldPathPattern is a field path of the decoder such as event_log.head_seq; unknown keys
	// never reach it, DisallowUnknownFields stops them first.
	fieldPathPattern = regexp.MustCompile(`^[a-z_][a-z0-9_.]{0,63}$`)
)

// invalid returns ErrInvalidManifest for field with the rule it breaks.
func invalid(field, rule string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidManifest, field, rule)
}

// Validate checks m against backup_manifest.v1 (docs/spec/03-datenmodell.md 4.12,
// docs/operations/backup-restore.md 3): the schema name; backup_id, instance_id and sync_epoch
// as UUIDs; kind one of Kinds(); both times set with finished_at not before started_at;
// server_version of version characters only; schema_version, the seqs, the counts and the sizes
// not negative; min_retained_seq not above head_seq; audit.last_hash lowercase hex, empty exactly
// when audit.last_seq is 0; row_counts keyed by table names; pgbackrest set exactly for pg_full
// and pg_diff with a pgBackRest label; restic required for logical, optional for objects (absent
// for the driver s3) and absent for the physical kinds, with a full snapshot id.
//
// It returns nil or an error wrapping ErrInvalidManifest that names the first failing field.
func (m Manifest) Validate() error {
	if err := m.validateRun(); err != nil {
		return err
	}
	if err := m.validateFacts(); err != nil {
		return err
	}
	return m.validateRefs()
}

// validateRun checks the fields that describe the run and the instance.
func (m Manifest) validateRun() error {
	switch {
	case m.Schema != ManifestSchema:
		return invalid("schema", "must be "+ManifestSchema)
	case !uuidPattern.MatchString(m.BackupID):
		return invalid("backup_id", "must be a UUID")
	case !validKind(m.Kind):
		return invalid("kind", "must be pg_full, pg_diff, logical or objects")
	case m.StartedAt.IsZero():
		return invalid("started_at", "must be set")
	case m.FinishedAt.IsZero() || m.FinishedAt.Before(m.StartedAt):
		return invalid("finished_at", "must be set and not before started_at")
	case !serverVersionPattern.MatchString(m.ServerVersion):
		return invalid("server_version", "must be 1 to 64 letters, digits, '.', '+' or '-'")
	case m.SchemaVersion < 0:
		return invalid("schema_version", "must not be negative")
	case !uuidPattern.MatchString(m.InstanceID):
		return invalid("instance_id", "must be a UUID")
	case !uuidPattern.MatchString(m.SyncEpoch):
		return invalid("sync_epoch", "must be a UUID")
	}
	return nil
}

// validateFacts checks the counts and heads read from the database.
func (m Manifest) validateFacts() error {
	switch {
	case m.EventLog.HeadSeq < 0:
		return invalid("event_log.head_seq", "must not be negative")
	case m.EventLog.MinRetainedSeq < 0 || m.EventLog.MinRetainedSeq > m.EventLog.HeadSeq:
		return invalid("event_log.min_retained_seq", "must be between 0 and head_seq")
	case m.Audit.LastSeq < 0:
		return invalid("audit.last_seq", "must not be negative")
	case m.Audit.LastSeq == 0 && m.Audit.LastHash != "":
		return invalid("audit.last_hash", "must be empty when audit.last_seq is 0")
	case m.Audit.LastSeq > 0 && !hashPattern.MatchString(m.Audit.LastHash):
		return invalid("audit.last_hash", "must be lowercase hex")
	case m.RowCounts == nil:
		return invalid("row_counts", "must be an object")
	case m.Files.Count < 0:
		return invalid("files.count", "must not be negative")
	case m.Files.TotalBytes < 0:
		return invalid("files.total_bytes", "must not be negative")
	}
	for table, n := range m.RowCounts {
		if !tableNamePattern.MatchString(table) || n < 0 {
			return invalid("row_counts", "must map table names to counts that are not negative")
		}
	}
	return nil
}

// validateRefs checks the references to the stored backup, which depend on the kind.
func (m Manifest) validateRefs() error {
	if m.Kind.Physical() {
		if m.PgBackRest == nil {
			return invalid("pgbackrest", "is required for pg_full and pg_diff")
		}
		if !labelPattern.MatchString(m.PgBackRest.Label) {
			return invalid("pgbackrest.label", "must be a pgBackRest backup label")
		}
		if m.Restic != nil {
			return invalid("restic", "must be absent for pg_full and pg_diff")
		}
		return nil
	}
	if m.PgBackRest != nil {
		return invalid("pgbackrest", "must be absent for logical and objects")
	}
	if m.Restic == nil {
		if m.Kind == KindLogical {
			return invalid("restic", "is required for logical")
		}
		return nil
	}
	if !snapshotPattern.MatchString(m.Restic.SnapshotID) {
		return invalid("restic.snapshot_id", "must be 64 lowercase hex characters")
	}
	return nil
}

// validKind reports whether k is one of Kinds().
func validKind(k Kind) bool {
	_, ok := ParseKind(string(k))
	return ok
}

// ParseManifest decodes b strictly (an unknown field at any level, a second value or a wrong
// type fails) and validates the result.
//
// It returns the manifest or an error wrapping ErrInvalidManifest; the error names the field
// where the decoder tells it, never the value.
//
// Example:
//
//	m, err := domain.ParseManifest(row.Manifest)
func ParseManifest(b []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, decodeError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("%w: trailing data after the object", ErrInvalidManifest)
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// decodeError maps a decoder error to ErrInvalidManifest without its text: the messages of
// encoding/json quote unknown field names, which may be anything.
func decodeError(err error) error {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		// Below row_counts the path holds a key of the input; only the fixed names are echoed.
		if strings.HasPrefix(typeErr.Field, "row_counts") {
			return invalid("row_counts", "has the wrong type")
		}
		if fieldPathPattern.MatchString(typeErr.Field) {
			return invalid(typeErr.Field, "has the wrong type")
		}
	}
	return fmt.Errorf("%w: not a JSON object of backup_manifest.v1", ErrInvalidManifest)
}

// Marshal validates m and returns its JSON, the stored form of the manifest.
//
// It returns nil and an error wrapping ErrInvalidManifest when Validate fails, so an invalid
// manifest is never written.
func (m Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("%w: encoding failed", ErrInvalidManifest)
	}
	return b, nil
}
