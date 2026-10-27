// SPDX-License-Identifier: Apache-2.0

// Screen M9 and the routes of the phone terminal (01 5.3 M8, M9; brief M5-12 Design note 1):
// `/hosts` and `/hosts/<id>` lie under Me; M9 lists the hosts with connection, the LLM login
// badge and the person's own sessions, and a tap on a detached session goes to M8 and attaches
// it.

import 'dart:io';

import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:client/app/providers/ports.dart';
import 'package:client/app/router.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/terminal/hosts_screen.dart';
import 'package:client/terminal/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../../support/fakes.dart';
import '../../terminal/support.dart';

void main() {
  group('routes', () {
    test('the hosts and the terminal belong to Me', () {
      for (final path in ['/hosts', '/hosts/$hostIdText']) {
        for (final tab in PhoneTab.values) {
          expect(
            tabOf(parseLocation(Uri.parse(path)), tab),
            PhoneTab.me,
            reason: '$path from $tab',
          );
        }
      }
    });

    test('M9 lies on Me, M8 on M9', () {
      expect(phoneStack(PhoneTab.me, Uri.parse('/hosts')), const [
        PhonePage(PhonePageKind.me, '/settings'),
        PhonePage(PhonePageKind.hosts, '/hosts'),
      ]);
      expect(phoneStack(PhoneTab.me, Uri.parse('/hosts/$hostIdText')), [
        const PhonePage(PhonePageKind.me, '/settings'),
        const PhonePage(PhonePageKind.hosts, '/hosts'),
        PhonePage(PhonePageKind.terminal, '/hosts/$hostIdText'),
      ]);
    });
  });

  testWidgets(
    'M9 lists the host, its badge and the own sessions; a tap attaches in M8',
    (tester) async {
      tester.view
        ..physicalSize = const Size(390, 844)
        ..devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final world = await TerminalWorld.create(tester);
      final sessionId = FakeHostsApi.sessionId(7);
      world.api.listed.add(
        FakeHostsApi.session(
          sessionId,
          state: api.TerminalSessionState.detached,
        ),
      );
      final overview = HostsOverview(
        hosts: [world.api.current],
        ownSessions: {hostIdText: List.of(world.api.listed)},
        connected: const {hostIdText: true},
      );
      final container = ProviderContainer(
        overrides: [
          terminalServicesProvider.overrideWithValue(world.services()),
          terminalSelfUserIdProvider.overrideWithValue(alexId),
          hostsListProvider.overrideWith(() => FixedHosts(overview)),
          appPortsProvider.overrideWithValue(
            testPorts(dataRoot: Directory.systemTemp, platform: 'android'),
          ),
        ],
      );
      addTearDown(container.dispose);
      final visited = <String>[];
      final router = GoRouter(
        initialLocation: '/hosts',
        routes: [
          GoRoute(
            path: '/hosts',
            builder: (_, _) => const Scaffold(body: HostsScreen()),
          ),
          GoRoute(
            path: '/hosts/:id',
            builder: (_, state) {
              visited.add(state.uri.path);
              return const Scaffold(body: Text('M8'));
            },
          ),
        ],
      );
      const ds = DsContext(
        theme: DsThemeMode.light,
        density: DsDensityMode.touch,
        motion: DsMotionMode.reduced,
        accent: dsDefaultAccent,
      );
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp.router(
            routerConfig: router,
            theme: buildDsTheme(ds),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            builder: (context, child) =>
                DsThemeScope(dsContext: ds, child: child!),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.textContaining('Build host'), findsOneWidget);
      expect(find.text(en.hostConnected), findsOneWidget);
      expect(
        find.bySemanticsLabel(
          en.hostSessionRowLabel(
            en.terminalTabLabel('shell', 1),
            en.hostSessionStateDetached,
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.byKey(ValueKey('phone.hosts.new.$hostIdText')),
        findsOneWidget,
      );
      await tester.tap(find.byKey(ValueKey('phone.hosts.session.$sessionId')));
      await tester.pump();
      final tabs = container.read(terminalTabsProvider(hostIdText))!;
      await pumpUntil(
        tester,
        () => tabs.current?.state.phase == TerminalPhase.connected,
      );
      expect(visited, ['/hosts/$hostIdText']);
      expect(tabs.current!.state.sessionId, sessionId);
      await tabs.current!.detach();
      await tester.pump(const Duration(seconds: 1));
    },
  );
}
