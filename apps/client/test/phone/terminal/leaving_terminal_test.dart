// SPDX-License-Identifier: Apache-2.0

// The coordinator attaches a terminal session of the phone again only while its screen M8
// shows, the app is in front and the app lock is open (01 5.3 M8, 5.4, 5.6 E6, R-PR-12;
// D-M5-12-4, D-M5-12-5; docs/decisions.d/AUD-client-phone-01.md): a push or a link that
// replaces M8 ends the reattaching of the coordinator, an opening or an attach on its way is
// stopped when M8 goes, and one that lands while the app is in the background or behind the
// lock is detached and attaches on the return. E6 replaces M8 too, but that is no leave: with
// the lock open and M8 back the session attaches. A session that turns live after M8 was left
// (the reconnector of D-TSL-5 attaches one whose stream dropped) still detaches in the
// background and at the lock, and nothing attaches it again.

import 'dart:async';

import 'package:api_client/api_client.dart'
    show
        ApiProblem,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionOpen;
import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;
import 'package:terminal_ui/terminal_ui.dart';

import '../../terminal/support.dart';
import 'support.dart';

/// A hosts API whose attach and opening wait for the gate the test holds, like a request on a
/// slow network; [attachErrors] are thrown by the next attaches in order once their gate opened.
final class _Gated extends FakeHostsApi {
  _Gated(super.host);

  Completer<void>? attachGate;
  Completer<void>? openGate;
  final List<Object> attachErrors = [];
  int attaches = 0;
  int openings = 0;

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async {
    attaches++;
    await attachGate?.future;
    if (attachErrors.isNotEmpty) throw attachErrors.removeAt(0);
    return super.attachSession(sessionId, request);
  }

  @override
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  ) async {
    openings++;
    await openGate?.future;
    return super.openSession(hostId, request);
  }
}

Future<(PhoneTerminalHarness, _Gated)> _pump(WidgetTester tester) async {
  final world = await TerminalWorld.create(tester, api: _Gated.new);
  final h = await PhoneTerminalHarness.pump(tester, world: world);
  return (h, world.api as _Gated);
}

/// Takes M8 off the stack without a pop, as a tapped push, a link or E6 replaces the page. The
/// navigator removes it once the routes above it (a dialog that still closes) are gone.
Future<void> _replaceM8(WidgetTester tester, PhoneTerminalHarness h) async {
  h.world.navigatorKey.currentState!.removeRoute(
    ModalRoute.of(tester.element(find.byType(TerminalSessionScreen)))!,
  );
  await pumpUntil(
    tester,
    () => find.byType(TerminalSessionScreen).evaluate().isEmpty,
  );
}

/// Shows M8 of the host again.
Future<void> _showM8(WidgetTester tester, PhoneTerminalHarness h) async {
  unawaited(
    h.world.navigatorKey.currentState!.push(
      MaterialPageRoute<void>(
        builder: (_) =>
            const Scaffold(body: TerminalSessionScreen(hostId: hostIdText)),
      ),
    ),
  );
  await tester.pump();
}

/// Lets the crypto of a controller and the frames run for a while without a condition.
Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 40; i++) {
    await tester.runAsync(() => Future<void>.delayed(Duration.zero));
    await tester.pump(const Duration(milliseconds: 1));
  }
}

bool _onItsWay(TerminalController c) =>
    c.state.phase == TerminalPhase.preparing ||
    c.state.phase == TerminalPhase.connecting;

/// Starts a new shell through "New session" and the dialog; the opening then waits where the
/// API holds it.
Future<void> _startShell(WidgetTester tester) async {
  await tester.tap(find.byKey(const ValueKey('phone.terminal.new')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const ValueKey('phone.terminal.new.shell')).last);
  await tester.pumpAndSettle();
  await tester.tap(find.text(en.terminalOpenConfirm));
  await tester.pump();
}

/// M8 with a connected shell whose stream drops while the engine is offline, then left by
/// system back. The last value is the engine's connection: `true` lets the reconnector of
/// D-TSL-5 attach the session again, without its view (AUD-CP1/7 (a)).
Future<
  (PhoneTerminalHarness, _Gated, TerminalController, StreamController<bool>)
>
_lostAndLeft(WidgetTester tester) async {
  final world = await TerminalWorld.create(tester, api: _Gated.new);
  final online = StreamController<bool>();
  addTearDown(online.close);
  final h = await PhoneTerminalHarness.pump(
    tester,
    world: world,
    services: (w) => w.services(online: online.stream),
  );
  final c = await h.openShell(tester);
  online.add(false);
  await world.host.close(1006);
  await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
  expect(c.state.detachCause, DetachCause.connectionLost);

  await tester.binding.handlePopRoute();
  await tester.pumpAndSettle();
  expect(find.byType(TerminalSessionScreen), findsNothing);
  expect(find.byKey(const ValueKey('below')), findsOneWidget);
  return (h, world.api as _Gated, c, online);
}

void main() {
  group('M8 replaced by a push or a link', () {
    testWidgets('while the app was away: the return attaches nothing', (
      tester,
    ) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      h.lifecycle.set(AppActivity.background);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);

      await _replaceM8(tester, h);
      h.lifecycle.set(AppActivity.foreground);
      await _settle(tester);

      expect(c.state.phase, TerminalPhase.detached);
      expect(api.attaches, 0);
      expect(h.world.host.deviceCloseCodes, [1000]);
      expect(h.coordinator.detached, isEmpty);

      // Another return finds nothing to attach either.
      h.lifecycle.set(AppActivity.background);
      h.lifecycle.set(AppActivity.foreground);
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(api.attaches, 0);
      await h.close(tester);
    });

    testWidgets('right after the return: the attach on its way is stopped', (
      tester,
    ) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      h.lifecycle.set(AppActivity.background);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);

      // The app is in front first, as the systems deliver a tapped push after it.
      final gate = api.attachGate = Completer<void>();
      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => api.attaches == 1);
      expect(_onItsWay(c), isTrue);

      await _replaceM8(tester, h);
      expect(c.state.phase, TerminalPhase.detached);

      gate.complete();
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(c.state.detachCause, DetachCause.person);
      expect(h.world.host.openStreams, 0);
      expect(h.world.host.deviceCloseCodes, [1000]);
      await h.close(tester);
    });

    testWidgets('after the quick reattach window: no step-up is asked from '
        'nowhere', (tester) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      h.lifecycle.set(AppActivity.background);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);
      final stepUps = h.world.stepUps.length;

      final gate = api.attachGate = Completer<void>();
      api.attachErrors.add(
        const ApiProblem(status: 401, code: 'step_up_required'),
      );
      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => api.attaches == 1);
      await _replaceM8(tester, h);
      gate.complete();
      await _settle(tester);

      expect(h.world.stepUps, hasLength(stepUps));
      expect(api.attaches, 1);
      expect(c.state.phase, TerminalPhase.detached);
      await h.close(tester);
    });
  });

  testWidgets('E6 replaces M8: with the lock open the session attaches once M8 '
      'shows again, not before', (tester) async {
    final (h, api) = await _pump(tester);
    final c = await h.openShell(tester);
    h.gate.lock();
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
    await pumpUntil(tester, () => h.world.host.openStreams == 0);
    // The router shows E6 instead of the shell: M8 leaves the tree while the lock is closed.
    await _replaceM8(tester, h);

    h.gate.unlock();
    await _settle(tester);
    expect(c.state.phase, TerminalPhase.detached);
    expect(api.attaches, 0);

    await _showM8(tester, h);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
    expect(api.attaches, 1);
    await h.close(tester);
  });

  group('a flow that lands while the terminal must be detached', () {
    testWidgets('an attach that completes in the background is detached and '
        'attaches on the return', (tester) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      h.lifecycle.set(AppActivity.background);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);

      final gate = api.attachGate = Completer<void>();
      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => api.attaches == 1);
      h.lifecycle.set(AppActivity.background);
      expect(_onItsWay(c), isTrue);

      gate.complete();
      await pumpUntil(tester, () => !_onItsWay(c));
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(h.world.host.openStreams, 0);
      expect(h.coordinator.detached, contains(c));

      api.attachGate = null;
      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      await h.close(tester);
    });

    testWidgets('an opening that completes in the background is detached and '
        'attaches on the return', (tester) async {
      final (h, api) = await _pump(tester);
      final gate = api.openGate = Completer<void>();
      await _startShell(tester);
      await pumpUntil(tester, () => api.openings == 1);
      final c = h.tabs.current!;
      expect(_onItsWay(c), isTrue);

      h.lifecycle.set(AppActivity.background);
      gate.complete();
      await pumpUntil(tester, () => !_onItsWay(c));
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(h.world.host.openStreams, 0);
      expect(h.world.api.closes, isEmpty, reason: 'the session keeps running');

      h.lifecycle.set(AppActivity.foreground);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      await h.close(tester);
    });

    testWidgets('an attach that completes while the lock still decides is '
        'detached and attaches after it', (tester) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      await c.detach();
      await pumpUntil(tester, () => h.world.host.openStreams == 0);

      // The person taps "Reattach"; the lock starts deciding before the server answered.
      final gate = api.attachGate = Completer<void>();
      unawaited(c.attach());
      await pumpUntil(tester, () => api.attaches == 1);
      h.gate.deciding();
      gate.complete();
      await pumpUntil(tester, () => !_onItsWay(c));
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);

      api.attachGate = null;
      h.gate.decided();
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      await h.close(tester);
    });
  });

  group('leaving M8 with a flow on its way', () {
    testWidgets('system back stops an attach: no stream opens afterwards', (
      tester,
    ) async {
      final (h, api) = await _pump(tester);
      final c = await h.openShell(tester);
      await c.detach();
      await pumpUntil(tester, () => h.world.host.openStreams == 0);

      final gate = api.attachGate = Completer<void>();
      unawaited(c.attach());
      await pumpUntil(tester, () => api.attaches == 1);
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('below')), findsOneWidget);
      expect(c.state.phase, TerminalPhase.detached);

      gate.complete();
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(h.world.host.openStreams, 0);
      await h.close(tester);
    });

    testWidgets('system back stops an opening: the session the server made is '
        'closed and no stream opens', (tester) async {
      final (h, api) = await _pump(tester);
      final gate = api.openGate = Completer<void>();
      await _startShell(tester);
      await pumpUntil(tester, () => api.openings == 1);
      final c = h.tabs.current!;
      expect(_onItsWay(c), isTrue);

      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('below')), findsOneWidget);
      expect(c.state.phase, TerminalPhase.idle);

      gate.complete();
      await pumpUntil(tester, () => h.world.api.closes.isNotEmpty);
      await _settle(tester);
      expect(h.world.api.closes, [FakeHostsApi.sessionId(1)]);
      expect(c.state.phase, TerminalPhase.idle);
      expect(h.world.host.connections, 0);
      await h.close(tester);
    });
  });

  group('a session that turns live after M8 was left', () {
    testWidgets('the background detaches it, and the return attaches nothing', (
      tester,
    ) async {
      final (h, api, c, online) = await _lostAndLeft(tester);
      online.add(true);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      expect(h.world.host.openStreams, 1);

      h.lifecycle.set(AppActivity.background);
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached, reason: 'R-PR-12');
      expect(c.state.detachCause, DetachCause.person);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);
      expect(h.world.host.deviceCloseCodes, [1000]);
      expect(h.world.api.closes, isEmpty, reason: 'the session keeps running');
      expect(h.coordinator.detached, isEmpty);

      h.lifecycle.set(AppActivity.foreground);
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(api.attaches, 1);
      await h.close(tester);
    });

    testWidgets('the app lock detaches it, and the unlock attaches nothing', (
      tester,
    ) async {
      final (h, api, c, online) = await _lostAndLeft(tester);
      online.add(true);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
      expect(h.world.host.openStreams, 1);

      h.gate.lock();
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached, reason: '01 5.6 E6');
      expect(c.state.detachCause, DetachCause.person);
      await pumpUntil(tester, () => h.world.host.openStreams == 0);
      expect(h.world.host.deviceCloseCodes, [1000]);
      expect(h.coordinator.detached, isEmpty);

      h.gate.unlock();
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(api.attaches, 1);
      await h.close(tester);
    });

    testWidgets('an attach of it that completes in the background is detached '
        'at once and stays detached', (tester) async {
      final (h, api, c, online) = await _lostAndLeft(tester);
      final gate = api.attachGate = Completer<void>();
      online.add(true);
      await pumpUntil(tester, () => api.attaches == 1);
      expect(_onItsWay(c), isTrue);

      h.lifecycle.set(AppActivity.background);
      gate.complete();
      await pumpUntil(tester, () => !_onItsWay(c));
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached, reason: 'R-PR-12');
      expect(c.state.detachCause, DetachCause.person);
      expect(h.world.host.openStreams, 0);
      expect(h.coordinator.detached, isEmpty);

      api.attachGate = null;
      h.lifecycle.set(AppActivity.foreground);
      await _settle(tester);
      expect(c.state.phase, TerminalPhase.detached);
      expect(api.attaches, 1);
      await h.close(tester);
    });
  });
}
