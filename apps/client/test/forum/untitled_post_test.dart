// SPDX-License-Identifier: Apache-2.0

// A post without a title (brief M4-41 requirement 14; 01 6.7; 09 12): until the agent names
// it, the row of the post list and the header of its history read "New chat", the header with
// the "automatic" mark.

import 'package:client/forum/forum_model.dart';
import 'package:client/thread/thread_header.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/widgets.dart';

void main() {
  test('the row of a post without a title reads New chat', () {
    String rowTitle(String title) => PostItem(
      thread: thread(1, title: title),
      counts: ThreadCounts.zero,
    ).rowData(en, tags: const {}, names: const {}, timeLabel: '').title;
    expect(rowTitle(''), en.sidebarNewChat);
    expect(rowTitle('  '), en.sidebarNewChat);
    expect(rowTitle('Fix the login page'), 'Fix the login page');
  });

  testWidgets(
    'the header of a post without a title reads New chat with the automatic mark',
    (tester) async {
      Widget header(String title, {bool main = false}) => Scaffold(
        body: ThreadHeader(
          data: ThreadHeaderData(
            title: title,
            automaticTitle: true,
            operatorName: 'Robin',
            mainHistory: main,
            done: false,
          ),
          onEditTitle: () {},
          onDone: () {},
          onMove: () {},
          onPin: () {},
          onColumn: () {},
        ),
      );
      Finder title() => find.byKey(const ValueKey('thread.title'));
      await tester.pumpWidget(hostScreen(header('')));
      expect(tester.widget<Text>(title()).data, en.sidebarNewChat);
      expect(find.byKey(const ValueKey('thread.automatic')), findsOneWidget);
      // A title the agent set replaces it.
      await tester.pumpWidget(hostScreen(header('Fix the login page')));
      expect(
        tester.widget<Text>(title()).data,
        dsIsolate('Fix the login page'),
      );
      // The main history of a channel keeps its channel name.
      await tester.pumpWidget(hostScreen(header('general', main: true)));
      expect(tester.widget<Text>(title()).data, dsIsolate('# general'));
    },
  );
}
