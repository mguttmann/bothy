// SPDX-License-Identifier: Apache-2.0

// The new chat view in a short window at text scale 2 (brief M4-41 requirement 12; the
// goldens of the brief use text scaling 1 and 2): the view scrolls instead of overflowing, so
// the composer stays reachable.

import 'package:client/forum/new_chat_view.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/new_chat_fixtures.dart';

void main() {
  testWidgets(
    'at text scale 2 in a short window the view scrolls instead of overflowing',
    (tester) async {
      final host = await ColumnHost.open(tester);
      await tester.runAsync(() => fillForums(host.db));
      await host.pump(
        tester,
        Builder(
          builder: (context) => MediaQuery(
            data: MediaQuery.of(context)
                .copyWith(textScaler: const TextScaler.linear(2)),
            child: NewChatView(forum: server.id),
          ),
        ),
        location: newChatPath(server),
        overrides: shellOverrides(),
      );
      tester.view.physicalSize = const Size(440, 560);
      await host.settle(tester);
      expect(tester.takeException(), isNull);
      // The view's own scrollable (the first below it; the composer's field has another).
      await tester.scrollUntilVisible(
        composerField,
        100,
        scrollable: find
            .descendant(
              of: find.byKey(const ValueKey('newChat.view')),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      await host.settle(tester);
      expect(composerField.hitTestable(), findsOneWidget);
      await host.dispose(tester);
    },
  );
}
