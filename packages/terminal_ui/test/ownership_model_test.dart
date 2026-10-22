// SPDX-License-Identifier: Apache-2.0

// The ownership rule of the controller (docs/decisions.d/M4-35.md D-M4-35-17): an opening, an attach
// and its stream loop own the tab only until the person or the server ends what runs (End,
// Detach, a revocation, a closed tab). The model test runs every flow and interrupts it after
// every single microtask with every action, for every connection outcome, and checks the
// invariants after the dust settles:
//
// 1. A final state set by the action (ended, blocked, detached by the person) is never
//    overwritten by the interrupted flow.
// 2. Only a connected or exited tab holds a stream; every other tab holds none.
// 3. Every ephemeral key is destroyed.
// 4. After the action the host receives no frame but `close`.
// 5. A disposed tab notifies nobody.
// 6. After the action no step-up prompt appears.
//
// The red team repros of M4-35 (N2: End or a revocation while the stream connects and the
// connect fails; N3: End or a revocation during message 2) are cases of this matrix and are
// also kept as named regressions below.

import 'dart:async';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:cryptography/cryptography.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import 'support/fakes.dart';

enum _Flow { open, attach }

enum _Action { end, detach, revoke, dispose }

enum _Outcome { normal, connectFails, corruptMessage2, attachNeedsStepUp }

final class _SyncEvents implements HostEvents {
  final StreamController<HostChange> changes = StreamController.broadcast(
    sync: true,
  );

  @override
  Stream<HostChange> watch(String hostId) => changes.stream;
}

final class _Run {
  _Run(this.h, this.events, this.keys);

  final Harness h;
  final _SyncEvents events;
  final List<SimpleKeyPair> keys;
  TerminalTabState? afterAction;
  // FakeHost.deviceMessages when the action fired.
  int framesAtAction = 0;
  int stepUpsAtAction = 0;
  int notifiedAfterDispose = 0;
  bool disposed = false;
}

Future<_Run> _prepare(_Flow flow, _Outcome outcome) async {
  final events = _SyncEvents();
  final keys = <SimpleKeyPair>[];
  final h = await Harness.create(
    events: events,
    newEphemeralKey: () async {
      final pair = await generateEphemeralKeyPair();
      keys.add(pair);
      return pair;
    },
  );
  if (flow == _Flow.attach) {
    await h.open();
    await h.connected();
    await h.controller.detach();
    await h.settle();
  }
  switch (outcome) {
    case _Outcome.normal:
      break;
    case _Outcome.connectFails:
      h.transport.connectError = StateError('upgrade refused');
    case _Outcome.corruptMessage2:
      h.host.handshake = FakeHostHandshake.corruptMessage2;
    case _Outcome.attachNeedsStepUp:
      h.api.attachErrors.add(stepUpRequired);
  }
  return _Run(h, events, keys);
}

void _act(_Run r, _Action action) {
  final c = r.h.controller;
  r.framesAtAction = r.h.host.deviceMessages;
  r.stepUpsAtAction = r.h.stepUps.length;
  switch (action) {
    case _Action.end:
      unawaited(c.end());
    case _Action.detach:
      unawaited(c.detach());
    case _Action.revoke:
      r.events.changes.add(
        const TerminalSessionUpdated(
          sessionIdText,
          closed: true,
          closeReason: 'access_revoked',
        ),
      );
    case _Action.dispose:
      c.addListener(() => r.notifiedAfterDispose++);
      c.dispose();
      r.disposed = true;
  }
  if (!r.disposed) r.afterAction = c.state;
}

/// Runs [flow] and fires [action] after microtask [at] of the flow (0: never). Returns the
/// number of microtasks the flow used.
Future<(int, _Run)> _runCase(
  _Flow flow,
  _Outcome outcome,
  _Action action,
  int at,
) async {
  final r = await _prepare(flow, outcome);
  var count = 0;
  final spec = ZoneSpecification(
    scheduleMicrotask: (self, parent, zone, f) {
      parent.scheduleMicrotask(zone, () {
        f();
        count++;
        if (count == at) _act(r, action);
      });
    },
  );
  final done = runZoned(
    () => flow == _Flow.open ? r.h.open() : r.h.controller.attach(),
    zoneSpecification: spec,
  );
  await done;
  await r.h.settle(400);
  return (count, r);
}

String _describe(_Run r) =>
    'after action ${r.afterAction}, final ${r.disposed ? 'disposed' : r.h.controller.state}, '
    'open streams ${r.h.host.openStreams}';

void _checkInvariants(_Run r, String label) {
  final h = r.h;
  final why = '$label: ${_describe(r)}';
  // 3. Every key destroyed.
  expect(r.keys.every((k) => k.hasBeenDestroyed), isTrue, reason: why);
  // 4. No frame but close after the action.
  if (r.afterAction != null || r.disposed) {
    final late = [
      for (var i = 0; i < h.host.frames.length; i++)
        if (h.host.frameSentAt[i] > r.framesAtAction) h.host.frames[i],
    ];
    final finalByAction = r.disposed || _isFinal(r.afterAction!);
    if (finalByAction) {
      expect(late.where((f) => f is! CloseFrame), isEmpty, reason: why);
      // 6. No step-up prompt after the flow lost the tab (red team round 3).
      expect(h.stepUps.length, r.stepUpsAtAction, reason: why);
    }
  }
  if (r.disposed) {
    // 5. and 2.
    expect(r.notifiedAfterDispose, 0, reason: why);
    expect(h.host.openStreams, 0, reason: why);
    return;
  }
  final s = h.controller.state;
  // 1. A final state of the action stays.
  final after = r.afterAction;
  if (after != null && _isFinal(after)) {
    expect(s.phase, after.phase, reason: why);
    expect(s.blockCause, after.blockCause, reason: why);
    expect(s.detachCause, after.detachCause, reason: why);
  }
  // 2. Streams only on a live tab.
  final live =
      s.phase == TerminalPhase.connected || s.phase == TerminalPhase.exited;
  expect(h.host.openStreams, live ? 1 : 0, reason: why);
  if (!live) expect(s.acceptsInput, isFalse, reason: why);
}

bool _isFinal(TerminalTabState s) =>
    s.phase == TerminalPhase.ended ||
    s.phase == TerminalPhase.blocked ||
    (s.phase == TerminalPhase.detached && s.detachCause == DetachCause.person);

void main() {
  for (final flow in _Flow.values) {
    for (final outcome in _Outcome.values) {
      if (flow == _Flow.open && outcome == _Outcome.attachNeedsStepUp) continue;
      test(
        'model: ${flow.name}, ${outcome.name}, every action after every microtask',
        () async {
          final (total, base) = await _runCase(flow, outcome, _Action.end, 0);
          _checkInvariants(base, 'no action');
          expect(total, greaterThan(0));
          for (final action in _Action.values) {
            // Every microtask of the flow, plus a few after it ended.
            for (var at = 1; at <= total + 3; at++) {
              final (_, r) = await _runCase(flow, outcome, action, at);
              _checkInvariants(r, '${action.name} at $at of $total');
            }
          }
        },
        timeout: const Timeout(Duration(minutes: 10)),
      );
    }
  }

  // Named regressions of the red team (delta round, findings N2 and N3): the action at every
  // microtask of the window, with the outcome that exposed it.
  Future<void> window(_Flow flow, _Outcome outcome, _Action action) async {
    for (var at = 1; ; at++) {
      final (total, r) = await _runCase(flow, outcome, action, at);
      _checkInvariants(r, '${action.name} at $at of $total');
      if (at > total) break;
    }
  }

  test(
    'N2: End while the stream connects and the connect fails stays ended',
    () async {
      await window(_Flow.attach, _Outcome.connectFails, _Action.end);
      await window(_Flow.open, _Outcome.connectFails, _Action.end);
    },
    timeout: const Timeout(Duration(minutes: 10)),
  );

  test('N2: a revocation while the connect fails stays blocked', () async {
    await window(_Flow.attach, _Outcome.connectFails, _Action.revoke);
  });

  test(
    'N3: End or a revocation during message 2 stays final and holds no stream',
    () async {
      await window(_Flow.open, _Outcome.normal, _Action.end);
      await window(_Flow.open, _Outcome.normal, _Action.revoke);
    },
    timeout: const Timeout(Duration(minutes: 10)),
  );
}
