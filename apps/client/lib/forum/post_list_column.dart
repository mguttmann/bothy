// SPDX-License-Identifier: Apache-2.0

/// The forum overview D15 (01 5.1 D1 and D15, 6.1; components.md 6; briefs M4-19 Goal and
/// M4-40): head with the kind sentence, "New chat" or "New post", the search in the forum,
/// "Pin" and the filters, the virtualized list of open posts and the Done section. The same
/// widget shows in the content column and, pinned, in its own column (Design note 4).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../keyboard/app_shortcuts.dart';
import '../keyboard/intents.dart';
import '../menus/thread_menu.dart';
import '../sync/thread_ops.dart';
import 'done_section.dart';
import 'failed_post_notice.dart';
import 'forum_model.dart';
import 'new_post_dialog.dart';
import 'post_list_header.dart';
import 'post_row.dart';
import 'providers.dart';

/// The post list as it looks for given data, without providers (widget and golden tests use
/// it directly).
///
/// The list builds its rows lazily; [onNearEnd] is called when it is scrolled close to its
/// end (and after each build while it does not fill its height), so the next page of
/// `listChannelThreads` loads.
class PostListView extends StatefulWidget {
  /// Creates the view.
  const PostListView({
    super.key,
    required this.forum,
    required this.filter,
    required this.model,
    required this.tags,
    required this.names,
    required this.now,
    required this.doneCollapsed,
    required this.morePages,
    required this.onNewPost,
    required this.onView,
    required this.onTags,
    required this.onSort,
    required this.onToggleDone,
    required this.onOpenPost,
    this.onNearEnd,
    this.autoLoad = true,
    this.selectedThread,
    this.failedPosts = const [],
    this.onRetryPost,
    this.onDiscardPost,
    this.onEditPost,
    this.initialOffset = 0,
    this.onOffset,
    this.pinned = false,
    this.onPin,
    this.onSearch,
  });

  /// The forum.
  final Channel forum;

  /// Whether the overview is pinned as a column on this device (01 D15).
  final bool pinned;

  /// Pins or unpins the overview; null hides "Pin".
  final VoidCallback? onPin;

  /// Opens the search in this forum; null hides it.
  final VoidCallback? onSearch;

  /// The filters.
  final PostListFilter filter;

  /// The open posts and the Done section.
  final PostListModel model;

  /// The tags of the forum.
  final List<ForumTag> tags;

  /// The display names of agents.
  final Map<PrincipalId, String> names;

  /// Now, for the time labels.
  final DateTime now;

  /// Whether the Done section is collapsed.
  final bool doneCollapsed;

  /// Whether more pages may exist ("Done (n+)").
  final bool morePages;

  /// The open post, or null.
  final ThreadId? selectedThread;

  /// Opens the dialog of a new post.
  final VoidCallback onNewPost;

  /// Chooses the view chip.
  final ValueChanged<PostView> onView;

  /// Sets the selected tags.
  final ValueChanged<Set<ForumTagId>> onTags;

  /// Sets the sort.
  final ValueChanged<ForumSort> onSort;

  /// Collapses or expands the Done section.
  final ValueChanged<bool> onToggleDone;

  /// Opens a post.
  final ValueChanged<ThreadId> onOpenPost;

  /// Loads the next page.
  final VoidCallback? onNearEnd;

  /// Whether a page loads by itself while the list does not fill its height; false after a
  /// failed page, so only the person's scrolling tries again (no request loop).
  final bool autoLoad;

  /// The new posts the server refused, shown first.
  final List<FailedPost> failedPosts;

  /// Sends a refused post again.
  final ValueChanged<FailedPost>? onRetryPost;

  /// Deletes a refused post.
  final ValueChanged<FailedPost>? onDiscardPost;

  /// Opens a refused post in the "New post" dialog.
  final ValueChanged<FailedPost>? onEditPost;

  /// Where the list starts (the offset it had when the person last left it).
  final double initialOffset;

  /// Called with the offset where a scroll stopped, to keep it.
  final ValueChanged<double>? onOffset;

  @override
  State<PostListView> createState() => _PostListViewState();
}

class _PostListViewState extends State<PostListView> {
  /// How close to the end (in logical pixels) the next page starts to load.
  static const double _nearEnd = 400;

  // Opening a post builds the column again (another route); it starts where it was left.
  late final ScrollController _scroll = ScrollController(
    initialScrollOffset: widget.initialOffset,
  );

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _checkNearEnd({bool scrolled = false}) {
    if (!scrolled && !widget.autoLoad) return;
    final onNearEnd = widget.onNearEnd;
    if (onNearEnd == null || !widget.morePages || !_scroll.hasClients) return;
    // A list built under an opaque page (a post opened by its address on a phone) is never laid
    // out, so its position has no extent to measure yet.
    final position = _scroll.position;
    if (!position.hasContentDimensions) return;
    if (position.extentAfter < _nearEnd) onNearEnd();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final tags = {for (final t in w.tags) t.id: t};
    final open = w.model.open;
    final done = w.doneCollapsed ? const <PostItem>[] : w.model.done;
    // Rows: refused new posts, the open posts (or the empty text), the row of a failed page,
    // the Done row, the expanded done posts.
    final empty = open.isEmpty;
    final failed = w.failedPosts;
    // After a failed page a row offers to try again: a list that cannot scroll has no other
    // way to load more.
    final retry = !w.autoLoad && w.morePages && w.onNearEnd != null ? 1 : 0;
    final count =
        failed.length + (empty ? 1 : open.length) + retry + 1 + done.length;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _checkNearEnd();
    });
    // A right click, the menu key or Shift+F10 opens the post's menu (M4-33).
    Widget post(PostItem item) => ThreadMenu(
      thread: item.thread.id,
      title: item.thread.title,
      channel: w.forum,
      child: PostRow(
        item: item,
        tags: tags,
        names: w.names,
        now: w.now,
        selected: item.thread.id == w.selectedThread,
        onOpen: () => w.onOpenPost(item.thread.id),
      ),
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        PostListHeader(
          forumName: w.forum.name,
          projectBound: w.forum.projectBound,
          pinned: w.pinned,
          onPin: w.onPin,
          onSearch: w.onSearch,
          filter: w.filter,
          sort: w.filter.sortFor(w.forum),
          tags: w.tags,
          onNewPost: w.onNewPost,
          onView: w.onView,
          onTags: w.onTags,
          onSort: w.onSort,
        ),
        Expanded(
          child: Semantics(
            container: true,
            explicitChildNodes: true,
            label: l.postListLabel(w.forum.name),
            // The first layout after the list was built under an opaque page (a post opened by its
            // address on a phone) sends no scroll notification; its metrics notification asks for
            // the next page then, also for a list too short to scroll.
            child: NotificationListener<ScrollMetricsNotification>(
              onNotification: (n) {
                if (n.depth == 0) _checkNearEnd();
                return false;
              },
              child: NotificationListener<ScrollNotification>(
                onNotification: (n) {
                  if (n is ScrollEndNotification && n.depth == 0) {
                    widget.onOffset?.call(n.metrics.pixels);
                  }
                  _checkNearEnd(scrolled: true);
                  return false;
                },
                child: ListView.builder(
                  key: const ValueKey('postList.list'),
                  controller: _scroll,
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s4),
                  itemCount: count,
                  itemBuilder: (context, at) {
                    if (at < failed.length) {
                      final f = failed[at];
                      return FailedPostNotice(
                        key: ValueKey('postList.failed.${f.entryId}'),
                        post: f,
                        onRetry: () => w.onRetryPost?.call(f),
                        onDiscard: () => w.onDiscardPost?.call(f),
                        onEdit: () => w.onEditPost?.call(f),
                      );
                    }
                    final i = at - failed.length;
                    if (i < (empty ? 1 : open.length)) {
                      if (empty) {
                        // A forum without any post explains its kind (01 6.8 item 11); a
                        // filter that hides every post says so.
                        final bare =
                            w.model.done.isEmpty &&
                            failed.isEmpty &&
                            w.filter == const PostListFilter();
                        return Padding(
                          padding: const EdgeInsets.all(DsSpace.s12),
                          child: Text(
                            bare
                                ? dsChannelKindSentence(
                                    l,
                                    DsChannelKind.forum,
                                    projectBound: w.forum.projectBound,
                                  )
                                : l.postListEmpty,
                            key: const ValueKey('postList.empty'),
                            style: dsTextStyle(
                              DsType.bodySm,
                            ).copyWith(color: context.dsColors.textSecondary),
                          ),
                        );
                      }
                      return post(open[i]);
                    }
                    if (retry == 1 && i == (empty ? 1 : open.length)) {
                      return _LoadFailedRow(onRetry: w.onNearEnd!);
                    }
                    final rest = i - (empty ? 1 : open.length) - retry;
                    if (rest == 0) {
                      return DoneSection(
                        count: w.model.done.length,
                        more: w.morePages,
                        collapsed: w.doneCollapsed,
                        onToggle: w.onToggleDone,
                      );
                    }
                    return post(done[rest - 1]);
                  },
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// The forum overview of [channelId] over the providers (R-CL-03).
///
/// It loads the first page of `listChannelThreads` when it opens and the next ones when
/// scrolled near the end ([ForumPager]); "New post" (or "New chat") puts `createThread` into the
/// outbox, also when the "+" of the sidebar asked for it ([NewPostRequest], until M4-41); the
/// Done state and the pinned state are kept on this device; "Pin" and Cmd+Shift+O pin or unpin
/// it; the search opens `/search` with `in:<forum>`; a post opens `/t/<id>`.
class PostListColumn extends ConsumerWidget {
  /// Creates the column.
  const PostListColumn({
    super.key,
    required this.channelId,
    this.selectedThread,
  });

  /// The forum.
  final ChannelId channelId;

  /// The open post, or null.
  final ThreadId? selectedThread;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final id = channelId.value;
    final forum = ref.watch(storedChannelProvider(id)).value;
    final filter = ref.watch(postListFilterOfProvider(id));
    final model = ref.watch(postListProvider(id, filter));
    if (forum == null || model == null) return const SizedBox.expand();
    final sort = filter.sortFor(forum).code;
    final pager = ref.watch(forumPagerProvider(id, sort));
    final tags = ref.watch(forumTagsProvider(id)).value ?? const [];
    void loadMore() =>
        unawaited(ref.read(forumPagerProvider(id, sort).notifier).loadMore());
    Future<void> newPost() async {
      final input = await showNewPostDialog(context, tags);
      final engine = ref.read(syncEngineProvider);
      if (input == null || engine == null) return;
      await ThreadOps.enqueue(
        engine,
        forum: channelId,
        title: input.title,
        contentMd: input.contentMd,
        tagIds: input.tagIds,
      );
    }

    // The "+" of the sidebar asked for a new post in this forum: open the dialog once the
    // overview shows (brief M4-40 Scope; M4-41 replaces it with the new chat view).
    if (ref.watch(newPostRequestProvider) == channelId) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!context.mounted) return;
        if (ref.read(newPostRequestProvider.notifier).take(channelId)) {
          unawaited(newPost());
        }
      });
    }
    final pinned = ref.watch(overviewPinnedProvider).value ?? false;
    // M4-24: Cmd+N or Ctrl+N opens "New post" of the forum on screen (01 7).
    return KeyboardActions(
      handlers: {
        ShortcutAction.newPost: KeyboardHandler(() => unawaited(newPost())),
      },
      child: PostListView(
        forum: forum,
        filter: filter,
        model: model,
        tags: tags,
        names: ref.watch(principalNamesProvider).value ?? const {},
        now: ref.watch(appPortsProvider).clock.now(),
        doneCollapsed: ref.watch(doneCollapsedProvider(id)).value ?? true,
        morePages: pager.hasMore,
        selectedThread: selectedThread,
        onNearEnd: pager.loading ? null : loadMore,
        autoLoad: !pager.failed,
        initialOffset: ref.read(postListOffsetProvider(id)),
        onOffset: (o) => ref.read(postListOffsetProvider(id).notifier).keep(o),
        failedPosts: ref.watch(failedPostsProvider(id)).value ?? const [],
        onRetryPost: (f) {
          final engine = ref.read(syncEngineProvider);
          if (engine != null) unawaited(engine.outbox.retry(f.entryId));
        },
        onEditPost: (f) async {
          final input = await showNewPostDialog(
            context,
            tags,
            initial: NewPost(
              title: f.title,
              contentMd: f.contentMd,
              tagIds: f.tagIds,
            ),
          );
          final engine = ref.read(syncEngineProvider);
          if (input == null || engine == null) return;
          // A failed entry is replaced by a new one with the changed post (04 13.3).
          await engine.outbox.edit(f.entryId, {
            'title': input.title,
            'content_md': input.contentMd,
            'tag_ids': [for (final t in input.tagIds) t.value],
          });
        },
        onDiscardPost: (f) {
          final engine = ref.read(syncEngineProvider);
          if (engine != null) unawaited(engine.outbox.discard(f.entryId));
        },
        onNewPost: newPost,
        onView: (view) =>
            ref.read(postListFilterOfProvider(id).notifier).setView(view),
        onTags: (tags) =>
            ref.read(postListFilterOfProvider(id).notifier).setTags(tags),
        onSort: (sort) =>
            ref.read(postListFilterOfProvider(id).notifier).setSort(sort),
        onToggleDone: (collapsed) => unawaited(
          ref
              .read(localDatabaseProvider)
              ?.forumViewDao
              .setDoneCollapsed(channelId, collapsed: collapsed),
        ),
        onOpenPost: (thread) => context.go('/t/${thread.value}'),
        pinned: pinned,
        onPin: () => unawaited(
          ref
              .read(localDatabaseProvider)
              ?.forumViewDao
              .setOverviewPinned(pinned: !pinned),
        ),
        onSearch: () => context.go(
          Uri(
            path: '/search',
            queryParameters: {'q': '${forumFilter(forum.name)} '},
          ).toString(),
        ),
      ),
    );
  }
}

/// The search notation that keeps the search to the forum [name] (`in:<name>`, quoted when it
/// holds a space; brief M4-23, 14 4.1).
String forumFilter(String name) =>
    name.contains(' ') ? 'in:"$name"' : 'in:$name';

/// The row of a page that failed to load, with "Try again".
class _LoadFailedRow extends StatelessWidget {
  const _LoadFailedRow({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Padding(
      padding: const EdgeInsets.all(DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Text(
            l.postListLoadFailed,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textSecondary),
          ),
          DsButton(
            key: const ValueKey('postList.loadFailed.retry'),
            label: l.postListTryAgain,
            style: DsButtonStyle.secondary,
            onPressed: onRetry,
          ),
        ],
      ),
    );
  }
}
