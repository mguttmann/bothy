// SPDX-License-Identifier: Apache-2.0

// The keyboard walkthrough of M4-24 against the real server (brief M4-34 product requirement 13;
// 01 7 and 8 "Focus"): with the keys of `bindings.dart` for the running platform, from the
// sidebar to the forum, into a post, F6 to the history, Esc then Tab to the input, Enter sends
// (Sam reads it through the API), arrow up edits, the quick switcher jumps to general, and
// search, Running now, mentions, settings and the list of shortcuts open.

import 'package:client/keyboard/bindings.dart';
import 'package:client/keyboard/intents.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../test/support/keys.dart';
import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';
import 'messaging_flows.dart' show composerText;

/// The first app stroke of [action] on the running platform.
KeyStroke strokeOf(ShortcutAction action) => allShortcutBindings
    .firstWhere((b) => b.action == action && b.kind == ShortcutKind.app)
    .on(ShortcutPlatform.current)
    .first
    .single;

/// Registers the keyboard walkthrough.
void keyboardFlows(E2eContext c) {
  final world = c.world;

  testWidgets('a keyboard pass through the main view', (tester) async {
    final nonce = randomNonce().substring(0, 8);
    final thread = (await tester.runAsync(
      () => c.owner.createPost(
        world.forumId,
        'Keyboard checklist $nonce',
        'Walk through with the keyboard',
      ),
    ))!;
    final app = await E2eApp.start(tester, world);

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
      await app.signIn(await c.alexCode());
      final forum = world.forumId;
      await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));

      // F6 puts the focus into the sidebar; Option or Alt plus arrow walks to the forum.
      await press(ShortcutAction.nextColumn);
      await app.until(
        'the focus in the sidebar',
        () => focusIn(find.byKey(const ValueKey('column.sidebar'))),
      );
      for (var i = 0; i < 10 && route() != '/c/$forum'; i++) {
        await press(ShortcutAction.nextChannel);
        await app.until('a channel route', () => route().startsWith('/c/'));
      }
      expect(route(), '/c/$forum');

      // Into the post list with F6 and Tab, open the post with Enter.
      final post = find.byKey(ValueKey('postList.post.$thread'));
      await app.waitFor(post);
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
      await app.until('the thread route', () => route() == '/t/$thread');
      await app.waitFor(find.byKey(ValueKey('history.$thread')));

      // F6 to the history; Esc then Tab focuses the input; Enter sends.
      final history = find.byKey(const ValueKey('column.history'));
      for (var i = 0; i < 6 && !focusIn(history); i++) {
        await press(ShortcutAction.nextColumn);
      }
      expect(focusIn(history), isTrue, reason: 'F6 reaches the history');
      await key(LogicalKeyboardKey.escape);
      await key(LogicalKeyboardKey.tab);
      await app.until('the focus in the input', () => focusIn(composerText));
      await tester.enterText(composerText, 'Typed with the keyboard $nonce');
      await key(LogicalKeyboardKey.enter);
      String? sent;
      await app.untilAsync('Sam reads the message', () async {
        sent = (await c.sam.messages(thread))
            .where((m) => m.contentMd == 'Typed with the keyboard $nonce')
            .firstOrNull
            ?.id;
        return sent != null;
      });
      // Arrow up edits only a confirmed message (M4-20).
      await app.waitFor(find.byKey(ValueKey('message.$sent')));
      await app.until(
        'the input empty again',
        () => tester.widget<TextField>(composerText).controller!.text.isEmpty,
      );
      await key(LogicalKeyboardKey.arrowUp);
      await app.waitFor(find.byKey(const ValueKey('edit.field')));
      await app.tap('edit.cancel');

      // The quick switcher jumps to general.
      await press(ShortcutAction.quickSwitcher);
      await app.waitFor(find.byKey(const ValueKey('switcher.field')));
      await tester.enterText(
        find.byKey(const ValueKey('switcher.field')),
        'general',
      );
      await app.tap('switcher.row.channel.${world.generalId}');
      await app.until(
        'the switcher jump',
        () => route() == '/c/${world.generalId}',
      );

      // Search, Running now, mentions, settings and the list of shortcuts.
      await press(ShortcutAction.search);
      await app.waitFor(find.byKey(const ValueKey('search.view')));
      await press(ShortcutAction.runningNow);
      await app.until('Running now', () => route() == '/running');
      await press(ShortcutAction.mentions);
      await app.until('the mentions', () => route() == '/mentions');
      await press(ShortcutAction.settings);
      await app.waitFor(find.byKey(const ValueKey('screen.settings')));
      GoRouter.of(tester.element(find.byKey(const ValueKey('screen.settings'))))
          .go('/t/$thread');
      await app.waitFor(find.byKey(ValueKey('history.$thread')));
      await press(ShortcutAction.shortcutList);
      await app.waitFor(find.byKey(const ValueKey('shortcutList.dialog')));
    } finally {
      await app.stop();
    }
  });
}
