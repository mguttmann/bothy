// SPDX-License-Identifier: Apache-2.0

// The arrow keys of the sidebar tree (brief M4-40 rule 14; 01 6.8 item 7; D-SB-8): arrow right
// on a channel expands its group or enters its first row, arrow left goes back to the channel
// or collapses the group; a thread row has no group of its own.

import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/sidebar/sidebar_model.dart';
import 'package:client/sidebar/sidebar_order.dart';
import 'package:client/sidebar/sidebar_threads.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/widgets.dart';

final Channel forum = channel(1, kind: ChannelKind.forum, name: 'server');

SidebarChannel entry({required bool expanded}) {
  final threads = [
    thread(1, title: 'Gateway resume', createdMinutes: 1),
    thread(2, title: 'Check migration', createdMinutes: 2),
  ];
  return SidebarChannel(
    channel: forum,
    counts: ThreadCounts.zero,
    runActive: false,
    muted: false,
    expanded: expanded,
    group: selectSidebarThreads(
      channel: forum,
      threads: threads,
      counts: const {},
      runs: const [],
      selected: null,
      me: alex,
      now: t0,
      limits: const SidebarThreadLimits(
        window: Duration(hours: 72),
        perChannel: 8,
      ),
      expanded: expanded,
    ),
  );
}

/// The sidebar with [expanded] as the group state; [toggles] records the requested states.
class Tree extends StatefulWidget {
  const Tree({super.key, required this.toggles});

  final List<bool> toggles;

  @override
  State<Tree> createState() => _TreeState();
}

class _TreeState extends State<Tree> {
  bool expanded = true;

  @override
  Widget build(BuildContext context) => Scaffold(
    body: SizedBox(
      width: 280,
      child: SidebarView(
        productName: 'Team Chat',
        nodes: [
          SidebarSpaceNode(
            entry: SidebarSpace(space: space(1)),
            channels: [entry(expanded: expanded)],
          ),
        ],
        onToggle: (_, _) {},
        onMove: (_, _) {},
        onOpenChannel: (_) {},
        onSettings: () {},
        onLogout: () {},
        onOpenThread: (_) {},
        onToggleGroup: (_, next) {
          widget.toggles.add(next);
          setState(() => expanded = next);
        },
        onAdd: (_) {},
      ),
    ),
  );
}

Finder row(int n) =>
    find.byKey(ValueKey('sidebar.thread.${threadId(n).value}'));

final Finder channelRow = find.byKey(
  ValueKey('sidebar.channel.${channelId(1).value}'),
);

bool focusIn(Finder finder) {
  final context = FocusManager.instance.primaryFocus?.context;
  if (context == null || finder.evaluate().isEmpty) return false;
  final target = finder.evaluate().first;
  var inside = context == target;
  context.visitAncestorElements((e) {
    if (e == target) inside = true;
    return !inside;
  });
  return inside;
}

void main() {
  testWidgets('arrow keys walk into and out of a group', (tester) async {
    final toggles = <bool>[];
    await tester.pumpWidget(hostScreen(Tree(toggles: toggles)));
    // Focus the newest post, the first row of the group (its row takes the node of the tree).
    tester
        .widget<InkWell>(
          find.descendant(of: row(2), matching: find.byType(InkWell)).first,
        )
        .focusNode!
        .requestFocus();
    await tester.pump();
    expect(focusIn(row(2)), isTrue);

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
    await tester.pump();
    expect(
      focusIn(channelRow),
      isTrue,
      reason: 'arrow left goes to the channel',
    );

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowLeft);
    await tester.pumpAndSettle();
    expect(toggles, [false], reason: 'arrow left collapses the group');
    expect(row(2), findsNothing);

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
    await tester.pumpAndSettle();
    expect(toggles, [false, true], reason: 'arrow right expands it');
    expect(row(2), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.arrowRight);
    await tester.pump();
    expect(focusIn(row(2)), isTrue, reason: 'arrow right enters the first row');
  });
}
