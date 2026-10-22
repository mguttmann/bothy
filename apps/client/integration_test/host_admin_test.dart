// SPDX-License-Identifier: Apache-2.0

// Screen A8 end to end against FakeServer (brief M4-37 Tests): Alex creates a host, gets a
// code, the fake plays `bothy-host pair` reporting `term-alex` and `term-sam`, Alex types a
// wrong prefix once and then the right one, chooses organization and "no" for the server
// host, assigns Sam to `term-sam`, grants Sam `terminal` (the preview says "not effective"
// while Sam is not the assignee), makes Sam the assignee, and the preview becomes effective.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('create, pair, confirm, assign and grant a host', (tester) async {
    final app = await TestApp.start(tester);
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      final server = app.fake.server;
      final world = server.world;
      final sam = app.fake.seed.sam.id;

      final tap = app.tap;

      Future<void> type(String key, String text) async {
        await app.waitFor(find.byKey(ValueKey(key)));
        await tester.enterText(find.byKey(ValueKey(key)).first, text);
        await tester.pumpAndSettle();
      }

      Future<void> pick(String key, String label) async {
        await tap(key);
        await tester.tap(find.text(label).last);
        await tester.pumpAndSettle();
      }

      /// Continues the step-up dialog when the server asked for one.
      Future<void> stepUpIfAsked() async {
        await tester.pumpAndSettle();
        final dialog = find.byKey(const ValueKey('stepUp.continue'));
        if (dialog.evaluate().isNotEmpty) {
          await tester.tap(dialog);
          await tester.pumpAndSettle();
        }
      }

      String previewText() =>
          tester.widget<Text>(find.byKey(const ValueKey('preview.text'))).data!;

      GoRouter.of(tester.element(find.byType(Navigator).first))
          .go('/admin/hosts');
      await tester.pumpAndSettle();
      await app.waitFor(find.byKey(const ValueKey('admin.nav.hosts')));

      // Step 1 and 2: create the host and get the code with the command.
      await tap('hosts.add');
      await type('createHost.name', 'Build host');
      await type('createHost.reason', 'Shared build machine');
      await tap('pairing.create');
      await stepUpIfAsked();
      await eventually(
        'createHost answered',
        () => server.requestsOf('createHost').any((r) => r.status == 201),
      );
      await tester.pumpAndSettle();
      final hostId = world.hosts.keys.single;
      final command = tester
          .widget<SelectableText>(find.byKey(const ValueKey('pairing.command')))
          .data!;
      expect(command, startsWith('bothy-host pair --server '));
      expect(command, contains('--code bothy_hpc_'));

      // The fake plays `bothy-host pair` on the host.
      world.pairHost(hostId, ['term-alex', 'term-sam']);
      await tap('pairing.next');
      await app.waitFor(find.byKey(const ValueKey('fingerprint.prefix')));

      // Step 3: a wrong prefix once, then the right one.
      await type('fingerprint.prefix', 'ZZZZ ZZZZ');
      await tap('llmChoice.organization');
      await tap('serverHost.no');
      await tap('sessionRows.add');
      await pick('sessionRow.0.person', 'Sam');
      await pick('sessionRow.0.name', 'term-sam');
      await type('pairing.reason', 'Shared build machine');
      await tap('pairing.confirm');
      await stepUpIfAsked();
      await app.waitFor(find.byKey(const ValueKey('fingerprint.mismatch')));
      expect(find.textContaining('2 attempts left'), findsOneWidget);

      await type('fingerprint.prefix', '7kq2 9xta');
      await tap('pairing.confirm');
      await stepUpIfAsked();
      await eventually(
        'host active',
        () => world.hosts[hostId]!.state == 'active',
      );
      await tester.pumpAndSettle();
      expect(world.hosts[hostId]!.sessionUsers[sam]?.$1, 'term-sam');
      await app.waitFor(find.byKey(ValueKey('sessionUser.$sam')));

      // Grant Sam `terminal`: not effective while Sam is not the assignee (SR10).
      await tap('hostGrants.add');
      await tap('subject.$sam');
      await tap('wizard.next');
      await tap('wizard.next');
      await app.waitFor(find.byKey(const ValueKey('preview.notEffective')));
      expect(previewText(), contains('not the assignee'));
      await type('wizard.reason', 'Builds');
      // "Confirm" waits for the preview of the request with the reason (D-M4-27-1).
      await eventually('confirm enabled', () async {
        await tester.pump(const Duration(milliseconds: 100));
        return tester
                .widget<FilledButton>(
                  find.byKey(const ValueKey('wizard.confirm')),
                )
                .onPressed !=
            null;
      });
      await tap('wizard.confirm');
      await stepUpIfAsked();
      await eventually(
        'createGrant answered',
        () => server.requestsOf('createGrant').any((r) => r.status == 201),
      );

      await tester.pumpAndSettle();
      await app.waitFor(find.byKey(const ValueKey('hostGrants.add')));

      // Sam becomes the assignee; the preview of the same grant is effective.
      await tap('host.assignee.change');
      await pick('assignee.person', 'Sam');
      await type('confirm.reason', 'Sam owns the builds');
      await tap('confirm.submit');
      await stepUpIfAsked();
      await eventually(
        'Sam is the assignee',
        () => world.hosts[hostId]!.assignedUserId == sam,
      );
      await tester.pumpAndSettle();

      await tap('hostGrants.add');
      await tap('subject.$sam');
      await tap('wizard.next');
      await tap('wizard.next');
      await app.waitFor(find.byKey(const ValueKey('preview.text')));
      expect(previewText(), isNot(contains('Not effective')));
      expect(find.byKey(const ValueKey('preview.notEffective')), findsNothing);
      await tap('wizard.cancel');

      expect(server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
