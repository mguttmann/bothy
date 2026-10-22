// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

const _sso = ConsoleProvider(
  id: '0192f4a0-7b1c-7474-8636-4dbb076ee0db',
  kind: ConsoleProviderKind.oidc,
  name: 'Company SSO',
);

/// Opens the dialog with [ways] and returns the future of its answer.
Future<Future<ConsoleStepUpAnswer?>> open(
  WidgetTester tester,
  ConsoleStepUpWays ways,
) async {
  final key = GlobalKey<NavigatorState>();
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      navigatorKey: key,
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
      home: const SizedBox(),
    ),
  );
  final answer = DialogStepUpPrompt(() => key.currentContext)
      .ask('Create the group Support Team', ways);
  await tester.pumpAndSettle();
  return answer;
}

void main() {
  testWidgets('with a passkey the dialog says the browser asks next', (
    tester,
  ) async {
    final answer = await open(
      tester,
      const ConsoleStepUpWays(passkey: true, password: false, oidc: []),
    );
    expect(find.text('Create the group Support Team'), findsOneWidget);
    expect(
      find.text('Next, your browser asks for your passkey.'),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('consoleStepUp.passwordField')),
      findsNothing,
    );
    await tester.tap(find.byKey(const ValueKey('consoleStepUp.passkey')));
    await tester.pumpAndSettle();
    expect(await answer, isA<StepUpWithPasskey>());
  });

  testWidgets('without a passkey it takes password and one-time code', (
    tester,
  ) async {
    final answer = await open(
      tester,
      const ConsoleStepUpWays(passkey: false, password: true, oidc: [_sso]),
    );
    await tester.enterText(
      find.byKey(const ValueKey('consoleStepUp.passwordField')),
      'pw-Aa1!',
    );
    await tester.enterText(
      find.byKey(const ValueKey('consoleStepUp.codeField')),
      '492039',
    );
    await tester.tap(find.byKey(const ValueKey('consoleStepUp.password')));
    await tester.pumpAndSettle();
    final a = await answer;
    expect(a, isA<StepUpWithPassword>());
    expect((a! as StepUpWithPassword).password, 'pw-Aa1!');
    expect((a as StepUpWithPassword).code, '492039');
  });

  testWidgets(
    'only OIDC offers the new login and says the change is not repeated',
    (tester) async {
      final answer = await open(
        tester,
        const ConsoleStepUpWays(passkey: false, password: false, oidc: [_sso]),
      );
      expect(find.textContaining('make the change again'), findsOneWidget);
      await tester.tap(find.text('Log in again with Company SSO'));
      await tester.pumpAndSettle();
      expect(((await answer)! as StepUpWithOidc).provider.name, 'Company SSO');
    },
  );

  testWidgets('no way says so; Cancel answers null', (tester) async {
    final answer = await open(
      tester,
      const ConsoleStepUpWays(passkey: false, password: false, oidc: []),
    );
    expect(find.textContaining('No way to confirm is set up'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('consoleStepUp.cancel')));
    await tester.pumpAndSettle();
    expect(await answer, isNull);
  });

  test('without a window the prompt cancels', () async {
    final prompt = DialogStepUpPrompt(() => null);
    expect(
      await prompt.ask(
        '',
        const ConsoleStepUpWays(passkey: true, password: false, oidc: []),
      ),
      isNull,
    );
  });
}
