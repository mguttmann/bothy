// SPDX-License-Identifier: Apache-2.0

// "Permission hints" (brief M6-07 Design note 4; 07 6, 9.5): each hint shows its kind as icon
// and text and a sentence from the templates of l10n built from the page's labels, in the
// caller's language; the filters ask the server for one kind or one space; "Load more"
// continues with the cursor; "Explain" opens "Who has access?" (A3) of the channel or space;
// a grant can be revoked, an agent opened; a refused list shows the server's reason and keeps
// the filters. The screen decides nothing itself (15 1 principle 1).

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
    PermissionHintsView(
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

  Future<void> choose(WidgetTester tester, String menu, String item) async {
    await tap(tester, menu);
    await tester.tap(find.byKey(ValueKey(item)).last);
    await tester.pumpAndSettle();
  }

  const h1 = 'hint.h1_read_horizon.texter.sam.internal';
  const disabled = 'hint.grant_to_disabled.g-robin';
  const blocked = 'hint.trigger_blocked.g-support';
  const personal = 'hint.agent_personal.texter';

  testWidgets('every kind with its icon, its kind text and its sentence', (
    tester,
  ) async {
    await pump(tester);

    expect(repository.hintRequests, ['hints - - -']);
    for (final key in [h1, disabled, blocked, personal]) {
      expect(find.byKey(ValueKey(key)), findsOneWidget, reason: key);
      expect(
        find.descendant(
          of: find.byKey(ValueKey(key)),
          matching: find.byKey(ValueKey('$key.icon')),
        ),
        findsOneWidget,
        reason: key,
      );
    }
    expect(find.text('Read horizon (H1)'), findsWidgets);
    expect(
      find.text(
        'Sam can learn content of Marketing/Intern through agent Texter.',
      ),
      findsOneWidget,
    );
    expect(
      find.text('Access grant to a disabled or deleted account'),
      findsWidgets,
    );
    expect(
      find.text(
        'Robin is disabled or deleted and still holds an access grant on '
        'Channel Marketing/Kampagnen: Read.',
      ),
      findsOneWidget,
    );
    expect(find.text('Trigger agents without effect'), findsWidgets);
    expect(
      find.text(
        'Support has the level Trigger agents on Space Marketing but cannot '
        'trigger an agent there.',
      ),
      findsOneWidget,
    );
    expect(find.textContaining('SR2: '), findsOneWidget);
    expect(find.textContaining('SR3: '), findsNothing);
    expect(find.text('Agent in Personal mode'), findsWidgets);
    expect(
      find.text('Agent Texter uses Personal mode on the subscription of Sam.'),
      findsOneWidget,
    );
    expect(
      find.text(
        'I1 asks that only the holder can trigger it. These people can '
        'trigger it now: Sam, Robin.',
      ),
      findsOneWidget,
    );
    // The actions of each kind.
    expect(find.byKey(const ValueKey('$h1.explain')), findsOneWidget);
    expect(find.byKey(const ValueKey('$h1.revoke')), findsNothing);
    expect(find.byKey(const ValueKey('$disabled.explain')), findsOneWidget);
    expect(find.byKey(const ValueKey('$disabled.revoke')), findsOneWidget);
    // Extending needs an end; these grants have none.
    expect(find.byKey(const ValueKey('$disabled.extend')), findsNothing);
    expect(find.byKey(const ValueKey('$blocked.revoke')), findsOneWidget);
    expect(find.byKey(const ValueKey('$personal.agent')), findsOneWidget);
    expect(find.byKey(const ValueKey('hints.more')), findsNothing);
  });

  testWidgets('the sentences in German come from the German templates', (
    tester,
  ) async {
    await pump(tester, locale: const Locale('de'));
    final de = lookupAppLocalizations(const Locale('de'));

    expect(
      find.text(de.hintsH1Text('Sam', 'Marketing/Intern', 'Texter')),
      findsOneWidget,
    );
    expect(find.text(de.hintsKindH1), findsWidgets);
    expect(
      find.text(de.hintsAgentPersonalText('Texter', 'Sam')),
      findsOneWidget,
    );
    expect(find.text(de.hintsRuleSr2), findsOneWidget);
  });

  testWidgets('the kind and the space filter ask the server again', (
    tester,
  ) async {
    await pump(tester);

    await choose(tester, 'hints.kind', 'hints.kind.trigger_blocked');
    expect(repository.hintRequests.last, 'hints - trigger_blocked -');
    expect(find.byKey(const ValueKey(blocked)), findsOneWidget);
    expect(find.byKey(const ValueKey(h1)), findsNothing);

    await choose(tester, 'hints.kind', 'hints.kind.all');
    await choose(tester, 'hints.space', 'hints.space.marketing');
    expect(repository.hintRequests.last, 'hints marketing - -');
    expect(find.byKey(const ValueKey(h1)), findsOneWidget);
    expect(find.byKey(const ValueKey(personal)), findsNothing);

    await choose(tester, 'hints.space', 'hints.space.instance');
    expect(repository.hintRequests.last, 'hints - - -');
    expect(find.byKey(const ValueKey(personal)), findsOneWidget);
  });

  testWidgets('"Load more" continues with the cursor', (tester) async {
    repository.hintPageSize = 2;
    await pump(tester);
    expect(find.byKey(const ValueKey(blocked)), findsNothing);

    await tap(tester, 'hints.more');

    expect(repository.hintRequests, ['hints - - -', 'hints - - 2']);
    for (final key in [h1, disabled, blocked, personal]) {
      expect(find.byKey(ValueKey(key)), findsOneWidget, reason: key);
    }
    expect(find.byKey(const ValueKey('hints.more')), findsNothing);
  });

  testWidgets('"Explain" opens Who has access? of the hint\'s channel', (
    tester,
  ) async {
    await pump(tester);

    await tap(tester, '$h1.explain');

    final explained = tester.widget<WhoHasAccess>(find.byType(WhoHasAccess));
    expect(
      explained.target,
      const AdminTarget(AdminTargetKind.channel, id: 'internal'),
    );
    expect(explained.target.path, 'Marketing/Intern');
    await tap(tester, 'hints.explainClose');
    expect(find.byType(WhoHasAccess), findsNothing);

    await tap(tester, '$blocked.explain');
    expect(
      tester.widget<WhoHasAccess>(find.byType(WhoHasAccess)).target,
      marketing,
    );
  });

  testWidgets('"Revoke" revokes the grant with a reason and loads again', (
    tester,
  ) async {
    final changed = <int>[];
    await pumpInApp(
      tester,
      PermissionHintsView(
        repository: repository,
        onNavigate: opened.add,
        onChanged: () => changed.add(1),
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
      navigatorKey: navigator,
    );

    await tap(tester, '$disabled.revoke');
    await tester.enterText(
      find.byKey(const ValueKey('revokeGrant.reason')),
      'Robin left',
    );
    await tester.pumpAndSettle();
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['revokeGrant g-robin Robin left']);
    expect(repository.stepUps, ['Revoke the access grant for Robin']);
    expect(changed, [1]);
    expect(find.byKey(const ValueKey(disabled)), findsNothing);
    expect(
      find.text('The access grant for Robin was revoked.'),
      findsOneWidget,
    );
  });

  testWidgets('"Open agent" opens the page of the personal agent', (
    tester,
  ) async {
    await pump(tester);

    await tap(tester, '$personal.agent');

    expect(opened, const [AdminLocation(AdminSection.agents, id: 'texter')]);
  });

  testWidgets('a refused list shows the reason and keeps the filters', (
    tester,
  ) async {
    repository.refuse['permissionHints'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pump(tester);

    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.textContaining('Only an owner can do this'), findsOneWidget);
    expect(find.byKey(const ValueKey(h1)), findsNothing);
    expect(find.byKey(const ValueKey('hints.space')), findsOneWidget);

    repository.refuse.remove('permissionHints');
    await choose(tester, 'hints.space', 'hints.space.marketing');
    expect(find.byKey(const ValueKey('admin.denied')), findsNothing);
    expect(find.byKey(const ValueKey(h1)), findsOneWidget);
  });

  testWidgets('an empty list says so', (tester) async {
    repository.hintList = [];
    await pump(tester);

    expect(find.text('No permission hints.'), findsOneWidget);
  });

  testWidgets('the body of the administration shows both lists', (
    tester,
  ) async {
    for (final (section, type) in [
      (AdminSection.hints, PermissionHintsView),
      (AdminSection.expiring, ExpiringAccessView),
    ]) {
      await pumpInApp(
        tester,
        SizedBox(
          height: 900,
          child: AdminBody(
            repository: repository,
            location: AdminLocation(section),
            onNavigate: opened.add,
            timeZone: 'Europe/Berlin',
            clock: testClock,
          ),
        ),
        navigatorKey: navigator,
      );
      expect(find.byType(type), findsOneWidget, reason: section.name);
    }
  });
}
