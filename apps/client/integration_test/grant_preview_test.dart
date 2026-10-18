// SPDX-License-Identifier: Apache-2.0

// The example of 07 9.3 end to end through the screens (brief M4-27 Tests): Alex opens the
// grant wizard on the channel Marketing/Kampagnen, chooses Sam, `trigger` until 2026-12-31
// with a reason, sees the preview text of 07 9.3 unchanged (read from the specification at
// run time by the fake and by this test, D-W-94) and the H1 hint, confirms with step-up, and
// "Who has access?" then lists Sam with the direct source.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('the example of 07 9.3 from the wizard to Who has access?', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      final world = app.fake.server.world;
      final alex = app.fake.seed.alex.id;
      final sam = app.fake.seed.sam.id;
      final texter = world.addPerson(
        handle: 'texter',
        displayName: 'Texter',
        kind: 'agent',
      );
      final marketing = world.addSpace('Marketing');
      final campaigns = world.addChannel(marketing, 'Kampagnen', {
        alex: 'trigger',
        texter.id: 'write',
      });
      world.addChannel(marketing, 'Intern', {
        alex: 'trigger',
        texter.id: 'read',
      });

      Future<void> tap(String key) async {
        await app.waitFor(find.byKey(ValueKey(key)));
        await tester.ensureVisible(find.byKey(ValueKey(key)).first);
        await tester.tap(find.byKey(ValueKey(key)).first);
        await tester.pumpAndSettle();
      }

      GoRouter.of(tester.element(find.byType(Navigator).first))
          .go('/admin/grants/channel.$campaigns');
      await tester.pumpAndSettle();

      await tap('grants.create');
      await tap('subject.$sam');
      await tap('wizard.next');
      await tap('wizard.next');
      await tap('level.trigger');
      await tester.enterText(
        find.byKey(const ValueKey('wizard.until')),
        '2026-12-31',
      );
      await tester.enterText(
        find.byKey(const ValueKey('wizard.reason')),
        'Campaign work',
      );
      await tester.pumpAndSettle();

      final example = grantPreviewExample();
      await app.waitFor(find.text(example));
      final text = tester.widget<Text>(
        find.byKey(const ValueKey('preview.text')),
      );
      expect(text.data, example);
      expect(
        find.byKey(const ValueKey('preview.hint.read_horizon')),
        findsOneWidget,
      );

      await tap('wizard.confirm');
      await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
      await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
      await eventually(
        'createGrant answered',
        () => app.fake.server
            .requestsOf('createGrant')
            .any((r) => r.status == 201),
      );
      await tester.pumpAndSettle();

      await tap('grants.access');
      await app.waitFor(find.byKey(ValueKey('access.entry.$sam')));
      // The direct source carries the grant, so its row offers "Revoke" for it.
      final grant = world.grants.keys.single;
      expect(
        find.descendant(
          of: find.byKey(ValueKey('access.entry.$sam')),
          matching: find.byKey(ValueKey('access.revoke.$grant')),
        ),
        findsOneWidget,
      );
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
