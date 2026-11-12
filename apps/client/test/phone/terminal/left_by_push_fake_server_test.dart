// SPDX-License-Identifier: Apache-2.0

// A terminal session of the phone after its screen M8 was left by a tapped push
// (docs/decisions.d/AUD-client-phone-01.md; 01 5.3 M8, 5.6 E6; D-M5-12-4, D-M5-12-5). The phone
// app runs as it ships, with its router, the phone shell, the coordinator and the app lock of
// its providers, against the Dart fake server on 127.0.0.1: the REST client with its generated
// decoders opens and attaches the session, the stream identifies with DPoP over a WebSocket, and
// the Noise handshake runs against an in-process host behind the server's stream endpoint.
//
// The session detaches in the background. A push tapped then opens its thread, which replaces
// M8: the session stays detached and the server sees no attach, whether the push arrives right
// after the app is in front (the order of iOS and Android) or before it. E6 replaces M8 too, but
// after the unlock M8 is back and the session attaches. A session whose stream dropped is
// attached again by the reconnector of D-TSL-5 also after M8 was left: it detaches in the
// background like every other, and no return attaches it again.

import 'dart:async';
import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/providers.dart';
import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:client/platform/push_platform.dart';
import 'package:client/terminal/providers.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart' as ui;

import '../../../integration_test/support/terminal_host.dart';
import '../../lock/support/lock_fakes.dart';
import '../../support/fake_instance.dart';
import '../../support/fakes.dart';
import '../../support/until.dart';
import '../support/phone_app.dart' show phoneWindow;

/// The push platform of the build as far as the app widget needs it: the taps the test sends.
final class _Push implements PushPlatform {
  final StreamController<String> _targets = StreamController<String>.broadcast(
    sync: true,
  );

  /// The person taps a push of the thread at [path].
  void tap(String path) => _targets.add(path);

  @override
  Stream<String> get targets => _targets.stream;

  @override
  Future<bool> isAvailable() async => true;

  @override
  Future<void> listen() async {}

  @override
  Future<void> signedIn(
    PushAccount account, {
    required String previewMode,
  }) async {}

  @override
  Future<void> threadOpened() async {}

  @override
  Future<void> previewModeChanged(String previewMode) async {}

  @override
  Future<void> logout() async {}

  @override
  Future<void> forget(String instanceId) async {}
}

/// Lets real IO run and advances the fake clock of the test zone a little each round, until
/// [condition] holds; bounded by the patience of the test kits.
Future<void> _until(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final watch = Stopwatch()..start();
  while (!condition()) {
    if (watch.elapsed >= testPatience) {
      fail('$what did not happen within $testPatience');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

/// Lets real IO and the app run for about [rounds] rounds without a condition, long enough for
/// a request, a stream and a handshake that were on their way to arrive.
Future<void> _settle(WidgetTester tester, {int rounds = 150}) async {
  for (var i = 0; i < rounds; i++) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

/// Taps [target] once it shows and the page transition that brought it has ended.
Future<void> _tap(WidgetTester tester, String what, Finder target) async {
  await _until(tester, what, () => target.evaluate().isNotEmpty);
  await tester.pump(const Duration(milliseconds: 500));
  await tester.tap(target.last);
  await tester.pump();
}

void main() {
  HttpOverrides? saved;
  late FakeInstance fake;
  late SwitchLifecycle lifecycle;
  late _Push push;
  late HostBridge bridge;
  late String hostId;
  ProviderContainer? container;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    lifecycle = SwitchLifecycle();
    push = _Push();
    fake = await FakeInstance.start(
      platform: 'android',
      lifecycle: lifecycle,
      push: push,
    );
    final world = fake.server.world;
    // The in-process host and the fake's host share the ID: it is in the Noise prologue.
    final host = await ui.FakeHost.create(hostId: world.nextId());
    bridge = HostBridge(host);
    world.terminal.hostLink = bridge;
    hostId = world
        .addTerminalHost(
          id: host.hostId,
          name: 'Build host',
          noisePublicKey: host.noisePublicKey,
          sessionUsers: {fake.seed.alex.id: 'term-alex'},
          grantees: [fake.seed.alex.id],
        )
        .id;
    container = null;
  });

  tearDown(() async {
    container?.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  /// Ends the app, and [lock] when the test made it, inside the test zone, where their
  /// periodic timers live.
  Future<void> end(WidgetTester tester, {AppLockController? lock}) async {
    await tester.pumpWidget(const SizedBox());
    lock?.dispose();
    container?.dispose();
    container = null;
    await tester.pump(const Duration(minutes: 5));
  }

  AppLocalizations l(WidgetTester tester) =>
      AppLocalizations.of(tester.element(find.byType(Navigator).first));

  String location(WidgetTester tester) =>
      GoRouter.of(tester.element(find.byKey(const ValueKey('phone.shell'))))
          .routerDelegate
          .currentConfiguration
          .uri
          .toString();

  /// The session of [tab] as the server keeps it.
  FakeTerminalSession onServer(TerminalController tab) =>
      fake.server.world.terminal.sessions[tab.state.sessionId]!;

  /// Signs Alex in, starts the phone app (its providers with [overrides]) and opens a shell on
  /// the host from M9 with the step-up; returns its tab, connected in M8.
  Future<TerminalController> shellInM8(
    WidgetTester tester, {
    List<Override> overrides = const [],
  }) async {
    phoneWindow(tester);
    final c = container = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(fake.ports), ...overrides],
    );
    await tester.runAsync(() async {
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      );
    });
    await tester.pumpWidget(
      UncontrolledProviderScope(container: c, child: const ClientApp()),
    );
    await _tap(
      tester,
      'the tab Me',
      find.byKey(const ValueKey('phone.tab.me')),
    );
    await _tap(
      tester,
      'the entry Hosts',
      find.byKey(const ValueKey('phone.me.hosts')),
    );
    await _tap(
      tester,
      '"New session" of the host',
      find.byKey(ValueKey('phone.hosts.new.$hostId')),
    );
    await _tap(
      tester,
      'the profile Shell',
      find.byKey(ValueKey('phone.hosts.new.$hostId.shell')),
    );
    await _until(
      tester,
      'the dialog "Open terminal"',
      () => find.text(l(tester).terminalOpenConfirm).evaluate().isNotEmpty,
    );
    await _tap(tester, '"Open"', find.text(l(tester).terminalOpenConfirm));
    await _tap(
      tester,
      'the step-up',
      find.byKey(const ValueKey('stepUp.continue')),
    );
    final tabs = c.read(terminalTabsProvider(hostId))!;
    await _until(
      tester,
      'the shell connected',
      () => tabs.current?.state.phase == TerminalPhase.connected,
    );
    await _until(tester, 'the handshake', () => bridge.host.handshakeDone);
    expect(find.byType(TerminalSessionScreen), findsOneWidget);
    expect(location(tester), '/hosts/$hostId');
    return tabs.current!;
  }

  /// The app moves to the background on M8: the session detaches here and on the server.
  Future<void> background(WidgetTester tester, TerminalController tab) async {
    lifecycle.set(AppActivity.background);
    await _until(
      tester,
      'detached in the app',
      () => tab.state.phase == TerminalPhase.detached,
    );
    await _until(
      tester,
      'detached on the server',
      () => onServer(tab).state == 'detached',
    );
  }

  /// Nothing reached the server or the host for [tab] since its opening, and it is detached.
  void expectLeftDetached(TerminalController tab) {
    final server = fake.server;
    expect(tab.state.phase, TerminalPhase.detached);
    expect(tab.state.detachCause, DetachCause.person);
    expect(server.requestsOf('attachTerminalSession'), isEmpty);
    expect(
      server.requestsOf('createStepUpChallenge'),
      hasLength(1),
      reason: 'only the step-up of the opening',
    );
    expect(
      bridge.pipes,
      hasLength(1),
      reason: 'only the stream of the opening',
    );
    expect(onServer(tab).state, 'detached');
    expect(server.violations, isEmpty);
  }

  testWidgets('a push tapped right after the return replaces M8: the attach on '
      'its way stops and the session stays detached', (tester) async {
    final tab = await shellInM8(tester);
    final thread = '/t/${fake.seed.releaseNotes}';
    await background(tester, tab);

    // The app is in front first; the coordinator starts to attach and waits for the server.
    lifecycle.set(AppActivity.foreground);
    await tester.pump();
    expect(
      tab.state.phase,
      anyOf(TerminalPhase.preparing, TerminalPhase.connecting),
    );
    // Then the system delivers the tap, and the thread replaces M8.
    push.tap(thread);
    await tester.pump();
    await tester.pump();
    expect(find.byType(TerminalSessionScreen), findsNothing);
    expect(location(tester), thread);
    expect(tab.state.phase, TerminalPhase.detached);

    await _settle(tester);
    expectLeftDetached(tab);

    // Later returns to the app attach nothing either.
    lifecycle.set(AppActivity.background);
    await tester.pump();
    lifecycle.set(AppActivity.foreground);
    await _settle(tester);
    expectLeftDetached(tab);
    await end(tester);
  });

  testWidgets('a push tapped before the app is in front replaces M8: the '
      'return attaches nothing', (tester) async {
    final tab = await shellInM8(tester);
    final thread = '/t/${fake.seed.releaseNotes}';
    await background(tester, tab);

    push.tap(thread);
    await tester.pump();
    await tester.pump();
    expect(find.byType(TerminalSessionScreen), findsNothing);
    expect(location(tester), thread);
    lifecycle.set(AppActivity.foreground);

    await _settle(tester);
    expectLeftDetached(tab);
    await end(tester);
  });

  testWidgets('a session the reconnector attaches after M8 was left detaches '
      'in the background and stays detached', (tester) async {
    final tab = await shellInM8(tester);
    final thread = '/t/${fake.seed.releaseNotes}';
    final server = fake.server;

    // The server ends the stream while M8 shows: a cause the reconnector attaches again.
    await tester.runAsync(server.terminalStreams.close);
    await _until(
      tester,
      'the stream dropped',
      () => tab.state.phase == TerminalPhase.detached,
    );
    expect(tab.state.detachCause, DetachCause.serverRestart);
    await _until(
      tester,
      'detached on the server',
      () => onServer(tab).state == 'detached',
    );

    // A tapped push replaces M8.
    push.tap(thread);
    await tester.pump();
    await tester.pump();
    expect(find.byType(TerminalSessionScreen), findsNothing);
    expect(location(tester), thread);

    // The app leaves and returns: the reconnector attaches the session, without its view.
    lifecycle.set(AppActivity.background);
    await tester.pump();
    lifecycle.set(AppActivity.foreground);
    await _until(
      tester,
      'the session attached again',
      () => tab.state.phase == TerminalPhase.connected,
    );
    await _until(
      tester,
      'active on the server',
      () => onServer(tab).state == 'active',
    );
    expect(find.byType(TerminalSessionScreen), findsNothing);
    expect(server.requestsOf('attachTerminalSession'), hasLength(1));

    // R-PR-12: the background detaches it, here and on the server.
    await background(tester, tab);
    expect(tab.state.detachCause, DetachCause.person);

    // No return attaches it again: its screen does not show.
    lifecycle.set(AppActivity.foreground);
    await _settle(tester);
    expect(tab.state.phase, TerminalPhase.detached);
    expect(server.requestsOf('attachTerminalSession'), hasLength(1));
    expect(onServer(tab).state, 'detached');
    expect(location(tester), thread);
    expect(server.violations, isEmpty);
    await end(tester);
  });

  testWidgets('E6 replaces M8 while the lock is closed: after the unlock M8 is '
      'back and the session attaches', (tester) async {
    // The app lock of the app, turned on, over clocks the test moves.
    final clock = TestClock(lockStart);
    final monotonic = FakeMonotonic();
    final lock = AppLockController(
      port: FakeAppLockPort(),
      clock: clock,
      lifecycle: lifecycle,
      store: MemorySwitch(true),
      policy: Stream.value(policy()),
      lockAtStart: false,
      monotonic: monotonic,
    );
    final tab = await shellInM8(
      tester,
      overrides: [appLockProvider.overrideWithValue(lock)],
    );
    await _until(tester, 'the lock applies', () => lock.applies);

    // Longer than `client.app_lock_after` in the background: the return closes the lock.
    await background(tester, tab);
    clock.start = clock.start.add(const Duration(minutes: 10));
    monotonic.elapsed += const Duration(minutes: 10);
    lifecycle.set(AppActivity.foreground);
    await _until(
      tester,
      'E6',
      () => find.byKey(const ValueKey('screen.e6')).evaluate().isNotEmpty,
    );
    await _settle(tester, rounds: 40);
    expect(find.byType(TerminalSessionScreen), findsNothing);
    expect(tab.state.phase, TerminalPhase.detached);
    expect(fake.server.requestsOf('attachTerminalSession'), isEmpty);

    await tester.tap(find.byKey(const ValueKey('e6.unlock')));
    await _until(
      tester,
      'M8 again',
      () => find.byType(TerminalSessionScreen).evaluate().isNotEmpty,
    );
    await _until(
      tester,
      'the session attached again',
      () => tab.state.phase == TerminalPhase.connected,
    );
    expect(fake.server.requestsOf('attachTerminalSession'), hasLength(1));
    expect(onServer(tab).state, 'active');
    expect(fake.server.violations, isEmpty);
    await end(tester, lock: lock);
  });
}
