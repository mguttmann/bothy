// SPDX-License-Identifier: Apache-2.0

// The quick switcher, the mentions view and Running now against the fake server (brief M4-23
// Tests; 01 D6, 3.5, D10): the action of Cmd+K or Ctrl+K (the sidebar's "Jump to"; the binding
// is M4-24) opens the switcher and jumps to a thread; the mentions view lists a mention of Alex
// by Sam and jumps to it; Running now lists the agent's run and stops it.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('switcher jump, mentions jump, Running now stop', (tester) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    Future<void> until(String what, bool Function() condition) async {
      final end = DateTime.now().add(const Duration(seconds: 30));
      while (!condition()) {
        if (DateTime.now().isAfter(end)) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    String route() => GoRouter.of(
      tester.element(find.byKey(const ValueKey('column.sidebar'))),
    ).routerDelegate.currentConfiguration.uri.toString();

    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();

      // Quick switcher: "Jump to" opens it, a thread title by prefix, Enter-equivalent tap.
      await tester.tap(find.byKey(const ValueKey('sidebar.jumpTo')));
      await app.waitFor(find.byKey(const ValueKey('switcher.field')));
      await tester.enterText(
        find.byKey(const ValueKey('switcher.field')),
        'release no',
      );
      final row = find.byKey(
        ValueKey('switcher.row.thread.${seed.releaseNotes}'),
      );
      await app.waitFor(row);
      await tester.tap(row);
      await until(
        'the thread route',
        () => route() == '/t/${seed.releaseNotes}',
      );
      await until(
        'the switcher closed',
        () => find.byKey(const ValueKey('switcher.dialog')).evaluate().isEmpty,
      );

      // Mentions: Sam mentions Alex; the view lists it and jumps to the message.
      final mention = world.createMessage(seed.sam.id, seed.releaseNotes, {
        'client_msg_id': world.nextId(),
        'content_md': '@alex please review the notes',
        'mention_ids': [seed.alex.id],
      }).message;
      GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
          .go('/mentions');
      final entry = find.byKey(ValueKey('mentions.entry.${mention['id']}'));
      await app.waitFor(entry);
      await tester.tap(entry);
      await until(
        'the message route',
        () => route() == '/t/${seed.releaseNotes}?m=${mention['id']}',
      );

      // Running now: Alex triggered Robin in `general`, where Alex holds `trigger`.
      final trigger = world.createMessage(seed.alex.id, seed.releaseNotes, {
        'client_msg_id': world.nextId(),
        'content_md': '@robin summarize the notes',
        'mention_ids': [seed.robin.id],
      }).message;
      final run = world.createRun(
        agentId: seed.robin.id,
        triggeringMessageId: trigger['id']! as String,
        triggeredBy: seed.alex.id,
      );
      final runId = run['id']! as String;
      world.setRunState(runId, 'running', triggerState: 'working');
      await tester.tap(find.byKey(const ValueKey('sidebar.runningNow')));
      await app.waitFor(find.byKey(ValueKey('running.row.$runId')));
      final stop = find.byKey(ValueKey('running.stop.$runId'));
      await app.waitFor(stop);
      await tester.tap(stop);
      await until(
        'the run cancelled on the server',
        () => world.runs[runId]!['state'] == 'cancelled',
      );
      await until(
        'the row leaves the list',
        () => find.byKey(ValueKey('running.row.$runId')).evaluate().isEmpty,
      );
    } finally {
      await app.stop();
    }
  });
}
