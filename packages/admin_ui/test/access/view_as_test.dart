// SPDX-License-Identifier: Apache-2.0

// "View as person" of A4 (brief M4-27 Tests): the step-up first, then the read-only frame with
// "Viewing as <name> (read-only). Nothing is sent. Logged.", the end of the view and the spaces
// and channels of the snapshot; nothing in it writes (07 9.2).

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

ViewAsResult samSnapshot() => ViewAsResult(
  user: const AccessPrincipal(
    id: 'sam',
    kind: 'user',
    handle: 'sam',
    displayName: 'Sam',
  ),
  expiresAt: DateTime.utc(2026, 10, 17, 9, 15),
  spaces: const [
    ViewAsItem(
      id: 'marketing',
      name: 'Marketing',
      level: AccessLevel.read,
      capabilities: [],
    ),
  ],
  channels: const [
    ViewAsItem(
      id: 'campaigns',
      name: 'Kampagnen',
      level: AccessLevel.trigger,
      capabilities: [AccessCapability.upload],
      spaceId: 'marketing',
    ),
  ],
);

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late int exits;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    )..snapshots['sam'] = samSnapshot();
    exits = 0;
  });

  Future<void> pump(WidgetTester tester) => pumpInApp(
    tester,
    ViewAsPerson(
      repository: repository,
      userId: 'sam',
      name: 'Sam',
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onExit: () => exits++,
    ),
    navigatorKey: navigator,
  );

  testWidgets('the step-up comes first, then the frame with its expiry', (
    tester,
  ) async {
    await pump(tester);
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text('View as Sam'), findsOneWidget);
    expect(find.byKey(const ValueKey('viewAs.frame')), findsNothing);

    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();

    expect(repository.calls, ['viewAs sam']);
    expect(
      find.text('Viewing as Sam (read-only). Nothing is sent. Logged.'),
      findsOneWidget,
    );
    // 09:15 UTC is 11:15 in Berlin; intl may separate "AM" with a narrow space.
    final until = tester
        .widget<Text>(find.byKey(const ValueKey('viewAs.until')))
        .data!;
    expect(until, startsWith('The view ends Today 11:15'));
    expect(
      find.byKey(const ValueKey('viewAs.space.marketing')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('viewAs.channel.campaigns')),
      findsOneWidget,
    );
    expect(find.textContaining('Marketing/Kampagnen'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('viewAs.exit')));
    expect(exits, 1);
  });

  testWidgets('nothing in the frame writes', (tester) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();

    final frame = find.byKey(const ValueKey('viewAs.frame'));
    for (final type in [
      TextField,
      Checkbox,
      Switch,
      FilledButton,
      ElevatedButton,
      InkWell,
    ]) {
      expect(
        find.descendant(of: frame, matching: find.byType(type)),
        findsNothing,
        reason: '$type in the read-only frame',
      );
    }
  });

  testWidgets('a cancelled step-up shows no snapshot', (tester) async {
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('viewAs.frame')), findsNothing);
    expect(find.byKey(const ValueKey('viewAs.space.marketing')), findsNothing);
  });

  testWidgets('a refusal shows the server reason', (tester) async {
    repository.snapshots.clear();
    await pump(tester);
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();

    expect(find.text('Not found or no access'), findsOneWidget);
    expect(find.byKey(const ValueKey('viewAs.frame')), findsNothing);
  });
}
