// SPDX-License-Identifier: Apache-2.0

/// Access to `forum_view_states`, `channel_view_states`, `space_view_states` and
/// `shell_view_state`: how forums, channel groups and spaces look on this device (brief M4-19 Design note 4, docs/DECISIONS.md D-M4-19-4;
/// 01 6.8 item 7, D-SB-6), and whether the channel column next to an open thread is hidden (01
/// D1, D2; M4-42), whether the session column shows (01 D1, D2; M4-45) and whether a forum group
/// shows every open post (01 6.8 item 3; M4-46) and whether a space is collapsed (01 6.7,
/// R-PR-24; FIX-device-mirroring-2) and whether code blocks wrap their long lines (01 6.14;
/// M4-49). Nothing of it is synchronized: 01 11 has no key for it.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';

/// Data access object of the local view state of forums, channel groups and the forum overview.
///
/// Example:
/// ```dart
/// await db.forumViewDao.setDoneCollapsed(forum, collapsed: false);
/// db.forumViewDao.watchDoneCollapsed(forum).listen(print); // false
/// await db.forumViewDao.setGroupCollapsed(forum, collapsed: true);
/// db.forumViewDao.watchCollapsedGroups().listen(print); // {forum}
/// await db.forumViewDao.setSpaceCollapsed(space, collapsed: true);
/// db.forumViewDao.watchCollapsedSpaces().listen(print); // {space}
/// await db.forumViewDao.setPostListShownInFull(forum, inFull: true);
/// db.forumViewDao.watchPostListsShownInFull().listen(print); // {forum}
/// await db.forumViewDao.setOverviewPinned(pinned: true);
/// print(await db.forumViewDao.overviewPinned()); // true
/// await db.forumViewDao.setChannelColumnHidden(hidden: true);
/// print(await db.forumViewDao.channelColumnHidden()); // true
/// await db.forumViewDao.setSessionColumnHidden(textThread: false, hidden: true);
/// print(await db.forumViewDao.sessionColumnHidden(textThread: false)); // true
/// await db.forumViewDao.setCodeBlocksUnwrapped(unwrapped: true);
/// print(await db.forumViewDao.codeBlocksUnwrapped()); // true
/// ```
final class ForumViewDao {
  /// Creates the DAO on a database; use `LocalDatabase.forumViewDao`.
  ForumViewDao(this._db);

  final LocalDatabase _db;

  /// Whether the Done section of [forum] is collapsed; true without a stored row (01 6.1:
  /// collapsed by default).
  Stream<bool> watchDoneCollapsed(ChannelId forum) =>
      (_db.select(_db.forumViewStates)
            ..where((t) => t.channelId.equals(forum.value)))
          .map((row) => row.doneCollapsed)
          .watchSingleOrNull()
          .map((collapsed) => collapsed ?? true);

  /// Stores whether the Done section of [forum] is [collapsed].
  Future<void> setDoneCollapsed(
    ChannelId forum, {
    required bool collapsed,
  }) async {
    await _db
        .into(_db.forumViewStates)
        .insertOnConflictUpdate(
          ForumViewStatesCompanion.insert(
            channelId: forum.value,
            doneCollapsed: collapsed,
          ),
        );
  }

  /// The channels whose thread group in the sidebar is collapsed on this device, emitted on
  /// every change; a channel without a stored row is expanded (01 6.8 item 7: expanded by
  /// default).
  Stream<Set<ChannelId>> watchCollapsedGroups() =>
      (_db.select(_db.channelViewStates)
            ..where((t) => t.groupCollapsed.equals(true)))
          .map((row) => ChannelId.parse(row.channelId))
          .watch()
          .map((channels) => channels.toSet());

  /// Stores whether the thread group of [channel] is [collapsed] on this device.
  Future<void> setGroupCollapsed(
    ChannelId channel, {
    required bool collapsed,
  }) async {
    await _db
        .into(_db.channelViewStates)
        .insertOnConflictUpdate(
          ChannelViewStatesCompanion.insert(
            channelId: channel.value,
            groupCollapsed: collapsed,
          ),
        );
  }

  /// The spaces that are collapsed in the sidebar on this device, emitted on every change; a
  /// space without a stored row is expanded (01 6.7, R-PR-24: another device of the person
  /// never changes it).
  Stream<Set<SpaceId>> watchCollapsedSpaces() =>
      (_db.select(_db.spaceViewStates)..where((t) => t.collapsed.equals(true)))
          .map((row) => SpaceId.parse(row.spaceId))
          .watch()
          .map((spaces) => spaces.toSet());

  /// Stores whether [space] is [collapsed] in the sidebar on this device.
  Future<void> setSpaceCollapsed(
    SpaceId space, {
    required bool collapsed,
  }) async {
    await _db
        .into(_db.spaceViewStates)
        .insertOnConflictUpdate(
          SpaceViewStatesCompanion.insert(
            spaceId: space.value,
            collapsed: collapsed,
          ),
        );
  }

  /// The forums whose sidebar group shows every open post on this device ("Show more", 01 6.8
  /// item 3), emitted on every change; a channel without a stored row shows the newest
  /// `ui.sidebar_forum_posts_visible`.
  Stream<Set<ChannelId>> watchPostListsShownInFull() =>
      (_db.select(_db.channelViewStates)
            ..where((t) => t.postsShownInFull.equals(true)))
          .map((row) => ChannelId.parse(row.channelId))
          .watch()
          .map((channels) => channels.toSet());

  /// Stores whether the sidebar group of [forum] shows every open post ([inFull]) on this
  /// device; its collapse state stays (a new row is expanded).
  Future<void> setPostListShownInFull(
    ChannelId forum, {
    required bool inFull,
  }) async {
    await _db
        .into(_db.channelViewStates)
        .insert(
          ChannelViewStatesCompanion.insert(
            channelId: forum.value,
            groupCollapsed: false,
            postsShownInFull: Value(inFull),
          ),
          onConflict: DoUpdate(
            (_) => ChannelViewStatesCompanion(postsShownInFull: Value(inFull)),
          ),
        );
  }

  /// Whether the forum overview is pinned as its own column on this device, emitted on every
  /// change; false without the stored row (01 D15: not pinned by default).
  Stream<bool> watchOverviewPinned() => _db
      .select(_db.shellViewStates)
      .map((row) => row.overviewPinned)
      .watchSingleOrNull()
      .map((pinned) => pinned ?? false);

  /// Whether the forum overview is pinned, read once (the shell reads it at start, so a restart
  /// restores the column; brief M4-40 Design note 4); false without the stored row.
  Future<bool> overviewPinned() async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => row.overviewPinned)
          .getSingleOrNull() ??
      false;

  /// Stores whether the forum overview is [pinned] as its own column on this device.
  Future<void> setOverviewPinned({required bool pinned}) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          ShellViewStatesCompanion.insert(
            id: const Value(1),
            overviewPinned: Value(pinned),
          ),
        );
  }

  /// Whether the channel column next to an open thread is hidden on this device, emitted on
  /// every change; false without the stored row (01 D1, D2: shown by default).
  Stream<bool> watchChannelColumnHidden() => _db
      .select(_db.shellViewStates)
      .map((row) => row.channelColumnHidden)
      .watchSingleOrNull()
      .map((hidden) => hidden ?? false);

  /// Whether the channel column next to an open thread is hidden, read once; false without the
  /// stored row.
  Future<bool> channelColumnHidden() async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => row.channelColumnHidden)
          .getSingleOrNull() ??
      false;

  /// Stores whether the channel column next to an open thread is [hidden] on this device; the
  /// pinned state of the overview stays.
  Future<void> setChannelColumnHidden({required bool hidden}) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          ShellViewStatesCompanion.insert(
            id: const Value(1),
            channelColumnHidden: Value(hidden),
          ),
        );
  }

  /// Whether the session column is hidden on this device in the layout of a thread of a text
  /// channel ([textThread]) or in every other layout (a forum post, a main history alone),
  /// emitted on every change. Without the stored row it shows in the forum layout and is
  /// collapsed beside a thread of a text channel (01 D1, D2).
  Stream<bool> watchSessionColumnHidden({required bool textThread}) => _db
      .select(_db.shellViewStates)
      .map((row) => _sessionHidden(row, textThread: textThread))
      .watchSingleOrNull()
      .map((hidden) => hidden ?? textThread);

  /// Whether the session column is hidden in the layout [textThread] names, read once; the
  /// default of [watchSessionColumnHidden] without the stored row.
  Future<bool> sessionColumnHidden({required bool textThread}) async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => _sessionHidden(row, textThread: textThread))
          .getSingleOrNull() ??
      textThread;

  /// Stores whether the session column is [hidden] in the layout [textThread] names on this
  /// device; the other layout, the pin and the channel column stay.
  Future<void> setSessionColumnHidden({
    required bool textThread,
    required bool hidden,
  }) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          textThread
              ? ShellViewStatesCompanion.insert(
                  id: const Value(1),
                  textThreadSessionShown: Value(!hidden),
                )
              : ShellViewStatesCompanion.insert(
                  id: const Value(1),
                  sessionColumnHidden: Value(hidden),
                ),
        );
  }

  /// Whether the code blocks of messages show one line per line with a horizontal scroll on
  /// this device ("Wrap lines" off, 01 6.14 item 1; M4-49), emitted on every change; false
  /// (long lines wrap) without the stored row.
  Stream<bool> watchCodeBlocksUnwrapped() => _db
      .select(_db.shellViewStates)
      .map((row) => row.codeBlocksUnwrapped)
      .watchSingleOrNull()
      .map((unwrapped) => unwrapped ?? false);

  /// Whether the code blocks show unwrapped, read once; false without the stored row.
  Future<bool> codeBlocksUnwrapped() async =>
      await _db
          .select(_db.shellViewStates)
          .map((row) => row.codeBlocksUnwrapped)
          .getSingleOrNull() ??
      false;

  /// Stores whether the code blocks show [unwrapped] on this device; the rest of the shell's
  /// view state stays.
  Future<void> setCodeBlocksUnwrapped({required bool unwrapped}) async {
    await _db
        .into(_db.shellViewStates)
        .insertOnConflictUpdate(
          ShellViewStatesCompanion.insert(
            id: const Value(1),
            codeBlocksUnwrapped: Value(unwrapped),
          ),
        );
  }

  // Beside a thread of a text channel the column is stored as shown (collapsed by default,
  // D2); elsewhere as hidden (shown by default, D1).
  static bool _sessionHidden(
    ShellViewStateRow row, {
    required bool textThread,
  }) => textThread ? !row.textThreadSessionShown : row.sessionColumnHidden;
}
