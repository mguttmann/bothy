// SPDX-License-Identifier: Apache-2.0

// Accessibility of M1 with the thread groups on iOS and Android (brief M5-15 requirement 15,
// Tests; 01 8; components.md 7): the semantics snapshot of the tree, the labels of the thread
// rows, "All posts", the draft row and "+", the touch targets, text scale 2 in de and en, and
// the levels space, channel, thread as nesting of the semantics tree.

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import '../../support/structure.dart';
import 'support.dart';

/// The depth of the semantics node of [finder] below the root.
int depthOf(WidgetTester tester, Finder finder) {
  var depth = 0;
  SemanticsNode? node = tester.getSemantics(finder);
  while (node?.parent != null) {
    node = node!.parent;
    depth++;
  }
  return depth;
}

/// Whether the node of [inner] lies inside the node of [outer].
bool inside(WidgetTester tester, Finder inner, SemanticsNode outer) {
  SemanticsNode? node = tester.getSemantics(inner).parent;
  while (node != null) {
    if (node == outer) return true;
    node = node.parent;
  }
  return false;
}

void main() {
  for (final android in [false, true]) {
    phoneScreenTests(
      android ? 'M1 with threads on Android' : 'M1 with threads',
      build: spacesThreadsScene,
      platform: android ? TargetPlatform.android : TargetPlatform.iOS,
      semantics: (tester, l) {
        // The thread rows say title, state and counters (components.md 7).
        expect(
          find.bySemanticsLabel(RegExp('^Review: grant wizard, .*2 unread')),
          findsOneWidget,
        );
        expect(
          find.bySemanticsLabel(RegExp('^${l.sidebarAllPosts}')),
          findsOneWidget,
        );
        // "+" is a button with its label on every forum, always on touch.
        for (final label in [
          l.sidebarNewPostIn('requests'),
          l.sidebarNewChatIn('server'),
        ]) {
          expect(
            tester.getSemantics(find.bySemanticsLabel(label)),
            isSemantics(isButton: true, hasTapAction: true),
          );
        }
        // Levels: space group > channel group > thread row.
        final space = tester.getSemantics(
          find.bySemanticsLabel(l.sidebarSpaceGroup('Support')),
        );
        final channel = find.byKey(
          ValueKey('sidebar.channel.${channelId(2).value}'),
        );
        final row = find.byKey(ValueKey('sidebar.thread.${threadId(3).value}'));
        expect(inside(tester, channel, space), isTrue);
        expect(inside(tester, row, space), isTrue);
        expect(depthOf(tester, row), greaterThan(depthOf(tester, channel)));
        expect(
          inside(tester, row, tester.getSemantics(channel).parent!),
          isTrue,
          reason: 'the row lies in the group node of its channel',
        );
        expect(
          inside(tester, channel, tester.getSemantics(row).parent!),
          isFalse,
          reason: 'the channel row is one level above its rows',
        );
      },
      reachable: (l) => [
        find.byTooltip(l.sidebarNewPostIn('requests')),
        find.byKey(ValueKey('sidebar.thread.${threadId(4).value}')),
        find.byKey(ValueKey('sidebar.allPosts.${channelId(2).value}')),
        find.byTooltip(l.sidebarNewChatIn('server')),
      ],
    );
  }
}
