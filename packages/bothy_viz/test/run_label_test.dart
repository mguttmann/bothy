// SPDX-License-Identifier: Apache-2.0

// The shared run label of the live topology, its table, TV mode and the timeline (owner request
// 2026-11-01, docs/spec/25-visualization.md 4 rule 6 and 7.5): project and chat as the main text,
// shortened with the full text on hover and long press, agent and short ID only as secondary text,
// and a tap that opens the chat only where the server named it.
import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';
import 'support/views.dart';

/// IDs of the fixtures (fixtures/README.md, the test world of 07 10).
const _prefix = '0192f4a0-7b1c-7000-8a00-000000000';
const _paymentThread = '${_prefix}301';
const _loginThread = '${_prefix}303';

void main() {
  group('VizRunLabel', () {
    test('names project and chat only from the chat of the server', () async {
      final model = LiveTopologyModel.fromFrame(
        await fixtures.liveFrame('normal'),
        names: names,
      );
      final cards = {for (final c in model.cards) VizNames.short(c.runId): c};
      final payment = cards['000401']!.label;
      expect(payment.project, 'backend');
      expect(payment.chat, 'Payment integration');
      expect(payment.agent, 'dev-runner');
      expect(payment.shortId, '000401');
      expect(payment.threadId, _paymentThread);
      final hidden = cards['000404']!.label;
      expect(hidden.project, isNull, reason: 'no chat, no name');
      expect(hidden.chat, isNull);
      expect(hidden.threadId, isNull, reason: 'no chat, no link');
      expect(hidden.agent, 'support-agent');
    });

    test(
      'the timeline has one row per chat and agent, labelled by the chat',
      () async {
        final model = TimelineModel.fromTimeline(
          await fixtures.timeline('normal'),
          names: names,
        );
        final labels = [for (final r in model.bars.rows) r.run!];
        expect(
          labels.where((l) => l.named).map((l) => '${l.project} / ${l.chat}'),
          containsAll([
            'tickets / Login fails after update',
            'backend / Payment integration',
            'release / Release notes 2.4',
          ]),
        );
        final login = labels.firstWhere((l) => l.threadId == _loginThread);
        expect(login.agent, 'support-agent');
        expect(login.shortId, VizNames.short(_loginThread));
        final unnamed = labels.where((l) => !l.named).toList();
        expect(unnamed, hasLength(1), reason: 'the run without a chat');
        expect(unnamed.single.threadId, isNull);
      },
    );
  });

  group('live topology', () {
    Future<ViewWorld> pumpTopology(WidgetTester tester) async {
      final world = ViewWorld();
      await tester.runAsync(world.load);
      await pumpWorld(tester, world, world.view(VizViewKind.liveTopology));
      return world;
    }

    testWidgets('a card shows project and chat, agent and short ID below', (
      tester,
    ) async {
      await pumpTopology(tester);
      final card = find.ancestor(
        of: find.text('backend / Payment integration'),
        matching: find.byType(RunCardMark),
      );
      expect(card, findsOneWidget);
      expect(
        find.descendant(of: card, matching: find.text('dev-runner · 000401')),
        findsOneWidget,
      );
      expect(
        find.text('dev-runner'),
        findsNothing,
        reason: 'the agent is never the main text of a named card',
      );
    });

    testWidgets('the full text is on hover and long press', (tester) async {
      await pumpTopology(tester);
      final tooltip = find.ancestor(
        of: find.text('backend / Payment integration'),
        matching: find.byType(Tooltip),
      );
      expect(tooltip, findsOneWidget);
      expect(
        tester.widget<Tooltip>(tooltip).message,
        'backend / Payment integration\ndev-runner · 000401',
      );
      final title = tester.widget<Text>(
        find.text('backend / Payment integration'),
      );
      expect(title.maxLines, 1);
      expect(title.overflow, TextOverflow.ellipsis);
    });

    testWidgets('a tap on a named card opens its chat', (tester) async {
      final world = await pumpTopology(tester);
      await tester.tap(find.text('backend / Payment integration'));
      expect(world.opened, [_paymentThread]);
    });

    testWidgets('a card without a chat has no name and no link', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      final world = await pumpTopology(tester);
      final hidden = find.byWidgetPredicate(
        (w) => w is RunCardMark && VizNames.short(w.model.runId) == '000404',
      );
      expect(hidden, findsOneWidget);
      expect(
        find.descendant(of: hidden, matching: find.text('support-agent')),
        findsOneWidget,
      );
      await tester.ensureVisible(hidden);
      await tester.pumpAndSettle();
      await tester.tap(hidden);
      expect(world.opened, isEmpty);
      expect(tester.getSemantics(hidden).flagsCollection.isButton, isFalse);
      handle.dispose();
    });

    testWidgets('the table names the chats and opens them', (tester) async {
      final world = await pumpTopology(tester);
      await tester.tap(find.byKey(const ValueKey('viz-as-table')));
      await tester.pumpAndSettle();
      final table = tester.widget<VizTable>(find.byType(VizTable).first).data;
      expect(table.columns.take(3), ['Space', 'Chat', 'Run']);
      expect(
        table.texts,
        containsAll(['backend / Payment integration', 'dev-runner · 000401']),
      );
      await tester.tap(find.text('backend / Payment integration'));
      expect(world.opened, [_paymentThread]);
    });

    testWidgets('TV mode shows the chats', (tester) async {
      final world = ViewWorld();
      await tester.runAsync(world.load);
      await pumpWorld(
        tester,
        world,
        TvMode(
          views: const [VizViewKind.liveTopology],
          options: world.options,
          live: world.live,
          queries: world.queries,
          onExit: () {},
        ),
        scaffold: false,
      );
      expect(find.text('backend / Payment integration'), findsOneWidget);
    });
  });

  group('timeline', () {
    Future<ViewWorld> pumpTimeline(WidgetTester tester) async {
      final world = ViewWorld();
      await tester.runAsync(world.load);
      await pumpWorld(tester, world, world.view(VizViewKind.timeline));
      return world;
    }

    testWidgets('a row shows project and chat and opens the chat', (
      tester,
    ) async {
      final world = await pumpTimeline(tester);
      final row = find.text('tickets / Login fails after update');
      expect(row, findsOneWidget);
      expect(
        find.text('support-agent · ${VizNames.short(_loginThread)}'),
        findsOneWidget,
      );
      await tester.ensureVisible(row);
      await tester.tap(row);
      expect(world.opened, [_loginThread]);
    });

    testWidgets('a row without a chat shows the agent and opens nothing', (
      tester,
    ) async {
      final world = await pumpTimeline(tester);
      final bars = tester.widget<BarMark>(find.byType(BarMark)).model;
      final unnamed = bars.rows.singleWhere((r) => !r.run!.named);
      final title = find.text(unnamed.run!.agent);
      expect(title, findsOneWidget);
      await tester.ensureVisible(title);
      await tester.tap(title);
      expect(world.opened, isEmpty);
    });

    testWidgets('the table names the chat of every bar', (tester) async {
      await pumpTimeline(tester);
      await tester.tap(find.byKey(const ValueKey('viz-as-table')));
      await tester.pumpAndSettle();
      final table = tester.widget<VizTable>(find.byType(VizTable).first).data;
      expect(table.columns.take(2), ['Chat', 'Run']);
      expect(table.texts, contains('tickets / Login fails after update'));
    });
  });
}
