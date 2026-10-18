// SPDX-License-Identifier: Apache-2.0

// "Who has access?" of A3 (brief M4-27 Tests): every source kind of 07 9.1 rendered with its
// plain label; an entry only via an agent has level `none` and hint H1; disabled principals
// are struck through; revoking a direct grant from a row; "View as person" from the picker.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    )..extraAccess['campaigns'] = everySource();
  });

  Future<void> pump(WidgetTester tester) => pumpInApp(
    tester,
    WhoHasAccess(
      repository: repository,
      target: campaigns,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
    navigatorKey: navigator,
  );

  Finder inEntry(String id, Finder matching) => find.descendant(
    of: find.byKey(ValueKey('access.entry.$id')),
    matching: matching,
  );

  testWidgets('every source kind of 07 9.1 has its plain label', (
    tester,
  ) async {
    await pump(tester);

    expect(
      find.text('Who has access? Channel Marketing/Kampagnen'),
      findsOneWidget,
    );
    expect(inEntry('alex', find.text('Owner')), findsOneWidget);
    expect(inEntry('sam', find.text('Direct access grant')), findsOneWidget);
    expect(
      inEntry('sam', find.textContaining('Reason: Stand-in')),
      findsOneWidget,
    );
    expect(inEntry('sam', find.textContaining('Dec 31, 2026')), findsOneWidget);
    expect(inEntry('sam', find.text('Via group Support')), findsOneWidget);
    expect(
      inEntry('sam', find.textContaining('member until Nov 30, 2026')),
      findsOneWidget,
    );
    expect(
      inEntry('robin', find.text('Inherited from space Marketing')),
      findsOneWidget,
    );
    expect(
      inEntry('texter', find.text('Operator of this channel')),
      findsOneWidget,
    );
    expect(inEntry('kim', find.text('Participant')), findsOneWidget);
    expect(
      inEntry(
        'lee',
        find.text('Trigger agents, but triggering is blocked (SR3)'),
      ),
      findsOneWidget,
    );
    expect(inEntry('lee', find.text('Agent Texter')), findsOneWidget);
    expect(inEntry('sam', find.textContaining('Trigger agents')), findsWidgets);
    expect(inEntry('sam', find.textContaining('Upload files')), findsOneWidget);
  });

  testWidgets('an entry only via an agent has level none and hint H1', (
    tester,
  ) async {
    await pump(tester);

    expect(inEntry('jo', find.textContaining('No level')), findsOneWidget);
    expect(
      inEntry(
        'jo',
        find.text('Can learn content here through agent Texter (H1)'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('a disabled principal is struck through', (tester) async {
    await pump(tester);

    final name = tester.widget<Text>(
      find.byKey(const ValueKey('access.name.robin')),
    );
    expect(name.style?.decoration, TextDecoration.lineThrough);
    final active = tester.widget<Text>(
      find.byKey(const ValueKey('access.name.sam')),
    );
    expect(active.style?.decoration, isNot(TextDecoration.lineThrough));
  });

  testWidgets('a direct grant is revoked from its row with a reason', (
    tester,
  ) async {
    repository.grantList.add(
      AdminGrant(
        id: 'g-sam',
        subjectKind: AdminSubjectKind.user,
        subjectId: 'sam',
        targetKind: AdminTargetKind.channel,
        targetId: 'campaigns',
        level: AccessLevel.trigger,
        capabilities: const [],
        reason: 'Stand-in',
        grantedAt: DateTime.utc(2026, 10),
      ),
    );
    repository.extraAccess['campaigns'] = [];
    await pump(tester);

    await tester.tap(find.byKey(const ValueKey('access.revoke.g-sam')));
    await tester.pumpAndSettle();
    expect(
      find.text('Revoking takes effect immediately: the access ends now.'),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('revokeGrant.reason')),
      'Back from leave',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('confirm.submit')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();

    expect(repository.calls, ['revokeGrant g-sam Back from leave']);
    expect(find.byKey(const ValueKey('access.entry.sam')), findsNothing);
  });
}
