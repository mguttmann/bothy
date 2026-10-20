// SPDX-License-Identifier: Apache-2.0

// The client part of TM-T26 (docs/spec/24-terminal.md 8.4 and 14, threat model T-TM-12): an
// owner opening or attaching on a host assigned to another person sees the warning before the
// step-up, and no step-up is requested without the confirmation; the person's own host and a
// host without assignee show none.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

const String _title = 'This host belongs to another person';

Future<Harness> _setUp(WidgetTester tester, {String? assignee}) async {
  final nav = GlobalKey<NavigatorState>();
  final h = (await tester.runAsync(
    () => Harness.create(
      prompts: DialogTerminalPrompts(
        () => nav.currentContext,
        selfUserId: alexId,
      ),
    ),
  ))!;
  h.api.assignedUserId = assignee;
  await pumpApp(tester, const SizedBox.expand(), navigatorKey: nav);
  return h;
}

Future<void> _confirmOpen(WidgetTester tester) async {
  await pumpUntil(
    tester,
    () => find.text('Open terminal').evaluate().isNotEmpty,
  );
  await tester.tap(find.text('Confirm with passkey or device key'));
  await tester.pump();
}

void main() {
  testWidgets(
    'open on the host of another person: warning first, no step-up without the '
    'confirmation',
    (tester) async {
      final h = await _setUp(tester, assignee: samId);
      final done = h.open();
      await pumpUntil(tester, () => find.text(_title).evaluate().isNotEmpty);
      expect(find.textContaining('Build host'), findsOneWidget);
      expect(h.stepUps, isEmpty);
      await tester.tap(find.text('Do not connect'));
      await tester.pumpAndSettle();
      await done;
      expect(h.stepUps, isEmpty);
      expect(h.api.opens, isEmpty);
      expect(h.controller.state.phase, TerminalPhase.idle);

      final g = await _setUp(tester, assignee: samId);
      final opening = g.open();
      await pumpUntil(tester, () => find.text(_title).evaluate().isNotEmpty);
      expect(g.stepUps, isEmpty);
      await tester.tap(find.text('Connect anyway'));
      await tester.pump();
      await _confirmOpen(tester);
      await pumpUntil(
        tester,
        () => g.controller.state.phase == TerminalPhase.connected,
      );
      await opening;
      expect(g.log, ['getHost', 'stepUp', 'open']);
    },
  );

  testWidgets('attach on the host of another person warns before the step-up', (
    tester,
  ) async {
    final h = await _setUp(tester, assignee: samId);
    final opening = h.open();
    await pumpUntil(tester, () => find.text(_title).evaluate().isNotEmpty);
    await tester.tap(find.text('Connect anyway'));
    await tester.pump();
    await _confirmOpen(tester);
    await pumpUntil(
      tester,
      () => h.controller.state.phase == TerminalPhase.connected,
    );
    await opening;
    await h.controller.detach();
    h.api.attachErrors.add(stepUpRequired);
    final stepUpsBefore = h.stepUps.length;

    var attaching = h.controller.attach();
    await pumpUntil(tester, () => find.text(_title).evaluate().isNotEmpty);
    await tester.tap(find.text('Do not connect'));
    await tester.pumpAndSettle();
    await attaching;
    expect(h.stepUps, hasLength(stepUpsBefore));
    expect(h.api.attaches, isEmpty);

    attaching = h.controller.attach();
    await pumpUntil(tester, () => find.text(_title).evaluate().isNotEmpty);
    await tester.tap(find.text('Connect anyway'));
    await tester.pump();
    await pumpUntil(
      tester,
      () => h.controller.state.phase == TerminalPhase.connected,
    );
    await attaching;
    expect(h.stepUps, hasLength(stepUpsBefore + 1));
  });

  for (final assignee in [alexId, null]) {
    testWidgets(
      'no warning on ${assignee == null ? 'a host without assignee' : 'the own host'}',
      (tester) async {
        final h = await _setUp(tester, assignee: assignee);
        final done = h.open();
        await _confirmOpen(tester);
        await pumpUntil(
          tester,
          () => h.controller.state.phase == TerminalPhase.connected,
        );
        await done;
        expect(find.text(_title), findsNothing);
      },
    );
  }
}
