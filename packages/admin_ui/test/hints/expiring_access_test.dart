// SPDX-License-Identifier: Apache-2.0

// "Expiring soon" (brief M6-07 Design notes 3 and 4; 07 9.4): grants, memberships and tokens
// with their end in the caller's time zone; "Extend" opens the grant wizard with the old
// grant's subject, target, level and capabilities, an empty reason and a new last day, and
// confirming creates the new grant first and then revokes the old one, with one step-up for
// both; "Explain" opens "Who has access?"; "Renew token" opens the agent's page with its
// tokens, an app token has no action; a membership opens its group.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<AdminLocation> opened;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    )..seedHints();
    opened = [];
  });

  Future<void> pump(
    WidgetTester tester, {
    Locale locale = const Locale('en'),
  }) => pumpInApp(
    tester,
    ExpiringAccessView(
      repository: repository,
      onNavigate: opened.add,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
    navigatorKey: navigator,
    locale: locale,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  /// Types [text] and lets the fake time pass the wizard's pause before its preview (300 ms
  /// by default), so the preview of the new request arrives.
  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
  }

  const grant = 'expiring.grant.g-sam';
  const membership = 'expiring.membership.m-robin';
  const agentToken = 'expiring.token.t-texter';
  const appToken = 'expiring.token.t-board';

  testWidgets('grants, memberships and tokens with their kind and end', (
    tester,
  ) async {
    await pump(tester);

    expect(repository.hintRequests, ['expiring - - -']);
    for (final key in [grant, membership, agentToken, appToken]) {
      expect(find.byKey(ValueKey(key)), findsOneWidget, reason: key);
      expect(find.byKey(ValueKey('$key.icon')), findsOneWidget, reason: key);
    }
    expect(
      find.text(
        'Sam on Channel Marketing/Kampagnen: Trigger agents, Upload files',
      ),
      findsOneWidget,
    );
    expect(find.text('Robin in the group Support'), findsOneWidget);
    expect(find.text('Token of the agent Texter'), findsOneWidget);
    expect(find.text('Token of the app Board'), findsOneWidget);
    // No token screen for apps here: the note says how to renew it (08 12: the owner creates
    // a new token and revokes the old one; nothing renews it by itself).
    expect(
      find.text(
        'There is no token screen for apps here yet: create a new token for '
        'the app, switch the app over, then revoke the old one.',
      ),
      findsOneWidget,
    );
    // The end in Europe/Berlin: the grant ends at 2026-10-22 00:00 local time, so its last
    // day is Wednesday 2026-10-21; the tokens end during their day.
    final day = L10nFormat(const Locale('en')).relativeDay(
      DateTime.utc(2026, 10, 21, 21, 59, 59),
      now: testNow,
      timeZone: 'Europe/Berlin',
    );
    expect(
      find.descendant(
        of: find.byKey(const ValueKey(grant)),
        matching: find.text('until $day'),
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('$grant.explain')), findsOneWidget);
    expect(find.byKey(const ValueKey('$grant.revoke')), findsOneWidget);
    expect(find.byKey(const ValueKey('$grant.extend')), findsOneWidget);
    expect(find.byKey(const ValueKey('$membership.group')), findsOneWidget);
    expect(find.byKey(const ValueKey('$membership.extend')), findsNothing);
    expect(find.byKey(const ValueKey('$membership.revoke')), findsNothing);
    expect(find.byKey(const ValueKey('$agentToken.renew')), findsOneWidget);
    expect(find.byKey(const ValueKey('$appToken.renew')), findsNothing);
  });

  testWidgets('the texts in German come from the German templates', (
    tester,
  ) async {
    await pump(tester, locale: const Locale('de'));
    final de = lookupAppLocalizations(const Locale('de'));

    expect(
      find.text(de.expiringMembershipText('Robin', 'Support')),
      findsOneWidget,
    );
    expect(find.text(de.expiringAgentTokenText('Texter')), findsOneWidget);
    expect(find.text(de.expiringAppTokenNote), findsOneWidget);
    expect(find.text(de.expiringKindGrant), findsWidgets);
  });

  testWidgets(
    '"Extend" prefills the wizard, then creates and revokes with one step-up',
    (tester) async {
      final changed = <int>[];
      await pumpInApp(
        tester,
        ExpiringAccessView(
          repository: repository,
          onNavigate: opened.add,
          onChanged: () => changed.add(1),
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
        navigatorKey: navigator,
      );

      await tap(tester, '$grant.extend');
      // The wizard previews its prefilled request after its pause.
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey('hints.extendDialog')), findsOneWidget);
      expect(find.text('Extend the access grant for Sam'), findsOneWidget);
      expect(
        find.text(
          'Extending creates a new access grant with a new reason and the last '
          'day you enter, then revokes the old one.',
        ),
        findsOneWidget,
      );
      final asked = repository.previews.last;
      expect(asked.subjectKind, AdminSubjectKind.user);
      expect(asked.subjectId, 'sam');
      expect(asked.target, campaigns);
      expect(asked.level, AccessLevel.trigger);
      expect(asked.capabilities, {AccessCapability.upload});
      expect(asked.reason, isEmpty, reason: 'a new reason is required');
      expect(
        asked.expiresAt,
        isNull,
        reason: 'a new last day, not the old one',
      );

      await type(tester, 'wizard.until', '2027-01-31');
      await type(tester, 'wizard.reason', 'Next quarter');
      await tap(tester, 'wizard.confirm');
      await tap(tester, 'stepUp.continue');

      expect(repository.calls, [
        'createGrant sam channel.campaigns trigger Next quarter',
        'revokeGrant g-sam Next quarter',
      ]);
      expect(repository.stepUps, hasLength(1), reason: 'one step-up for both');
      expect(find.byKey(const ValueKey('hints.extendDialog')), findsNothing);
      expect(
        find.text(
          'The access grant for Sam was extended: the new one applies, the old '
          'one is revoked.',
        ),
        findsOneWidget,
      );
      final renewed = repository.grantList.last;
      expect(renewed.capabilities, [AccessCapability.upload]);
      expect(renewed.expiresAt, DateTime.utc(2027, 1, 31, 23));
      expect(repository.grantList.any((g) => g.id == 'g-sam'), isFalse);
      expect(find.byKey(const ValueKey(grant)), findsNothing);
      expect(changed, isNotEmpty);
    },
  );

  testWidgets('a failed revocation after the new grant says both', (
    tester,
  ) async {
    await pump(tester);
    repository.refuse['revokeGrant'] = const ApiProblem(
      status: 409,
      code: 'conflict',
    );

    await tap(tester, '$grant.extend');
    await type(tester, 'wizard.until', '2027-01-31');
    await type(tester, 'wizard.reason', 'Next quarter');
    await tap(tester, 'wizard.confirm');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'createGrant sam channel.campaigns trigger Next quarter',
      'revokeGrant g-sam Next quarter',
    ]);
    expect(
      find.textContaining(
        'The new access grant for Sam applies, but the old one could not be '
        'revoked: ',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey(grant)), findsOneWidget);
  });

  testWidgets('cancelling the wizard changes nothing', (tester) async {
    await pump(tester);

    await tap(tester, '$grant.extend');
    await tap(tester, 'wizard.cancel');

    expect(repository.calls, isEmpty);
    expect(find.byKey(const ValueKey(grant)), findsOneWidget);
  });

  testWidgets('"Explain" opens Who has access? of the grant\'s target', (
    tester,
  ) async {
    await pump(tester);

    await tap(tester, '$grant.explain');

    expect(
      tester.widget<WhoHasAccess>(find.byType(WhoHasAccess)).target,
      campaigns,
    );
  });

  testWidgets('"Renew token" opens the agent, a membership its group', (
    tester,
  ) async {
    await pump(tester);

    await tap(tester, '$agentToken.renew');
    await tap(tester, '$membership.group');

    expect(opened, const [
      AdminLocation(AdminSection.agents, id: 'texter'),
      AdminLocation(AdminSection.groups, id: 'support'),
    ]);
  });

  testWidgets('the space filter asks the server for one space', (tester) async {
    await pump(tester);

    await tap(tester, 'expiring.space');
    await tester.tap(
      find.byKey(const ValueKey('expiring.space.marketing')).last,
    );
    await tester.pumpAndSettle();

    expect(repository.hintRequests.last, 'expiring marketing - -');
    expect(find.byKey(const ValueKey(grant)), findsOneWidget);
    expect(find.byKey(const ValueKey(agentToken)), findsNothing);
  });

  testWidgets('"Load more" continues with the cursor', (tester) async {
    repository.hintPageSize = 3;
    await pump(tester);
    expect(find.byKey(const ValueKey(appToken)), findsNothing);

    await tap(tester, 'expiring.more');

    expect(repository.hintRequests, ['expiring - - -', 'expiring - - 3']);
    expect(find.byKey(const ValueKey(appToken)), findsOneWidget);
  });

  testWidgets('a refused list shows the reason; nothing expiring says so', (
    tester,
  ) async {
    repository.refuse['expiringAccess'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pump(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey(grant)), findsNothing);

    repository
      ..refuse.remove('expiringAccess')
      ..expiringList = [];
    await tap(tester, 'expiring.space');
    await tester.tap(
      find.byKey(const ValueKey('expiring.space.marketing')).last,
    );
    await tester.pumpAndSettle();
    expect(find.text('Nothing expires soon.'), findsOneWidget);
  });
}
