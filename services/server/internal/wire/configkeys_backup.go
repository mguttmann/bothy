// SPDX-License-Identifier: Apache-2.0

package wire

import "github.com/OWNER/bothy/services/server/internal/platform/config"

// The keys of the cross-checks backup_schedules and backup_offsite_target.
const (
	keyBackupPGFullSchedule  = "backup.pg.full_schedule"
	keyBackupPGDiffSchedule  = "backup.pg.diff_schedule"
	keyBackupLogicalSchedule = "backup.logical.schedule"
	keyBackupObjectsSchedule = "backup.objects.schedule"
	keyBackupOffsiteTarget   = "backup.offsite.target"
	keyRestoreTestSchedule   = "backup.restore_test.schedule"
)

// The defaults of the retentions and the region of the off-site target (16-operations.md
// "Configuration values of this chapter").
const (
	backupLogicalRetentionDefault = "30d"
	backupObjectsRetentionDefault = "90d"
	backupOffsiteRegionDefault    = "us-east-1"
)

// backupScheduleKeys are the cron schedules of bothy-backupd, in the order of
// docs/spec/16-operations.md 8.1, and the schedule of the restore test of 8.3 (M7-03).
var backupScheduleKeys = []string{keyBackupPGFullSchedule, keyBackupPGDiffSchedule, keyBackupLogicalSchedule, keyBackupObjectsSchedule, keyRestoreTestSchedule}

// backupConfigKeys returns the keys of the backup service bothy-backupd (M7-02) from
// docs/spec/16-operations.md "Configuration values of this chapter". backup.pg.archive_timeout is
// set by the PostgreSQL image (M7-01); backup.max_age, backup.restore_test.max_age and
// dr.rpo_target are read by the alert rules only and stay planned until a consumer in the server
// reads them. The restore test (M7-03) reads backup.restore_test.schedule, sample_files,
// sample_threads and dr.rto_target.
func backupConfigKeys() []config.Key {
	return []config.Key{
		{Name: keyBackupPGFullSchedule, Kind: config.KindString, Default: "0 2 * * 0", Chapter: chapter16,
			Doc: "Five-field cron schedule in UTC of the full pgBackRest backup."},
		{Name: keyBackupPGDiffSchedule, Kind: config.KindString, Default: "0 2 * * 1-6", Chapter: chapter16,
			Doc: "Five-field cron schedule in UTC of the differential pgBackRest backup."},
		{Name: "backup.pg.retention_full", Kind: config.KindInt, Default: "4", Min: "1", Chapter: chapter16,
			Doc: "Number of full backups kept in the off-site repository repo2."},
		{Name: "backup.pg.local_retention_full", Kind: config.KindInt, Default: "1", Min: "1", Chapter: chapter16,
			Doc: "Number of full backups kept in the local repository repo1."},
		{Name: keyBackupLogicalSchedule, Kind: config.KindString, Default: "0 3 * * *", Chapter: chapter16,
			Doc: "Five-field cron schedule in UTC of the logical backup with pg_dump."},
		{Name: "backup.logical.retention", Kind: config.KindDuration, Default: backupLogicalRetentionDefault, Min: "1d", Chapter: chapter16,
			Doc: "How long the snapshots of the logical backup are kept."},
		{Name: keyBackupObjectsSchedule, Kind: config.KindString, Default: "30 3 * * *", Chapter: chapter16,
			Doc: "Five-field cron schedule in UTC of the object backup."},
		{Name: "backup.objects.retention", Kind: config.KindDuration, Default: backupObjectsRetentionDefault, Min: "1d", Chapter: chapter16,
			Doc: "How long the snapshots of the object backup are kept."},
		{Name: keyBackupOffsiteTarget, Kind: config.KindString, Default: "", Chapter: chapter16,
			Doc: "Off-site target s3:https://<host>[:<port>]/<bucket>[/<prefix>]; empty means local backup only."},
		{Name: "backup.offsite.region", Kind: config.KindString, Default: backupOffsiteRegionDefault, Chapter: chapter16,
			Doc: "Region of the S3 off-site target."},
		{Name: "backup.offsite.use_path_style", Kind: config.KindBool, Default: "true", Chapter: chapter16,
			Doc: "Whether the off-site target is addressed path-style instead of by bucket host name."},
		{Name: keyRestoreTestSchedule, Kind: config.KindString, Default: "0 5 * * 6", Chapter: chapter16,
			Doc: "Five-field cron schedule in UTC of the automatic restore test."},
		{Name: "backup.restore_test.sample_files", Kind: config.KindInt, Default: "200", Min: "1", Chapter: chapter16,
			Doc: "Number of files whose object the restore test fetches from the object backup and hashes."},
		{Name: "backup.restore_test.sample_threads", Kind: config.KindInt, Default: "200", Min: "1", Chapter: chapter16,
			Doc: "Number of threads whose thread_seq the restore test checks for gaps."},
		{Name: "dr.rto_target", Kind: config.KindDuration, Default: "4h", Min: "1s", Chapter: chapter16,
			Doc: "Target for the time to restore operation; the restore test fails when it takes longer."},
	}
}
