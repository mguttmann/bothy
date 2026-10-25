// SPDX-License-Identifier: Apache-2.0

// A catch-up without events (04 7.2): when `ready` (with a cursor) or `resumed` names a head_seq
// equal to the cursor, nothing is missing, and the server goes live without a
// `catchup.complete`. The client goes live at once instead of buffering every later live event
// until a frame that never comes (found by the desktop E2E suite of M4-34 against the real server:
// after a short network loss the app stayed "catching up").

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  Future<void> reconnect(int headSeq) async {
    w.ch.serverClose(4000);
    await w.idle();
    await w.h.clock.elapse(const Duration(minutes: 2));
    await w.idle();
    w.ch.push('hello', {
      'heartbeat_interval_ms': 10000,
      'resume_window_ms': 60000,
      'protocol_versions': [1],
      'dpop_nonce': 'nonce-9',
      'server_time': at,
      'sync_epoch': 'epoch-1',
    });
    await w.idle();
    w.ch.push('resumed', {
      'gateway_session_id': 'gs-1',
      'head_seq': headSeq,
      'sync_epoch': 'epoch-1',
    });
    await w.idle();
  }

  test('a resume at the cursor goes live at once and applies the next live '
      'event', () async {
    await w.connect(cursor: 30, headSeq: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    expect(w.h.phase, ConnectionPhase.live);

    await reconnect(30);
    expect(w.h.phase, ConnectionPhase.live);
    await w.dispatch([threadCreated(31, 1)]);
    expect(await w.cursor(), 31);
    // A catchup.complete of a server that sends one anyway changes nothing.
    await w.push('catchup.complete', {'head_seq': 31});
    expect(w.h.phase, ConnectionPhase.live);
    expect(await w.cursor(), 31);
  });

  test('a resume behind the head waits for catchup.complete', () async {
    await w.connect(cursor: 30, headSeq: 30);
    await w.push('catchup.complete', {'head_seq': 30});

    await reconnect(32);
    expect(w.h.phase, ConnectionPhase.catchingUp);
    await w.dispatch([threadCreated(31, 1), threadCreated(32, 2)]);
    await w.push('catchup.complete', {'head_seq': 32});
    expect(w.h.phase, ConnectionPhase.live);
    expect(await w.cursor(), 32);
  });

  test('an identify with a cursor at the head goes live at once', () async {
    await w.connect(cursor: 40, headSeq: 40);
    expect(w.h.phase, ConnectionPhase.live);
    await w.dispatch([threadCreated(41, 1)]);
    expect(await w.cursor(), 41);
  });
}
