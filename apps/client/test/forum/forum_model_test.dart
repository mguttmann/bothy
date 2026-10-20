// SPDX-License-Identifier: Apache-2.0

// The pure rules of the post list (brief M4-19 Design notes 1 and 3; components.md 6; 01 6.1):
// filters, order and the Done section, "Waiting for me", the row data and the time label.

import 'package:client/forum/forum_model.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/structure.dart';

void main() {
  final l = lookupAppLocalizations(const Locale('en'));
  final forum = channel(1, kind: ChannelKind.forum, sort: ForumSort.created);
  final threads = [
    thread(1, activeMinutes: 50, createdMinutes: 1, tags: [tagId(1)]),
    thread(2, activeMinutes: 10, createdMinutes: 2),
    thread(3, activeMinutes: 40, createdMinutes: 3, state: ThreadState.closed),
    thread(4, activeMinutes: 30, createdMinutes: 4, tags: [tagId(2)]),
  ];
  List<ThreadId> ids(List<PostItem> items) => [
    for (final i in items) i.thread.id,
  ];

  PostListModel build(
    PostListFilter filter, {
    Map<ThreadId, ThreadCounts> counts = const {},
    List<RunSummary> runs = const [],
  }) => buildPostList(
    threads: threads,
    channel: forum,
    filter: filter,
    counts: counts,
    runs: runs,
    me: alex,
  );

  test('the default sort of the forum, then the chosen one', () {
    expect(ids(build(const PostListFilter()).open), [
      threadId(4),
      threadId(2),
      threadId(1),
    ]);
    expect(
      ids(build(const PostListFilter(sort: ForumSort.lastActivity)).open),
      [threadId(1), threadId(4), threadId(2)],
    );
    expect(ids(build(const PostListFilter()).done), [threadId(3)]);
  });

  test('tags: a post with any selected tag', () {
    final model = build(PostListFilter(tags: {tagId(1), tagId(2)}));
    expect(ids(model.open), [threadId(4), threadId(1)]);
    expect(model.done, isEmpty);
  });

  test('waiting for me: my run asks, or an unread mention of me', () {
    final model = build(
      const PostListFilter(view: PostView.waitingForMe),
      counts: {threadId(2): const ThreadCounts(unread: 1, mentions: 1)},
      runs: [
        run(1, RunState.waitingInput, threadN: 1),
        run(2, RunState.waitingInput, threadN: 4, triggeredBy: sam),
      ],
    );
    expect(ids(model.open), [threadId(2), threadId(1)]);
    expect(
      model.open.firstWhere((i) => i.thread.id == threadId(1)).waitingForMe,
      isTrue,
    );
  });

  test('the row data: run state, mention, tags, operator, failure, done', () {
    final model = build(
      const PostListFilter(),
      counts: {threadId(4): const ThreadCounts(unread: 2, mentions: 1)},
      runs: [
        run(1, RunState.running, threadN: 4),
        run(2, RunState.failed, threadN: 2, reason: 'adapter_error'),
        run(3, RunState.completed, threadN: 2),
        run(4, RunState.failed, threadN: 1, reason: 'stale'),
      ],
    );
    final tags = {
      tagId(1): tag(1, 'bug', color: 'rowan'),
      tagId(2): tag(2, 'idea'),
    };
    DsThreadRowData row(PostItem item) => item.rowData(
      l,
      tags: tags,
      names: {robin: 'Robin'},
      timeLabel: '9:00 AM',
    );
    final first = row(model.open.first);
    expect(first.id, threadId(4).value);
    expect(first.runState, DsRunState.running);
    expect(first.unreadCount, 2);
    expect(first.mentionCount, 1);
    expect(first.tags.single.name, 'idea');
    expect(first.tags.single.category, DsCategory.granite);
    expect(first.errorText, isNull);
    expect(first.done, isFalse);
    // The newest run of post 2 completed: no failure is shown.
    expect(row(model.open[1]).errorText, isNull);
    final failed = row(model.open[2]);
    expect(failed.errorText, l.liveBoxReasonStale);
    expect(failed.tags.single.category, DsCategory.rowan);
    expect(row(model.done.single).done, isTrue);
    final withOperator = PostItem(
      thread: thread(9, operator: robin),
      counts: ThreadCounts.zero,
    );
    expect(
      withOperator
          .rowData(l, tags: const {}, names: {robin: 'Robin'}, timeLabel: '')
          .operatorName,
      'Robin',
    );
  });

  test('the time label: the clock today, else the day', () {
    final fmt = L10nFormat(const Locale('en'));
    final now = DateTime(2026, 10, 17, 15, 30);
    expect(
      timeLabel(DateTime(2026, 10, 17, 9, 5), now: now, format: fmt),
      fmt.time(DateTime.utc(2026, 10, 17, 9, 5), timeZone: 'UTC'),
    );
    expect(
      timeLabel(DateTime(2026, 10, 16, 9, 5), now: now, format: fmt),
      'Yesterday',
    );
    expect(
      timeLabel(DateTime(2026, 10, 1), now: now, format: fmt),
      'Oct 1, 2026',
    );
  });
}
