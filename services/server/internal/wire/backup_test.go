// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestBackupSettingsDefaults: BackupSettings binds the defaults of docs/spec/16-operations.md
// "Configuration values of this chapter" and the file storage the object backup reads.
func TestBackupSettingsDefaults(t *testing.T) {
	t.Parallel()
	got, err := wire.BackupSettings(loadValues(t, schema(t)))
	if err != nil {
		t.Fatal(err)
	}
	want := wire.BackupConfig{
		PGFullSchedule: "0 2 * * 0", PGDiffSchedule: "0 2 * * 1-6", PGRetentionFull: 4, PGLocalRetentionFull: 1,
		LogicalSchedule: "0 3 * * *", LogicalRetention: 30 * 24 * time.Hour,
		ObjectsSchedule: "30 3 * * *", ObjectsRetention: 90 * 24 * time.Hour,
		OffsiteTarget: "", OffsiteRegion: "us-east-1", OffsiteUsePathStyle: true,
		FilesDriver: "fs", FilesRoot: "/var/lib/bothy/files",
	}
	if got != want {
		t.Errorf("BackupSettings = %+v\nwant %+v", got, want)
	}
}

// TestBackupSettingsValues: every field takes the value of its key.
func TestBackupSettingsValues(t *testing.T) {
	t.Parallel()
	got, err := wire.BackupSettings(loadValues(t, schema(t),
		"BOTHY_BACKUP__PG__FULL_SCHEDULE=0 1 * * 6", "BOTHY_BACKUP__PG__DIFF_SCHEDULE=0 1 * * 0-5",
		"BOTHY_BACKUP__PG__RETENTION_FULL=8", "BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL=2",
		"BOTHY_BACKUP__LOGICAL__SCHEDULE=0 4 * * *", "BOTHY_BACKUP__LOGICAL__RETENTION=14d",
		"BOTHY_BACKUP__OBJECTS__SCHEDULE=45 4 * * *", "BOTHY_BACKUP__OBJECTS__RETENTION=60d",
		"BOTHY_BACKUP__OFFSITE__TARGET=s3:https://s3.example.com/bothy-backup/instance-1",
		"BOTHY_BACKUP__OFFSITE__REGION=eu-central-1", "BOTHY_BACKUP__OFFSITE__USE_PATH_STYLE=false",
		"BOTHY_FILE__FS__ROOT=/srv/files",
	))
	if err != nil {
		t.Fatal(err)
	}
	want := wire.BackupConfig{
		PGFullSchedule: "0 1 * * 6", PGDiffSchedule: "0 1 * * 0-5", PGRetentionFull: 8, PGLocalRetentionFull: 2,
		LogicalSchedule: "0 4 * * *", LogicalRetention: 14 * 24 * time.Hour,
		ObjectsSchedule: "45 4 * * *", ObjectsRetention: 60 * 24 * time.Hour,
		OffsiteTarget: "s3:https://s3.example.com/bothy-backup/instance-1", OffsiteRegion: "eu-central-1",
		OffsiteUsePathStyle: false, FilesDriver: "fs", FilesRoot: "/srv/files",
	}
	if got != want {
		t.Errorf("BackupSettings = %+v\nwant %+v", got, want)
	}
}

// TestBackupSettingsNeedsValues: a nil configuration is an error, not a zero configuration.
func TestBackupSettingsNeedsValues(t *testing.T) {
	t.Parallel()
	if _, err := wire.BackupSettings(nil); err == nil {
		t.Error("BackupSettings(nil) returned no error")
	}
}
