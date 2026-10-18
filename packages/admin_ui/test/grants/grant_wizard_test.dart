// SPDX-License-Identifier: Apache-2.0

// The grant wizard of A2 (brief M4-27 Tests): three steps; the preview is called with the full
// request after every change and shows the server's text, effects, invariants and hints;
// "Confirm" stays disabled without a reason or with `allowed = false`; I1 is explained with
// "switch to organization first"; a delegation denial shows its reason; the step-up follows
// the confirmation; revoking says it takes effect immediately and needs a reason (07 9.3,
// 9.5).

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<AdminGrant> created;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    created = [];
  });

  Future<void> wizard(WidgetTester tester, {AdminTarget? target}) => pumpInApp(
    tester,
    GrantWizard(
      repository: repository,
      target: target,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      debounce: Duration.zero,
      onCreated: created.add,
    ),
    navigatorKey: navigator,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pumpAndSettle();
  }

  /// Steps 1 and 2 for Sam on the channel of the example; ends on step 3.
  Future<void> samOnCampaigns(WidgetTester tester) async {
    await wizard(tester);
    expect(find.text('Step 1 of 3'), findsOneWidget);
    await tap(tester, 'subject.sam');
    await tap(tester, 'wizard.next');
    expect(find.text('Step 2 of 3'), findsOneWidget);
    await tap(tester, 'target.channel.campaigns');
    await tap(tester, 'wizard.next');
    expect(find.text('Step 3 of 3'), findsOneWidget);
  }

  testWidgets(
    'three steps, the preview after every change with the full request',
    (tester) async {
      await samOnCampaigns(tester);
      expect(repository.previews, isEmpty, reason: 'no level chosen yet');

      await tap(tester, 'level.trigger');
      expect(repository.previews, hasLength(1));
      await type(tester, 'wizard.until', '2026-12-31');
      await tap(tester, 'capability.upload');
      await type(tester, 'wizard.reason', 'Campaign work');

      final last = repository.previews.last;
      expect(last.subjectKind, AdminSubjectKind.user);
      expect(last.subjectId, 'sam');
      expect(last.target, campaigns);
      expect(last.level, AccessLevel.trigger);
      expect(last.capabilities, {AccessCapability.upload});
      // The typed day holds through its end in the person's time zone (07 2.5).
      expect(last.expiresAt, DateTime.utc(2026, 12, 31, 23));
      expect(last.reason, 'Campaign work');
      expect(repository.previews.length, greaterThanOrEqualTo(4));

      expect(find.byKey(const ValueKey('preview.text')), findsOneWidget);
      expect(find.byKey(const ValueKey('preview.effect.sam')), findsOneWidget);
      expect(find.text('Gets Trigger agents'), findsOneWidget);
    },
  );

  testWidgets('the example of 07 9.3: server text unchanged, hint H1', (
    tester,
  ) async {
    await samOnCampaigns(tester);
    await tap(tester, 'level.trigger');
    await type(tester, 'wizard.until', '2026-12-31');
    await type(tester, 'wizard.reason', 'Campaign work');

    final text = tester.widget<Text>(
      find.byKey(const ValueKey('preview.text')),
    );
    expect(text.data, previewExample());
    expect(text.locale, const Locale('de'));
    expect(
      find.byKey(const ValueKey('preview.hint.read_horizon')),
      findsOneWidget,
    );
    expect(
      find.text(
        'Read horizon (H1): Sam can learn content of Marketing/Intern '
        'through agent Texter',
      ),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('preview.hint.expiry_missing')),
      findsNothing,
    );
  });

  testWidgets('a person without an expiry gets the expiry_missing hint', (
    tester,
  ) async {
    await samOnCampaigns(tester);
    await tap(tester, 'level.read');
    expect(
      find.byKey(const ValueKey('preview.hint.expiry_missing')),
      findsOneWidget,
    );
  });

  testWidgets('confirm is disabled without a reason and with allowed false', (
    tester,
  ) async {
    await samOnCampaigns(tester);
    await tap(tester, 'level.write');
    FilledButton confirm() => tester.widget<FilledButton>(
      find.byKey(const ValueKey('wizard.confirm')),
    );
    expect(confirm().onPressed, isNull, reason: 'no reason');

    await type(tester, 'wizard.reason', 'Campaign work');
    expect(confirm().onPressed, isNotNull);

    repository.previewAnswer = (r) => const GrantPreviewResult(
      allowed: false,
      denialReason: 'delegation_exceeds_own_level',
      effects: [],
      violations: [],
      hints: [],
      text: 'Refused',
      textLocale: 'en',
    );
    await type(tester, 'wizard.reason', 'Campaign work again');
    expect(confirm().onPressed, isNull, reason: 'allowed = false');
    expect(find.byKey(const ValueKey('preview.denied')), findsOneWidget);
    expect(
      find.text(
        'The access grant would be refused: You cannot give more than your '
        'own level',
      ),
      findsOneWidget,
    );
  });

  testWidgets('an I1 violation is explained with the switch hint', (
    tester,
  ) async {
    repository.previewAnswer = (r) => const GrantPreviewResult(
      allowed: false,
      effects: [],
      violations: [
        InvariantFinding(
          code: 'i1_violation',
          agentId: 'texter',
          principalIds: ['sam'],
          organizationReady: true,
        ),
      ],
      hints: [],
      text: 'Rejected',
      textLocale: 'en',
    );
    await samOnCampaigns(tester);
    await tap(tester, 'level.trigger');
    await type(tester, 'wizard.reason', 'Campaign work');

    expect(
      find.byKey(const ValueKey('preview.violation.i1_violation')),
      findsOneWidget,
    );
    expect(find.text('Agent: Texter'), findsOneWidget);
    expect(find.text('Affected: Sam'), findsOneWidget);
    expect(find.byKey(const ValueKey('preview.i1Hint')), findsOneWidget);
    expect(
      find.text('Switch the agent to organization first.'),
      findsOneWidget,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('wizard.confirm')))
          .onPressed,
      isNull,
    );
  });

  testWidgets('confirm creates the grant after the step-up', (tester) async {
    await samOnCampaigns(tester);
    await tap(tester, 'level.trigger');
    await type(tester, 'wizard.until', '2026-12-31');
    await type(tester, 'wizard.reason', 'Campaign work');
    final previewsBefore = repository.previews.length;

    await tap(tester, 'wizard.confirm');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(
      find.text(
        'Create the access grant for Sam on Channel Marketing/Kampagnen',
      ),
      findsOneWidget,
    );
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'createGrant sam channel.campaigns trigger Campaign work',
    ]);
    expect(created, hasLength(1));
    expect(repository.grantList.single.subjectId, 'sam');
    // The preview before the confirmation was the one of the final request.
    expect(repository.previews.length, previewsBefore);
  });

  testWidgets('groups and agents are subjects; search filters by name', (
    tester,
  ) async {
    await wizard(tester, target: campaigns);
    await type(tester, 'wizard.search', 'ro');
    expect(find.byKey(const ValueKey('subject.robin')), findsOneWidget);
    expect(find.byKey(const ValueKey('subject.sam')), findsNothing);

    await tap(tester, 'wizard.kind.group');
    await type(tester, 'wizard.search', '');
    expect(find.byKey(const ValueKey('subject.support')), findsOneWidget);
    await tap(tester, 'wizard.kind.agent');
    await tap(tester, 'subject.texter');
    await tap(tester, 'wizard.next');
    // The target given by the caller is already chosen.
    await tap(tester, 'wizard.next');
    await tap(tester, 'level.read');
    expect(repository.previews.last.subjectKind, AdminSubjectKind.agent);
    expect(repository.previews.last.target, campaigns);
  });

  testWidgets('revoking says it takes effect immediately and needs a reason', (
    tester,
  ) async {
    repository.grantList.add(
      AdminGrant(
        id: 'g1',
        subjectKind: AdminSubjectKind.user,
        subjectId: 'sam',
        targetKind: AdminTargetKind.channel,
        targetId: 'campaigns',
        level: AccessLevel.write,
        capabilities: const [],
        reason: 'Campaign work',
        grantedAt: DateTime.utc(2026, 10),
      ),
    );
    await pumpInApp(
      tester,
      GrantsList(
        repository: repository,
        targetKey: campaigns.key,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        onTarget: (_) {},
      ),
      navigatorKey: navigator,
    );
    expect(find.byKey(const ValueKey('grant.g1')), findsOneWidget);

    await tap(tester, 'grant.g1.revoke');
    expect(
      find.text('Revoking takes effect immediately: the access ends now.'),
      findsOneWidget,
    );
    FilledButton submit() => tester.widget<FilledButton>(
      find.byKey(const ValueKey('confirm.submit')),
    );
    expect(submit().onPressed, isNull, reason: 'the reason is required');
    await type(tester, 'revokeGrant.reason', 'Campaign over');
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['revokeGrant g1 Campaign over']);
    expect(find.byKey(const ValueKey('grant.g1')), findsNothing);
    expect(find.text('The access grant for Sam was revoked.'), findsOneWidget);
  });

  testWidgets('the shell lists Access grants and opens Who has access?', (
    tester,
  ) async {
    expect(
      AdminLocation.parse(['grants', 'channel.campaigns']),
      const AdminLocation(AdminSection.grants, id: 'channel.campaigns'),
    );
    final opened = <AdminLocation>[];
    repository.extraAccess['campaigns'] = everySource();
    await pumpInApp(
      tester,
      SizedBox(
        height: 900,
        child: AdminShell(
          repository: repository,
          location: const AdminLocation(
            AdminSection.grants,
            id: 'channel.campaigns',
          ),
          onNavigate: opened.add,
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
      ),
      navigatorKey: navigator,
    );
    expect(find.byKey(const ValueKey('admin.nav.grants')), findsOneWidget);
    expect(find.text('Access grants'), findsWidgets);
    await tap(tester, 'grants.access');
    expect(find.byKey(const ValueKey('access.entry.jo')), findsOneWidget);

    await tap(tester, 'grants.target');
    await tester.tap(find.text('Space Marketing').last);
    await tester.pumpAndSettle();
    expect(opened, [
      const AdminLocation(AdminSection.grants, id: 'space.marketing'),
    ]);
  });
}
