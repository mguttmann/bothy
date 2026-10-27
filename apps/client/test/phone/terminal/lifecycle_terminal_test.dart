// SPDX-License-Identifier: Apache-2.0

// Detaching and reattaching the phone terminal (01 5.3 M8, 5.4, 5.6 E6, R-PR-12; 15 23.2 items
// 6 and 7; 24 6.2; brief M5-12 Design note 5): the background and the app lock detach the
// session (the stream closes with 1000, the session keeps running), the return to the
// foreground waits for the lock and then attaches, a 401 step_up_required runs the step-up once
// and repeats, and system back in M8 detaches and goes back to the page below.

import 'dart:async';

import 'package:api_client/api_client.dart'
    show ApiProblem, TerminalSessionAttach, TerminalSessionGrant;

import 'package:client/phone/terminal/terminal_session_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity;
import 'package:terminal_ui/terminal_ui.dart';

import '../../terminal/support.dart';
import 'support.dart';

/// Answers the first attach with 401 step_up_required, like a server after the quick
/// reattach window (24 6.2).
final class _StepUpOnce extends FakeHostsApi {
  _StepUpOnce(super.host);

  int attaches = 0;

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async {
    attaches++;
    if (attaches == 1) {
      throw const ApiProblem(status: 401, code: 'step_up_required');
    }
    return super.attachSession(sessionId, request);
  }
}

void main() {
  testWidgets('paused detaches with 1000; resumed attaches again', (
    tester,
  ) async {
    final h = await PhoneTerminalHarness.pump(tester);
    final c = await h.openShell(tester);
    final stepUps = h.world.stepUps.length;
    h.lifecycle.set(AppActivity.background);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
    await pumpUntil(tester, () => h.world.host.openStreams == 0);
    expect(h.world.host.deviceCloseCodes, [1000]);
    expect(h.world.api.closes, isEmpty, reason: 'the session keeps running');
    h.lifecycle.set(AppActivity.foreground);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
    // Within the window the server asks for no step-up; the app asks none of its own.
    expect(h.world.stepUps.length, stepUps);
    await h.close(tester);
  });

  testWidgets('locked detaches; resumed waits for unlocked, then attaches', (
    tester,
  ) async {
    final h = await PhoneTerminalHarness.pump(tester);
    final c = await h.openShell(tester);
    h.gate.lock();
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
    expect(h.world.host.deviceCloseCodes, [1000]);
    // A move to the background and back while E6 shows attaches nothing.
    h.lifecycle.set(AppActivity.background);
    h.lifecycle.set(AppActivity.foreground);
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 1));
    }
    expect(c.state.phase, TerminalPhase.detached);
    h.gate.unlock();
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
    await h.close(tester);
  });

  testWidgets('a return while the lock decides attaches only after it', (
    tester,
  ) async {
    final h = await PhoneTerminalHarness.pump(tester);
    final c = await h.openShell(tester);
    h.lifecycle.set(AppActivity.background);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
    h.gate.deciding();
    h.lifecycle.set(AppActivity.foreground);
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 1));
    }
    expect(c.state.phase, TerminalPhase.detached);
    h.gate.decided();
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
    await h.close(tester);
  });

  testWidgets('401 step_up_required runs the step-up once and repeats', (
    tester,
  ) async {
    final world = await TerminalWorld.create(tester, api: _StepUpOnce.new);
    final h = await PhoneTerminalHarness.pump(tester, world: world);
    final c = await h.openShell(tester);
    final stepUps = world.stepUps.length;
    h.lifecycle.set(AppActivity.background);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
    h.lifecycle.set(AppActivity.foreground);
    await pumpUntil(tester, () => c.state.phase == TerminalPhase.connected);
    expect(world.stepUps.length, stepUps + 1);
    expect((world.api as _StepUpOnce).attaches, 2);
    await h.close(tester);
  });

  testWidgets('a session the person detached stays detached on a return', (
    tester,
  ) async {
    final h = await PhoneTerminalHarness.pump(tester);
    final c = await h.openShell(tester);
    await c.detach();
    h.lifecycle.set(AppActivity.background);
    h.lifecycle.set(AppActivity.foreground);
    for (var i = 0; i < 20; i++) {
      await tester.pump(const Duration(milliseconds: 1));
    }
    expect(c.state.phase, TerminalPhase.detached);
    expect(h.coordinator.detached, isEmpty);
    await h.close(tester);
  });

  testWidgets(
    'Android system back in M8 detaches and goes back to the page below',
    (tester) async {
      final h = await PhoneTerminalHarness.pump(tester);
      final c = await h.openShell(tester);
      expect(h.guard.secure, [true]);
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('below')), findsOneWidget);
      await pumpUntil(tester, () => c.state.phase == TerminalPhase.detached);
      expect(h.world.host.deviceCloseCodes, [1000]);
      expect(h.world.api.closes, isEmpty);
      // Leaving M8 gives the window back to the recent apps rule (D-M5-12-3).
      expect(h.guard.secure, [true, false]);
      await h.close(tester);
    },
  );

  testWidgets(
    'a second M8 over the first keeps the window secure until the last one leaves',
    (tester) async {
      final h = await PhoneTerminalHarness.pump(tester);
      expect(h.guard.secure, [true]);
      // A link to another host's M8 puts a second M8 on the stack before the first one goes.
      unawaited(
        h.world.navigatorKey.currentState!.push(
          MaterialPageRoute<void>(
            builder: (_) =>
                const Scaffold(body: TerminalSessionScreen(hostId: hostIdText)),
          ),
        ),
      );
      await tester.pumpAndSettle();
      h.world.navigatorKey.currentState!.removeRoute(
        ModalRoute.of(
          tester.element(
            find.byType(TerminalSessionScreen, skipOffstage: false).first,
          ),
        )!,
      );
      await tester.pumpAndSettle();
      expect(h.guard.secure.last, isTrue, reason: '${h.guard.secure}');
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(h.guard.secure.last, isFalse);
    },
  );
}
