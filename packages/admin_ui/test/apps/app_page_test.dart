// SPDX-License-Identifier: Apache-2.0

// The page of one app (brief M6-08; 11 4, 5): header with name, version, publisher and state,
// the description, the homepage as text, the granted scopes with their meaning, the event
// subscriptions, the destinations, the commands and data_egress; the buttons by state; the
// tokens of the app with "New token" (secret once, DPoP optional) and revoke; the
// subscriptions section; a refused app shows the server's reason and nothing else.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
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
    final f = find.byKey(ValueKey(key)).last;
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  Future<void> page(
    WidgetTester tester, {
    Key? key,
    VoidCallback? onBack,
  }) async {
    await pumpInApp(
      tester,
      AppPage(
        key: key,
        repository: repository,
        appId: 'app-repo-bridge',
        timeZone: 'Europe/Berlin',
        clock: testClock,
        onBack: onBack,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  String pageText(WidgetTester tester) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(const ValueKey('appPage')),
        matching: find.byType(Text),
      ),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' | ');

  testWidgets('the page shows what the app may do in plain words', (
    tester,
  ) async {
    repository.seedApp(
      repoBridgeManifest(),
      grantedScopes: const [
        'messages:write',
        'commands:write',
        'messages:read',
      ],
    );
    final back = <int>[];
    await page(tester, onBack: () => back.add(1));

    final text = pageText(tester);
    expect(text, contains('Repo Bridge'));
    expect(text, contains('Version 1.2.0'));
    expect(text, contains('by Example Org'));
    expect(text, contains('Brings events from a code hosting service'));
    expect(text, contains('Homepage: https://bridge.example.com'));
    expect(text, contains('Read messages and threads and search in them'));
    expect(text, contains('Register and remove commands'));
    // events:subscribe is optional and was not granted: not on the page.
    expect(text, isNot(contains('Receive events and catch up')));
    expect(text, contains('bridge.example.com'));
    expect(text, contains('/issue'));
    expect(text, contains('Messages of the granted channels go to'));
    // The homepage is text only: the server never loads it and the page links nothing.
    expect(find.byKey(const ValueKey('appPage.homepage')), findsOneWidget);
    expect(
      tester.widget(find.byKey(const ValueKey('appPage.homepage'))),
      isA<Text>(),
    );

    expect(find.byKey(const ValueKey('appPage.state')), findsOneWidget);
    expect(find.byKey(const ValueKey('appPage.suspend')), findsOneWidget);
    expect(find.byKey(const ValueKey('appPage.uninstall')), findsOneWidget);
    expect(find.byType(AppSubscriptionsSection), findsOneWidget);

    await tap(tester, 'appPage.back');
    expect(back, [1]);
  });

  testWidgets('an uninstalled app offers no lifecycle button', (tester) async {
    repository.seedApp(
      repoBridgeManifest(),
      state: AppInstallState.uninstalled,
    );
    await page(tester);
    expect(find.byKey(const ValueKey('appPage.uninstall')), findsNothing);
    expect(find.byKey(const ValueKey('appPage.suspend')), findsNothing);
    expect(find.byKey(const ValueKey('appPage.resume')), findsNothing);
  });

  testWidgets('a refused app shows the reason and nothing else', (
    tester,
  ) async {
    repository.seedApp(repoBridgeManifest());
    repository.refuse['app'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'not_owner',
    );
    await page(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('appPage.suspend')), findsNothing);
    expect(find.byKey(const ValueKey('appPage.update')), findsNothing);
    expect(find.byType(AppSubscriptionsSection), findsNothing);
  });

  testWidgets('the tokens of the app show prefix and validity, newest first', (
    tester,
  ) async {
    repository.seedApp(repoBridgeManifest());
    repository.tokensById['app-repo-bridge'] = [
      AgentToken(
        id: 'old',
        name: 'Old',
        prefix: 'bothy_app_OLDPRE',
        createdAt: DateTime.utc(2026, 9),
        revokedAt: DateTime.utc(2026, 9, 20),
      ),
      AgentToken(
        id: 'bridge',
        name: 'Bridge',
        prefix: 'bothy_app_BRIDGE',
        createdAt: DateTime.utc(2026, 10, 2),
        expiresAt: DateTime.utc(2026, 12, 31, 23),
      ),
    ];
    await page(tester);
    final bridge = tester.getTopLeft(
      find.byKey(const ValueKey('appToken.bridge')),
    );
    final old = tester.getTopLeft(find.byKey(const ValueKey('appToken.old')));
    expect(bridge.dy, lessThan(old.dy));
    expect(find.text('bothy_app_BRIDGE'), findsOneWidget);
    expect(find.textContaining('valid until Dec 31, 2026'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('appToken.bridge.revoke')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('appToken.old.revoke')), findsNothing);
  });

  testWidgets(
    '"New token" asks for step-up, needs no DPoP thumbprint and shows '
    'the secret once',
    (tester) async {
      repository.seedApp(repoBridgeManifest());
      await page(tester);
      await tap(tester, 'appTokens.create');
      expect(find.text('Create a token for Repo Bridge'), findsOneWidget);
      await type(tester, 'appTokenForm.name', 'Bridge');
      await type(tester, 'appTokenForm.lastDay', '2026-12-31');
      await tap(tester, 'confirm.submit');
      await tap(tester, 'stepUp.continue');

      expect(repository.calls, [
        'createToken app-repo-bridge Bridge 2026-12-31T23:00:00.000Z',
      ]);
      expect(repository.stepUps, ['Create a token for Repo Bridge']);
      expect(
        find.byKey(const ValueKey('appToken.secretDialog')),
        findsOneWidget,
      );
      expect(find.text(_secret), findsOneWidget);
      await tap(tester, 'appToken.secretClose');

      expect(find.textContaining(_secret), findsNothing);
      expect(find.byKey(const ValueKey('appToken.token-0')), findsOneWidget);
      await page(tester, key: const ValueKey('reloaded'));
      expect(find.textContaining(_secret), findsNothing);
    },
  );

  testWidgets('revoke asks once and needs no step-up', (tester) async {
    repository.seedApp(repoBridgeManifest());
    repository.tokensById['app-repo-bridge'] = [
      AgentToken(
        id: 'bridge',
        name: 'Bridge',
        prefix: 'bothy_app_BRIDGE',
        createdAt: DateTime.utc(2026, 10, 2),
      ),
    ];
    await page(tester);
    await tap(tester, 'appToken.bridge.revoke');
    expect(
      find.text('The app loses the access of this token at once.'),
      findsOneWidget,
    );
    await tap(tester, 'confirm.submit');
    expect(repository.calls, ['revokeToken bridge']);
    expect(repository.stepUps, isEmpty);
    expect(find.text('Revoked'), findsOneWidget);
  });
}
