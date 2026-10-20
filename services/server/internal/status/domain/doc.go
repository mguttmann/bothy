// SPDX-License-Identifier: Apache-2.0

// Package domain holds the pure rules of the backup service of the status module (brief M7-02,
// docs/spec/16-betrieb.md 8): the backup kinds, the five-field cron schedules of backup.*, which
// kinds are due in a window of time, the manifest backup_manifest.v1 of
// docs/spec/03-datenmodell.md 4.12 and docs/operations/backup-restore.md 3, and the form of an
// error code. It needs no storage and no clock; callers pass the instants.
//
// Example:
//
//	full, _ := domain.ParseSchedule("0 2 * * 0")
//	diff, _ := domain.ParseSchedule("0 2 * * 1-6")
//	due := domain.DueKinds(map[domain.Kind]domain.Schedule{domain.KindPGFull: full, domain.KindPGDiff: diff}, from, to)
package domain
