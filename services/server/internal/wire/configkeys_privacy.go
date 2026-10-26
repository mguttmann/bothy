// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	privacydomain "github.com/OWNER/bothy/services/server/internal/privacy/domain"
	statusdomain "github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The keys of the cross-check deletion_journal_retention (DP-T08, M7-07).
const (
	keyDeletionJournalRetention = "privacy.deletion_journal_retention"
	keyBackupLogicalRetention   = "backup.logical.retention"
	keyBackupObjectsRetention   = "backup.objects.retention"
	keyBackupPGRetentionFull    = "backup.pg.retention_full"
	keyBackupPGLocalRetention   = "backup.pg.local_retention_full"
)

// deletionJournalRetentionKeys are the keys the cross-check reads.
var deletionJournalRetentionKeys = []string{keyDeletionJournalRetention, keyBackupLogicalRetention, keyBackupObjectsRetention,
	keyBackupPGRetentionFull, keyBackupPGLocalRetention, keyBackupPGFullSchedule}

// privacyConfigKeys returns the keys of the privacy requests (docs/spec/20-privacy.md section 13,
// M7-06). The statutory deadline of a request is no key (R-DP-05).
func privacyConfigKeys() []config.Key {
	return []config.Key{
		{Name: "privacy.export_ttl", Kind: config.KindDuration, Default: "7d", Min: "1h", Chapter: chapter20,
			Doc: "How long an export package can be downloaded before it is deleted."},
		{Name: "privacy.export_requires_approval", Kind: config.KindBool, Default: "false", Chapter: chapter20,
			Doc: "Whether the export of one's own data needs the approval of an owner."},
		{Name: "privacy.account_deletion_content", Kind: config.KindEnum, Default: string(personaldata.KeepPseudonymized),
			Enum: []string{string(personaldata.KeepPseudonymized), string(personaldata.DeleteContent)}, Chapter: chapter20,
			Doc: "Contents of a deleted account: keep them under the pseudonym, or delete own messages and files."},
		{Name: "privacy.request_reminder_lead", Kind: config.KindDuration, Default: "7d", Min: "1h", Max: "27d", Chapter: chapter20,
			Doc: "How long before the statutory deadline of an open privacy request the owners are reminded."},
		{Name: "retention.privacy_job", Kind: config.KindDuration, Default: "1095d", Min: "1d", Chapter: chapter20,
			Doc: "How long finished privacy requests are kept (without export packages)."},
		{Name: keyDeletionJournalRetention, Kind: config.KindDuration, Default: "400d", Min: "1d", Chapter: chapter20,
			Doc: "How long the objects of the deletion journal are kept; at least the longest backup period."},
	}
}

// BackupPeriodsFrom returns the backup periods of 16 section 8.1 from v (M7-07): the retention of
// the logical and of the object backup, the larger pgBackRest count of full backups and the longest
// interval of backup.pg.full_schedule. ok is false when the schedule does not parse; the
// cross-check backup_schedules names that key.
func BackupPeriodsFrom(v *config.Values) (privacydomain.BackupPeriods, bool) {
	s, err := statusdomain.ParseSchedule(v.String(keyBackupPGFullSchedule))
	if err != nil {
		return privacydomain.BackupPeriods{}, false
	}
	return privacydomain.BackupPeriods{
		Logical: v.Duration(keyBackupLogicalRetention), Objects: v.Duration(keyBackupObjectsRetention),
		FullCount: int(max(v.Int(keyBackupPGRetentionFull), v.Int(keyBackupPGLocalRetention))), FullInterval: s.LongestGap(),
	}, true
}

// deletionJournalRetention is the cross-check deletion_journal_retention (DP-T08, 20 section 4.6):
// privacy.deletion_journal_retention must be at least the longest backup period, or a restore of a
// kept backup could miss journal objects and bring back deleted data; every process that loads the
// configuration (api, worker, bothy config check) stops then. The pgBackRest retention counts full
// backups and becomes a period as the count times the longest interval of backup.pg.full_schedule,
// rounded up to whole days (docs/decisions.d/M7-07.md). The error names the keys, never a value.
func deletionJournalRetention(v *config.Values) error {
	p, ok := BackupPeriodsFrom(v)
	if !ok {
		return nil
	}
	if !privacydomain.JournalRetentionCovers(v.Duration(keyDeletionJournalRetention), p) {
		return errors.New(keyDeletionJournalRetention + " must be at least the longest backup period: " + keyBackupLogicalRetention +
			", " + keyBackupObjectsRetention + " and the larger of " + keyBackupPGRetentionFull + " and " + keyBackupPGLocalRetention +
			" times the interval of " + keyBackupPGFullSchedule)
	}
	return nil
}
