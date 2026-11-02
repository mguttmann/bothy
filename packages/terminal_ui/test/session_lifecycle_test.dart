// SPDX-License-Identifier: Apache-2.0

// The life cycle of a tab (FIX-terminal-session-lifecycle, docs/decisions.d/FIX-terminal-session-
// lifecycle.md): an ended tab offers one "Close" instead of the dead "Detach" and "End", on the
// desktop tabs and on the phone switcher; "End" of a live session goes to the app, which ends it
// on the host and closes the tab; a detached tab remembers since when; a session the server
// closed meanwhile ends the tab on reconcile; the session list names every own session with its
// start, its detach time and "Reattach".

import 'package:api_client/api_client.dart'
    show TerminalCloseReason, TerminalSession, TerminalSessionState;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

TerminalSession listed(
  Harness h, {
  TerminalSessionState state = TerminalSessionState.detached,
  DateTime? detachedAt,
}) => h.api.grant().session.rebuild(
  (b) => b
    ..state = state
    ..openedAt = DateTime.utc(2026, 10, 20, 8)
    ..detachedAt = detachedAt,
);

/// A tab whose detached session the server ended with detach_expired.
Future<Harness> endedTab() async {
  final events = FakeHostEvents();
  final h = await Harness.create(events: events);
  h.controller.adopt(host: h.api.currentHost(), session: listed(h));
  events.changes.add(
    const TerminalSessionUpdated(
      sessionIdText,
      closed: true,
      closeReason: 'detach_expired',
    ),
  );
  await h.settle();
  expect(h.controller.state.phase, TerminalPhase.ended);
  return h;
}

void main() {
  for (final phone in [false, true]) {
    final where = phone ? 'phone' : 'desktop';
    testWidgets('$where: an ended tab offers Close and no dead Detach or End', (
      tester,
    ) async {
      final h = (await tester.runAsync(endedTab))!;
      var closed = 0;
      await pumpApp(
        tester,
        TerminalPane(
          controller: h.controller,
          fontSize: 14,
          phone: phone,
          onClose: () => closed++,
        ),
      );
      expect(
        find.textContaining('it was detached for too long'),
        findsOneWidget,
      );
      expect(find.widgetWithText(OutlinedButton, 'Detach'), findsNothing);
      expect(find.widgetWithText(OutlinedButton, 'End'), findsNothing);
      await tester.tap(find.widgetWithText(OutlinedButton, 'Close'));
      await tester.pump();
      expect(closed, 1);
    });

    testWidgets('$where: End of a live session goes to the app', (
      tester,
    ) async {
      final h = (await tester.runAsync(Harness.create))!;
      var ended = 0;
      await pumpApp(
        tester,
        TerminalPane(
          controller: h.controller,
          fontSize: 14,
          phone: phone,
          onEnd: () => ended++,
          onClose: () {},
        ),
      );
      final opening = h.open();
      await pumpUntil(
        tester,
        () => h.controller.state.phase == TerminalPhase.connected,
      );
      await tester.runAsync(() => opening);
      await tester.pump();
      expect(find.widgetWithText(OutlinedButton, 'Close'), findsNothing);
      expect(find.widgetWithText(OutlinedButton, 'Detach'), findsOneWidget);
      await tester.tap(find.widgetWithText(OutlinedButton, 'End'));
      await tester.pump();
      expect(ended, 1);
      expect(h.api.closes, isEmpty, reason: 'the app ends it, not the pane');
    });
  }

  // language-check: off
  testWidgets(
    'German: the detached banner says "Wieder verbinden", an ended tab '
    '"Schließen"',
    (tester) async {
      final h = (await tester.runAsync(Harness.create))!;
      h.controller.adopt(host: h.api.currentHost(), session: listed(h));
      await pumpApp(
        tester,
        TerminalPane(controller: h.controller, fontSize: 14, onClose: () {}),
        locale: const Locale('de'),
      );
      expect(find.text('Wieder verbinden'), findsOneWidget);
      final e = (await tester.runAsync(endedTab))!;
      await pumpApp(
        tester,
        TerminalPane(controller: e.controller, fontSize: 14, onClose: () {}),
        locale: const Locale('de'),
      );
      expect(find.widgetWithText(OutlinedButton, 'Schließen'), findsOneWidget);
    },
  );
  // language-check: on

  test(
    'detach records since when; adopt takes detached_at of the server',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      h.clock.current = DateTime.utc(2026, 10, 20, 11, 5);
      await h.controller.detach();
      expect(h.controller.state.detachedAt, DateTime.utc(2026, 10, 20, 11, 5));
      final a = await Harness.create();
      a.controller.adopt(
        host: a.api.currentHost(),
        session: listed(a, detachedAt: DateTime.utc(2026, 10, 20, 9, 40)),
      );
      expect(a.controller.state.detachedAt, DateTime.utc(2026, 10, 20, 9, 40));
    },
  );

  test(
    'reconcile ends a detached tab whose session the server closed',
    () async {
      final h = await Harness.create();
      h.controller.adopt(host: h.api.currentHost(), session: listed(h));
      h.controller.reconcile(
        listed(h).rebuild(
          (b) => b
            ..state = TerminalSessionState.closed
            ..closeReason = TerminalCloseReason.detachExpired,
        ),
      );
      expect(h.controller.state.phase, TerminalPhase.ended);
      expect(h.controller.state.closeReason, 'detach_expired');
      // A session that still runs changes nothing.
      final r = await Harness.create();
      r.controller.adopt(host: r.api.currentHost(), session: listed(r));
      r.controller.reconcile(listed(r));
      expect(r.controller.state.phase, TerminalPhase.detached);
    },
  );

  testWidgets('the session list names each session with start, detach time and '
      'Reattach', (tester) async {
    final reattached = <String>[];
    await pumpApp(
      tester,
      TerminalSessionList(
        now: () => DateTime(2026, 11, 2, 12),
        entries: [
          TerminalSessionEntry(
            label: 'shell 1',
            phase: TerminalPhase.detached,
            startedAt: DateTime(2026, 11, 2, 8, 27),
            detachedAt: DateTime(2026, 11, 2, 8, 37),
            onReattach: () => reattached.add('shell 1'),
          ),
          TerminalSessionEntry(
            label: 'shell 2',
            phase: TerminalPhase.connected,
            startedAt: DateTime(2026, 11, 2, 9, 38),
          ),
        ],
      ),
    );
    expect(find.text('shell 1'), findsOneWidget);
    expect(find.text('shell 2'), findsOneWidget);
    expect(find.textContaining('Started 08:27'), findsOneWidget);
    expect(find.textContaining('Detached since 08:37'), findsOneWidget);
    expect(find.textContaining('Connected'), findsOneWidget);
    expect(find.widgetWithText(TextButton, 'Reattach'), findsOneWidget);
    await tester.tap(find.widgetWithText(TextButton, 'Reattach'));
    expect(reattached, ['shell 1']);
  });
}
