// SPDX-License-Identifier: Apache-2.0

// The kind of a channel (brief M4-40 Tests; 01 4 and 6.8 item 11; D-SB-10): `channel.project`
// for a forum with a project binding, the kind sentence as tooltip and screen reader text, the
// "New chat" and "New post" labels, and the kind sentence as the subtitle and the empty state of
// the forum overview.

import 'package:client/forum/forum_model.dart';
import 'package:client/forum/post_list_column.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/widgets.dart';

Channel bound(Channel c) => c.copyWith(projectBound: true);

final Channel text = channel(1, name: 'general');
final Channel forum = channel(2, kind: ChannelKind.forum, name: 'requests');
final Channel project = bound(
  channel(3, kind: ChannelKind.forum, name: 'server'),
);

SidebarChannel entry(Channel c) => SidebarChannel(
  channel: c,
  counts: ThreadCounts.zero,
  runActive: false,
  muted: false,
);

Widget sidebar() => Scaffold(
  body: SidebarView(
    productName: 'Team Chat',
    nodes: [
      SidebarSpaceNode(
        entry: SidebarSpace(space: space(1)),
        channels: [entry(text), entry(forum), entry(project)],
      ),
    ],
    onToggle: (_, _) {},
    onMove: (_, _) {},
    onOpenChannel: (_) {},
    onSettings: () {},
    onLogout: () {},
    onOpenThread: (_) {},
    onToggleGroup: (_, _) {},
    onAdd: (_) {},
  ),
);

Finder rowOf(Channel c) =>
    find.byKey(ValueKey('sidebar.channel.${c.id.value}'));

Widget overview(Channel c, {PostListModel? model}) => Scaffold(
  body: PostListView(
    forum: c,
    filter: const PostListFilter(),
    model: model ?? const PostListModel(open: [], done: []),
    tags: const [],
    names: const {},
    now: t0,
    doneCollapsed: true,
    morePages: false,
    onNewPost: () {},
    onView: (_) {},
    onTags: (_) {},
    onSort: (_) {},
    onToggleDone: (_) {},
    onOpenPost: (_) {},
    onPin: () {},
  ),
);

void main() {
  testWidgets('the icon of each kind; channel.project for a project forum', (
    tester,
  ) async {
    await tester.pumpWidget(hostScreen(sidebar()));
    DsGlyph glyphIn(Channel c) => tester
        .widget<DsIcon>(
          find.descendant(of: rowOf(c), matching: find.byType(DsIcon)).first,
        )
        .glyph;
    expect(glyphIn(text), DsIcons.channelText);
    expect(glyphIn(forum), DsIcons.channelForum);
    expect(glyphIn(project), DsIcons.channelProject);
  });

  testWidgets('the tooltip and the screen reader text name the kind', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(hostScreen(sidebar()));
    expect(find.byTooltip(en.channelKindSentenceText), findsOneWidget);
    expect(find.byTooltip(en.channelKindSentenceForum), findsOneWidget);
    expect(find.byTooltip(en.channelKindSentenceProject), findsOneWidget);
    expect(
      find.bySemanticsLabel(RegExp('^server, ${en.channelKindNameProject}')),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(RegExp('^general, ${en.channelKindNameText}')),
      findsOneWidget,
    );
    semantics.dispose();
  });

  testWidgets('"+" says New chat in a project forum and New post otherwise', (
    tester,
  ) async {
    await tester.pumpWidget(hostScreen(sidebar()));
    final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
    addTearDown(mouse.removePointer);
    await mouse.addPointer(location: tester.getCenter(rowOf(project)));
    await tester.pumpAndSettle();
    expect(find.byTooltip(en.sidebarNewChatIn('server')), findsOneWidget);
    await mouse.moveTo(tester.getCenter(rowOf(forum)));
    await tester.pumpAndSettle();
    expect(find.byTooltip(en.sidebarNewPostIn('requests')), findsOneWidget);
    // A text channel has no "+" (01 6.8 item 8).
    await mouse.moveTo(tester.getCenter(rowOf(text)));
    await tester.pumpAndSettle();
    expect(find.byTooltip(RegExp('general')), findsNothing);
  });

  testWidgets('the overview: subtitle, button and empty state by kind', (
    tester,
  ) async {
    await tester.pumpWidget(hostScreen(overview(project)));
    expect(find.text(en.postListNewChat), findsOneWidget);
    // The subtitle and the empty state of an empty forum both explain the kind.
    expect(find.text(en.channelKindSentenceProject), findsNWidgets(2));
    await tester.pumpWidget(hostScreen(overview(forum)));
    expect(find.text(en.postListNewPost), findsOneWidget);
    expect(find.text(en.channelKindSentenceForum), findsNWidgets(2));
  });
}
