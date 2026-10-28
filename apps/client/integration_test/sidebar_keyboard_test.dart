// SPDX-License-Identifier: Apache-2.0

// A keyboard pass through the sidebar tree against the fake server (brief M4-40 Tests; 01 6.8
// item 7, 7 and 8 "Focus"; D-SB-8): the arrow keys walk spaces, channels and thread rows, arrow
// right and left expand, enter and leave a group, "next unread" stops at an unread post, and F6
// visits the pinned overview column. The keys come from `bindings.dart` for the running
// platform.

import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';

import '../test/support/keys.dart';
import '../test/support/until.dart' show testPatience;
import 'support/test_app.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

/// The first app stroke of [action] on the running platform.
KeyStroke strokeOf(ShortcutAction action) => allShortcutBindings
    .firstWhere((b) => b.action == action && b.kind == ShortcutKind.app)
    .on(ShortcutPlatform.current)
    .first
    .single;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('a keyboard pass through spaces, channels and thread rows', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'server', {
      seed.alex.id: 'trigger',
      seed.sam.id: 'write',
    });
    final older = world.addThread(
      forum,
      'Gateway resume',
      createdBy: seed.sam.id,
    );
    final newer = world.addThread(
      forum,
      'Check migration',
      createdBy: seed.sam.id,
    );

    Future<void> until(String what, bool Function() condition) async {
      final watch = Stopwatch()..start();
      while (!condition()) {
        if (watch.elapsed > testPatience) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

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
      var inside = context == target;
      context.visitAncestorElements((e) {
        if (e == target) inside = true;
        return !inside;
      });
      return inside;
    }

    Future<void> press(ShortcutAction action) async {
      await pressStroke(tester, strokeOf(action));
      await tester.pump();
    }

    Future<void> key(LogicalKeyboardKey key) async {
      await pressStroke(tester, KeyStroke(key));
      await tester.pump();
    }

    Finder thread(String id) => find.byKey(ValueKey('sidebar.thread.$id'));
    final forumRow = find.byKey(ValueKey('sidebar.channel.$forum'));
    final spaceRow = find.byKey(ValueKey('sidebar.space.${seed.space}'));

    try {
      await app.connect();
      await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      await app.waitFor(thread(older));
      await app.waitFor(thread(newer));

      // F6 puts the focus into the sidebar, on its first row.
      await press(ShortcutAction.nextColumn);
      await until(
        'the focus in the sidebar',
        () => focusIn(find.byKey(const ValueKey('column.sidebar'))),
      );

      // Arrow down walks through the space, the channels and the thread rows.
      var sawSpace = false;
      for (var i = 0; i < 40 && !focusIn(thread(older)); i++) {
        sawSpace = sawSpace || focusIn(spaceRow);
        await key(LogicalKeyboardKey.arrowDown);
      }
      expect(sawSpace, isTrue, reason: 'the space row is on the way');
      expect(
        focusIn(thread(older)),
        isTrue,
        reason: 'arrow down reaches a post',
      );
      await key(LogicalKeyboardKey.arrowUp);
      expect(focusIn(thread(newer)), isTrue, reason: 'arrow up goes back');

      // Arrow left goes to the channel; on it, arrow left collapses and arrow right expands.
      await key(LogicalKeyboardKey.arrowLeft);
      await until('the focus on the forum', () => focusIn(forumRow));
      await key(LogicalKeyboardKey.arrowLeft);
      await until(
        'the group collapsed',
        () => thread(newer).evaluate().isEmpty,
      );
      await key(LogicalKeyboardKey.arrowRight);
      await until(
        'the group expanded',
        () => thread(newer).evaluate().isNotEmpty,
      );
      // Arrow right on an expanded group enters its first row; Enter opens it.
      await key(LogicalKeyboardKey.arrowRight);
      await until('the first row', () => focusIn(thread(newer)));
      await key(LogicalKeyboardKey.enter);
      await until('the post opens', () => route() == '/t/$newer');

      // Next unread stops at an unread post (Sam writes into the older one).
      final semantics = tester.ensureSemantics();
      seed.post(seed.sam, older, 'Resume works again.');
      await app.waitFor(
        find.bySemanticsLabel(
          'Gateway resume, ${en.threadRowStateOpen}, ${en.threadRowUnread(1)}',
        ),
      );
      semantics.dispose();
      final before = route();
      await press(ShortcutAction.nextUnreadChannel);
      await until('next unread', () => route() != before);
      expect(route(), '/t/$older');

      // Pinned, the overview is a column F6 visits.
      await press(ShortcutAction.pinOverview);
      final column = find.byKey(const ValueKey('column.post_list'));
      final history = find.byKey(const ValueKey('column.history'));
      await until(
        'the overview pinned beside the history',
        () => column.evaluate().isNotEmpty && history.evaluate().isNotEmpty,
      );
      for (var i = 0; i < 6 && !focusIn(column); i++) {
        await press(ShortcutAction.nextColumn);
      }
      expect(focusIn(column), isTrue, reason: 'F6 visits the pinned overview');
      await press(ShortcutAction.pinOverview);
      await until('unpinned', () => column.evaluate().isEmpty);
    } finally {
      await app.stop();
    }
  });
}
