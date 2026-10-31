// SPDX-License-Identifier: Apache-2.0

/// Access to the sidebar structure: spaces, sidebar states, channels with their operator
/// setting, forum tags and the caller's access as the server reported it (15 6.1).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../mappers/structure.dart';
import '../models.dart';
import 'upsert.dart';

/// Data access object of the structure tables; hands out domain types only.
final class StructureDao {
  /// Creates the DAO on a database; use `LocalDatabase.structureDao`.
  StructureDao(this._db);

  final LocalDatabase _db;

  /// Stores [space] if its version is greater than the stored one (04 9 item 3); returns
  /// whether it applied.
  Future<bool> upsertSpaceIfNewer(Space space) {
    final row = spaceToCompanion(space);
    return upsertIfNewerVersion(
      _db,
      _db.spaces,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// The space [id], or null.
  Future<Space?> spaceById(SpaceId id) async {
    final row = await (_db.select(
      _db.spaces,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : spaceFromRow(row);
  }

  /// All spaces in global order (`sort_key`, compared as bytes).
  Stream<List<Space>> watchSpaces() =>
      (_db.select(_db.spaces)
            ..orderBy([(t) => OrderingTerm(expression: t.sortKey)]))
          .map(spaceFromRow)
          .watch();

  /// Stores the caller's [access] to space [id] from a `SpaceView`; does nothing when the
  /// space is not stored.
  Future<void> putSpaceAccess(SpaceId id, AccessView access) async {
    final columns = accessToColumns(access);
    await (_db.update(_db.spaces)..where((t) => t.id.equals(id.value))).write(
      SpacesCompanion(
        level: Value(columns.level),
        capabilities: Value(columns.capabilities),
      ),
    );
  }

  /// The caller's access to space [id], or null when none was reported.
  Stream<AccessView?> watchSpaceAccess(SpaceId id) =>
      (_db.select(_db.spaces)..where((t) => t.id.equals(id.value)))
          .map((row) => accessFromColumns(row.level, row.capabilities))
          .watchSingleOrNull();

  /// Stores the sidebar [state] if its version is newer; returns whether it applied.
  Future<bool> upsertUserSpaceStateIfNewer(UserSpaceState state) {
    final row = userSpaceStateToCompanion(state);
    return upsertIfNewerVersion(
      _db,
      _db.userSpaceStates,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Writes the sidebar state of [spaceId] at once on this device, before the server confirms
  /// it (brief M4-19 Design note 2): [collapsed], [sortKey] when given (null keeps the stored
  /// order) and [channelOrder] when given (null keeps the stored list, an empty list clears it
  /// to the default order; M4-51). The version stays, so the server's state of the write
  /// (stored by the executor) and a later change of another device win by version (04 14); a
  /// missing row starts at version 0.
  Future<void> putUserSpaceStateLocally(
    SpaceId spaceId, {
    required bool collapsed,
    String? sortKey,
    List<ChannelId>? channelOrder,
  }) => _db.transaction(() async {
    final order = channelOrder == null
        ? const Value<String?>.absent()
        : Value(encodeChannelOrder(channelOrder));
    final updated =
        await (_db.update(
          _db.userSpaceStates,
        )..where((t) => t.spaceId.equals(spaceId.value))).write(
          UserSpaceStatesCompanion(
            collapsed: Value(collapsed),
            sortKey: sortKey == null ? const Value.absent() : Value(sortKey),
            channelOrder: order,
          ),
        );
    if (updated > 0) return;
    await _db
        .into(_db.userSpaceStates)
        .insert(
          UserSpaceStatesCompanion.insert(
            spaceId: spaceId.value,
            collapsed: collapsed,
            sortKey: Value(sortKey),
            channelOrder: order,
            version: 0,
          ),
        );
  });

  /// The sidebar states of all spaces.
  Stream<List<UserSpaceState>> watchUserSpaceStates() =>
      _db.select(_db.userSpaceStates).map(userSpaceStateFromRow).watch();

  /// Stores the channel sidebar [state] (the own thread order) if its version is newer
  /// (04 9 item 3, 04 14); returns whether it applied (M4-51).
  Future<bool> upsertUserChannelStateIfNewer(UserChannelState state) {
    final row = userChannelStateToCompanion(state);
    return upsertIfNewerVersion(
      _db,
      _db.userChannelStates,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Writes the own [threadOrder] of [channelId] at once on this device, before the server
  /// confirms it, as [putUserSpaceStateLocally] does (M4-51, D-M4-51-3): the version and the
  /// device of the last write stay, so the server's state of the write and a later change of
  /// another device win by version (04 14); a missing row starts at version 0.
  Future<void> putUserChannelStateLocally(
    ChannelId channelId,
    List<ThreadId> threadOrder,
  ) => _db.transaction(() async {
    final order = encodeTexts(threadOrder);
    final updated =
        await (_db.update(_db.userChannelStates)
              ..where((t) => t.channelId.equals(channelId.value)))
            .write(UserChannelStatesCompanion(threadOrder: Value(order)));
    if (updated > 0) return;
    await _db
        .into(_db.userChannelStates)
        .insert(
          UserChannelStatesCompanion.insert(
            channelId: channelId.value,
            threadOrder: Value(order),
            version: 0,
          ),
        );
  });

  /// The sidebar states of all channels that have one (M4-51).
  Stream<List<UserChannelState>> watchUserChannelStates() =>
      _db.select(_db.userChannelStates).map(userChannelStateFromRow).watch();

  /// Stores [channel] and its operator setting if its version is newer, in one transaction;
  /// returns whether it applied.
  Future<bool> upsertChannelIfNewer(Channel channel) =>
      _db.transaction(() async {
        final row = channelToCompanion(channel);
        final applied = await upsertIfNewerVersion(
          _db,
          _db.channels,
          row,
          update: row,
          version: (t) => t.version,
        );
        if (applied) {
          await _db
              .into(_db.channelOperators)
              .insertOnConflictUpdate(channelOperatorToCompanion(channel));
        }
        return applied;
      });

  /// The channel [id], or null.
  Future<Channel?> channelById(ChannelId id) async {
    final rows = await _channelQuery((t) => t.id.equals(id.value)).get();
    return rows.isEmpty ? null : rows.first;
  }

  /// The channel [id] on every change, null while it is not stored.
  Stream<Channel?> watchChannel(ChannelId id) =>
      _channelQuery((t) => t.id.equals(id.value)).watchSingleOrNull();

  /// The channels of space [spaceId] in their order; with null the direct messages.
  Stream<List<Channel>> watchChannels(SpaceId? spaceId) => _channelQuery(
    (t) =>
        spaceId == null ? t.spaceId.isNull() : t.spaceId.equals(spaceId.value),
  ).watch();

  Selectable<Channel> _channelQuery(
    Expression<bool> Function($ChannelsTable) filter,
  ) {
    final query =
        _db.select(_db.channels).join([
            leftOuterJoin(
              _db.channelOperators,
              _db.channelOperators.channelId.equalsExp(_db.channels.id),
            ),
          ])
          ..where(filter(_db.channels))
          ..orderBy([OrderingTerm(expression: _db.channels.sortKey)]);
    return query.map(
      (r) => channelFromRow(
        r.readTable(_db.channels),
        r.readTableOrNull(_db.channelOperators),
      ),
    );
  }

  /// Stores the caller's [access] to channel [id] from a `ChannelView`; does nothing when the
  /// channel is not stored.
  Future<void> putChannelAccess(ChannelId id, AccessView access) async {
    final columns = accessToColumns(access);
    await (_db.update(_db.channels)..where((t) => t.id.equals(id.value))).write(
      ChannelsCompanion(
        level: Value(columns.level),
        capabilities: Value(columns.capabilities),
      ),
    );
  }

  /// The caller's access to channel [id], or null when none was reported.
  Stream<AccessView?> watchChannelAccess(ChannelId id) =>
      (_db.select(_db.channels)..where((t) => t.id.equals(id.value)))
          .map((row) => accessFromColumns(row.level, row.capabilities))
          .watchSingleOrNull();

  /// Replaces the forum tags of [channelId] with [tags] in one transaction (a tag change
  /// raises the channel version and the client reloads all tags, 03 `forum_tags`).
  Future<void> putForumTags(ChannelId channelId, List<ForumTag> tags) =>
      _db.transaction(() async {
        await (_db.delete(
          _db.forumTags,
        )..where((t) => t.channelId.equals(channelId.value))).go();
        await _db.batch(
          (b) => b.insertAll(_db.forumTags, tags.map(forumTagToCompanion)),
        );
      });

  /// The forum tags of [channelId] in their order.
  Stream<List<ForumTag>> watchForumTags(ChannelId channelId) =>
      (_db.select(_db.forumTags)
            ..where((t) => t.channelId.equals(channelId.value))
            ..orderBy([(t) => OrderingTerm(expression: t.sortKey)]))
          .map(forumTagFromRow)
          .watch();
}
