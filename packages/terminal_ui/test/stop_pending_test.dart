// SPDX-License-Identifier: Apache-2.0

// Stopping an opening or an attach that is on its way (docs/decisions.d/AUD-client-phone-01.md,
// ruling AUD-CP1/3; D-M4-35-17): the phone leaves the terminal screen while a flow still waits
// for the server, a prompt or the handshake. Nothing of that flow may follow: no request, no
// step-up, no stream. A tab with a session is detached as "Detach" leaves it, a tab without one
// is idle again, and the next attach starts at once, also while the stopped one still waits for
// its answer.

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import 'support/fakes.dart';

/// A harness whose session is detached by the person, as after "Detach".
Future<Harness> detached() async {
  final h = await Harness.create();
  await h.open();
  await h.connected();
  await h.controller.detach();
  await h.until((s) => s.phase == TerminalPhase.detached);
  return h;
}

void main() {
  test('an attach stopped while the server answers stays detached and opens '
      'no stream', () async {
    final h = await detached();
    final gate = h.api.attachGate = Completer<void>();
    final attaching = h.controller.attach();
    await h.until((s) => s.phase == TerminalPhase.connecting);
    await h.settle();
    expect(h.log.last, 'attach');
    final streams = h.host.deviceCloseCodes.length;

    await h.controller.stopPending();
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.controller.state.detachCause, DetachCause.person);
    expect(h.controller.state.detachedAt, isNotNull);

    gate.complete();
    await attaching;
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.host.openStreams, 0);
    expect(h.host.deviceCloseCodes, hasLength(streams));
  });

  test('an attach stopped before the server asks for a step-up runs no '
      'step-up and no second request', () async {
    final h = await detached();
    final gate = h.api.attachGate = Completer<void>();
    h.api.attachErrors.add(stepUpRequired);
    final attaching = h.controller.attach();
    await h.until((s) => s.phase == TerminalPhase.connecting);
    await h.settle();
    final stepUps = h.stepUps.length;

    await h.controller.stopPending();
    gate.complete();
    await attaching;
    await h.settle();
    expect(h.stepUps, hasLength(stepUps));
    expect(h.log.where((e) => e == 'attach'), hasLength(1));
    expect(h.controller.state.phase, TerminalPhase.detached);
  });

  test('the next attach starts at once, also while the stopped one still '
      'waits for its answer', () async {
    final h = await detached();
    final gate = h.api.attachGate = Completer<void>();
    final stopped = h.controller.attach();
    await h.until((s) => s.phase == TerminalPhase.connecting);
    await h.settle();
    await h.controller.stopPending();

    // The stopped request still hangs (a suspended app, a slow network).
    h.api.attachGate = null;
    await h.controller.attach();
    await h.connected();
    expect(h.log.where((e) => e == 'attach'), hasLength(2));

    // Its late answer changes nothing of the tab that attached meanwhile.
    gate.complete();
    await stopped;
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.connected);
    expect(h.host.openStreams, 1);
  });

  test('an opening stopped before the server made the session is idle again, '
      'and the session the server opens meanwhile is closed', () async {
    final h = await Harness.create();
    final gate = h.api.openGate = Completer<void>();
    final opening = h.open();
    await h.until((s) => s.phase == TerminalPhase.connecting);
    await h.settle();
    expect(h.log.last, 'open');

    await h.controller.stopPending();
    expect(h.controller.state.phase, TerminalPhase.idle);
    expect(h.controller.state.sessionId, isNull);

    gate.complete();
    await opening;
    await h.settle();
    expect(h.api.closes, [sessionIdText]);
    expect(h.controller.state.phase, TerminalPhase.idle);
    expect(h.host.openStreams, 0);
  });

  test('an opening stopped while its stream connects leaves the session '
      'detached, so "Reattach" reaches it', () async {
    final h = await Harness.create();
    // The host never answers message 1: the tab stays connecting with its session.
    h.host.handshake = FakeHostHandshake.silent;
    await h.open();
    await h.until(
      (s) => s.sessionId != null && s.phase == TerminalPhase.connecting,
    );
    await h.settle();

    await h.controller.stopPending();
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.controller.state.detachCause, DetachCause.person);
    expect(h.controller.state.sessionId, sessionIdText);
    expect(h.host.openStreams, 0);
    expect(h.api.closes, isEmpty, reason: 'the session is not ended');
  });

  test('nothing changes in a phase without a flow on its way', () async {
    final h = await Harness.create();
    await h.controller.stopPending();
    expect(h.controller.state.phase, TerminalPhase.idle);

    await h.open();
    await h.connected();
    await h.controller.stopPending();
    expect(h.controller.state.phase, TerminalPhase.connected);
    expect(h.host.openStreams, 1);

    await h.controller.detach();
    await h.until((s) => s.phase == TerminalPhase.detached);
    final before = h.controller.state;
    await h.controller.stopPending();
    expect(h.controller.state, same(before));
  });
}
