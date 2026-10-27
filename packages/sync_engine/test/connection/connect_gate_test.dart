// SPDX-License-Identifier: Apache-2.0

// The version gate before every connection attempt (docs/spec/18-release.md 9 item 2; brief
// M7-13 Design note 1): the app asks it before it opens a socket, at the start and on every
// reconnect; a refusal ends in `updateRequired` without a socket and without a reconnect.

import 'dart:async';

import 'package:domain/domain.dart' show AppConfigValues;
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

/// A gate whose answers the test sets; counts its calls.
final class ScriptedGate {
  bool admit = true;
  Completer<bool>? pending;
  Object? error;
  int calls = 0;

  Future<bool> call() async {
    calls++;
    final e = error;
    if (e != null) throw e;
    final p = pending;
    if (p != null) return p.future;
    return admit;
  }
}

void main() {
  late ScriptedGate gate;
  late Harness h;
  setUp(() {
    gate = ScriptedGate();
    h = Harness(connectGate: gate.call);
  });
  tearDown(() => h.dispose());

  test('an admitted attempt opens its socket', () async {
    final ch = await h.open();
    expect(gate.calls, 1);
    expect(h.transport.channels, [ch]);
    expect(h.phase, ConnectionPhase.connecting);
  });

  test('a refusal at the start opens no socket and never reconnects', () async {
    gate.admit = false;
    await h.signIn();
    h.connection.start();
    await settle();
    expect(h.transport.channels, isEmpty);
    expect(h.phase, ConnectionPhase.updateRequired);
    expect(h.connection.state.reason, ConnectionReason.belowMinimumVersion);
    await h.clock.elapse(const Duration(minutes: 10));
    h.network.set(false);
    h.network.set(true);
    await h.connection.connectNow();
    await settle();
    expect(h.transport.channels, isEmpty);
    expect(gate.calls, 1);
    expect(h.phase, ConnectionPhase.updateRequired);
  });

  test('a refusal on a reconnect ends the backoff in updateRequired', () async {
    final ch = await h.goLive();
    expect(gate.calls, 1);
    gate.admit = false;
    ch.serverClose(1001, null);
    await settle();
    expect(h.phase, ConnectionPhase.backoff);
    await h.clock.elapse(const Duration(seconds: 10));
    expect(gate.calls, 2);
    expect(h.transport.channels, [ch]);
    expect(h.phase, ConnectionPhase.updateRequired);
    expect(h.connection.state.reason, ConnectionReason.belowMinimumVersion);
    await h.clock.elapse(const Duration(minutes: 10));
    expect(h.transport.channels, [ch]);
  });

  test('a failing gate admits: the server stays the judge with 4007', () async {
    gate.error = StateError('no answer');
    final ch = await h.open();
    expect(h.transport.channels, [ch]);
    expect(h.log, contains('gateway: connect gate failed: StateError'));
  });

  test('a gate that never answers ends the attempt at its deadline', () async {
    gate.pending = Completer<bool>();
    await h.signIn();
    h.connection.start();
    await settle();
    expect(h.phase, ConnectionPhase.connecting);
    await h.clock.elapse(h.config.syncConnectTimeout);
    await settle();
    expect(h.phase, ConnectionPhase.backoff);
    // The late answer of the abandoned attempt changes nothing.
    gate.pending!.complete(false);
    await settle();
    expect(h.phase, isNot(ConnectionPhase.updateRequired));
  });

  test('a stop while the gate runs opens no socket', () async {
    gate.pending = Completer<bool>();
    await h.signIn();
    h.connection.start();
    await settle();
    await h.connection.stop();
    gate.pending!.complete(true);
    await settle();
    expect(h.transport.channels, isEmpty);
    expect(h.phase, ConnectionPhase.offline);
  });

  test('offline asks no gate', () async {
    h.network.set(false);
    await h.signIn();
    h.connection.start();
    await settle();
    expect(gate.calls, 0);
    expect(h.connection.state.reason, ConnectionReason.networkUnavailable);
  });
}
