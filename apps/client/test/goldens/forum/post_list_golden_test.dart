// SPDX-License-Identifier: Apache-2.0

// The goldens of the post list (brief M4-19 Tests; components.md 6; 01 6.1): every row state
// (active run with operator, waiting for me, mention, unread, tags, failure reason, read) and
// the Done section expanded, light and dark, densities comfortable and compact, text scaling 1
// and 2, German and English. One file per theme keeps each file below repo.max_file_kb.
//
// Every post was last active on an earlier day at 09:00 UTC, so the time labels are days and
// read the same in every time zone of the test machines.

import 'package:alchemist/alchemist.dart';
import 'package:client/forum/forum_model.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../../support/structure.dart';

/// The size of the post list column in a cell.
const Size column = Size(380, 860);

/// The width of a cell: wider than the longest scenario label of alchemist, so the matrix has
/// a whole number of pixels on every host (docs/DECISIONS.md D-M4-07-10).
const double cellWidth = 460;

/// [child] at the top left of a cell of [cellWidth].
Widget cell(Widget child) => SizedBox(
  width: cellWidth,
  child: Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    mainAxisSize: MainAxisSize.min,
    children: [child],
  ),
);

const int day = 24 * 60;

final Channel forum = channel(1, name: 'ideas', kind: ChannelKind.forum);

final List<ForumTag> tags = [
  tag(1, 'bug', color: 'rowan'),
  tag(2, 'idea', color: 'spruce'),
];

PostListModel model() => buildPostList(
  threads: [
    thread(
      1,
      title: 'Nightly build fails',
      activeMinutes: -day,
      operator: robin,
    ),
    thread(2, title: 'Which logo for the login?', activeMinutes: -2 * day),
    thread(3, title: 'Translate the help pages', activeMinutes: -3 * day),
    thread(
      4,
      title: 'Export to CSV',
      activeMinutes: -4 * day,
      tags: [tagId(2)],
    ),
    thread(
      5,
      title: 'Search is slow',
      activeMinutes: -5 * day,
      tags: [tagId(1)],
    ),
    thread(6, title: 'Dark mode colors', activeMinutes: -6 * day),
    thread(
      7,
      title: 'Printer setup',
      activeMinutes: -7 * day,
      state: ThreadState.closed,
    ),
    thread(
      8,
      title: 'Welcome text',
      activeMinutes: -8 * day,
      state: ThreadState.closed,
      tags: [tagId(2)],
    ),
  ],
  channel: forum,
  filter: const PostListFilter(),
  counts: {
    threadId(3): const ThreadCounts(unread: 3, mentions: 1),
    threadId(4): const ThreadCounts(unread: 5, mentions: 0),
  },
  runs: [
    run(1, RunState.running, threadN: 1),
    run(2, RunState.waitingInput, threadN: 2, triggeredBy: alex),
    run(3, RunState.failed, threadN: 5, reason: 'stale'),
  ],
  me: alex,
);

void main() {
  for (final theme in DsThemeMode.values) {
    goldenTest(
      'the post list in ${theme.name}: every row state and the Done section, '
      'two densities, text scale 1 and 2, de and en',
      fileName: 'post_list_${theme.name}',
      builder: () => dsGoldenMatrix(
        name: 'post list',
        themes: [theme],
        densities: DsDensityMode.values,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: () => cell(
          SizedBox.fromSize(
            size: column,
            child: PostListView(
              forum: forum,
              filter: const PostListFilter(),
              model: model(),
              tags: tags,
              names: {robin: 'Robin'},
              now: t0,
              doneCollapsed: false,
              morePages: false,
              selectedThread: threadId(6),
              onNewPost: () {},
              onView: (_) {},
              onTags: (_) {},
              onSort: (_) {},
              onToggleDone: (_) {},
              onOpenPost: (_) {},
            ),
          ),
        ),
      ),
    );
  }
}
