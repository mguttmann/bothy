// SPDX-License-Identifier: Apache-2.0

// The ports of `terminal_ui` in the app (15 23.1; brief M4-36 Scope): the pins live in
// `host_key_pins` of the local database, the host events of the engine become the changes the
// controller reacts to, and the stream URL follows the instance origin.

import 'dart:async';

import 'package:client/terminal/host_events_adapter.dart';
import 'package:client/terminal/pin_store_adapter.dart';
import 'package:client/terminal/terminal_transport_io.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart' as sync;
import 'package:terminal_ui/terminal_ui.dart';

const String hostA = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0a';
const String hostB = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0b';

void main() {
  test('pins are written to and read from host_key_pins', () async {
    final db = openInMemoryForTests();
    addTearDown(db.close);
    final pins = LocalHostKeyPinStore(db.hostKeyPinsDao);
    expect(await pins.read(hostA), isNull);
    final first = DateTime.utc(2026, 10, 20, 10);
    await pins.write(
      HostKeyPin(
        hostId: hostA,
        noisePublicKey: 'key-a',
        keyGeneration: 2,
        fingerprint: 'FP-A',
        firstSeen: first,
      ),
    );
    final read = (await pins.read(hostA))!;
    expect(
      (read.noisePublicKey, read.keyGeneration, read.fingerprint),
      ('key-a', 2, 'FP-A'),
    );
    expect(read.firstSeen, first);
    expect((await db.hostKeyPinsDao.all()).single.hostId.value, hostA);
  });

  test('engine events become the changes of the watched host only', () async {
    final source = StreamController<sync.HostEvent>.broadcast(sync: true);
    addTearDown(source.close);
    final changes = <HostChange>[];
    final sub = EngineHostEvents(source.stream)
        .watch(hostA)
        .listen(changes.add);
    addTearDown(sub.cancel);
    source
      ..add(
        const sync.HostChanged(
          hostId: hostA,
          version: 2,
          state: 'active',
          connected: true,
        ),
      )
      ..add(const sync.HostPresenceChanged(hostId: hostB, connected: false))
      ..add(
        const sync.TerminalSessionChanged(
          sessionId: 's1',
          hostId: hostA,
          userId: 'u',
          deviceId: 'd',
          state: 'closed',
          version: 3,
          closeReason: 'access_revoked',
        ),
      )
      ..add(const sync.HostRemoved(hostId: hostA));
    await Future<void>.delayed(Duration.zero);
    expect(changes, hasLength(3));
    expect((changes[0] as HostUpdated).hostId, hostA);
    final closed = changes[1] as TerminalSessionUpdated;
    expect(
      (closed.sessionId, closed.closed, closed.closeReason),
      ('s1', true, 'access_revoked'),
    );
    expect(changes[2], isA<HostUpdated>());
  });

  test('the stream URL is wss of the origin; ws only for the test fake', () {
    const path = '/api/v1/terminal/sessions/$hostA/stream';
    expect(
      IoTerminalTransport.streamUrlOf(
        Uri.parse('https://chat.example.com'),
        path,
      ).toString(),
      'wss://chat.example.com$path',
    );
    expect(
      () => IoTerminalTransport.streamUrlOf(
        Uri.parse('http://127.0.0.1:8'),
        path,
      ),
      throwsArgumentError,
    );
    expect(
      IoTerminalTransport.streamUrlOf(
        Uri.parse('http://127.0.0.1:8'),
        path,
        allowPlainHttp: true,
      ).scheme,
      'ws',
    );
  });
}
