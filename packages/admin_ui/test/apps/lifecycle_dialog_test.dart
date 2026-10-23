// SPDX-License-Identifier: Apache-2.0

// Suspend, resume and uninstall (brief M6-08 Ruling 3; 11 4.3): the dialog shows the column of
// the table of 11 4.3 for the action before anything is sent, then the change goes to the
// server with its purpose and the server asks for the step-up; uninstall asks a reason.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    changed = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key)).last;
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> page(WidgetTester tester) async {
    await pumpInApp(
      tester,
      AppPage(
        repository: repository,
        appId: 'app-repo-bridge',
        clock: testClock,
        onChanged: () => changed++,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  String effects(WidgetTester tester) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(const ValueKey('lifecycle.effects')),
        matching: find.byType(Text),
      ),
    ))
      t.data ?? '',
  ].join(' | ');

  String state(WidgetTester tester) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(const ValueKey('appPage.state')),
        matching: find.byType(Text),
      ),
    ))
      t.data ?? '',
  ].join();

  testWidgets('suspend shows its effects before the step-up', (tester) async {
    repository.seedApp(repoBridgeManifest());
    await page(tester);
    expect(find.byKey(const ValueKey('appPage.resume')), findsNothing);
    await tap(tester, 'appPage.suspend');

    expect(find.byKey(const ValueKey('lifecycle.dialog')), findsOneWidget);
    expect(find.text('Suspend Repo Bridge?'), findsOneWidget);
    final table = effects(tester);
    expect(table, contains('Tokens | Blocked, not revoked'));
    expect(table, contains('Deliveries | Paused, the position is kept'));
    expect(table, contains('Commands | Hidden'));
    expect(table, contains('Components | Disabled'));
    expect(
      table,
      contains(
        'Access grants and incoming webhooks | Remain; incoming webhooks '
        'answer 404 until the app is resumed',
      ),
    );
    expect(table, contains('Messages of the app | Remain'));
    expect(find.byKey(const ValueKey('lifecycle.reason')), findsNothing);
    // Nothing is sent and no step-up asked before the confirmation.
    expect(repository.calls, isEmpty);
    expect(repository.stepUps, isEmpty);

    await tap(tester, 'lifecycle.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['suspendApp app-repo-bridge']);
    expect(repository.stepUps, ['Suspend the app Repo Bridge']);
    expect(state(tester), 'Suspended');
    expect(find.byKey(const ValueKey('appPage.resume')), findsOneWidget);
    expect(find.byKey(const ValueKey('appPage.suspend')), findsNothing);
    expect(find.text('Repo Bridge is suspended.'), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets('resume says what comes back', (tester) async {
    repository.seedApp(repoBridgeManifest(), state: AppInstallState.suspended);
    await page(tester);
    await tap(tester, 'appPage.resume');
    final table = effects(tester);
    expect(table, contains('Tokens | Work again'));
    expect(table, contains('Deliveries | Continue from the kept position'));
    expect(
      table,
      contains(
        'Access grants and incoming webhooks | Unchanged; incoming webhooks '
        'accept calls again',
      ),
    );
    await tap(tester, 'lifecycle.submit');
    await tap(tester, 'stepUp.continue');
    expect(repository.calls, ['resumeApp app-repo-bridge']);
    expect(repository.stepUps, ['Resume the app Repo Bridge']);
    expect(state(tester), 'Active');
  });

  testWidgets('uninstall shows its effects and needs a reason', (tester) async {
    repository.seedApp(repoBridgeManifest());
    await page(tester);
    await tap(tester, 'appPage.uninstall');
    final table = effects(tester);
    expect(table, contains('Tokens | Revoked'));
    expect(table, contains('Deliveries | Subscriptions are disabled'));
    expect(table, contains('Commands | Removed'));
    expect(table, contains('Access grants and incoming webhooks | Revoked'));
    expect(
      table,
      contains(
        'Messages of the app | Remain; the author is kept and the app is '
        'disabled',
      ),
    );
    final submit = find.byKey(const ValueKey('lifecycle.submit'));
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);
    await tester.enterText(
      find.byKey(const ValueKey('lifecycle.reason')),
      'Not used any more',
    );
    await tester.pump();
    expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
    await tap(tester, 'lifecycle.submit');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'uninstallApp app-repo-bridge Not used any more',
    ]);
    expect(repository.stepUps, ['Uninstall the app Repo Bridge']);
    expect(state(tester), 'Uninstalled');
    expect(find.byKey(const ValueKey('appPage.uninstall')), findsNothing);
    expect(find.byKey(const ValueKey('appPage.suspend')), findsNothing);
    expect(find.byKey(const ValueKey('appPage.resume')), findsNothing);
  });

  testWidgets('cancel sends nothing', (tester) async {
    repository.seedApp(repoBridgeManifest());
    await page(tester);
    await tap(tester, 'appPage.suspend');
    await tap(tester, 'lifecycle.cancel');
    expect(find.byKey(const ValueKey('lifecycle.dialog')), findsNothing);
    expect(repository.calls, isEmpty);
  });

  testWidgets('a cancelled step-up changes nothing and says nothing', (
    tester,
  ) async {
    repository.seedApp(repoBridgeManifest());
    await page(tester);
    await tap(tester, 'appPage.suspend');
    await tap(tester, 'lifecycle.submit');
    await tap(tester, 'stepUp.cancel');
    expect(state(tester), 'Active');
    expect(find.text('Repo Bridge is suspended.'), findsNothing);
    expect(changed, 0);
  });
}
