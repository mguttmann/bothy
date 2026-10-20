// SPDX-License-Identifier: Apache-2.0

// The guided flow of 10 11 end to end through the screens (brief M4-28 Tests, BR-07 from the
// point of view of the interface): a personal agent held by Alex operates the space Support;
// Alex grants Sam `trigger` there; the preview shows the I1 conflict; Alex takes the guided
// option with a reason and step-up, which is one `createGrant` with `resolve_i1`; afterwards the
// agent runs on organization, Sam has the grant, and the agent page warns that the adapter still
// reports personal until the fake's heartbeat changes.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('the guided flow switches the agent and grants in one request', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      final world = app.fake.server.world;
      final server = app.fake.server;
      final alex = app.fake.seed.alex.id;
      final sam = app.fake.seed.sam.id;
      final support = world.addSpace('Support');
      world.addChannel(support, 'Queue', {alex: 'trigger'});
      final agent = world.addAgent(
        handle: 'helper',
        displayName: 'Helper',
        billingMode: 'personal',
        credentialHolderId: alex,
        reportedBillingMode: 'personal',
      );
      world.setSpaceOperator(support, agent.id);

      Future<void> tap(String key) async {
        await app.waitFor(find.byKey(ValueKey(key)));
        await tester.ensureVisible(find.byKey(ValueKey(key)).first);
        await tester.tap(find.byKey(ValueKey(key)).first);
        await tester.pumpAndSettle();
      }

      void go(String path) =>
          GoRouter.of(tester.element(find.byType(Navigator).first)).go(path);

      go('/admin/grants/space.$support');
      await tester.pumpAndSettle();

      await tap('grants.create');
      await tap('subject.$sam');
      await tap('wizard.next');
      await tap('wizard.next');
      await tap('level.trigger');
      await tester.enterText(
        find.byKey(const ValueKey('wizard.reason')),
        'Support rota',
      );
      await tester.pumpAndSettle();

      // 10 11 point 1: the preview reports the conflict and offers the guided way.
      await app.waitFor(
        find.byKey(const ValueKey('preview.violation.i1_violation')),
      );
      // The offer follows the preview of the current request, which includes the reason.
      await app.waitFor(
        find.byWidgetPredicate(
          (w) =>
              w is ButtonStyleButton &&
              w.key == const ValueKey('guided.offer') &&
              w.onPressed != null,
        ),
      );
      await tap('guided.offer');
      await tester.enterText(
        find.byKey(const ValueKey('confirm.reason')),
        'Team uses it',
      );
      await tester.pumpAndSettle();
      await tap('confirm.submit');
      await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
      await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
      await eventually(
        'createGrant answered',
        () => server.requestsOf('createGrant').any((r) => r.status == 201),
      );
      await tester.pumpAndSettle();

      // One request carried both changes (10 11 point 4, BR-07).
      final grants = server.requestsOf('createGrant');
      expect(grants.where((r) => r.status == 201), hasLength(1));
      expect(server.requestsOf('setAgentBillingMode'), isEmpty);
      expect(world.agents[agent.id]!.billingMode, 'organization');
      expect(world.agents[agent.id]!.credentialHolderId, isNull);
      expect(
        world.grants.values.where(
          (g) =>
              g['subject_id'] == sam &&
              g['target_id'] == support &&
              g['level'] == 'trigger',
        ),
        hasLength(1),
      );
      final rows = [
        for (final r in server.audit.rows)
          if (r['action'] == 'agent.billing_mode.set' ||
              r['action'] == 'grant.create')
            r,
      ];
      expect(rows.map((r) => r['action']), [
        'agent.billing_mode.set',
        'grant.create',
      ]);
      expect(rows.first['correlation_id'], rows.last['correlation_id']);

      // Step 5: the result says the adapter still reports personal.
      await app.waitFor(
        find.byKey(ValueKey('guided.stillReports.${agent.id}')),
      );
      await tap('guided.close');

      go('/admin/agents/${agent.id}');
      await tester.pumpAndSettle();
      // The server's comparison is false at once; the hint says how to switch the adapter.
      await app.waitFor(find.byKey(const ValueKey('agentPage.switchHint')));

      // A heartbeat that still reports personal brings the warning code (10 8).
      world.agentHeartbeat(agent.id);
      go('/admin/agents');
      await tester.pumpAndSettle();
      go('/admin/agents/${agent.id}');
      await tester.pumpAndSettle();
      await app.waitFor(
        find.byKey(const ValueKey('agentPage.warning.billing_mode_mismatch')),
      );

      // The adapter switches its login; the next heartbeat reports organization.
      world.agentHeartbeat(agent.id, reportedBillingMode: 'organization');
      go('/admin/agents');
      await tester.pumpAndSettle();
      go('/admin/agents/${agent.id}');
      await tester.pumpAndSettle();
      await app.waitFor(find.byKey(const ValueKey('agentPage.reported')));
      expect(
        find.byKey(const ValueKey('agentPage.warning.billing_mode_mismatch')),
        findsNothing,
      );
      expect(find.byKey(const ValueKey('agentPage.switchHint')), findsNothing);
      expect(server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
