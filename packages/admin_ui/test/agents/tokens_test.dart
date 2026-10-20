// SPDX-License-Identifier: Apache-2.0

// The API tokens of an agent (brief M4-28 Tests and Design note 4; 08 12, ADR 0061): the list
// newest first, create through step-up with the secret shown once and never kept, revoke after
// one confirmation without step-up (05: token.create has step-up, token.revoke has none).

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

const _secret = 'bothy_agt_EXAMP0EXAMPLESecretShownOnce000000000000000_000000';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  Future<void> section(WidgetTester tester, {Key? key}) async {
    await pumpInApp(
      tester,
      AgentTokens(
        key: key,
        repository: repository,
        agent: repository.agentRecordsById['texter']!,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  bool canSubmit(WidgetTester tester) =>
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('confirm.submit')))
          .onPressed !=
      null;

  testWidgets(
    'the list shows prefix, validity, use and binding, newest first',
    (tester) async {
      repository.tokensById['texter'] = [
        AgentToken(
          id: 'old',
          name: 'Old',
          prefix: 'bothy_agt_OLDPRE',
          createdAt: DateTime.utc(2026, 9),
          revokedAt: DateTime.utc(2026, 9, 20),
        ),
        AgentToken(
          id: 'deploy',
          name: 'Deploy',
          prefix: 'bothy_agt_DEPLOY',
          createdAt: DateTime.utc(2026, 10, 2),
          expiresAt: DateTime.utc(2026, 12, 31, 23),
          lastUsedAt: DateTime.utc(2026, 10, 17, 8),
          dpopBound: true,
        ),
      ];
      await section(tester);

      expect(find.text('API tokens'), findsOneWidget);
      final deploy = tester.getTopLeft(
        find.byKey(const ValueKey('token.deploy')),
      );
      final old = tester.getTopLeft(find.byKey(const ValueKey('token.old')));
      expect(deploy.dy, lessThan(old.dy));

      Finder inRow(String id, Finder f) =>
          find.descendant(of: find.byKey(ValueKey('token.$id')), matching: f);
      expect(inRow('deploy', find.text('bothy_agt_DEPLOY')), findsOneWidget);
      expect(
        inRow('deploy', find.textContaining('valid until Dec 31, 2026')),
        findsOneWidget,
      );
      expect(
        inRow('deploy', find.textContaining('last used Today')),
        findsOneWidget,
      );
      expect(
        inRow('deploy', find.textContaining('DPoP-bound')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('token.deploy.revoke')), findsOneWidget);
      expect(inRow('old', find.text('Revoked')), findsOneWidget);
      expect(find.byKey(const ValueKey('token.old.revoke')), findsNothing);
    },
  );

  testWidgets('create asks for step-up and shows the secret once only', (
    tester,
  ) async {
    await section(tester);
    expect(find.text('No tokens.'), findsOneWidget);

    await tap(tester, 'tokens.create');
    expect(find.text('Create a token for Texter'), findsOneWidget);
    expect(canSubmit(tester), isFalse);
    await type(tester, 'token.name', 'Deploy');
    await type(tester, 'token.lastDay', '2026-02-30');
    expect(find.text('Enter a day as YYYY-MM-DD.'), findsOneWidget);
    expect(canSubmit(tester), isFalse);
    await type(tester, 'token.lastDay', '2026-12-31');
    // 08 12: the DPoP thumbprint is required for an agent token.
    expect(canSubmit(tester), isFalse);
    await type(
      tester,
      'token.dpop',
      'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs',
    );
    expect(canSubmit(tester), isTrue);
    await tap(tester, 'confirm.submit');

    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text('Create a token for Texter'), findsWidgets);
    await tap(tester, 'stepUp.continue');

    // The token holds through the typed day in Europe/Berlin (start of 1 January there).
    expect(repository.calls, [
      'createToken texter Deploy 2026-12-31T23:00:00.000Z',
    ]);
    expect(repository.stepUps, ['Create a token for Texter']);

    expect(find.byKey(const ValueKey('token.secret')), findsOneWidget);
    expect(find.text(_secret), findsOneWidget);
    expect(
      find.text(
        "This secret is shown only once. Copy it into the adapter's secret "
        'store now.',
      ),
      findsOneWidget,
    );
    await tap(tester, 'token.secretClose');

    // Gone from the tree and from the state; the list shows the token by its prefix.
    expect(find.textContaining(_secret), findsNothing);
    expect(find.byKey(const ValueKey('token.token-0')), findsOneWidget);
    expect(find.text('bothy_agt_EXAMP0'), findsOneWidget);

    // A reload never shows the secret again.
    await section(tester, key: const ValueKey('reloaded'));
    expect(find.byKey(const ValueKey('token.token-0')), findsOneWidget);
    expect(find.textContaining(_secret), findsNothing);
  });

  testWidgets('a DPoP thumbprint is passed on', (tester) async {
    await section(tester);
    await tap(tester, 'tokens.create');
    await type(tester, 'token.name', 'Bound');
    await type(tester, 'token.lastDay', '2026-11-30');
    await type(
      tester,
      'token.dpop',
      'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs',
    );
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.continue');
    await tap(tester, 'token.secretClose');
    expect(repository.tokensById['texter']!.single.dpopBound, isTrue);
  });

  testWidgets('revoke asks once and needs no step-up', (tester) async {
    repository.tokensById['texter'] = [
      AgentToken(
        id: 'token-0',
        name: 'Deploy',
        prefix: 'bothy_agt_DEPLOY',
        createdAt: DateTime.utc(2026, 10, 2),
        expiresAt: DateTime.utc(2026, 12, 31, 23),
      ),
    ];
    await section(tester);

    await tap(tester, 'token.token-0.revoke');
    expect(find.text('Revoke the token Deploy?'), findsOneWidget);
    expect(
      find.text('An adapter that uses this token loses its access at once.'),
      findsOneWidget,
    );
    await tap(tester, 'confirm.submit');

    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(repository.calls, ['revokeToken token-0']);
    expect(repository.stepUps, isEmpty);
    expect(find.text('The token Deploy is revoked.'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('token.token-0')),
        matching: find.text('Revoked'),
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('token.token-0.revoke')), findsNothing);
  });

  testWidgets('a cancelled revoke changes nothing', (tester) async {
    repository.tokensById['texter'] = [
      AgentToken(
        id: 'token-0',
        name: 'Deploy',
        prefix: 'bothy_agt_DEPLOY',
        createdAt: DateTime.utc(2026, 10, 2),
      ),
    ];
    await section(tester);
    await tap(tester, 'token.token-0.revoke');
    await tap(tester, 'confirm.cancel');
    expect(repository.calls, isEmpty);
  });

  testWidgets('a 403 hides the control the server refused', (tester) async {
    repository.tokensById['texter'] = [
      AgentToken(
        id: 'token-0',
        name: 'Deploy',
        prefix: 'bothy_agt_DEPLOY',
        createdAt: DateTime.utc(2026, 10, 2),
      ),
    ];
    repository.refuse['revokeToken'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    repository.refuse['createToken'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await section(tester);

    await tap(tester, 'token.token-0.revoke');
    await tap(tester, 'confirm.submit');
    expect(find.byKey(const ValueKey('token.token-0.revoke')), findsNothing);

    await tap(tester, 'tokens.create');
    await type(tester, 'token.name', 'Deploy');
    await type(tester, 'token.lastDay', '2026-12-31');
    await type(
      tester,
      'token.dpop',
      'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs',
    );
    await tap(tester, 'confirm.submit');
    expect(find.byKey(const ValueKey('tokens.create')), findsNothing);
    expect(find.byKey(const ValueKey('token.secret')), findsNothing);
  });

  testWidgets('a refused list shows the server\'s reason and nothing else', (
    tester,
  ) async {
    repository.refuse['tokens'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await section(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('tokens.create')), findsNothing);
  });
}
