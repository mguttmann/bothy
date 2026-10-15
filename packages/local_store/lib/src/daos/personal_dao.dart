// SPDX-License-Identifier: Apache-2.0

/// Access to the personal sync data (04 14): read states through `mergeReadState` of M4-08,
/// drafts, settings, notification preferences and quiet hours under the version rule. The
/// caller ignores the echo of its own device (`updated_by_device_id`); the DAO stores what it is
/// given.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/personal.dart';
import 'upsert.dart';

/// Data access object of the personal data; hands out domain types only.
final class PersonalDao {
  /// Creates the DAO on a database; use `LocalDatabase.personalDao`.
  PersonalDao(this._db);

  final LocalDatabase _db;

  /// Merges [incoming] into the stored read state of its thread with `mergeReadState`
  /// (monotonic `last_read_seq`, 04 14; [reset] for `markThreadUnread`) and returns the
  /// stored result.
  Future<ReadState> applyReadState(ReadState incoming, {bool reset = false}) =>
      _db.transaction(() async {
        final local = await readState(incoming.threadId);
        final merged = local == null
            ? incoming
            : mergeReadState(local, incoming, reset: reset);
        if (merged != local) {
          await _db
              .into(_db.readStates)
              .insertOnConflictUpdate(readStateToCompanion(merged));
        }
        return merged;
      });

  /// The read state of [threadId], or null.
  Future<ReadState?> readState(ThreadId threadId) async {
    final row = await (_db.select(
      _db.readStates,
    )..where((t) => t.threadId.equals(threadId.value))).getSingleOrNull();
    return row == null ? null : readStateFromRow(row);
  }

  /// The read states of the threads of [channelId].
  Stream<List<ReadState>> watchReadStates(ChannelId channelId) =>
      (_db.select(_db.readStates)
            ..where((t) => t.channelId.equals(channelId.value)))
          .map(readStateFromRow)
          .watch();

  /// Stores [draft] if its version is newer; returns whether it applied.
  Future<bool> upsertDraftIfNewer(Draft draft) {
    final row = draftToCompanion(draft);
    return upsertIfNewerVersion(
      _db,
      _db.drafts,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Deletes draft [id] (sent or discarded).
  Future<void> deleteDraft(DraftId id) async {
    await (_db.delete(_db.drafts)..where((t) => t.id.equals(id.value))).go();
  }

  /// The draft of [threadId], or of a new post in [channelId] when [threadId] is null.
  Stream<Draft?> watchDraft({
    required ChannelId channelId,
    ThreadId? threadId,
  }) =>
      (_db.select(_db.drafts)..where(
            (t) =>
                t.channelId.equals(channelId.value) &
                (threadId == null
                    ? t.threadId.isNull()
                    : t.threadId.equals(threadId.value)),
          ))
          .map(draftFromRow)
          .watchSingleOrNull();

  /// All drafts.
  Future<List<Draft>> allDrafts() async {
    final rows = await (_db.select(
      _db.drafts,
    )..orderBy([(t) => OrderingTerm(expression: t.id)])).get();
    return [for (final row in rows) draftFromRow(row)];
  }

  /// Stores [setting] if its version is newer; returns whether it applied.
  Future<bool> upsertSettingIfNewer(UserSetting setting) {
    final row = settingToCompanion(setting);
    return upsertIfNewerVersion(
      _db,
      _db.userSettings,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// All settings by key.
  Stream<List<UserSetting>> watchSettings() =>
      (_db.select(_db.userSettings)
            ..orderBy([(t) => OrderingTerm(expression: t.key)]))
          .map(settingFromRow)
          .watch();

  /// Stores [preference] if its version is newer; returns whether it applied.
  Future<bool> upsertNotificationPreferenceIfNewer(
    NotificationPreference preference,
  ) {
    final row = preferenceToCompanion(preference);
    return upsertIfNewerVersion(
      _db,
      _db.notificationPreferences,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// All notification preferences.
  Future<List<NotificationPreference>> notificationPreferences() async {
    final rows = await _db.select(_db.notificationPreferences).get();
    return [for (final row in rows) preferenceFromRow(row)];
  }

  /// Stores the quiet hours [rule] if its version is newer; returns whether it applied.
  Future<bool> upsertQuietHoursIfNewer(QuietHours rule) {
    final row = quietHoursToCompanion(rule);
    return upsertIfNewerVersion(
      _db,
      _db.quietHoursRules,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Deletes the quiet hours rule [id].
  Future<void> deleteQuietHours(QuietHoursId id) async {
    await (_db.delete(
      _db.quietHoursRules,
    )..where((t) => t.id.equals(id.value))).go();
  }

  /// All quiet hours rules.
  Future<List<QuietHours>> quietHours() async {
    final rows = await (_db.select(
      _db.quietHoursRules,
    )..orderBy([(t) => OrderingTerm(expression: t.id)])).get();
    return [for (final row in rows) quietHoursFromRow(row)];
  }
}
