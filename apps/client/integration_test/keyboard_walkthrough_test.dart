// SPDX-License-Identifier: Apache-2.0

// The keyboard walkthrough of the desktop app against the fake server (brief M4-24 Tests;
// 01 7 and 8 "Focus": "Integration test of a keyboard pass"), on macOS with Cmd and Option and
// on Windows with Ctrl and Alt: the keys come from the table of `bindings.dart` for the running
// platform. From the sidebar to a forum with Option or Alt plus arrows, into a post, F6 to the
// history, Esc marks read, Esc then Tab focuses the input, Enter sends, arrow up edits, the
// quick switcher jumps, search, Running now, mentions and settings open, a run is stopped, the
// live box and the session column toggle, and the list of shortcuts opens. Every step is
// asserted by what the app shows (or, for the read position and the stop, by the server).

import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import '../test/support/keys.dart';
import 'support/agent_flow.dart' show composerText;
import 'support/test_app.dart';

/// The patience of the repository's test kits: the only upper bound of a wait.
const Duration patience = Duration(minutes: 3);

/// The first app stroke of [action] on the running platform.
KeyStroke strokeOf(ShortcutAction action) {
  final platform = ShortcutPlatform.current;
  return allShortcutBindings
      .firstWhere((b) => b.action == action && b.kind == ShortcutKind.app)
      .on(platform)
      .first
      .single;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('a keyboard pass through the main view', (tester) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'deploys', {
      seed.alex.id: 'trigger',
      seed.sam.id: 'write',
      seed.robin.id: 'write',
    });
    final thread = world.addThread(
      forum,
      'Release checklist',
      createdBy: seed.alex.id,
    );
    world.setChannelOperator(forum, seed.robin.id);

    Future<void> until(String what, bool Function() condition) async {
      final watch = Stopwatch()..start();
      while (!condition()) {
        if (watch.elapsed > patience) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    Future<void> shows(Finder finder) =>
        until('$finder', () => finder.evaluate().isNotEmpty);

    String route() =>
        GoRouter.of(tester.element(find.byType(Navigator).first))
            .routerDelegate
            .currentConfiguration
            .uri
            .toString();

    bool focusIn(Finder finder) {
      final context = FocusManager.instance.primaryFocus?.context;
      if (context == null || finder.evaluate().isEmpty) return false;
      final target = finder.evaluate().first;
      var inside = false;
      context.visitAncestorElements((e) {
        if (e == target) inside = true;
        return !inside;
      });
      return inside || context == target;
    }

    Future<void> press(ShortcutAction action) async {
      await pressStroke(tester, strokeOf(action));
      await tester.pump();
    }

    Future<void> key(LogicalKeyboardKey key) async {
      await pressStroke(tester, KeyStroke(key));
      await tester.pump();
    }

    try {
      await app.connect();
      await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await shows(find.byKey(ValueKey('sidebar.channel.$forum')));

      // F6 puts the focus into the sidebar.
      await press(ShortcutAction.nextColumn);
      await until(
        'the focus in the sidebar',
        () => focusIn(find.byKey(const ValueKey('column.sidebar'))),
      );

      // Option or Alt plus arrow down walks the channels until the forum `deploys`.
      for (var i = 0; i < 10 && route() != '/c/$forum'; i++) {
        await press(ShortcutAction.nextChannel);
        await until('a channel route', () => route().startsWith('/c/'));
      }
      expect(route(), '/c/$forum');
      await press(ShortcutAction.previousChannel);
      await until('the previous channel', () => route() != '/c/$forum');
      await press(ShortcutAction.nextChannel);
      await until('the forum again', () => route() == '/c/$forum');

      // Into the post list with F6 and Tab, open the post with Enter.
      final post = find.byKey(ValueKey('postList.post.$thread'));
      await shows(post);
      for (
        var i = 0;
        i < 6 && !focusIn(find.byKey(const ValueKey('column.post_list')));
        i++
      ) {
        await press(ShortcutAction.nextColumn);
      }
      for (var i = 0; i < 30 && !focusIn(post); i++) {
        await key(LogicalKeyboardKey.tab);
      }
      expect(focusIn(post), isTrue, reason: 'Tab reaches the post');
      await key(LogicalKeyboardKey.enter);
      await until('the thread route', () => route() == '/t/$thread');
      await shows(find.byKey(ValueKey('history.$thread')));

      // F6 to the history; Sam writes; Esc marks the thread read on the server.
      final history = find.byKey(const ValueKey('column.history'));
      for (var i = 0; i < 6 && !focusIn(history); i++) {
        await press(ShortcutAction.nextColumn);
      }
      expect(focusIn(history), isTrue, reason: 'F6 reaches the history');
      seed.post(seed.sam, thread, 'The checklist is ready');
      await shows(find.text('The checklist is ready'));
      final newest = world.messagesOf(thread).last['thread_seq']! as int;
      await key(LogicalKeyboardKey.escape);
      await until(
        'the read position on the server',
        () =>
            (world.readStates['${seed.alex.id}|$thread']?['last_read_seq']
                    as int? ??
                0) >=
            newest,
      );

      // Esc then Tab focuses the input; Enter sends.
      await key(LogicalKeyboardKey.tab);
      await until('the focus in the input', () => focusIn(composerText));
      await tester.enterText(composerText, 'Deploying now');
      await key(LogicalKeyboardKey.enter);
      await until(
        'the message on the server',
        () => world
            .messagesOf(thread)
            .any((m) => m['content_md'] == 'Deploying now'),
      );
      await until(
        'the input empty again',
        () => tester.widget<TextField>(composerText).controller!.text.isEmpty,
      );

      // Arrow up in the empty field edits the last own message; Esc cancels the edit.
      await key(LogicalKeyboardKey.arrowUp);
      await shows(find.byKey(const ValueKey('edit.field')));
      await tester.tap(find.byKey(const ValueKey('edit.cancel')));
      await tester.pump();

      // Cmd+L or Ctrl+L focuses the input as well.
      await press(ShortcutAction.nextColumn);
      await press(ShortcutAction.focusInput);
      await until('the focus in the input again', () => focusIn(composerText));

      // The quick switcher jumps to the thread "Release notes".
      await press(ShortcutAction.quickSwitcher);
      await shows(find.byKey(const ValueKey('switcher.field')));
      await tester.enterText(
        find.byKey(const ValueKey('switcher.field')),
        'release no',
      );
      await shows(
        find.byKey(ValueKey('switcher.row.thread.${seed.releaseNotes}')),
      );
      await key(LogicalKeyboardKey.enter);
      await until(
        'the switcher jump',
        () => route() == '/t/${seed.releaseNotes}',
      );

      // Search, Running now, mentions and settings.
      await press(ShortcutAction.search);
      await shows(find.byKey(const ValueKey('search.view')));
      await press(ShortcutAction.runningNow);
      await until('Running now', () => route() == '/running');
      await press(ShortcutAction.mentions);
      await until('the mentions', () => route() == '/mentions');
      await press(ShortcutAction.settings);
      await shows(find.byKey(const ValueKey('screen.settings')));
      GoRouter.of(tester.element(find.byKey(const ValueKey('screen.settings'))))
          .go('/t/$thread');
      await shows(find.byKey(ValueKey('history.$thread')));

      // A run in the post: the live box toggles and Stop cancels it.
      final trigger = world.createMessage(seed.alex.id, thread, {
        'client_msg_id': world.nextId(),
        'content_md': '@robin check the list',
        'mention_ids': [seed.robin.id],
      }).message;
      final run = world.createRun(
        agentId: seed.robin.id,
        triggeringMessageId: trigger['id']! as String,
        triggeredBy: seed.alex.id,
      );
      final runId = run['id']! as String;
      world.setRunState(runId, 'running', triggerState: 'working');
      await shows(find.byKey(ValueKey('livebox.$runId')));
      await press(ShortcutAction.toggleLiveBox);
      await shows(find.byKey(ValueKey('livebox.line.$runId')));
      await press(ShortcutAction.toggleLiveBox);
      await shows(find.byKey(ValueKey('livebox.$runId')));
      await press(ShortcutAction.stopRun);
      await until(
        'the run cancelled on the server',
        () => world.runs[runId]!['state'] == 'cancelled',
      );

      // The session column hides and shows.
      final session = find.byKey(const ValueKey('column.session'));
      final before = session.evaluate().isNotEmpty;
      await press(ShortcutAction.toggleSessionColumn);
      await until(
        'the session column toggled',
        () => session.evaluate().isNotEmpty != before,
      );
      await press(ShortcutAction.toggleSessionColumn);
      await until(
        'the session column back',
        () => session.evaluate().isNotEmpty == before,
      );

      // The list of shortcuts.
      await press(ShortcutAction.shortcutList);
      await shows(find.byKey(const ValueKey('shortcutList.dialog')));
      await shows(find.byKey(const ValueKey('shortcutList.row.quickSwitcher')));
    } finally {
      await app.stop();
    }
  });
}
