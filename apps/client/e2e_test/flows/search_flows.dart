// SPDX-License-Identifier: Apache-2.0

// The server search (brief M4-34 product requirement 13; 14 4): Sam writes a message with a word
// no other message has, and Alex finds it in D7.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';

/// Registers the search flow.
void searchFlows(E2eContext c) {
  final world = c.world;

  testWidgets('search finds a message of Sam', (tester) async {
    // Letters only: the search splits words at digits as well.
    final word =
        'zeppelin${randomNonce().substring(0, 6).replaceAll(RegExp('[0-9]'), 'q')}';
    final app = await E2eApp.start(tester, world);
    try {
      await app.signIn(await c.alexCode());
      final message = (await tester.runAsync(
        () => c.sam.post(world.generalThreadId, 'The $word lands at noon'),
      ))!;
      GoRouter.of(tester.element(find.byKey(const ValueKey('column.sidebar'))))
          .go('/search');
      await app.waitFor(find.byKey(const ValueKey('search.field')));
      final hit = find.byKey(ValueKey('search.hit.${message.id}'));
      // The server indexes asynchronously (14 4): ask again, every half second, until the
      // answer holds the hit.
      final watch = Stopwatch()..start();
      while (hit.evaluate().isEmpty) {
        if (watch.elapsed >= patience) fail('timed out waiting for the hit');
        await tester.enterText(
          find.byKey(const ValueKey('search.field')),
          word,
        );
        await tester.pump();
        await app.tap('search.submit');
        await tester.pump(const Duration(milliseconds: 500));
      }
    } finally {
      await app.stop();
    }
  });
}
