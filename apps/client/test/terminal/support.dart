// SPDX-License-Identifier: Apache-2.0

// Test support of the terminal of the app (brief M4-36 Tests): a server API in memory that
// issues grants for the in-process host of `terminal_ui/testing.dart`, pins and host events in
// memory, and the terminal view pumped with these ports over the providers.

import 'dart:async';

import 'package:api_client/api_client.dart'
    show
        ApiProblem,
        Host,
        HostProfile,
        HostProfileKindEnum,
        HostReportedSessionUser,
        HostSessionUser,
        HostState,
        LlmCredentials,
        TerminalSession,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionGrantNoiseProtocolEnum,
        TerminalSessionGrantPrologueVersionEnum,
        TerminalSessionOpen;
import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:auth/auth.dart' show Clock, SystemClock;
import 'package:built_collection/built_collection.dart';
import 'package:client/keyboard/app_shortcuts.dart';
import 'package:client/keyboard/bindings.dart';
import 'package:client/terminal/host_terminal_page.dart';
import 'package:client/terminal/providers.dart';
import 'package:client/terminal/terminal_api_adapter.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

const String hostIdText = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0a';
const String alexId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d0001';
const String samId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d0002';
const String fingerprint =
    '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000';

/// The host "Build host" with the profiles shell and claude, Alex as `term-alex` with the
/// folders "Project A" and "Project B", and Sam as `term-sam` with "Sam only".
Host buildHost(FakeHost host, {bool connected = true, HostState? state}) =>
    Host(
      (b) => b
        ..id = hostIdText
        ..name = 'Build host'
        ..state = state ?? HostState.active
        ..llmCredentials = LlmCredentials.organization
        ..declaredServerHost = false
        ..noisePublicKey = host.noisePublicKey
        ..keyFingerprint = fingerprint
        ..keyGeneration = 1
        ..profiles = ListBuilder<HostProfile>([
          HostProfile(
            (p) => p
              ..name = 'shell'
              ..displayName = 'Shell'
              ..kind = HostProfileKindEnum.shell,
          ),
          HostProfile(
            (p) => p
              ..name = 'claude'
              ..displayName = 'Claude Code'
              ..kind = HostProfileKindEnum.claude,
          ),
        ])
        ..sessionUsers = ListBuilder<HostSessionUser>([
          for (final (user, os) in [(alexId, 'term-alex'), (samId, 'term-sam')])
            HostSessionUser(
              (u) => u
                ..userId = user
                ..osUser = os
                ..createdAt = DateTime.utc(2026, 10),
            ),
        ])
        ..reportedSessionUsers = ListBuilder<HostReportedSessionUser>([
          HostReportedSessionUser(
            (r) => r
              ..osUser = 'term-alex'
              ..workdirs = ListBuilder<String>(['Project A', 'Project B']),
          ),
          HostReportedSessionUser(
            (r) => r
              ..osUser = 'term-sam'
              ..workdirs = ListBuilder<String>(['Sam only']),
          ),
        ])
        ..connected = connected
        ..createdAt = DateTime.utc(2026, 10)
        ..version = 1,
    );

/// The REST side in memory: grants for [host] with a fresh token each.
final class FakeHostsApi implements TerminalHostsApi {
  FakeHostsApi(this.host);

  final FakeHost host;
  final List<TerminalSessionOpen> opens = [];
  final List<String> closes = [];
  final List<TerminalSession> listed = [];
  Object? openError;
  int _tokens = 0;
  int _sessions = 0;

  Host get current => buildHost(host);

  TerminalSessionGrant _grant(String sessionId, String profile) {
    _tokens++;
    final token = 'token-$_tokens';
    host.expectToken(sessionId, token);
    return TerminalSessionGrant(
      (b) => b
        ..session.replace(session(sessionId, profile: profile))
        ..sessionToken = token
        ..sessionTokenExpiresAt = DateTime.utc(2030)
        ..streamPath = '/api/v1/terminal/sessions/$sessionId/stream'
        ..noisePublicKey = host.noisePublicKey
        ..keyFingerprint = fingerprint
        ..keyGeneration = 1
        ..noiseProtocol =
            TerminalSessionGrantNoiseProtocolEnum.noiseNK25519ChaChaPolySHA256
        ..prologueVersion = TerminalSessionGrantPrologueVersionEnum.values
            .firstWhere(
              (v) =>
                  v !=
                  TerminalSessionGrantPrologueVersionEnum.unknownDefaultOpenApi,
            ),
    );
  }

  /// A session of Alex on the host.
  static TerminalSession session(
    String id, {
    String profile = 'shell',
    String userId = alexId,
    api.TerminalSessionState? state,
  }) => TerminalSession(
    (s) => s
      ..id = id
      ..hostId = hostIdText
      ..userId = userId
      ..deviceId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e01'
      ..profile = profile
      ..state = state ?? api.TerminalSessionState.opening
      ..openedAt = DateTime.utc(2026, 10, 20, 9, 30)
      ..bytesIn = 0
      ..bytesOut = 0
      ..version = 1,
  );

  /// The ID of session [n].
  static String sessionId(int n) =>
      '0192f4a0-7b1c-7b22-9d4f-${n.toRadixString(16).padLeft(12, '0')}';

  @override
  Future<Host> getHost(String hostId) async => current;

  @override
  Future<List<Host>> listHosts() async => [current];

  @override
  Future<List<TerminalSession>> listSessions({String? hostId}) async =>
      List.of(listed);

  @override
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  ) async {
    opens.add(request);
    final error = openError;
    if (error != null) throw error;
    return _grant(sessionId(++_sessions), request.profile);
  }

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async => _grant(sessionId, 'shell');

  @override
  Future<TerminalSession> closeSession(String sessionId) async {
    closes.add(sessionId);
    return session(sessionId);
  }
}

/// Pins in memory.
final class MemoryPins implements HostKeyPinStore {
  final Map<String, HostKeyPin> pins = {};

  @override
  Future<HostKeyPin?> read(String hostId) async => pins[hostId];

  @override
  Future<void> write(HostKeyPin pin) async => pins[pin.hostId] = pin;
}

/// Host events the test sends.
final class ManualHostEvents implements HostEvents {
  final StreamController<HostChange> changes = StreamController.broadcast();

  @override
  Stream<HostChange> watch(String hostId) => changes.stream;
}

/// A refused opening with the server's reason.
ApiProblem denied(String reason) =>
    ApiProblem(status: 403, code: 'terminal_denied', reason: reason);

/// The ports of the terminal over the fakes, and the page pumped with them.
final class TerminalWorld {
  TerminalWorld._(this.host, this.api);

  static Future<TerminalWorld> create(WidgetTester tester) async {
    final host = (await tester.runAsync(
      () => FakeHost.create(hostId: hostIdText),
    ))!;
    return TerminalWorld._(host, FakeHostsApi(host));
  }

  final FakeHost host;
  final FakeHostsApi api;
  final MemoryPins pins = MemoryPins();
  final ManualHostEvents events = ManualHostEvents();
  final GlobalKey<NavigatorState> navigatorKey = GlobalKey();
  final List<String> stepUps = [];
  late final FakeTerminalTransport transport = FakeTerminalTransport(host);

  TerminalServices services({Clock clock = const SystemClock()}) =>
      TerminalServices(
        api: api,
        pins: pins,
        events: events,
        engineEvents: const Stream.empty(),
        stepUp: ({required purposeText}) async => stepUps.add(purposeText),
        newTransport: () => transport,
        links: SystemLinkOpener(launch: (_) async => true),
        clock: clock,
        config: AppConfig.defaults(),
        context: () => navigatorKey.currentContext,
      );

  /// Pumps the view of the host in a desktop window with the keyboard layer of the shell.
  Future<ProviderContainer> pump(
    WidgetTester tester, {
    ShortcutPlatform platform = ShortcutPlatform.windows,
  }) async {
    tester.view
      ..physicalSize = const Size(1440, 1000)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final overview = HostsOverview(
      hosts: [api.current],
      ownSessions: const {},
      connected: const {hostIdText: true},
    );
    final container = ProviderContainer(
      overrides: [
        terminalServicesProvider.overrideWithValue(services()),
        terminalSelfUserIdProvider.overrideWithValue(alexId),
        hostsListProvider.overrideWith(() => FixedHosts(overview)),
      ],
    );
    addTearDown(container.dispose);
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.comfortable,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          navigatorKey: navigatorKey,
          theme: buildDsTheme(ds),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          builder: (context, child) =>
              DsThemeScope(dsContext: ds, child: child!),
          home: Scaffold(
            body: AppShortcuts(
              platform: platform,
              child: HostTerminalPage(hostId: hostIdText, platform: platform),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    return container;
  }
}

/// The hosts list of a test, fixed.
final class FixedHosts extends HostsList {
  FixedHosts(this.overview);

  final HostsOverview overview;

  @override
  Future<HostsOverview?> build() async => overview;
}

/// Pumps frames until [condition] holds; the crypto of the controller runs between pumps.
Future<void> pumpUntil(
  WidgetTester tester,
  bool Function() condition, {
  int maxPumps = 2000,
}) async {
  for (var i = 0; i < maxPumps; i++) {
    if (condition()) return;
    await tester.runAsync(() => Future<void>.delayed(Duration.zero));
    await tester.pump(const Duration(milliseconds: 1));
  }
  throw StateError('condition never held');
}

/// Opens a shell through "New session", the dialog and the step-up, until it is connected.
Future<TerminalTabs> openShell(WidgetTester tester, ProviderContainer c) async {
  await tester.tap(find.byKey(const ValueKey('terminal.new')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const ValueKey('terminal.new.shell')).last);
  await tester.pumpAndSettle();
  await tester.tap(find.text(en.terminalOpenConfirm));
  await tester.pump();
  final tabs = c.read(terminalTabsProvider(hostIdText))!;
  await pumpUntil(
    tester,
    () => tabs.current?.state.phase == TerminalPhase.connected,
  );
  return tabs;
}

/// The English texts.
final AppLocalizations en = lookupAppLocalizations(const Locale('en'));
