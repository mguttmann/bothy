// SPDX-License-Identifier: Apache-2.0

/// The post list column of a forum (01 5.1 D1, 6.1; components.md 6; brief M4-19 Goal):
/// head with filters, the virtualized list of open posts and the Done section.
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
  });

  /// The forum.
  final Channel forum;

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
    if (_scroll.position.extentAfter < _nearEnd) onNearEnd();
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
                      return Padding(
                        padding: const EdgeInsets.all(DsSpace.s12),
                        child: Text(
                          l.postListEmpty,
                          key: const ValueKey('postList.empty'),
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: context.dsColors.textSecondary),
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
      ],
    );
  }
}

/// The post list of the forum [channelId] over the providers (R-CL-03).
///
/// It loads the first page of `listChannelThreads` when it opens and the next ones when
/// scrolled near the end ([ForumPager]); "New post" puts `createThread` into the outbox; the
/// Done state is kept on this device (`forum_view_states`); a post opens `/t/<id>`.
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
    return PostListView(
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
      onNewPost: () async {
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
      },
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
    );
  }
}

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
