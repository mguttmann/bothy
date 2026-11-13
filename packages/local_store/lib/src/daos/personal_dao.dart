// SPDX-License-Identifier: Apache-2.0

/// Access to the personal sync data (04 14): read states through `mergeReadState` of M4-08,
/// drafts, settings, notification preferences and quiet hours under the version rule; of the
/// drafts one per thread and per forum (docs/decisions.d/FIX-COMPOSER-DRAFT-LOST.md). The
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

  /// The read states of every thread (the counters of the sidebar, 04 14).
  Stream<List<ReadState>> watchAllReadStates() =>
      _db.select(_db.readStates).map(readStateFromRow).watch();

  /// Stores [draft] as the one draft of its target (its thread, or without a thread a new
  /// post in its channel); returns whether it applied.
  ///
  /// The server keeps one draft per person and target and deletes it when it is sent or
  /// emptied (04 14); the next draft of that target is a new row with a new ID whose version
  /// starts at 1 again. So versions compare only under the same ID (the version rule). A
  /// stored row of the target with another ID is the draft before [draft] and goes, unless it
  /// was written later (`updated_at`, the order of the server): then [draft] is a late change
  /// of a draft that was replaced already and is not stored. A row without a time never holds
  /// a change back.
  Future<bool> upsertDraftIfNewer(Draft draft) => _db.transaction(() async {
    final others =
        await (_db.select(_db.drafts)..where(
              (t) =>
                  _ofTarget(t, draft.channelId, draft.threadId) &
                  t.id.equals(draft.id.value).not(),
            ))
            .get();
    final at = draft.updatedAt;
    for (final other in others) {
      final stored = other.updatedAt;
      if (at != null && stored != null && stored.isAfter(at)) return false;
    }
    for (final other in others) {
      await _deleteRow(other.id);
    }
    final row = draftToCompanion(draft);
    return upsertIfNewerVersion(
      _db,
      _db.drafts,
      row,
      update: row,
      version: (t) => t.version,
    );
  });

  /// Deletes draft [id] (the server deleted it: sent, emptied or discarded), and with it
  /// every stored row of the same target that is not later: the server keeps one draft per
  /// target, so such a row is a draft that ended before, and would come back as the draft
  /// otherwise. A draft that is not stored deletes nothing, and a row without a time takes
  /// no other row with it.
  Future<void> deleteDraft(DraftId id) => _db.transaction(() async {
    final row = await (_db.select(
      _db.drafts,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    if (row == null) return;
    await _deleteRow(row.id);
    final written = row.updatedAt;
    if (written == null) return;
    final thread = row.threadId;
    await endDraft(
      channelId: ChannelId.parse(row.channelId),
      threadId: thread == null ? null : ThreadId.parse(thread),
      until: written,
    );
  });

  /// Ends the stored draft of [threadId], or without a thread of a new post in [channelId]:
  /// every stored row of that target written up to [until] goes (every row without [until]),
  /// and a row written later stays, because it is the next draft of the target.
  ///
  /// For a draft this device knows the server deleted without the ID of its row at hand: the
  /// answer to an empty draft write (04 14: an empty draft is deleted).
  Future<void> endDraft({
    required ChannelId channelId,
    ThreadId? threadId,
    DateTime? until,
  }) => _db.transaction(() async {
    final rows = await (_db.select(
      _db.drafts,
    )..where((t) => _ofTarget(t, channelId, threadId))).get();
    for (final row in rows) {
      final written = row.updatedAt;
      if (until != null && written != null && written.isAfter(until)) continue;
      await _deleteRow(row.id);
    }
  });

  Future<void> _deleteRow(String id) =>
      (_db.delete(_db.drafts)..where((t) => t.id.equals(id))).go();

  /// The rows of one target (03 `drafts`: one per person and thread, one per person and
  /// forum with `thread_id` NULL): the draft of [threadId], or without a thread the draft of a
  /// new post in [channelId].
  Expression<bool> _ofTarget(
    $DraftsTable t,
    ChannelId channelId,
    ThreadId? threadId,
  ) => threadId == null
      ? t.channelId.equals(channelId.value) & t.threadId.isNull()
      : t.threadId.equals(threadId.value);

  /// The draft of [threadId], or of a new post in [channelId] when [threadId] is null: the
  /// row written last. [upsertDraftIfNewer] keeps one row per target; where an earlier version
  /// of the app left several, the newest is the draft and the next write or delete of the
  /// target removes the others.
  Stream<Draft?> watchDraft({
    required ChannelId channelId,
    ThreadId? threadId,
  }) =>
      (_db.select(_db.drafts)
            ..where(
              (t) =>
                  t.channelId.equals(channelId.value) &
                  (threadId == null
                      ? t.threadId.isNull()
                      : t.threadId.equals(threadId.value)),
            )
            ..orderBy([
              (t) => OrderingTerm(
                expression: t.updatedAt,
                mode: OrderingMode.desc,
                nulls: NullsOrder.last,
              ),
              (t) => OrderingTerm(expression: t.rowId, mode: OrderingMode.desc),
            ])
            ..limit(1))
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

  /// Deletes setting [key] when [version] is greater than the stored version
  /// (`settings.updated` with `deleted: true`, 04 14); returns whether a row went.
  Future<bool> deleteSettingIfNewer(String key, ResourceVersion version) async {
    final deleted =
        await (_db.delete(_db.userSettings)..where(
              (t) =>
                  t.key.equals(key) &
                  t.version.isSmallerThanValue(version.value),
            ))
            .go();
    return deleted > 0;
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

  /// Deletes the preference of [targetKind] and [targetId] when [version] is greater than the
  /// stored version (`notification_pref.updated` with `deleted: true`, 04 14); returns whether
  /// a row went.
  Future<bool> deleteNotificationPreferenceIfNewer(
    NotificationTargetKind targetKind,
    String targetId,
    ResourceVersion version,
  ) async {
    final deleted =
        await (_db.delete(_db.notificationPreferences)..where(
              (t) =>
                  t.targetKind.equals(targetKind.code) &
                  t.targetId.equals(targetId) &
                  t.version.isSmallerThanValue(version.value),
            ))
            .go();
    return deleted > 0;
  }

  /// All notification preferences.
  Future<List<NotificationPreference>> notificationPreferences() async {
    final rows = await _db.select(_db.notificationPreferences).get();
    return [for (final row in rows) preferenceFromRow(row)];
  }

  /// Every notification preference, emitted on each change (the muted state of the sidebar).
  Stream<List<NotificationPreference>> watchNotificationPreferences() =>
      _db.select(_db.notificationPreferences).map(preferenceFromRow).watch();

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

  /// Every quiet hours rule by ID, emitted on each change (the screen D9, M4-33).
  Stream<List<QuietHours>> watchQuietHours() =>
      (_db.select(_db.quietHoursRules)
            ..orderBy([(t) => OrderingTerm(expression: t.id)]))
          .map(quietHoursFromRow)
          .watch();

  /// All quiet hours rules.
  Future<List<QuietHours>> quietHours() async {
    final rows = await (_db.select(
      _db.quietHoursRules,
    )..orderBy([(t) => OrderingTerm(expression: t.id)])).get();
    return [for (final row in rows) quietHoursFromRow(row)];
  }
}
