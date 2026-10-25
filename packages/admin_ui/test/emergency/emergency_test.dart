// SPDX-License-Identifier: Apache-2.0

// The emergency actions on the phone (brief M5-09 Tests): the person search, revoking a device,
// revoking a grant and disabling a person, each with its sheet that says it takes effect at
// once and the step-up the fake server asks for; a cancelled step-up changes nothing and says
// so; the last owner shows the server's SR8 answer with an explanation; a caller without the
// owner flag sees only the grants; a refused action hides its control; tap targets, labels and
// text scaling 2 on a phone screen.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../admin/support.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

/// A grant of Sam on the space Marketing.
AdminGrant samOnMarketing() => AdminGrant(
  id: 'g-sam',
  subjectKind: AdminSubjectKind.user,
  subjectId: 'sam',
  targetKind: AdminTargetKind.space,
  targetId: 'marketing',
  level: AccessLevel.write,
  capabilities: const [],
  reason: 'Campaign work',
  grantedAt: DateTime.utc(2026, 10, 3),
);

/// Pumps [child] on a phone screen of [platform] at [textScale].
Future<void> pumpPhone(
  WidgetTester tester,
  Widget child, {
  GlobalKey<NavigatorState>? navigatorKey,
  TargetPlatform platform = TargetPlatform.iOS,
  double textScale = 1,
}) async {
  tester.view
    ..physicalSize = const Size(390, 844)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.touch,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      navigatorKey: navigatorKey,
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => MediaQuery(
        data: MediaQuery.of(context)
            .copyWith(textScaler: TextScaler.linear(textScale)),
        child: DsThemeScope(
          dsContext: ds,
          // The scope installs its own theme; the platform goes on top of it.
          child: Builder(
            builder: (context) => Theme(
              data: Theme.of(context).copyWith(platform: platform),
              child: app!,
            ),
          ),
        ),
      ),
      home: Scaffold(body: child),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  late GlobalKey<NavigatorState> navigator;
  late RecordingPrompt prompt;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    prompt = RecordingPrompt();
    repository = FakeAdminRepository(prompt: prompt)
      ..grantList.add(samOnMarketing());
  });

  Widget person(String id, {bool owner = true}) => PersonEmergencyView(
    repository: repository,
    userId: id,
    owner: owner,
    timeZone: 'Europe/Berlin',
    clock: testClock,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  testWidgets('the search lists the people the server returns and opens one', (
    tester,
  ) async {
    final opened = <String>[];
    await pumpPhone(
      tester,
      EmergencyHome(repository: repository, onOpen: opened.add),
    );
    expect(find.text(en.emergencyIntro), findsOneWidget);
    expect(find.byKey(const ValueKey('emergency.person.sam')), findsOneWidget);

    await tester.enterText(
      find.byKey(const ValueKey('emergency.search')),
      'rob',
    );
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(repository.listRequests.last, 'rob|');
    expect(find.byKey(const ValueKey('emergency.person.sam')), findsNothing);

    await tap(tester, 'emergency.person.robin');
    expect(opened, ['robin']);
  });

  testWidgets('a refused people list shows the reason and no search', (
    tester,
  ) async {
    repository.refuse['people'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await pumpPhone(
      tester,
      EmergencyHome(repository: repository, onOpen: (_) {}),
    );
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('emergency.search')), findsNothing);
  });

  testWidgets('revoking a device says "at once" and runs the step-up', (
    tester,
  ) async {
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);
    expect(find.byKey(const ValueKey('emergency.disable')), findsOneWidget);

    await tap(tester, 'device.sam-laptop.revoke');
    expect(
      find.text(en.emergencyRevokeDeviceBody('Sam laptop', 'Sam')),
      findsOneWidget,
    );
    expect(find.textContaining('at once'), findsOneWidget);
    expect(find.text(en.emergencyConfirmFaceId), findsOneWidget);

    await tap(tester, 'emergency.sheet.confirm');
    expect(prompt.asked, [en.stepUpPurposeRevokeDevice('Sam laptop')]);
    expect(repository.calls, [
      'revokeDevice sam-laptop ${en.adminDefaultReason}',
    ]);
    expect(find.text(en.devicesRevoked('Sam laptop')), findsOneWidget);
    expect(find.byKey(const ValueKey('device.sam-laptop')), findsNothing);
  });

  testWidgets('a cancelled step-up changes nothing and says so', (
    tester,
  ) async {
    prompt.answer = false;
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);

    await tap(tester, 'device.sam-laptop.revoke');
    await tap(tester, 'emergency.sheet.confirm');

    expect(prompt.asked, hasLength(1));
    expect(find.text(en.emergencyCancelled), findsOneWidget);
    expect(find.byKey(const ValueKey('device.sam-laptop')), findsOneWidget);
    expect(repository.devices['sam']!.first.state, SelfDeviceState.active);
  });

  testWidgets('cancelling the sheet sends nothing', (tester) async {
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);
    await tap(tester, 'emergency.disable');
    await tap(tester, 'emergency.sheet.cancel');
    expect(repository.calls, isEmpty);
    expect(prompt.asked, isEmpty);
  });

  testWidgets('revoking a grant needs a reason and takes effect at once', (
    tester,
  ) async {
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);
    expect(find.text(en.adminTargetSpace('Marketing')), findsOneWidget);

    await tap(tester, 'emergency.grant.g-sam.revoke');
    expect(
      find.text(
        en.emergencyRevokeGrantBody('Sam', en.adminTargetSpace('Marketing')),
      ),
      findsOneWidget,
    );
    final confirm = find.byKey(const ValueKey('emergency.sheet.confirm'));
    expect(tester.widget<DsButton>(confirm).onPressed, isNull);

    await tester.enterText(
      find.byKey(const ValueKey('emergency.sheet.reason')),
      'Left the team',
    );
    await tester.pump();
    await tap(tester, 'emergency.sheet.confirm');

    expect(repository.calls, ['revokeGrant g-sam Left the team']);
    expect(prompt.asked, [en.stepUpPurposeRevokeGrant('Sam')]);
    expect(find.byKey(const ValueKey('emergency.grant.g-sam')), findsNothing);
    expect(find.text(en.emergencyGrantsEmpty), findsOneWidget);
  });

  testWidgets('disabling a person ends everything at once', (tester) async {
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);
    await tap(tester, 'emergency.disable');
    expect(find.text(en.emergencyDisableBody('Sam')), findsOneWidget);

    await tap(tester, 'emergency.sheet.confirm');
    expect(repository.calls, ['disablePerson sam ${en.adminDefaultReason}']);
    expect(prompt.asked, [en.stepUpPurposeDisablePerson('Sam')]);
    expect(find.text(en.adminPersonDisabled('Sam')), findsOneWidget);
    expect(find.byKey(const ValueKey('emergency.disable')), findsNothing);
    expect(find.text(en.emergencyDevicesEmpty), findsOneWidget);
  });

  testWidgets('the last owner: the server answers SR8 and the view explains', (
    tester,
  ) async {
    await pumpPhone(tester, person('alex'), navigatorKey: navigator);
    await tap(tester, 'emergency.disable');
    await tap(tester, 'emergency.sheet.confirm');

    // The client sent the request; the server, not the client, refused it.
    expect(repository.calls, ['disablePerson alex ${en.adminDefaultReason}']);
    expect(
      find.text('${en.problemLastOwner}. ${en.emergencyLastOwner}'),
      findsOneWidget,
    );
    expect(repository.peopleById['alex']!.status, AdminPersonStatus.active);
  });

  testWidgets('without the owner flag only the grants show', (tester) async {
    await pumpPhone(tester, person('sam', owner: false));
    expect(find.byKey(const ValueKey('emergency.disable')), findsNothing);
    expect(find.text(en.devicesTitle), findsNothing);
    expect(find.byKey(const ValueKey('device.sam-laptop')), findsNothing);
    expect(
      find.byKey(const ValueKey('emergency.grant.g-sam.revoke')),
      findsOneWidget,
    );
  });

  testWidgets('a refused revocation shows the reason and hides the button', (
    tester,
  ) async {
    repository.refuse['revokeGrant'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pumpPhone(tester, person('sam'), navigatorKey: navigator);
    await tap(tester, 'emergency.grant.g-sam.revoke');
    await tester.enterText(
      find.byKey(const ValueKey('emergency.sheet.reason')),
      'Left the team',
    );
    await tester.pump();
    await tap(tester, 'emergency.sheet.confirm');

    expect(
      find.text(problemText(en, 'forbidden', reason: 'owner_only')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('emergency.grant.g-sam.revoke')),
      findsNothing,
    );
    expect(find.byKey(const ValueKey('emergency.grant.g-sam')), findsOneWidget);
  });

  testWidgets('Android names the device lock on the confirm button', (
    tester,
  ) async {
    await pumpPhone(
      tester,
      person('sam'),
      navigatorKey: navigator,
      platform: TargetPlatform.android,
    );
    await tap(tester, 'emergency.disable');
    expect(find.text(en.emergencyConfirmDeviceLock), findsOneWidget);
    expect(find.text(en.emergencyConfirmFaceId), findsNothing);
  });

  for (final scale in const [1.0, 2.0]) {
    testWidgets('tap targets and labels at text scale $scale', (tester) async {
      final semantics = tester.ensureSemantics();
      await pumpPhone(tester, person('sam'), textScale: scale);
      expect(tester.takeException(), isNull);
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

      await tap(tester, 'emergency.disable');
      expect(tester.takeException(), isNull);
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      semantics.dispose();
    });
  }
}
