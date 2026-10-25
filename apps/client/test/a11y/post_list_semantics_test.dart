// SPDX-License-Identifier: Apache-2.0

// Accessibility of the post list (brief M4-32 Design note 1; 01 8; components.md 6): every row
// reads its title, state, time and unread count, a waiting run says "Waiting for you" in words,
// the filter chips say whether they are on, and "New post" is a labelled button.

import 'package:client/forum/forum_model.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/forum/post_list_golden_test.dart' as golden;
import '../support/structure.dart';

Widget postList() => SizedBox.fromSize(
  size: golden.column,
  child: PostListView(
    forum: golden.forum,
    filter: const PostListFilter(),
    model: golden.model(),
    tags: golden.tags,
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
);

void main() {
  dsAccessibilityTests(
    'the post list',
    build: postList,
    semantics: (tester, l) {
      expect(
        tester.getSemantics(find.byKey(const ValueKey('postList.newPost'))),
        isSemantics(isButton: true, label: l.postListNewPost),
      );
      expect(
        find.bySemanticsLabel(
          RegExp('^Which logo for the login\\?.*${l.threadRowWaitingForYou}'),
        ),
        findsOneWidget,
      );
      expect(
        find.bySemanticsLabel(RegExp('^Export to CSV.*5 unread')),
        findsOneWidget,
      );
    },
    reachable: (l) => [
      find.byKey(const ValueKey('postList.newPost')),
      find.byKey(const ValueKey('postList.filter.waiting')),
      find.byKey(const ValueKey('postList.filter.sort')),
    ],
  );
}
