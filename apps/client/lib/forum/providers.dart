// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the forum post list over the local database (15 4; brief M4-19
/// Design notes 1, 3 and 4): the threads, tags and runs of a forum, the filters, the list
/// with the Done section, the Done collapse state of this device and the paging through
/// `listChannelThreads`.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/session.dart';
import '../sidebar/providers.dart';
import '../sync/thread_ops.dart';
import 'failed_post_notice.dart';
import 'forum_model.dart';

part 'providers.g.dart';

/// The channel [channelId], or null when it is not stored.
@riverpod
Future<Channel?> storedChannel(Ref ref, String channelId) async {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return null;
  // A change of the channel arrives as a new list of its space; re-read on each.
  final channel = await db.structureDao.channelById(ChannelId.parse(channelId));
  final space = channel?.spaceId;
  if (space != null) ref.watch(spaceChannelsProvider(space.value));
  return channel;
}

/// The stored threads of [channelId].
@riverpod
Stream<List<Thread>> forumThreads(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.threadsDao.watchForChannel(ChannelId.parse(channelId));
}

/// The tags of the forum [channelId] in their order.
@riverpod
Stream<List<ForumTag>> forumTags(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.structureDao.watchForumTags(ChannelId.parse(channelId));
}

/// The stored runs of [channelId].
@riverpod
Stream<List<RunSummary>> channelRuns(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.runsDao.watchForChannel(ChannelId.parse(channelId));
}

/// Whether the Done section of [channelId] is collapsed on this device (01 6.1; collapsed by
/// default; docs/DECISIONS.md D-M4-19-4).
@riverpod
Stream<bool> doneCollapsed(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(true)
      : db.forumViewDao.watchDoneCollapsed(ChannelId.parse(channelId));
}

/// Whether the forum overview is pinned as its own column on this device (01 D15, 6.8 item 7;
/// kept in the local view state, never synchronized, D-SB-6). False until stored.
@riverpod
Stream<bool> overviewPinned(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(false)
      : db.forumViewDao.watchOverviewPinned();
}

/// The person's stored draft of a new post in [channelId] (03 `drafts` with `thread_id` NULL;
/// 01 6.8 item 8, brief M4-41), or null.
@riverpod
Stream<Draft?> channelDraft(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.personalDao.watchDraft(channelId: ChannelId.parse(channelId));
}

/// A new post of a forum this device sent and the server has not created yet: a `createThread`
/// entry of the outbox that is not confirmed (01 6.6; brief M4-41).
final class PendingPost {
  /// Creates the post of entry [entryId].
  const PendingPost({
    required this.entryId,
    required this.title,
    required this.failed,
    required this.createdAt,
  });

  /// The outbox entry.
  final String entryId;

  /// The title, '' without one.
  final String title;

  /// Whether the server refused it (`failed`).
  final bool failed;

  /// When it was put into the outbox.
  final DateTime createdAt;
}

/// The new posts of the forum [channelId] that wait in the outbox or were refused, newest first.
/// Threads from a message (`parent_message_id`) are not posts of the forum's list.
@riverpod
Stream<List<PendingPost>> pendingPosts(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const []);
  return db.outboxDao.watchEntriesFor(null).map((entries) {
    final posts = <PendingPost>[];
    for (final e in entries) {
      if (e.kind != ThreadOps.createThread ||
          e.state == OutboxState.confirmed) {
        continue;
      }
      final body = decodePayload(e.payload);
      if (body['channel_id'] != channelId ||
          body['parent_message_id'] != null) {
        continue;
      }
      posts.add(
        PendingPost(
          entryId: e.id,
          title: body['title'] as String? ?? '',
          failed: e.state == OutboxState.failed,
          createdAt: e.createdAt,
        ),
      );
    }
    return posts..sort((a, b) => b.createdAt.compareTo(a.createdAt));
  });
}

/// The text of the newest draft write of the person's draft of a new post in [channelId] that
/// this device has not had confirmed yet, or null when none waits (04 13; brief M4-41): newer
/// than the stored draft, so the draft row and the new chat view use it, also after a restart
/// while offline. A new post queued after that write ends the draft (`clear_draft`), so it
/// gives null then.
@riverpod
Stream<String?> unsentChannelDraft(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(null);
  return db.outboxDao.watchEntriesFor(null).map((entries) {
    String? text;
    for (final e in entries) {
      if (e.state.isFinal) continue;
      final body = decodePayload(e.payload);
      if (body['channel_id'] != channelId) continue;
      if (e.kind == ThreadOps.putChannelDraft) {
        text = body['content_md'] as String? ?? '';
      } else if (e.kind == ThreadOps.deleteChannelDraft ||
          (e.kind == ThreadOps.createThread &&
              body['parent_message_id'] == null)) {
        text = null;
      }
    }
    return text;
  });
}

/// A title and tags the new chat view of a forum starts with (brief M4-41): "Edit" of a refused
/// post hands them over, because the server's draft has neither (D-M4-41-2). The view takes
/// them once ([take]).
@Riverpod(keepAlive: true)
class NewChatPreset extends _$NewChatPreset {
  @override
  Map<ChannelId, (String, List<ForumTagId>)> build() => const {};

  /// Sets [title] and [tags] for the next new chat view of [forum].
  void set(ChannelId forum, String title, List<ForumTagId> tags) =>
      state = {...state, forum: (title, tags)};

  /// Takes the preset of [forum], or null.
  (String, List<ForumTagId>)? take(ChannelId forum) {
    final preset = state[forum];
    if (preset == null) return null;
    state = {
      for (final MapEntry(:key, :value) in state.entries)
        if (key != forum) key: value,
    };
    return preset;
  }
}

/// The new posts of [channelId] the server refused (`createThread` entries of the outbox in
/// `failed`), oldest first (04 13.2 item 5).
@riverpod
Stream<List<FailedPost>> failedPosts(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const []);
  return db.outboxDao
      .watchEntriesFor(null)
      .map(
        (entries) => [
          for (final e in entries)
            if (e.kind == ThreadOps.createThread &&
                e.state == OutboxState.failed)
              ?_failedPost(e, channelId),
        ],
      );
}

/// The refused post of entry [e] when it belongs to [channelId], else null.
FailedPost? _failedPost(OutboxEntry e, String channelId) {
  final body = decodePayload(e.payload);
  final title = body['title'];
  if (body['channel_id'] != channelId || title is! String) return null;
  return FailedPost(
    entryId: e.id,
    title: title,
    contentMd: body['content_md'] as String? ?? '',
    tagIds: [
      for (final t in body['tag_ids'] as List? ?? const [])
        if (t is String) ForumTagId.parse(t),
    ],
    code: e.lastErrorCode,
    reason: e.lastErrorReason,
  );
}

/// The display names of the stored principals (the operator agent of a row).
@riverpod
Stream<Map<PrincipalId, String>> principalNames(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const {});
  return db.principalsDao.watchAll().map(
    (all) => {for (final p in all) p.id: p.displayName},
  );
}

/// The filters of the post list of [channelId]; they last while the app runs.
@Riverpod(keepAlive: true)
class PostListFilterOf extends _$PostListFilterOf {
  @override
  PostListFilter build(String channelId) => const PostListFilter();

  /// Sets the view chip.
  void setView(PostView view) => state = state.copyWith(view: view);

  /// Sets the selected tags.
  void setTags(Set<ForumTagId> tags) => state = state.copyWith(tags: tags);

  /// Sets the sort.
  void setSort(ForumSort sort) => state = state.copyWith(sort: sort);
}

/// The post list of [channelId] with [filter] (`buildPostList`); null while the channel is
/// not stored.
@riverpod
PostListModel? postList(Ref ref, String channelId, PostListFilter filter) {
  final channel = ref.watch(storedChannelProvider(channelId)).value;
  if (channel == null) return null;
  return buildPostList(
    threads: ref.watch(forumThreadsProvider(channelId)).value ?? const [],
    channel: channel,
    filter: filter,
    counts: ref.watch(threadCountsOfProvider(channelId)),
    runs: ref.watch(channelRunsProvider(channelId)).value ?? const [],
    me: ref.watch(ownPrincipalProvider).value?.id,
  );
}

/// The scroll offset of the post list of [channelId] where the person left it; it lasts while
/// the app runs, so opening a post (another route, a new column) keeps the place.
@Riverpod(keepAlive: true)
class PostListOffset extends _$PostListOffset {
  @override
  double build(String channelId) => 0;

  /// Keeps [offset].
  void keep(double offset) => state = offset;
}

/// Where the paging of a post list stands.
final class PagerState {
  /// Creates the state.
  const PagerState({
    this.cursor,
    this.hasMore = true,
    this.loading = false,
    this.pages = 0,
    this.failed = false,
  });

  /// The cursor of the next page; null before the first page and after the last.
  final String? cursor;

  /// Whether a further page may exist (`next_cursor` was present, or nothing loaded yet).
  final bool hasMore;

  /// Whether a page is being loaded.
  final bool loading;

  /// The pages loaded so far.
  final int pages;

  /// Whether the last page failed; the list then loads again only when the person scrolls.
  final bool failed;
}

/// The paging of the post list of [channelId] in the order [sort] (a `ForumSort` code):
/// pages of `listChannelThreads` go into `threads` (brief M4-19 Design note 3;
/// docs/DECISIONS.md D-M4-19-5). A new engine (a logout, a new login) starts the paging again,
/// because the new database holds none of the loaded pages (D-M4-19-16).
@Riverpod(keepAlive: true)
class ForumPager extends _$ForumPager {
  @override
  PagerState build(String channelId, String sort) {
    ref.watch(syncEngineProvider);
    return const PagerState();
  }

  /// Loads the next page; does nothing while one loads, after the last one and without an
  /// engine. A failed read (offline, an error answer) keeps the cursor and marks the state
  /// [PagerState.failed], so the next try waits for the person's scrolling instead of looping.
  Future<void> loadMore() async {
    final engine = ref.read(syncEngineProvider);
    final now = state;
    if (engine == null || now.loading || !now.hasMore) return;
    state = PagerState(cursor: now.cursor, pages: now.pages, loading: true);
    try {
      final next = await engine.catchUp.loadThreadPage(
        ChannelId.parse(channelId),
        cursor: now.cursor,
        sort: ForumSort.fromCode(sort),
      );
      if (!ref.mounted) return;
      state = PagerState(
        cursor: next,
        hasMore: next != null,
        pages: now.pages + 1,
      );
    } on Object {
      if (!ref.mounted) return;
      state = PagerState(cursor: now.cursor, pages: now.pages, failed: true);
    }
  }
}

/// The threads this run's new posts created (brief M4-41), or an empty registry without a
/// session.
@riverpod
CreatedPosts createdPosts(Ref ref) =>
    ref.watch(signedInServicesProvider)?.createdPosts ?? CreatedPosts();
