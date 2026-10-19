// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

// BackupConfig holds the settings of the backup service bothy-backupd (M7-02,
// docs/spec/16-betrieb.md 8.1): the schedules and retentions per kind, the off-site target and
// the file storage the object backup reads. The schedules and the target passed the
// cross-checks backup_schedules and backup_offsite_target when the configuration loaded.
type BackupConfig struct {
	// PGFullSchedule is backup.pg.full_schedule, the cron schedule of pg_full.
	PGFullSchedule string `config:"backup.pg.full_schedule"`
	// PGDiffSchedule is backup.pg.diff_schedule, the cron schedule of pg_diff.
	PGDiffSchedule string `config:"backup.pg.diff_schedule"`
	// PGRetentionFull is backup.pg.retention_full, the full backups kept in repo2.
	PGRetentionFull int `config:"backup.pg.retention_full"`
	// PGLocalRetentionFull is backup.pg.local_retention_full, the full backups kept in repo1.
	PGLocalRetentionFull int `config:"backup.pg.local_retention_full"`
	// LogicalSchedule is backup.logical.schedule, the cron schedule of logical.
	LogicalSchedule string `config:"backup.logical.schedule"`
	// LogicalRetention is backup.logical.retention.
	LogicalRetention time.Duration `config:"backup.logical.retention"`
	// ObjectsSchedule is backup.objects.schedule, the cron schedule of objects.
	ObjectsSchedule string `config:"backup.objects.schedule"`
	// ObjectsRetention is backup.objects.retention.
	ObjectsRetention time.Duration `config:"backup.objects.retention"`
	// OffsiteTarget is backup.offsite.target; "" means local backup only.
	OffsiteTarget string `config:"backup.offsite.target"`
	// OffsiteRegion is backup.offsite.region.
	OffsiteRegion string `config:"backup.offsite.region"`
	// OffsiteUsePathStyle is backup.offsite.use_path_style.
	OffsiteUsePathStyle bool `config:"backup.offsite.use_path_style"`
	// FilesDriver is file.storage.driver: the object backup covers the files of the driver fs;
	// the driver s3 is mirrored instead (16 8.1).
	FilesDriver string `config:"file.storage.driver"`
	// FilesRoot is file.fs.root, the directory the object backup reads with the driver fs.
	FilesRoot string `config:"file.fs.root"`
}

// BackupSettings binds the keys of bothy-backupd from v.
//
// The error names the key whose binding failed (config.Bind), never a value; nil values are an
// error.
//
// Example:
//
//	cfg, err := wire.BackupSettings(values)
//	// cfg.PGFullSchedule "0 2 * * 0" with the defaults
func BackupSettings(v *config.Values) (BackupConfig, error) {
	var c BackupConfig
	if err := config.Bind(v, &c); err != nil {
		return BackupConfig{}, err
	}
	return c, nil
}
