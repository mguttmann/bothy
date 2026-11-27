// SPDX-License-Identifier: Apache-2.0

// The status chip of computer use in the thread (brief CU-07 requirements 13 and 16; 27 11
// items 1 and 2): every state (opening, active, paused, closed with each reason), stop with a
// refusal, keyboard and screen reader names, on the desktop and on the phone; the slot shows the
// chip of the thread's runs only, and nothing for a person who is no owner. No view shows an
// image.

import 'dart:async';

import 'package:client/agents/providers.dart' show threadRunsProvider;
import 'package:client/computer/computer_chip.dart';
import 'package:client/computer/computer_status.dart';
import 'package:client/computer/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show RunState, ThreadId;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show HostEvent;

import '../support/structure.dart' show run, threadId;
import 'support.dart';

/// The run of thread 1 that controls the desktop.
final String threadRun = run(1, RunState.running).id.value;

ComputerStatus status(
  ComputerPhase phase, {
  int actions = 4,
  String? app = 'com.example.editor',
  String? reason,
}) => ComputerStatus(
  sessionId: computerSessionId(1),
  hostId: deskHost,
  runId: threadRun,
  desktop: 'Desk',
  phase: phase,
  actionCount: actions,
  version: 2,
  frontApp: app,
  closeReason: reason,
);

Widget app(
  Widget child, {
  DsDensityMode density = DsDensityMode.comfortable,
  Locale locale = const Locale('en'),
}) {
  final ds = DsContext(
    theme: DsThemeMode.light,
    density: density,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  return MaterialApp(
    theme: buildDsTheme(ds),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    locale: locale,
    builder: (context, c) => DsThemeScope(dsContext: ds, child: c!),
    home: Scaffold(body: child),
  );
}

void main() {
  group('chip', () {
    testWidgets('opening, active, paused and the app-less active state', (
      tester,
    ) async {
      for (final (s, text) in [
        (status(ComputerPhase.opening), 'Connecting to Desk'),
        (
          status(ComputerPhase.active),
          'Controls Desk · com.example.editor · 4 actions',
        ),
        (
          status(ComputerPhase.active, actions: 1, app: null),
          'Controls Desk · 1 action',
        ),
        (status(ComputerPhase.paused), 'Desk locked, waiting'),
      ]) {
        await tester.pumpWidget(
          app(ComputerStatusChip(status: s, onStop: () {})),
        );
        expect(find.text(text), findsOneWidget, reason: '$s');
        expect(find.text('Stop'), findsOneWidget);
        expect(find.byType(Image), findsNothing);
        expect(find.byType(RawImage), findsNothing);
      }
    });

    testWidgets('the end names each reason once and offers dismiss', (
      tester,
    ) async {
      const reasons = {
        'done': 'Done',
        'stopped_by_user': 'Stopped',
        'device_disabled': 'Enablement switched off',
        'run_ended': 'Run ended',
        'access_revoked': 'Access revoked',
        'host_disabled': 'Host disabled',
        'host_retired': 'Host retired',
        'host_rekeyed': 'Host paired again',
        'host_restarted': 'Host service restarted',
        'connection_lost': 'Connection lost',
        'open_failed': 'Could not start',
        'error': 'Error',
        'something_new': 'Ended',
      };
      for (final MapEntry(key: code, value: text) in reasons.entries) {
        var dismissed = 0;
        await tester.pumpWidget(
          app(
            ComputerStatusChip(
              status: status(ComputerPhase.closed, reason: code),
              onDismiss: () => dismissed++,
            ),
          ),
        );
        expect(
          find.text('Computer control of Desk ended: $text'),
          findsOneWidget,
          reason: code,
        );
        expect(find.text('Stop'), findsNothing);
        await tester.tap(find.text('Dismiss'));
        expect(dismissed, 1);
      }
    });

    testWidgets('German texts', (tester) async {
      await tester.pumpWidget(
        app(
          ComputerStatusChip(
            status: status(ComputerPhase.paused),
            onStop: () {},
          ),
          locale: const Locale('de'),
        ),
      );
      expect(find.text('Desk gesperrt, wartet'), findsOneWidget);
      expect(find.text('Stoppen'), findsOneWidget);
    });

    testWidgets('stop has a name for screen readers and works by keyboard', (
      tester,
    ) async {
      final handle = tester.ensureSemantics();
      var stops = 0;
      await tester.pumpWidget(
        app(
          ComputerStatusChip(
            status: status(ComputerPhase.active),
            onStop: () => stops++,
          ),
        ),
      );
      expect(
        find.bySemanticsLabel('Stop the computer control of Desk'),
        findsOneWidget,
      );
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(stops, 1);
      await expectLater(tester, meetsGuideline(textContrastGuideline));
      handle.dispose();
    });
  });

  group('slot in the thread', () {
    late FakeComputerApi api;
    late StreamController<HostEvent> events;
    setUp(() {
      api = FakeComputerApi();
      events = StreamController<HostEvent>.broadcast(sync: true);
    });
    tearDown(() => events.close());

    Future<ProviderContainer> pump(
      WidgetTester tester, {
      bool owner = true,
      DsDensityMode density = DsDensityMode.comfortable,
      ThreadId? thread,
    }) async {
      final c = ProviderContainer(
        overrides: [
          computerPortsProvider.overrideWithValue(
            ComputerPorts(api: api, events: events.stream),
          ),
          isOwnerProvider.overrideWith((ref) async => owner),
          threadRunsProvider(threadId(1).value)
              .overrideWith((ref) => Stream.value([run(1, RunState.running)])),
          threadRunsProvider(threadId(2).value)
              .overrideWith((ref) => Stream.value(const [])),
        ],
      );
      addTearDown(c.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: c,
          child: app(
            ComputerChipSlot(threadId: thread ?? threadId(1)),
            density: density,
          ),
        ),
      );
      await tester.pumpAndSettle();
      return c;
    }

    for (final density in [DsDensityMode.comfortable, DsDensityMode.touch]) {
      testWidgets(
        'stop reaches the API and the end shows once (${density.name})',
        (tester) async {
          api.sessions = [computerSession(1, runId: threadRun)];
          await pump(tester, density: density);
          expect(
            find.text('Controls Desk · com.example.editor · 4 actions'),
            findsOneWidget,
          );
          events.add(computerEvent(1, state: 'paused', runId: threadRun));
          await tester.pump();
          expect(find.text('Desk locked, waiting'), findsOneWidget);
          await tester.tap(find.text('Stop'));
          await tester.pumpAndSettle();
          expect(api.calls, ['list', 'stop ${computerSessionId(1)}']);
          expect(
            find.text('Computer control of Desk ended: Stopped'),
            findsOneWidget,
          );
          await tester.tap(find.text('Dismiss'));
          await tester.pump();
          expect(find.byType(ComputerStatusChip), findsNothing);
        },
      );

      testWidgets('a refused stop is shown (${density.name})', (tester) async {
        api
          ..sessions = [computerSession(1, runId: threadRun)]
          ..failure = problem(404, 'not_found');
        await pump(tester, density: density);
        await tester.tap(find.text('Stop'));
        await tester.pumpAndSettle();
        expect(
          find.textContaining('Could not stop the computer control'),
          findsOneWidget,
        );
        expect(find.text('Desk locked, waiting'), findsNothing);
        expect(find.text('Stop'), findsOneWidget);
      });
    }

    testWidgets('another thread shows nothing', (tester) async {
      api.sessions = [computerSession(1, runId: threadRun)];
      await pump(tester, thread: threadId(2));
      expect(find.byType(ComputerStatusChip), findsNothing);
    });

    testWidgets('a person who is no owner sees nothing', (tester) async {
      api.sessions = [computerSession(1, runId: threadRun)];
      await pump(tester, owner: false);
      expect(find.byType(ComputerStatusChip), findsNothing);
      expect(api.calls, isEmpty);
    });
  });
}
